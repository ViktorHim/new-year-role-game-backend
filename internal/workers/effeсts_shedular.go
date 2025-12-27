// internal/workers/effects_scheduler.go
package workers

import (
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"
)

// EffectTimerKey - уникальный ключ для таймера эффекта
type EffectTimerKey struct {
	PlayerID int
	ItemID   int
	EffectID int
}

// EffectsScheduler управляет точными таймерами для каждого эффекта
type EffectsScheduler struct {
	db      *sql.DB
	timers  map[EffectTimerKey]*time.Timer
	mu      sync.RWMutex
	running bool
}

func NewEffectsScheduler(db *sql.DB) *EffectsScheduler {
	return &EffectsScheduler{
		db:      db,
		timers:  make(map[EffectTimerKey]*time.Timer),
		running: false,
	}
}

// Start загружает все активные эффекты и создаёт для них таймеры
func (s *EffectsScheduler) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("scheduler already running")
	}
	s.running = true
	s.mu.Unlock()

	// Загружаем все эффекты предметов игроков
	rows, err := s.db.Query(`
		SELECT 
			pi.player_id,
			i.id AS item_id,
			i.name AS item_name,
			e.id AS effect_id,
			e.description,
			e.period_seconds,
			iee.last_executed_at
		FROM player_items pi
		JOIN items i ON pi.item_id = i.id
		JOIN item_effects ie ON i.id = ie.item_id
		JOIN effects e ON ie.effect_id = e.id
		LEFT JOIN item_effect_executions iee ON 
			iee.player_id = pi.player_id AND 
			iee.item_id = i.id AND 
			iee.effect_id = e.id
		ORDER BY pi.player_id, i.id, e.id
	`)
	if err != nil {
		s.running = false
		return fmt.Errorf("failed to load effects: %w", err)
	}
	defer rows.Close()

	count := 0
	now := time.Now()

	for rows.Next() {
		var playerID, itemID, effectID, periodSeconds int
		var itemName string
		var description *string
		var lastExecutedAt *time.Time

		if err := rows.Scan(&playerID, &itemID, &itemName, &effectID,
			&description, &periodSeconds, &lastExecutedAt); err != nil {
			log.Printf("Error scanning effect: %v", err)
			continue
		}

		// Вычисляем следующее время выполнения
		var nextExecutionTime time.Time
		if lastExecutedAt == nil {
			// Если эффект никогда не выполнялся, выполним через period_seconds
			nextExecutionTime = now.Add(time.Duration(periodSeconds) * time.Second)
		} else {
			// Следующее выполнение = последнее + период
			nextExecutionTime = lastExecutedAt.Add(time.Duration(periodSeconds) * time.Second)

			// Если время уже прошло, выполним сразу
			if nextExecutionTime.Before(now) {
				nextExecutionTime = now
			}
		}

		// Создаём таймер
		s.scheduleEffect(playerID, itemID, effectID, nextExecutionTime, periodSeconds)
		count++
	}

	log.Printf("Effects scheduler started, loaded %d active effects", count)
	return nil
}

// ScheduleEffect создаёт точный таймер для конкретного эффекта
func (s *EffectsScheduler) ScheduleEffect(playerID, itemID, effectID int, nextExecutionTime time.Time, periodSeconds int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.scheduleEffect(playerID, itemID, effectID, nextExecutionTime, periodSeconds)
}

// scheduleEffect - внутренний метод (без блокировки)
func (s *EffectsScheduler) scheduleEffect(playerID, itemID, effectID int, nextExecutionTime time.Time, periodSeconds int) {
	key := EffectTimerKey{
		PlayerID: playerID,
		ItemID:   itemID,
		EffectID: effectID,
	}

	// Отменяем существующий таймер если есть
	if existingTimer, exists := s.timers[key]; exists {
		existingTimer.Stop()
		delete(s.timers, key)
	}

	now := time.Now()
	duration := nextExecutionTime.Sub(now)

	// Если эффект нужно выполнить сейчас или время прошло
	if duration <= 0 {
		go s.executeEffectAndReschedule(playerID, itemID, effectID, periodSeconds)
		return
	}

	// Создаём точный таймер
	timer := time.AfterFunc(duration, func() {
		s.executeEffectAndReschedule(playerID, itemID, effectID, periodSeconds)
	})

	s.timers[key] = timer

	log.Printf("Scheduled effect (player=%d, item=%d, effect=%d) to execute at %v (in %v)",
		playerID, itemID, effectID, nextExecutionTime.Format("15:04:05"), duration.Round(time.Second))
}

