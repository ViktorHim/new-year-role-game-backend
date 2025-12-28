// internal/handlers/admin_monitoring.go
package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type AdminMonitoringHandler struct {
	db *sql.DB
}

func NewAdminMonitoringHandler(db *sql.DB) *AdminMonitoringHandler {
	return &AdminMonitoringHandler{db: db}
}

// ============================================
// КОНТРАКТЫ (CONTRACTS)
// ============================================

type ContractResponse struct {
	ID                  int        `json:"id"`
	ContractType        string     `json:"contract_type"`
	CustomerPlayerID    int        `json:"customer_player_id"`
	CustomerPlayerName  string     `json:"customer_player_name"`
	ExecutorPlayerID    int        `json:"executor_player_id"`
	ExecutorPlayerName  string     `json:"executor_player_name"`
	CustomerFactionID   *int       `json:"customer_faction_id,omitempty"`
	CustomerFactionName *string    `json:"customer_faction_name,omitempty"`
	Status              string     `json:"status"`
	DurationSeconds     int        `json:"duration_seconds"`
	MoneyRewardCustomer int        `json:"money_reward_customer"`
	MoneyRewardExecutor int        `json:"money_reward_executor"`
	CreatedAt           time.Time  `json:"created_at"`
	SignedAt            *time.Time `json:"signed_at,omitempty"`
	ExpiresAt           *time.Time `json:"expires_at,omitempty"`
	CompletedAt         *time.Time `json:"completed_at,omitempty"`
	TerminatedAt        *time.Time `json:"terminated_at,omitempty"`
}

// GetAllContracts возвращает все контракты с фильтрацией
func (h *AdminMonitoringHandler) GetAllContracts(c *gin.Context) {
	status := c.Query("status") // pending, signed, completed, terminated

	query := `
		SELECT 
			c.id, c.contract_type, c.customer_player_id, p1.character_name as customer_name,
			c.executor_player_id, p2.character_name as executor_name,
			c.customer_faction_id, f.name as faction_name, c.status, c.duration_seconds,
			c.money_reward_customer, c.money_reward_executor, c.created_at,
			c.signed_at, c.expires_at, c.completed_at, c.terminated_at
		FROM contracts c
		JOIN players p1 ON c.customer_player_id = p1.id
		JOIN players p2 ON c.executor_player_id = p2.id
		LEFT JOIN factions f ON c.customer_faction_id = f.id
	`

	var rows *sql.Rows
	var err error

	if status != "" {
		query += " WHERE c.status = $1 ORDER BY c.created_at DESC"
		rows, err = h.db.Query(query, status)
	} else {
		query += " ORDER BY c.created_at DESC"
		rows, err = h.db.Query(query)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch contracts"})
		return
	}
	defer rows.Close()

	contracts := make([]ContractResponse, 0)
	for rows.Next() {
		var contract ContractResponse
		err := rows.Scan(
			&contract.ID,
			&contract.ContractType,
			&contract.CustomerPlayerID,
			&contract.CustomerPlayerName,
			&contract.ExecutorPlayerID,
			&contract.ExecutorPlayerName,
			&contract.CustomerFactionID,
			&contract.CustomerFactionName,
			&contract.Status,
			&contract.DurationSeconds,
			&contract.MoneyRewardCustomer,
			&contract.MoneyRewardExecutor,
			&contract.CreatedAt,
			&contract.SignedAt,
			&contract.ExpiresAt,
			&contract.CompletedAt,
			&contract.TerminatedAt,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan contract"})
			return
		}

		contracts = append(contracts, contract)
	}

	c.JSON(http.StatusOK, gin.H{
		"contracts": contracts,
		"count":     len(contracts),
	})
}

