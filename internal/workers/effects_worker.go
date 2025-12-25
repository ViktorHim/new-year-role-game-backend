// internal/workers/effects_worker.go
package workers

import (
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"
)

type EffectsWorker struct {
	db       *sql.DB
	interval time.Duration
	stopChan chan bool
	running  bool
	mu       sync.Mutex
}

func NewEffectsWorker(db *sql.DB, intervalSeconds int) *EffectsWorker {
	return &EffectsWorker{
		db:       db,
		interval: time.Duration(intervalSeconds) * time.Second,
		stopChan: make(chan bool),
		running:  false,
	}
}

// Start запускает worker в фоновом режиме
func (w *EffectsWorker) Start() {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		log.Println("Effects worker is already running")
		return
	}
	w.running = true
	w.mu.Unlock()

	log.Printf("Effects worker started, checking every %v", w.interval)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	// Основной цикл выполнения по таймеру
	for {
		select {
		case <-ticker.C:
			// Проверяем, активна ли игра перед выполнением
			if w.isGameActive() {
				w.executeAllEffects()
			}
		case <-w.stopChan:
			w.mu.Lock()
			w.running = false
			w.mu.Unlock()
			log.Println("Effects worker stopped")
			return
		}
	}
}

// Stop останавливает worker
func (w *EffectsWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.stopChan <- true
}

// IsRunning возвращает статус работы worker'а
func (w *EffectsWorker) IsRunning() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.running
}

// isGameActive проверяет, активна ли игра в данный момент
func (w *EffectsWorker) isGameActive() bool {
	var gameStarted, gameEnded *time.Time

	err := w.db.QueryRow(`
		SELECT game_started_at, game_ended_at
		FROM game_timeline
		ORDER BY id DESC
		LIMIT 1
	`).Scan(&gameStarted, &gameEnded)

	if err != nil {
		if err == sql.ErrNoRows {
			// Игра еще не начиналась
			return false
		}
		log.Printf("Error checking game status: %v", err)
		return false
	}

	// Игра активна если она начата и не завершена
	return gameStarted != nil && gameEnded == nil
}

// executeAllEffects выполняет все доступные эффекты для всех игроков
func (w *EffectsWorker) executeAllEffects() {
	now := time.Now()
	log.Println("Starting executing effects")
	// Получаем все эффекты, которые можно выполнить
	rows, err := w.db.Query(`
		SELECT 
			pi.player_id,
			i.id AS item_id,
			i.name AS item_name,
			e.id AS effect_id,
			e.effect_type,
			e.generated_resource,
			e.operation,
			e.value,
			e.spawned_item_id,
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
		WHERE 
			iee.last_executed_at IS NULL OR 
			iee.last_executed_at + (e.period_seconds || ' seconds')::INTERVAL <= $1
		ORDER BY pi.player_id, i.id, e.id
	`, now)

	if err != nil {
		log.Printf("Error fetching effects to execute: %v", err)
		return
	}
	defer rows.Close()

	var totalExecuted int
	var totalMoneyGenerated int
	var totalInfluenceGenerated int
	var totalItemsSpawned int

	for rows.Next() {
		var playerID, itemID, effectID, periodSeconds int
		var itemName, effectType string
		var generatedResource, operation *string
		var value, spawnedItemID *int
		var lastExecutedAt *time.Time

		err := rows.Scan(
			&playerID,
			&itemID,
			&itemName,
			&effectID,
			&effectType,
			&generatedResource,
			&operation,
			&value,
			&spawnedItemID,
			&periodSeconds,
			&lastExecutedAt,
		)

		if err != nil {
			log.Printf("Error scanning effect: %v", err)
			continue
		}

		// Выполняем эффект в отдельной транзакции
		err = w.executeEffect(playerID, itemID, itemName, effectID, effectType,
			generatedResource, operation, value, spawnedItemID, now)

		if err != nil {
			log.Printf("Error executing effect %d for player %d: %v", effectID, playerID, err)
			continue
		}

		totalExecuted++

		// Подсчет статистики
		switch effectType {
		case "generate_money":
			if value != nil {
				totalMoneyGenerated += w.calculateEffectValue(*value, *operation)
			}
		case "generate_influence":
			if value != nil {
				totalInfluenceGenerated += w.calculateEffectValue(*value, *operation)
			}
		case "spawn_item":
			totalItemsSpawned++
		}
	}

	if err = rows.Err(); err != nil {
		log.Printf("Error iterating effects: %v", err)
		return
	}

	if totalExecuted > 0 {
		log.Printf("Effects executed: %d | Money: %d | Influence: %d | Items spawned: %d",
			totalExecuted, totalMoneyGenerated, totalInfluenceGenerated, totalItemsSpawned)
	} else {
		log.Printf("No effects was executed")
	}
}

