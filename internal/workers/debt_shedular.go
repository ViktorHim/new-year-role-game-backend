// internal/workers/debt_scheduler.go
package workers

import (
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"
)

// DebtScheduler управляет точными таймерами для истечения долговых расписок
type DebtScheduler struct {
	db      *sql.DB
	timers  map[int]*time.Timer // map[debtReceiptID]*Timer
	mu      sync.RWMutex
	running bool
}

func NewDebtScheduler(db *sql.DB) *DebtScheduler {
	return &DebtScheduler{
		db:      db,
		timers:  make(map[int]*time.Timer),
		running: false,
	}
}

// Start загружает все активные долговые расписки и создаёт таймеры
func (s *DebtScheduler) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("debt scheduler already running")
	}
	s.running = true
	s.mu.Unlock()

	// Загружаем все активные долговые расписки (не возвращенные и не просроченные с примененным штрафом)
	rows, err := s.db.Query(`
		SELECT id, return_deadline
		FROM debt_receipts
		WHERE is_returned = false 
		  AND penalty_applied = false
		  AND return_deadline > NOW()
		ORDER BY return_deadline
	`)
	if err != nil {
		s.running = false
		return fmt.Errorf("failed to load debt receipts: %w", err)
	}
	defer rows.Close()

	count := 0
	now := time.Now()

	for rows.Next() {
		var debtID int
		var deadline time.Time

		if err := rows.Scan(&debtID, &deadline); err != nil {
			log.Printf("Error scanning debt receipt: %v", err)
			continue
		}

		// Создаём таймер
		s.scheduleDebtExpiration(debtID, deadline)
		count++

		log.Printf("Scheduled debt #%d to expire at %v (in %v)",
			debtID, deadline.Format("2006-01-02 15:04:05"), deadline.Sub(now).Round(time.Second))
	}

	log.Printf("Debt scheduler started, loaded %d active debt receipts", count)
	return nil
}

// ScheduleDebt создаёт точный таймер для долговой расписки
func (s *DebtScheduler) ScheduleDebt(debtID int, deadline time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.scheduleDebtExpiration(debtID, deadline)
}

// scheduleDebtExpiration - внутренний метод (без блокировки)
func (s *DebtScheduler) scheduleDebtExpiration(debtID int, deadline time.Time) {
	// Отменяем существующий таймер если есть
	if existingTimer, exists := s.timers[debtID]; exists {
		existingTimer.Stop()
		delete(s.timers, debtID)
	}

	now := time.Now()
	duration := deadline.Sub(now)

	// Если срок уже истёк
	if duration <= 0 {
		log.Printf("Debt #%d already expired, applying penalty immediately", debtID)
		go s.applyPenalty(debtID)
		return
	}

	// Создаём точный таймер
	timer := time.AfterFunc(duration, func() {
		log.Printf("Debt #%d expired at exact time, applying penalty", debtID)
		s.applyPenalty(debtID)
	})

	s.timers[debtID] = timer
}

// CancelDebt отменяет таймер для долговой расписки (при возврате долга)
func (s *DebtScheduler) CancelDebt(debtID int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if timer, exists := s.timers[debtID]; exists {
		timer.Stop()
		delete(s.timers, debtID)
		log.Printf("Cancelled debt timer #%d (debt returned)", debtID)
	}
}

