// internal/handlers/debt.go
package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"new-year-role-game-backend/internal/models"
	"new-year-role-game-backend/internal/workers"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type DebtHandler struct {
	db        *sql.DB
	scheduler *workers.DebtScheduler
}

func NewDebtHandler(db *sql.DB, scheduler *workers.DebtScheduler) *DebtHandler {
	return &DebtHandler{
		db:        db,
		scheduler: scheduler,
	}
}

// GetPlayerDebts возвращает список всех долговых расписок игрока
func (h *DebtHandler) GetPlayerDebts(c *gin.Context) {
	playerIDInterface, exists := c.Get("player_id")
	if !exists || playerIDInterface == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Player ID not found in token"})
		return
	}

	playerID := playerIDInterface.(*int)
	if playerID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User is not associated with a player"})
		return
	}

	// Получаем все долговые расписки, где игрок - кредитор или заемщик
	rows, err := h.db.Query(`
		SELECT 
			dr.id,
			dr.lender_player_id,
			lender.character_name AS lender_name,
			lender.avatar AS lender_avatar,
			dr.borrower_player_id,
			borrower.character_name AS borrower_name,
			borrower.avatar AS borrower_avatar,
			dr.loan_amount,
			dr.return_amount,
			dr.created_at,
			dr.return_deadline,
			dr.is_returned,
			dr.returned_at,
			dr.penalty_applied,
			dr.penalty_applied_at
		FROM debt_receipts dr
		JOIN players lender ON dr.lender_player_id = lender.id
		JOIN players borrower ON dr.borrower_player_id = borrower.id
		WHERE dr.lender_player_id = $1 OR dr.borrower_player_id = $1
		ORDER BY 
			CASE 
				WHEN dr.is_returned = false AND dr.penalty_applied = false THEN 1
				WHEN dr.penalty_applied = true THEN 2
				WHEN dr.is_returned = true THEN 3
			END,
			dr.return_deadline DESC
	`, *playerID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch debt receipts"})
		return
	}
	defer rows.Close()

	debts := make([]models.DebtReceipt, 0)
	now := time.Now()

	for rows.Next() {
		var debt models.DebtReceipt

		err := rows.Scan(
			&debt.ID,
			&debt.LenderPlayerID,
			&debt.LenderPlayerName,
			&debt.LenderPlayerAvatar,
			&debt.BorrowerPlayerID,
			&debt.BorrowerPlayerName,
			&debt.BorrowerPlayerAvatar,
			&debt.LoanAmount,
			&debt.ReturnAmount,
			&debt.CreatedAt,
			&debt.ReturnDeadline,
			&debt.IsReturned,
			&debt.ReturnedAt,
			&debt.PenaltyApplied,
			&debt.PenaltyAppliedAt,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan debt receipt"})
			return
		}

		// Определяем роль текущего игрока
		debt.IsLender = debt.LenderPlayerID == *playerID
		debt.IsBorrower = debt.BorrowerPlayerID == *playerID

		// Вычисляем оставшееся время для активных долгов
		if !debt.IsReturned && !debt.PenaltyApplied {
			if now.Before(debt.ReturnDeadline) {
				remaining := int(debt.ReturnDeadline.Sub(now).Seconds())
				debt.TimeRemaining = &remaining
			} else {
				zero := 0
				debt.TimeRemaining = &zero
			}

			// Определяем статус
			if now.After(debt.ReturnDeadline) {
				debt.Status = "overdue"
			} else {
				debt.Status = "active"
			}
		} else if debt.IsReturned {
			debt.Status = "returned"
		} else if debt.PenaltyApplied {
			debt.Status = "penalty_applied"
		}

		// Определяем возможные действия
		debt.CanReturn = !debt.IsReturned && !debt.PenaltyApplied && debt.IsLender

		debts = append(debts, debt)
	}

	if err = rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, models.DebtReceiptsResponse{Debts: debts})
}