// GetContract возвращает один контракт
func (h *AdminMonitoringHandler) GetContract(c *gin.Context) {
	contractIDStr := c.Param("id")
	contractID, err := strconv.Atoi(contractIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid contract ID"})
		return
	}

	var contract ContractResponse
	err = h.db.QueryRow(`
		SELECT 
			c.id, c.contract_type, c.customer_player_id, p1.character_name as customer_name,
			c.executor_player_id, p2.character_name as executor_name,
			c.customer_faction_id, f.name as faction_name, c.status, c.duration_seconds,
			c.money_reward_customer, c.money_reward_executor, c.created_at,
			c.signed_at, c.expires_at, c.completed_at, c.terminated_at
		FROM contracts c
		JOIN players p1 ON c.customer_player_id = p1.id
		JOIN players p2 ON c.executor_player_id = p2.id
		LEFT JOIN factions f ON c.customer_faction_id = f.id
		WHERE c.id = $1
	`, contractID).Scan(
		&contract.ID,
		&contract.ContractType,
		&contract.CustomerPlayerID,
		&contract.CustomerPlayerName,
		&contract.ExecutorPlayerID,
		&contract.ExecutorPlayerName,
		&contract.CustomerFactionID,
		&contract.CustomerFactionName,
		&contract.Status,
		&contract.DurationSeconds,
		&contract.MoneyRewardCustomer,
		&contract.MoneyRewardExecutor,
		&contract.CreatedAt,
		&contract.SignedAt,
		&contract.ExpiresAt,
		&contract.CompletedAt,
		&contract.TerminatedAt,
	)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Contract not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch contract"})
		return
	}

	c.JSON(http.StatusOK, contract)
}

// ============================================
// ДОЛГОВЫЕ РАСПИСКИ (DEBTS)
// ============================================

type DebtResponse struct {
	ID                 int        `json:"id"`
	LenderPlayerID     int        `json:"lender_player_id"`
	LenderPlayerName   string     `json:"lender_player_name"`
	BorrowerPlayerID   int        `json:"borrower_player_id"`
	BorrowerPlayerName string     `json:"borrower_player_name"`
	LoanAmount         int        `json:"loan_amount"`
	RepaymentAmount    int        `json:"repayment_amount"`
	DurationSeconds    int        `json:"duration_seconds"`
	Status             string     `json:"status"`
	CreatedAt          time.Time  `json:"created_at"`
	ExpiresAt          time.Time  `json:"expires_at"`
	RepaidAt           *time.Time `json:"repaid_at,omitempty"`
}

// GetAllDebts возвращает все долговые расписки с фильтрацией
func (h *AdminMonitoringHandler) GetAllDebts(c *gin.Context) {
	status := c.Query("status") // pending, active, repaid, defaulted

	query := `
		SELECT 
			d.id, d.lender_player_id, p1.character_name as lender_name,
			d.borrower_player_id, p2.character_name as borrower_name,
			d.loan_amount, d.repayment_amount, d.duration_seconds, d.status,
			d.created_at, d.expires_at, d.repaid_at
		FROM debts d
		JOIN players p1 ON d.lender_player_id = p1.id
		JOIN players p2 ON d.borrower_player_id = p2.id
	`

	var rows *sql.Rows
	var err error

	if status != "" {
		query += " WHERE d.status = $1 ORDER BY d.created_at DESC"
		rows, err = h.db.Query(query, status)
	} else {
		query += " ORDER BY d.created_at DESC"
		rows, err = h.db.Query(query)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch debts"})
		return
	}
	defer rows.Close()

	debts := make([]DebtResponse, 0)
	for rows.Next() {
		var debt DebtResponse
		err := rows.Scan(
			&debt.ID,
			&debt.LenderPlayerID,
			&debt.LenderPlayerName,
			&debt.BorrowerPlayerID,
			&debt.BorrowerPlayerName,
			&debt.LoanAmount,
			&debt.RepaymentAmount,
			&debt.DurationSeconds,
			&debt.Status,
			&debt.CreatedAt,
			&debt.ExpiresAt,
			&debt.RepaidAt,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan debt"})
			return
		}

		debts = append(debts, debt)
	}

	c.JSON(http.StatusOK, gin.H{
		"debts": debts,
		"count": len(debts),
	})
}

