// internal/workers/contract_scheduler.go
package workers

import (
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"
)

// ContractScheduler управляет точными таймерами для каждого договора
type ContractScheduler struct {
	db      *sql.DB
	timers  map[int]*time.Timer // contract_id -> timer
	mu      sync.RWMutex
	running bool
}

func NewContractScheduler(db *sql.DB) *ContractScheduler {
	return &ContractScheduler{
		db:      db,
		timers:  make(map[int]*time.Timer),
		running: false,
	}
}

// Start загружает все активные договоры и создаёт для них таймеры
func (s *ContractScheduler) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("scheduler already running")
	}
	s.running = true
	s.mu.Unlock()

	// Загружаем все подписанные договоры
	rows, err := s.db.Query(`
		SELECT id, expires_at
		FROM contracts
		WHERE status = 'signed' AND expires_at > NOW()
		ORDER BY expires_at ASC
	`)
	if err != nil {
		s.running = false
		return fmt.Errorf("failed to load contracts: %w", err)
	}
	defer rows.Close()

	count := 0
	now := time.Now()

	for rows.Next() {
		var contractID int
		var expiresAt time.Time

		if err := rows.Scan(&contractID, &expiresAt); err != nil {
			log.Printf("Error scanning contract: %v", err)
			continue
		}

		// Создаём таймер только для будущих договоров
		if expiresAt.After(now) {
			s.scheduleContract(contractID, expiresAt)
			count++
		}
	}

	log.Printf("Contract scheduler started, loaded %d active contracts", count)
	return nil
}

// ScheduleContract создаёт точный таймер для конкретного договора
func (s *ContractScheduler) ScheduleContract(contractID int, expiresAt time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.scheduleContract(contractID, expiresAt)
}

// scheduleContract - внутренний метод (без блокировки)
func (s *ContractScheduler) scheduleContract(contractID int, expiresAt time.Time) {
	// Отменяем существующий таймер если есть
	if existingTimer, exists := s.timers[contractID]; exists {
		existingTimer.Stop()
		delete(s.timers, contractID)
	}

	now := time.Now()
	duration := expiresAt.Sub(now)

	// Если договор уже истёк, выполняем сразу
	if duration <= 0 {
		log.Printf("Contract #%d already expired, completing immediately", contractID)
		go s.completeContract(contractID)
		return
	}

	// Создаём точный таймер
	timer := time.AfterFunc(duration, func() {
		s.completeContract(contractID)

		// Удаляем таймер из карты после выполнения
		s.mu.Lock()
		delete(s.timers, contractID)
		s.mu.Unlock()
	})

	s.timers[contractID] = timer

	log.Printf("Scheduled contract #%d to complete at %v (in %v)",
		contractID, expiresAt.Format("2006-01-02 15:04:05"), duration.Round(time.Second))
}

// CancelContract отменяет таймер для договора (при ручном завершении или расторжении)
func (s *ContractScheduler) CancelContract(contractID int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if timer, exists := s.timers[contractID]; exists {
		timer.Stop()
		delete(s.timers, contractID)
		log.Printf("Cancelled scheduled completion for contract #%d", contractID)
	}
}

// completeContract автоматически завершает договор в точное время
func (s *ContractScheduler) completeContract(contractID int) {
	log.Printf("Auto-completing contract #%d at exact time", contractID)

	// Начинаем транзакцию
	tx, err := s.db.Begin()
	if err != nil {
		log.Printf("Error starting transaction for contract #%d: %v", contractID, err)
		return
	}
	defer tx.Rollback()

	// Получаем информацию о договоре
	var contract struct {
		Status              string
		ContractType        string
		CustomerPlayerID    int
		ExecutorPlayerID    int
		CustomerFactionID   *int
		MoneyRewardCustomer int
		MoneyRewardExecutor int
	}

	err = tx.QueryRow(`
		SELECT status, contract_type, customer_player_id, executor_player_id, 
		       customer_faction_id, money_reward_customer, money_reward_executor
		FROM contracts
		WHERE id = $1
		FOR UPDATE
	`, contractID).Scan(
		&contract.Status,
		&contract.ContractType,
		&contract.CustomerPlayerID,
		&contract.ExecutorPlayerID,
		&contract.CustomerFactionID,
		&contract.MoneyRewardCustomer,
		&contract.MoneyRewardExecutor,
	)

	if err != nil {
		log.Printf("Error fetching contract #%d: %v", contractID, err)
		return
	}

	// Проверяем, что договор ещё в статусе signed
	if contract.Status != "signed" {
		log.Printf("Contract #%d is no longer signed (status: %s), skipping", contractID, contract.Status)
		return
	}

	now := time.Now()

	// Выдаём награды в зависимости от типа
	if err := s.distributeRewards(tx, contractID, &contract); err != nil {
		log.Printf("Error distributing rewards for contract #%d: %v", contractID, err)
		return
	}

	// Обновляем статус договора
	_, err = tx.Exec(`
		UPDATE contracts
		SET status = 'completed', completed_at = $1
		WHERE id = $2
	`, now, contractID)

	if err != nil {
		log.Printf("Error updating contract #%d status: %v", contractID, err)
		return
	}

	// Фиксируем транзакцию
	if err = tx.Commit(); err != nil {
		log.Printf("Error committing transaction for contract #%d: %v", contractID, err)
		return
	}

	log.Printf("Successfully auto-completed contract #%d", contractID)
}