// CreateDebtReceipt создает новую долговую расписку
func (h *DebtHandler) CreateDebtReceipt(c *gin.Context) {
	playerIDInterface, exists := c.Get("player_id")
	if !exists || playerIDInterface == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Player ID not found in token"})
		return
	}

	playerID := playerIDInterface.(*int)
	if playerID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User is not associated with a player"})
		return
	}

	var req models.CreateDebtReceiptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Валидация
	if req.LoanAmount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Loan amount must be positive"})
		return
	}

	if req.ReturnAmount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Return amount must be positive"})
		return
	}

	if req.ReturnAmount < req.LoanAmount {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Return amount must be greater than or equal to loan amount"})
		return
	}

	if req.DeadlineMinutes <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Deadline must be positive"})
		return
	}

	// Проверяем, что не создаем расписку с самим собой
	if req.BorrowerPlayerID == *playerID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot create debt receipt with yourself"})
		return
	}

	// Начинаем транзакцию
	tx, err := h.db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	// Проверяем баланс кредитора
	var lenderMoney int
	var lenderName string
	err = tx.QueryRow(`
		SELECT money, character_name
		FROM players
		WHERE id = $1
		FOR UPDATE
	`, *playerID).Scan(&lenderMoney, &lenderName)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if lenderMoney < req.LoanAmount {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Insufficient funds"})
		return
	}

	// Проверяем, что заемщик существует
	var borrowerExists bool
	var borrowerName string
	err = tx.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM players WHERE id = $1),
		       COALESCE((SELECT character_name FROM players WHERE id = $1), '')
		FROM players WHERE id = $2
		FOR UPDATE
	`, req.BorrowerPlayerID, req.BorrowerPlayerID).Scan(&borrowerExists, &borrowerName)

	if err != nil || !borrowerExists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Borrower player not found"})
		return
	}

	now := time.Now()
	deadline := now.Add(time.Duration(req.DeadlineMinutes) * time.Minute)

	// Создаем долговую расписку
	var debtID int
	err = tx.QueryRow(`
		INSERT INTO debt_receipts (
			lender_player_id,
			borrower_player_id,
			loan_amount,
			return_amount,
			created_at,
			return_deadline,
			is_returned,
			penalty_applied
		)
		VALUES ($1, $2, $3, $4, $5, $6, false, false)
		RETURNING id
	`, *playerID, req.BorrowerPlayerID, req.LoanAmount, req.ReturnAmount, now, deadline).Scan(&debtID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create debt receipt"})
		return
	}

	// Переводим деньги от кредитора к заемщику
	_, err = tx.Exec(`
		UPDATE players
		SET money = money - $1
		WHERE id = $2
	`, req.LoanAmount, *playerID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to deduct money from lender"})
		return
	}

	_, err = tx.Exec(`
		UPDATE players
		SET money = money + $1
		WHERE id = $2
	`, req.LoanAmount, req.BorrowerPlayerID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add money to borrower"})
		return
	}

	// Записываем транзакцию
	description := fmt.Sprintf("Debt loan: %s → %s (debt #%d, amount: %d, to return: %d)",
		lenderName, borrowerName, debtID, req.LoanAmount, req.ReturnAmount)
	_, err = tx.Exec(`
		INSERT INTO money_transactions (from_player_id, to_player_id, amount, transaction_type, reference_id, reference_type, description)
		VALUES ($1, $2, $3, 'debt', $4, 'debt_receipt', $5)
	`, *playerID, req.BorrowerPlayerID, req.LoanAmount, debtID, description)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record transaction"})
		return
	}

	// Фиксируем транзакцию
	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	// Создаём точный таймер для истечения долга
	h.scheduler.ScheduleDebt(debtID, deadline)

	// Получаем созданную расписку
	var debt models.DebtReceipt
	err = h.db.QueryRow(`
		SELECT 
			dr.id,
			dr.lender_player_id,
			lender.character_name,
			lender.avatar,
			dr.borrower_player_id,
			borrower.character_name,
			borrower.avatar,
			dr.loan_amount,
			dr.return_amount,
			dr.created_at,
			dr.return_deadline,
			dr.is_returned,
			dr.returned_at,
			dr.penalty_applied,
			dr.penalty_applied_at
		FROM debt_receipts dr
		JOIN players lender ON dr.lender_player_id = lender.id
		JOIN players borrower ON dr.borrower_player_id = borrower.id
		WHERE dr.id = $1
	`, debtID).Scan(
		&debt.ID,
		&debt.LenderPlayerID,
		&debt.LenderPlayerName,
		&debt.LenderPlayerAvatar,
		&debt.BorrowerPlayerID,
		&debt.BorrowerPlayerName,
		&debt.BorrowerPlayerAvatar,
		&debt.LoanAmount,
		&debt.ReturnAmount,
		&debt.CreatedAt,
		&debt.ReturnDeadline,
		&debt.IsReturned,
		&debt.ReturnedAt,
		&debt.PenaltyApplied,
		&debt.PenaltyAppliedAt,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch created debt receipt"})
		return
	}

	debt.IsLender = true
	debt.IsBorrower = false
	debt.Status = "active"
	remaining := int(deadline.Sub(now).Seconds())
	debt.TimeRemaining = &remaining
	debt.CanReturn = true

	c.JSON(http.StatusCreated, debt)
}

// ReturnDebt обрабатывает возврат долга (инициирует кредитор)
func (h *DebtHandler) ReturnDebt(c *gin.Context) {
	playerIDInterface, exists := c.Get("player_id")
	if !exists || playerIDInterface == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Player ID not found in token"})
		return
	}

	playerID := playerIDInterface.(*int)
	if playerID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User is not associated with a player"})
		return
	}

	debtIDStr := c.Param("id")
	debtID, err := strconv.Atoi(debtIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid debt ID"})
		return
	}

	// Начинаем транзакцию
	tx, err := h.db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
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
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Debt receipt not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Проверяем права доступа (только кредитор может инициировать возврат)
	if debt.LenderID != *playerID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only lender can confirm debt return"})
		return
	}

	// Проверяем, что долг ещё не возвращен
	if debt.IsReturned {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Debt already returned"})
		return
	}

	// Проверяем, что штраф не применён
	if debt.PenaltyApplied {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Penalty already applied, debt cannot be returned"})
		return
	}

	// Проверяем баланс заемщика
	var borrowerMoney int
	var borrowerName string
	err = tx.QueryRow(`
		SELECT money, character_name
		FROM players
		WHERE id = $1
		FOR UPDATE
	`, debt.BorrowerID).Scan(&borrowerMoney, &borrowerName)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if borrowerMoney < debt.ReturnAmount {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":          "Borrower has insufficient funds",
			"required":       debt.ReturnAmount,
			"borrower_money": borrowerMoney,
		})
		return
	}

	// Получаем имя кредитора
	var lenderName string
	tx.QueryRow(`SELECT character_name FROM players WHERE id = $1`, debt.LenderID).Scan(&lenderName)

	// Переводим деньги от заемщика к кредитору
	_, err = tx.Exec(`
		UPDATE players
		SET money = money - $1
		WHERE id = $2
	`, debt.ReturnAmount, debt.BorrowerID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to deduct money from borrower"})
		return
	}

	_, err = tx.Exec(`
		UPDATE players
		SET money = money + $1
		WHERE id = $2
	`, debt.ReturnAmount, debt.LenderID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add money to lender"})
		return
	}

	// Записываем транзакцию
	description := fmt.Sprintf("Debt return: %s → %s (debt #%d, amount: %d)",
		borrowerName, lenderName, debtID, debt.ReturnAmount)
	_, err = tx.Exec(`
		INSERT INTO money_transactions (from_player_id, to_player_id, amount, transaction_type, reference_id, reference_type, description)
		VALUES ($1, $2, $3, 'debt', $4, 'debt_receipt', $5)
	`, debt.BorrowerID, debt.LenderID, debt.ReturnAmount, debtID, description)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record transaction"})
		return
	}

	// Отмечаем расписку как возвращенную
	now := time.Now()
	_, err = tx.Exec(`
		UPDATE debt_receipts
		SET is_returned = true,
		    returned_at = $1
		WHERE id = $2
	`, now, debtID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update debt receipt"})
		return
	}

	// Фиксируем транзакцию
	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	// Отменяем таймер в scheduler
	h.scheduler.CancelDebt(debtID)

	c.JSON(http.StatusOK, gin.H{
		"message":     "Debt returned successfully",
		"debt_id":     debtID,
		"amount":      debt.ReturnAmount,
		"returned_at": now,
	})
}