// GetDebt возвращает одну долговую расписку
func (h *AdminMonitoringHandler) GetDebt(c *gin.Context) {
	debtIDStr := c.Param("id")
	debtID, err := strconv.Atoi(debtIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid debt ID"})
		return
	}

	var debt DebtResponse
	err = h.db.QueryRow(`
		SELECT 
			d.id, d.lender_player_id, p1.character_name as lender_name,
			d.borrower_player_id, p2.character_name as borrower_name,
			d.loan_amount, d.repayment_amount, d.duration_seconds, d.status,
			d.created_at, d.expires_at, d.repaid_at
		FROM debts d
		JOIN players p1 ON d.lender_player_id = p1.id
		JOIN players p2 ON d.borrower_player_id = p2.id
		WHERE d.id = $1
	`, debtID).Scan(
		&debt.ID,
		&debt.LenderPlayerID,
		&debt.LenderPlayerName,
		&debt.BorrowerPlayerID,
		&debt.BorrowerPlayerName,
		&debt.LoanAmount,
		&debt.RepaymentAmount,
		&debt.DurationSeconds,
		&debt.Status,
		&debt.CreatedAt,
		&debt.ExpiresAt,
		&debt.RepaidAt,
	)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Debt not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch debt"})
		return
	}

	c.JSON(http.StatusOK, debt)
}

// DeleteDebt удаляет долговую расписку (только pending)
func (h *AdminMonitoringHandler) DeleteDebt(c *gin.Context) {
	debtIDStr := c.Param("id")
	debtID, err := strconv.Atoi(debtIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid debt ID"})
		return
	}

	// Можно удалить только pending долги
	result, err := h.db.Exec(`
		DELETE FROM debts 
		WHERE id = $1 AND status = 'pending'
	`, debtID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete debt"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Debt not found or cannot be deleted (only pending debts can be deleted)"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Debt deleted successfully"})
}

// ============================================
// НАСТРОЙКИ КОНТРАКТОВ TYPE1 ПО ФРАКЦИЯМ
// ============================================

type Type1FactionRewardRequest struct {
	FactionID            int `json:"faction_id" binding:"required"`
	CustomerItemRewardID int `json:"customer_item_reward_id" binding:"required"`
}

type Type1FactionReward struct {
	ID                   int    `json:"id"`
	FactionID            int    `json:"faction_id"`
	FactionName          string `json:"faction_name"`
	CustomerItemRewardID int    `json:"customer_item_reward_id"`
	ItemName             string `json:"item_name"`
}

// GetType1FactionRewards возвращает настройки предметов по фракциям для Type1 контрактов
func (h *AdminMonitoringHandler) GetType1FactionRewards(c *gin.Context) {
	rows, err := h.db.Query(`
		SELECT 
			cts.id, cts.faction_id, f.name as faction_name,
			cts.customer_item_reward_id, i.name as item_name
		FROM contract_type1_settings cts
		JOIN factions f ON cts.faction_id = f.id
		JOIN items i ON cts.customer_item_reward_id = i.id
		ORDER BY f.name
	`)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch Type1 faction rewards"})
		return
	}
	defer rows.Close()

	rewards := make([]Type1FactionReward, 0)
	for rows.Next() {
		var reward Type1FactionReward
		err := rows.Scan(
			&reward.ID,
			&reward.FactionID,
			&reward.FactionName,
			&reward.CustomerItemRewardID,
			&reward.ItemName,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan reward"})
			return
		}

		rewards = append(rewards, reward)
	}

	c.JSON(http.StatusOK, gin.H{"faction_rewards": rewards})
}

// SetType1FactionReward устанавливает предмет для фракции в Type1 контрактах
func (h *AdminMonitoringHandler) SetType1FactionReward(c *gin.Context) {
	var req Type1FactionRewardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Проверяем, существует ли фракция
	var factionExists bool
	err := h.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM factions WHERE id = $1)`, req.FactionID).Scan(&factionExists)
	if err != nil || !factionExists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Faction not found"})
		return
	}

	// Проверяем, существует ли предмет
	var itemExists bool
	err = h.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM items WHERE id = $1)`, req.CustomerItemRewardID).Scan(&itemExists)
	if err != nil || !itemExists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Item not found"})
		return
	}

	// Вставляем или обновляем настройку
	_, err = h.db.Exec(`
		INSERT INTO contract_type1_settings (faction_id, customer_item_reward_id)
		VALUES ($1, $2)
		ON CONFLICT (faction_id) 
		DO UPDATE SET customer_item_reward_id = $2
	`, req.FactionID, req.CustomerItemRewardID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set faction reward"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Type1 faction reward set successfully"})
}

