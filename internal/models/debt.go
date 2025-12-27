// internal/models/debt.go
package models

import "time"

type DebtReceipt struct {
	ID                   int        `json:"id"`
	LenderPlayerID       int        `json:"lender_player_id"`
	LenderPlayerName     string     `json:"lender_player_name"`
	LenderPlayerAvatar   *string    `json:"lender_player_avatar"`
	BorrowerPlayerID     int        `json:"borrower_player_id"`
	BorrowerPlayerName   string     `json:"borrower_player_name"`
	BorrowerPlayerAvatar *string    `json:"borrower_player_avatar"`
	LoanAmount           int        `json:"loan_amount"`   // Сумма займа
	ReturnAmount         int        `json:"return_amount"` // Сумма возврата
	CreatedAt            time.Time  `json:"created_at"`
	ReturnDeadline       time.Time  `json:"return_deadline"`
	IsReturned           bool       `json:"is_returned"`
	ReturnedAt           *time.Time `json:"returned_at,omitempty"`
	PenaltyApplied       bool       `json:"penalty_applied"`
	PenaltyAppliedAt     *time.Time `json:"penalty_applied_at,omitempty"`

	// Дополнительные поля для удобства клиента
	IsLender      bool   `json:"is_lender"`                // true если текущий игрок - кредитор
	IsBorrower    bool   `json:"is_borrower"`              // true если текущий игрок - заемщик
	Status        string `json:"status"`                   // 'active', 'returned', 'overdue', 'penalty_applied'
	TimeRemaining *int   `json:"time_remaining,omitempty"` // секунды до истечения (для active)
	CanReturn     bool   `json:"can_return"`               // можно ли вернуть долг (для кредитора)
}

type DebtReceiptsResponse struct {
	Debts []DebtReceipt `json:"debts"`
}

type CreateDebtReceiptRequest struct {
	BorrowerPlayerID int `json:"borrower_player_id" binding:"required"`
	LoanAmount       int `json:"loan_amount" binding:"required,min=1"`      // Сумма займа
	ReturnAmount     int `json:"return_amount" binding:"required,min=1"`    // Сумма возврата
	DeadlineMinutes  int `json:"deadline_minutes" binding:"required,min=1"` // Срок в минутах
}

type DebtPenaltySettings struct {
	ID                     int `json:"id"`
	PenaltyInfluencePoints int `json:"penalty_influence_points"`
}

type UpdateDebtPenaltyRequest struct {
	PenaltyInfluencePoints int `json:"penalty_influence_points" binding:"required,min=0"`
}