// CancelEffect отменяет таймер для эффекта (при передаче предмета)
func (s *EffectsScheduler) CancelEffect(playerID, itemID, effectID int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := EffectTimerKey{
		PlayerID: playerID,
		ItemID:   itemID,
		EffectID: effectID,
	}

	if timer, exists := s.timers[key]; exists {
		timer.Stop()
		delete(s.timers, key)
		log.Printf("Cancelled effect timer (player=%d, item=%d, effect=%d)", playerID, itemID, effectID)
	}
}

// CancelAllEffectsForItem отменяет все таймеры для предмета игрока
func (s *EffectsScheduler) CancelAllEffectsForItem(playerID, itemID int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	keysToDelete := make([]EffectTimerKey, 0)

	for key, timer := range s.timers {
		if key.PlayerID == playerID && key.ItemID == itemID {
			timer.Stop()
			keysToDelete = append(keysToDelete, key)
		}
	}

	for _, key := range keysToDelete {
		delete(s.timers, key)
	}

	if len(keysToDelete) > 0 {
		log.Printf("Cancelled %d effect timers for item %d of player %d", len(keysToDelete), itemID, playerID)
	}
}

// executeEffectAndReschedule выполняет эффект и создаёт новый таймер
func (s *EffectsScheduler) executeEffectAndReschedule(playerID, itemID, effectID, periodSeconds int) {
	now := time.Now()

	// Выполняем эффект
	if err := s.executeEffect(playerID, itemID, effectID, now); err != nil {
		log.Printf("Error executing effect (player=%d, item=%d, effect=%d): %v",
			playerID, itemID, effectID, err)
		// Не пересоздаём таймер при ошибке
		return
	}

	// Создаём новый таймер для следующего выполнения
	nextExecutionTime := now.Add(time.Duration(periodSeconds) * time.Second)
	s.ScheduleEffect(playerID, itemID, effectID, nextExecutionTime, periodSeconds)
}