// DeleteType1FactionReward удаляет настройку предмета для фракции
func (h *AdminMonitoringHandler) DeleteType1FactionReward(c *gin.Context) {
	factionIDStr := c.Param("faction_id")
	factionID, err := strconv.Atoi(factionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid faction ID"})
		return
	}

	result, err := h.db.Exec(`
		DELETE FROM contract_type1_settings WHERE faction_id = $1
	`, factionID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete faction reward"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Faction reward not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Type1 faction reward deleted successfully"})
}

// ============================================
// ЖУРНАЛ ТРАНЗАКЦИЙ
// ============================================

type Transaction struct {
	ID               int       `json:"id"`
	Type             string    `json:"type"` // goal_completion, contract_completion, item_transfer, money_transfer, ability_use, debt_repayment
	PlayerID         *int      `json:"player_id,omitempty"`
	PlayerName       *string   `json:"player_name,omitempty"`
	TargetPlayerID   *int      `json:"target_player_id,omitempty"`
	TargetPlayerName *string   `json:"target_player_name,omitempty"`
	Description      string    `json:"description"`
	MoneyChange      *int      `json:"money_change,omitempty"`
	InfluenceChange  *int      `json:"influence_change,omitempty"`
	ItemID           *int      `json:"item_id,omitempty"`
	ItemName         *string   `json:"item_name,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

// GetTransactions возвращает журнал транзакций
func (h *AdminMonitoringHandler) GetTransactions(c *gin.Context) {
	limit := 100
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	playerIDStr := c.Query("player_id")
	transactionType := c.Query("type")

	// Собираем транзакции из разных источников
	transactions := make([]Transaction, 0)

	// 1. История выполнения целей
	query := `
		SELECT 
			gch.id,
			'goal_completion' as type,
			gch.player_id,
			p.character_name as player_name,
			NULL as target_player_id,
			NULL as target_player_name,
			CONCAT('Goal "', g.title, '" ', gch.action) as description,
			NULL as money_change,
			gch.influence_change,
			NULL as item_id,
			NULL as item_name,
			gch.created_at
		FROM goal_completion_history gch
		JOIN players p ON gch.player_id = p.id
		JOIN goals g ON gch.goal_id = g.id
	`

	where := make([]string, 0)
	args := make([]interface{}, 0)
	argNum := 1

	if playerIDStr != "" {
		where = append(where, "gch.player_id = $"+strconv.Itoa(argNum))
		playerID, _ := strconv.Atoi(playerIDStr)
		args = append(args, playerID)
		argNum++
	}

	if len(where) > 0 {
		query += " WHERE " + where[0]
		for i := 1; i < len(where); i++ {
			query += " AND " + where[i]
		}
	}

	query += " ORDER BY gch.created_at DESC LIMIT $" + strconv.Itoa(argNum)
	args = append(args, limit)

	if transactionType == "" || transactionType == "goal_completion" {
		rows, err := h.db.Query(query, args...)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var t Transaction
				rows.Scan(
					&t.ID, &t.Type, &t.PlayerID, &t.PlayerName,
					&t.TargetPlayerID, &t.TargetPlayerName, &t.Description,
					&t.MoneyChange, &t.InfluenceChange, &t.ItemID, &t.ItemName, &t.CreatedAt,
				)
				transactions = append(transactions, t)
			}
		}
	}

	// 2. Можно добавить другие источники транзакций...
	// (переводы денег, использование способностей, и т.д.)

	c.JSON(http.StatusOK, gin.H{
		"transactions": transactions,
		"count":        len(transactions),
	})
}