// applyPenalty применяет штраф за просроченный долг
func (s *DebtScheduler) applyPenalty(debtID int) {
	// Удаляем таймер из map
	s.mu.Lock()
	delete(s.timers, debtID)
	s.mu.Unlock()

	// Начинаем транзакцию
	tx, err := s.db.Begin()
	if err != nil {
		log.Printf("Error starting transaction for debt #%d: %v", debtID, err)
		return
	}
	defer tx.Rollback()

	// Получаем информацию о долговой расписке
	var debt struct {
		ID             int
		LenderID       int
		BorrowerID     int
		ReturnAmount   int
		IsReturned     bool
		PenaltyApplied bool
	}

	err = tx.QueryRow(`
		SELECT id, lender_player_id, borrower_player_id, return_amount, is_returned, penalty_applied
		FROM debt_receipts
		WHERE id = $1
		FOR UPDATE
	`, debtID).Scan(
		&debt.ID,
		&debt.LenderID,
		&debt.BorrowerID,
		&debt.ReturnAmount,
		&debt.IsReturned,
		&debt.PenaltyApplied,
	)

	if err != nil {
		log.Printf("Error fetching debt #%d: %v", debtID, err)
		return
	}

	// Проверяем, что долг ещё не возвращен и штраф не применён
	if debt.IsReturned {
		log.Printf("Debt #%d already returned, skipping penalty", debtID)
		return
	}

	if debt.PenaltyApplied {
		log.Printf("Penalty for debt #%d already applied, skipping", debtID)
		return
	}

	// Получаем баланс заемщика
	var borrowerMoney int
	var borrowerName string
	err = tx.QueryRow(`
		SELECT money, character_name
		FROM players
		WHERE id = $1
		FOR UPDATE
	`, debt.BorrowerID).Scan(&borrowerMoney, &borrowerName)

	if err != nil {
		log.Printf("Error fetching borrower for debt #%d: %v", debtID, err)
		return
	}

	// Получаем имя кредитора
	var lenderName string
	tx.QueryRow(`SELECT character_name FROM players WHERE id = $1`, debt.LenderID).Scan(&lenderName)

	// Вычисляем сумму списания (минимум из того что должен и того что есть)
	amountToDeduct := debt.ReturnAmount
	if borrowerMoney < amountToDeduct {
		amountToDeduct = borrowerMoney
	}

	// Списываем деньги с заемщика
	_, err = tx.Exec(`
		UPDATE players
		SET money = money - $1
		WHERE id = $2
	`, amountToDeduct, debt.BorrowerID)

	if err != nil {
		log.Printf("Error deducting money from borrower for debt #%d: %v", debtID, err)
		return
	}

	// Переводим деньги кредитору
	_, err = tx.Exec(`
		UPDATE players
		SET money = money + $1
		WHERE id = $2
	`, amountToDeduct, debt.LenderID)

	if err != nil {
		log.Printf("Error transferring money to lender for debt #%d: %v", debtID, err)
		return
	}

	// Записываем транзакцию
	description := fmt.Sprintf("Automatic debt collection: %s → %s (overdue debt #%d, amount: %d)",
		borrowerName, lenderName, debtID, amountToDeduct)
	_, err = tx.Exec(`
		INSERT INTO money_transactions (from_player_id, to_player_id, amount, transaction_type, reference_id, reference_type, description)
		VALUES ($1, $2, $3, 'debt', $4, 'debt_receipt', $5)
	`, debt.BorrowerID, debt.LenderID, amountToDeduct, debtID, description)

	if err != nil {
		log.Printf("Error recording money transaction for debt #%d: %v", debtID, err)
		return
	}

	// Получаем настройки штрафа по влиянию
	var influencePenalty int
	err = tx.QueryRow(`
		SELECT penalty_influence_points
		FROM debt_penalty_settings
		ORDER BY id DESC
		LIMIT 1
	`).Scan(&influencePenalty)

	if err != nil {
		log.Printf("Warning: Failed to fetch penalty settings for debt #%d: %v", debtID, err)
		influencePenalty = 0 // По умолчанию нет штрафа
	}

	// Применяем штраф по влиянию (если настроен)
	if influencePenalty > 0 {
		_, err = tx.Exec(`
			UPDATE players
			SET influence = GREATEST(0, influence - $1)
			WHERE id = $2
		`, influencePenalty, debt.BorrowerID)

		if err != nil {
			log.Printf("Error applying influence penalty for debt #%d: %v", debtID, err)
			return
		}

		// Записываем транзакцию влияния
		_, err = tx.Exec(`
			INSERT INTO influence_transactions (player_id, amount, transaction_type, reference_id, reference_type, description)
			VALUES ($1, $2, 'penalty', $3, 'debt_receipt', $4)
		`, debt.BorrowerID, -influencePenalty, debtID,
			fmt.Sprintf("Penalty for overdue debt #%d: -%d influence", debtID, influencePenalty))

		if err != nil {
			log.Printf("Error recording influence transaction for debt #%d: %v", debtID, err)
			return
		}

		log.Printf("Applied influence penalty to player %d: -%d points", debt.BorrowerID, influencePenalty)
	}

	// Отмечаем расписку как с примененным штрафом
	now := time.Now()
	_, err = tx.Exec(`
		UPDATE debt_receipts
		SET penalty_applied = true,
		    penalty_applied_at = $1
		WHERE id = $2
	`, now, debtID)

	if err != nil {
		log.Printf("Error updating debt receipt #%d: %v", debtID, err)
		return
	}

	// Фиксируем транзакцию
	if err = tx.Commit(); err != nil {
		log.Printf("Error committing penalty for debt #%d: %v", debtID, err)
		return
	}

	log.Printf("Successfully applied penalty for debt #%d: %d money transferred, %d influence penalty",
		debtID, amountToDeduct, influencePenalty)
}

// GetScheduledCount возвращает количество активных таймеров долговых расписок
func (s *DebtScheduler) GetScheduledCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.timers)
}

// Stop останавливает все таймеры
func (s *DebtScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, timer := range s.timers {
		timer.Stop()
		delete(s.timers, id)
	}
	s.running = false

	log.Println("Debt scheduler stopped")
}