// executeEffect выполняет один эффект для одного игрока
func (w *EffectsWorker) executeEffect(playerID, itemID int, itemName string, effectID int,
	effectType string, generatedResource, operation *string, value, spawnedItemID *int,
	executedAt time.Time) error {

	tx, err := w.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	switch effectType {
	case "generate_money":
		if value != nil && operation != nil {
			amount := w.calculateEffectValue(*value, *operation)

			_, err = tx.Exec(`
				UPDATE players
				SET money = money + $1
				WHERE id = $2
			`, amount, playerID)

			if err != nil {
				return fmt.Errorf("failed to generate money: %w", err)
			}

			_, err = tx.Exec(`
				INSERT INTO money_transactions (to_player_id, amount, transaction_type, reference_id, reference_type, description)
				VALUES ($1, $2, 'item_effect', $3, 'effect', $4)
			`, playerID, amount, effectID, fmt.Sprintf("Item effect: %s generated %d money", itemName, amount))

			if err != nil {
				return fmt.Errorf("failed to record money transaction: %w", err)
			}
		}

	case "generate_influence":
		if value != nil && operation != nil {
			amount := w.calculateEffectValue(*value, *operation)

			_, err = tx.Exec(`
				UPDATE players
				SET influence = influence + $1
				WHERE id = $2
			`, amount, playerID)

			if err != nil {
				return fmt.Errorf("failed to generate influence: %w", err)
			}

			_, err = tx.Exec(`
				INSERT INTO influence_transactions (player_id, amount, transaction_type, reference_id, reference_type, description)
				VALUES ($1, $2, 'item_effect', $3, 'effect', $4)
			`, playerID, amount, effectID, fmt.Sprintf("Item effect: %s generated %d influence", itemName, amount))

			if err != nil {
				return fmt.Errorf("failed to record influence transaction: %w", err)
			}
		}

	case "spawn_item":
		if spawnedItemID != nil {
			var spawnedItemName string
			err = tx.QueryRow(`
				SELECT name FROM items WHERE id = $1
			`, *spawnedItemID).Scan(&spawnedItemName)

			if err != nil {
				return fmt.Errorf("failed to fetch spawned item info: %w", err)
			}

			_, err = tx.Exec(`
				INSERT INTO player_items (player_id, item_id)
				VALUES ($1, $2)
				ON CONFLICT (player_id, item_id) DO NOTHING
			`, playerID, *spawnedItemID)

			if err != nil {
				return fmt.Errorf("failed to spawn item: %w", err)
			}

			_, err = tx.Exec(`
				INSERT INTO item_transactions (to_player_id, item_id, transaction_type, reference_id, reference_type, description)
				VALUES ($1, $2, 'spawned', $3, 'effect', $4)
			`, playerID, *spawnedItemID, effectID, fmt.Sprintf("Item effect: %s spawned %s", itemName, spawnedItemName))

			if err != nil {
				return fmt.Errorf("failed to record item transaction: %w", err)
			}
		}
	}

	// Обновляем время выполнения эффекта
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

// calculateEffectValue вычисляет значение эффекта с учетом операции
func (w *EffectsWorker) calculateEffectValue(value int, operation string) int {
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
