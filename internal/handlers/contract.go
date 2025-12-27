// internal/handlers/contract.go
package handlers

import (
	"database/sql"
	"net/http"
	"new-year-role-game-backend/internal/models"
	"time"

	"github.com/gin-gonic/gin"
)

type ContractHandler struct {
	db *sql.DB
}

func NewContractHandler(db *sql.DB) *ContractHandler {
	return &ContractHandler{db: db}
}

// GetPlayerContracts возвращает список всех договоров игрока
func (h *ContractHandler) GetPlayerContracts(c *gin.Context) {
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

	// Получаем все договоры, где игрок - заказчик или исполнитель
	rows, err := h.db.Query(`
		SELECT 
			c.id,
			c.contract_type,
			c.customer_player_id,
			customer.character_name AS customer_name,
			customer.avatar AS customer_avatar,
			c.executor_player_id,
			executor.character_name AS executor_name,
			executor.avatar AS executor_avatar,
			c.customer_faction_id,
			f.name AS customer_faction_name,
			c.status,
			c.duration_seconds,
			c.money_reward_customer,
			c.money_reward_executor,
			c.created_at,
			c.signed_at,
			c.expires_at,
			c.completed_at,
			c.terminated_at
		FROM contracts c
		JOIN players customer ON c.customer_player_id = customer.id
		JOIN players executor ON c.executor_player_id = executor.id
		LEFT JOIN factions f ON c.customer_faction_id = f.id
		WHERE c.customer_player_id = $1 OR c.executor_player_id = $1
		ORDER BY 
			CASE c.status
				WHEN 'pending' THEN 1
				WHEN 'signed' THEN 2
				WHEN 'completed' THEN 3
				WHEN 'terminated' THEN 4
			END,
			c.created_at DESC
	`, *playerID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch contracts"})
		return
	}
	defer rows.Close()

	contracts := make([]models.Contract, 0)
	now := time.Now()

	for rows.Next() {
		var contract models.Contract

		err := rows.Scan(
			&contract.ID,
			&contract.ContractType,
			&contract.CustomerPlayerID,
			&contract.CustomerPlayerName,
			&contract.CustomerPlayerAvatar,
			&contract.ExecutorPlayerID,
			&contract.ExecutorPlayerName,
			&contract.ExecutorPlayerAvatar,
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

		// Определяем роль текущего игрока в договоре
		contract.IsCustomer = contract.CustomerPlayerID == *playerID
		contract.IsExecutor = contract.ExecutorPlayerID == *playerID

		// Вычисляем оставшееся время для подписанных договоров
		if contract.Status == "signed" && contract.ExpiresAt != nil {
			if now.Before(*contract.ExpiresAt) {
				remaining := int(contract.ExpiresAt.Sub(now).Seconds())
				contract.TimeRemaining = &remaining
			} else {
				zero := 0
				contract.TimeRemaining = &zero
			}
		}

		// Определяем возможные действия
		contract.CanSign = contract.Status == "pending" && contract.IsCustomer
		contract.CanComplete = contract.Status == "signed" && contract.IsCustomer &&
			contract.ExpiresAt != nil && now.After(*contract.ExpiresAt)

		contracts = append(contracts, contract)
	}

	if err = rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, models.ContractsResponse{Contracts: contracts})
}

// CreateContract создает новый договор
func (h *ContractHandler) CreateContract(c *gin.Context) {
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

	// Парсим тело запроса
	var req models.CreateContractRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Проверяем тип договора
	if req.ContractType != "type1" && req.ContractType != "type2" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "contract_type must be 'type1' or 'type2'"})
		return
	}

	// Проверяем, что не пытаемся создать договор с собой
	if req.CustomerPlayerID == *playerID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot create contract with yourself"})
		return
	}

	// Начинаем транзакцию
	tx, err := h.db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	// Проверяем, что заказчик существует
	var customerExists bool
	var customerName string
	err = tx.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM players WHERE id = $1),
		       COALESCE((SELECT character_name FROM players WHERE id = $1), '')
	`, req.CustomerPlayerID).Scan(&customerExists, &customerName)

	if err != nil || !customerExists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Customer player not found"})
		return
	}

	// Получаем награды из настроек администратора в зависимости от типа договора
	var moneyRewardCustomer, moneyRewardExecutor int

	if req.ContractType == "type1" {
		err = tx.QueryRow(`
			SELECT money_reward_customer, money_reward_executor
			FROM contract_type1_reward_settings
			ORDER BY id DESC
			LIMIT 1
		`).Scan(&moneyRewardCustomer, &moneyRewardExecutor)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Contract Type 1 rewards not configured by admin"})
			return
		}
	} else if req.ContractType == "type2" {
		moneyRewardCustomer = 0 // Type 2 - заказчик всегда получает 0
		err = tx.QueryRow(`
			SELECT money_reward_executor
			FROM contract_type2_reward_settings
			ORDER BY id DESC
			LIMIT 1
		`).Scan(&moneyRewardExecutor)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Contract Type 2 rewards not configured by admin"})
			return
		}
	}

	// Создаем договор
	var contractID int
	err = tx.QueryRow(`
		INSERT INTO contracts (
			contract_type,
			customer_player_id,
			executor_player_id,
			status,
			duration_seconds,
			money_reward_customer,
			money_reward_executor,
			created_at
		)
		VALUES ($1, $2, $3, 'pending', $4, $5, $6, NOW())
		RETURNING id
	`, req.ContractType, req.CustomerPlayerID, *playerID, req.DurationSeconds,
		moneyRewardCustomer, moneyRewardExecutor).Scan(&contractID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create contract"})
		return
	}

	// Фиксируем транзакцию
	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	// Получаем созданный договор
	var contract models.Contract
	err = h.db.QueryRow(`
		SELECT 
			c.id,
			c.contract_type,
			c.customer_player_id,
			customer.character_name AS customer_name,
			customer.avatar AS customer_avatar,
			c.executor_player_id,
			executor.character_name AS executor_name,
			executor.avatar AS executor_avatar,
			c.customer_faction_id,
			f.name AS customer_faction_name,
			c.status,
			c.duration_seconds,
			c.money_reward_customer,
			c.money_reward_executor,
			c.created_at,
			c.signed_at,
			c.expires_at,
			c.completed_at,
			c.terminated_at
		FROM contracts c
		JOIN players customer ON c.customer_player_id = customer.id
		JOIN players executor ON c.executor_player_id = executor.id
		LEFT JOIN factions f ON c.customer_faction_id = f.id
		WHERE c.id = $1
	`, contractID).Scan(
		&contract.ID,
		&contract.ContractType,
		&contract.CustomerPlayerID,
		&contract.CustomerPlayerName,
		&contract.CustomerPlayerAvatar,
		&contract.ExecutorPlayerID,
		&contract.ExecutorPlayerName,
		&contract.ExecutorPlayerAvatar,
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch created contract"})
		return
	}

	// Устанавливаем дополнительные поля
	contract.IsCustomer = false
	contract.IsExecutor = true
	contract.CanSign = false
	contract.CanComplete = false

	c.JSON(http.StatusCreated, contract)
}