// distributeRewards выдаёт награды согласно типу договора
func (s *ContractScheduler) distributeRewards(tx *sql.Tx, contractID int, contract *struct {
	Status              string
	ContractType        string
	CustomerPlayerID    int
	ExecutorPlayerID    int
	CustomerFactionID   *int
	MoneyRewardCustomer int
	MoneyRewardExecutor int
}) error {

	if contract.ContractType == "type1" {
		// Type 1: заказчик получает деньги + предмет, исполнитель получает деньги

		// Даём деньги заказчику
		if contract.MoneyRewardCustomer > 0 {
			_, err := tx.Exec(`
				UPDATE players SET money = money + $1 WHERE id = $2
			`, contract.MoneyRewardCustomer, contract.CustomerPlayerID)
			if err != nil {
				return fmt.Errorf("failed to give money to customer: %w", err)
			}

			_, err = tx.Exec(`
				INSERT INTO money_transactions (to_player_id, amount, transaction_type, reference_id, reference_type, description)
				VALUES ($1, $2, 'contract', $3, 'contract', $4)
			`, contract.CustomerPlayerID, contract.MoneyRewardCustomer, contractID,
				fmt.Sprintf("Auto-completed contract %d reward", contractID))
			if err != nil {
				return fmt.Errorf("failed to record customer money transaction: %w", err)
			}
		}

		// Даём деньги исполнителю
		if contract.MoneyRewardExecutor > 0 {
			_, err := tx.Exec(`
				UPDATE players SET money = money + $1 WHERE id = $2
			`, contract.MoneyRewardExecutor, contract.ExecutorPlayerID)
			if err != nil {
				return fmt.Errorf("failed to give money to executor: %w", err)
			}

			_, err = tx.Exec(`
				INSERT INTO money_transactions (to_player_id, amount, transaction_type, reference_id, reference_type, description)
				VALUES ($1, $2, 'contract', $3, 'contract', $4)
			`, contract.ExecutorPlayerID, contract.MoneyRewardExecutor, contractID,
				fmt.Sprintf("Auto-completed contract %d reward", contractID))
			if err != nil {
				return fmt.Errorf("failed to record executor money transaction: %w", err)
			}
		}

		// Даём предмет заказчику (если у него есть фракция)
		if contract.CustomerFactionID != nil {
			var itemID *int
			err := tx.QueryRow(`
				SELECT customer_item_reward_id
				FROM contract_type1_settings
				WHERE faction_id = $1
			`, *contract.CustomerFactionID).Scan(&itemID)

			if err != nil && err != sql.ErrNoRows {
				return fmt.Errorf("failed to fetch item reward settings: %w", err)
			}

			if itemID != nil && *itemID > 0 {
				_, err = tx.Exec(`
					INSERT INTO player_items (player_id, item_id)
					VALUES ($1, $2)
					ON CONFLICT (player_id, item_id) DO NOTHING
				`, contract.CustomerPlayerID, *itemID)
				if err != nil {
					return fmt.Errorf("failed to give item to customer: %w", err)
				}

				// Инициализируем таймеры эффектов
				_, err = tx.Exec(`
					INSERT INTO item_effect_executions (player_id, item_id, effect_id, last_executed_at)
					SELECT $1, ie.item_id, ie.effect_id, NOW()
					FROM item_effects ie
					WHERE ie.item_id = $2
					ON CONFLICT (player_id, item_id, effect_id) 
					DO UPDATE SET last_executed_at = NOW()
				`, contract.CustomerPlayerID, *itemID)
				if err != nil {
					return fmt.Errorf("failed to initialize item effect timers: %w", err)
				}

				_, err = tx.Exec(`
					INSERT INTO item_transactions (to_player_id, item_id, transaction_type, reference_id, reference_type, description)
					VALUES ($1, $2, 'contract', $3, 'contract', $4)
				`, contract.CustomerPlayerID, *itemID, contractID,
					fmt.Sprintf("Auto-completed contract %d reward", contractID))
				if err != nil {
					return fmt.Errorf("failed to record item transaction: %w", err)
				}
			}
		}

	} else if contract.ContractType == "type2" {
		// Type 2: исполнитель получает деньги

		if contract.MoneyRewardExecutor > 0 {
			_, err := tx.Exec(`
				UPDATE players SET money = money + $1 WHERE id = $2
			`, contract.MoneyRewardExecutor, contract.ExecutorPlayerID)
			if err != nil {
				return fmt.Errorf("failed to give money to executor: %w", err)
			}

			_, err = tx.Exec(`
				INSERT INTO money_transactions (to_player_id, amount, transaction_type, reference_id, reference_type, description)
				VALUES ($1, $2, 'contract', $3, 'contract', $4)
			`, contract.ExecutorPlayerID, contract.MoneyRewardExecutor, contractID,
				fmt.Sprintf("Auto-completed contract %d reward", contractID))
			if err != nil {
				return fmt.Errorf("failed to record executor money transaction: %w", err)
			}
		}
	}

	return nil
}

// GetScheduledCount возвращает количество запланированных договоров
func (s *ContractScheduler) GetScheduledCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.timers)
}

// Stop останавливает все таймеры
func (s *ContractScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, timer := range s.timers {
		timer.Stop()
		delete(s.timers, id)
	}
	s.running = false

	log.Println("Contract scheduler stopped")
}