// executeEffect выполняет один эффект
func (s *EffectsScheduler) executeEffect(playerID, itemID, effectID int, executedAt time.Time) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Получаем информацию об эффекте
	var effect struct {
		EffectType        string
		GeneratedResource *string
		Operation         *string
		Value             *int
		SpawnedItemID     *int
	}

	err = tx.QueryRow(`
		SELECT effect_type, generated_resource, operation, value, spawned_item_id
		FROM effects
		WHERE id = $1
	`, effectID).Scan(
		&effect.EffectType,
		&effect.GeneratedResource,
		&effect.Operation,
		&effect.Value,
		&effect.SpawnedItemID,
	)

	if err != nil {
		return fmt.Errorf("failed to fetch effect: %w", err)
	}

	// Проверяем, что предмет всё ещё у игрока
	var hasItem bool
	err = tx.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM player_items WHERE player_id = $1 AND item_id = $2)
	`, playerID, itemID).Scan(&hasItem)

	if err != nil {
		return fmt.Errorf("failed to check item ownership: %w", err)
	}

	if !hasItem {
		log.Printf("Player %d no longer has item %d, skipping effect", playerID, itemID)
		return nil
	}

	// Выполняем эффект в зависимости от типа
	switch effect.EffectType {
	case "generate_money":
		if effect.Value != nil && effect.Operation != nil {
			amount := calculateEffectValue(*effect.Value, *effect.Operation)

			_, err = tx.Exec(`
				UPDATE players SET money = money + $1 WHERE id = $2
			`, amount, playerID)
			if err != nil {
				return fmt.Errorf("failed to generate money: %w", err)
			}

			// Получаем название предмета для описания
			var itemName string
			tx.QueryRow(`SELECT name FROM items WHERE id = $1`, itemID).Scan(&itemName)

			_, err = tx.Exec(`
				INSERT INTO money_transactions (to_player_id, amount, transaction_type, reference_id, reference_type, description)
				VALUES ($1, $2, 'item_effect', $3, 'effect', $4)
			`, playerID, amount, effectID, fmt.Sprintf("Item effect: %s generated %d money", itemName, amount))
			if err != nil {
				return fmt.Errorf("failed to record money transaction: %w", err)
			}

			log.Printf("Effect executed: player %d received %d money from item %d", playerID, amount, itemID)
		}

	case "generate_influence":
		if effect.Value != nil && effect.Operation != nil {
			amount := calculateEffectValue(*effect.Value, *effect.Operation)

			_, err = tx.Exec(`
				UPDATE players SET influence = influence + $1 WHERE id = $2
			`, amount, playerID)
			if err != nil {
				return fmt.Errorf("failed to generate influence: %w", err)
			}

			var itemName string
			tx.QueryRow(`SELECT name FROM items WHERE id = $1`, itemID).Scan(&itemName)

			_, err = tx.Exec(`
				INSERT INTO influence_transactions (player_id, amount, transaction_type, reference_id, reference_type, description)
				VALUES ($1, $2, 'item_effect', $3, 'effect', $4)
			`, playerID, amount, effectID, fmt.Sprintf("Item effect: %s generated %d influence", itemName, amount))
			if err != nil {
				return fmt.Errorf("failed to record influence transaction: %w", err)
			}

			log.Printf("Effect executed: player %d received %d influence from item %d", playerID, amount, itemID)
		}

	case "spawn_item":
		if effect.SpawnedItemID != nil {
			var spawnedItemName string
			err = tx.QueryRow(`SELECT name FROM items WHERE id = $1`, *effect.SpawnedItemID).Scan(&spawnedItemName)
			if err != nil {
				return fmt.Errorf("failed to fetch spawned item info: %w", err)
			}

			_, err = tx.Exec(`
				INSERT INTO player_items (player_id, item_id)
				VALUES ($1, $2)
				ON CONFLICT (player_id, item_id) DO NOTHING
			`, playerID, *effect.SpawnedItemID)
			if err != nil {
				return fmt.Errorf("failed to spawn item: %w", err)
			}

			// Инициализируем таймеры эффектов для нового предмета
			s.initializeItemEffects(tx, playerID, *effect.SpawnedItemID, executedAt)

			var itemName string
			tx.QueryRow(`SELECT name FROM items WHERE id = $1`, itemID).Scan(&itemName)

			_, err = tx.Exec(`
				INSERT INTO item_transactions (to_player_id, item_id, transaction_type, reference_id, reference_type, description)
				VALUES ($1, $2, 'spawned', $3, 'effect', $4)
			`, playerID, *effect.SpawnedItemID, effectID,
				fmt.Sprintf("Item effect: %s spawned %s", itemName, spawnedItemName))
			if err != nil {
				return fmt.Errorf("failed to record item transaction: %w", err)
			}

			log.Printf("Effect executed: player %d received item %d (%s) from item %d",
				playerID, *effect.SpawnedItemID, spawnedItemName, itemID)
		}
	}

	// Обновляем время последнего выполнения
	_, err = tx.Exec(`
		INSERT INTO item_effect_executions (player_id, item_id, effect_id, last_executed_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (player_id, item_id, effect_id) 
		DO UPDATE SET last_executed_at = $4
	`, playerID, itemID, effectID, executedAt)
	if err != nil {
		return fmt.Errorf("failed to update effect execution time: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// initializeItemEffects инициализирует таймеры для эффектов нового предмета
func (s *EffectsScheduler) initializeItemEffects(tx *sql.Tx, playerID, itemID int, baseTime time.Time) error {
	// Получаем все эффекты предмета
	rows, err := tx.Query(`
		SELECT e.id, e.period_seconds
		FROM item_effects ie
		JOIN effects e ON ie.effect_id = e.id
		WHERE ie.item_id = $1
	`, itemID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var effectID, periodSeconds int
		if err := rows.Scan(&effectID, &periodSeconds); err != nil {
			return err
		}

		// Устанавливаем last_executed_at в БД
		_, err = tx.Exec(`
			INSERT INTO item_effect_executions (player_id, item_id, effect_id, last_executed_at)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (player_id, item_id, effect_id) 
			DO UPDATE SET last_executed_at = $4
		`, playerID, itemID, effectID, baseTime)
		if err != nil {
			return err
		}

		// Создаём таймер (после коммита транзакции это будет вызвано)
		nextExecutionTime := baseTime.Add(time.Duration(periodSeconds) * time.Second)

		// Планируем выполнение через горутину, чтобы не блокировать транзакцию
		go func(pid, iid, eid int, next time.Time, period int) {
			time.Sleep(100 * time.Millisecond) // Даём время транзакции завершиться
			s.ScheduleEffect(pid, iid, eid, next, period)
		}(playerID, itemID, effectID, nextExecutionTime, periodSeconds)
	}

	return nil
}

// GetScheduledCount возвращает количество запланированных эффектов
func (s *EffectsScheduler) GetScheduledCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.timers)
}

// Stop останавливает все таймеры
func (s *EffectsScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for key, timer := range s.timers {
		timer.Stop()
		delete(s.timers, key)
	}
	s.running = false

	log.Println("Effects scheduler stopped")
}

// calculateEffectValue вычисляет значение эффекта с учетом операции
func calculateEffectValue(value int, operation string) int {
	switch operation {
	case "add":
		return value
	case "mul":
		return value
	case "sub":
		return -value
	case "div":
		return value
	default:
		return value
	}
}
