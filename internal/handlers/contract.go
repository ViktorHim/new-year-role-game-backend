// internal/handlers/contract.go
package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"new-year-role-game-backend/internal/models"
	"new-year-role-game-backend/internal/workers"
	"time"

	"github.com/gin-gonic/gin"
)

type ContractHandler struct {
	db        *sql.DB
	scheduler *workers.ContractScheduler
}

func NewContractHandler(db *sql.DB, scheduler *workers.ContractScheduler) *ContractHandler {
	return &ContractHandler{
		db:        db,
		scheduler: scheduler,
	}
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
	// Для type2 договоров заказчик их не видит
	// Для type2 добавляем информацию о раскрытом факте (если есть)
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
			c.terminated_at,
			ri.info_type,
			ri.revealed_data,
			ri.revealed_at
		FROM contracts c
		JOIN players customer ON c.customer_player_id = customer.id
		JOIN players executor ON c.executor_player_id = executor.id
		LEFT JOIN factions f ON c.customer_faction_id = f.id
		LEFT JOIN revealed_info ri ON c.id = ri.contract_id AND c.contract_type = 'type2'
		WHERE (
			c.executor_player_id = $1 OR 
			(c.customer_player_id = $1 AND c.contract_type = 'type1')
		)
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
		var revealedInfoType *string
		var revealedData *[]byte
		var revealedAt *time.Time

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
			&revealedInfoType,
			&revealedData,
			&revealedAt,
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
		// type2 договоры создаются сразу подписанными, так что для них CanSign всегда false
		contract.CanSign = contract.Status == "pending" && contract.IsCustomer && contract.ContractType == "type1"
		contract.CanComplete = contract.Status == "signed" && contract.IsCustomer &&
			contract.ExpiresAt != nil && now.After(*contract.ExpiresAt) && contract.ContractType == "type1"

		// Для type2 договоров добавляем информацию о раскрытом факте
		// info_revealed всегда установлен в true или false
		// Дополнительные поля (revealed_info_type, revealed_info_data, revealed_at) присутствуют только если info_revealed = true
		if contract.ContractType == "type2" {
			if revealedInfoType != nil && revealedData != nil {
				// Факт был раскрыт
				contract.InfoRevealed = true
				contract.RevealedInfoType = revealedInfoType

				// Парсим JSON данные
				var data map[string]interface{}
				if err := json.Unmarshal(*revealedData, &data); err == nil {
					contract.RevealedInfoData = data
				}

				contract.RevealedAt = revealedAt
			} else {
				// Факт еще не раскрыт
				contract.InfoRevealed = false
			}
		}

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

	// Проверяем лимит активных договоров этого типа для исполнителя
	var activeContractsCount int
	err = tx.QueryRow(`
		SELECT COUNT(*)
		FROM contracts
		WHERE executor_player_id = $1
		  AND contract_type = $2
		  AND status IN ('pending', 'signed')
	`, *playerID, req.ContractType).Scan(&activeContractsCount)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check active contracts"})
		return
	}

	if activeContractsCount >= 3 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Maximum 3 active contracts of this type allowed"})
		return
	}

	// Проверяем, что заказчик существует и получаем его фракцию
	var customerExists bool
	var customerName string
	var customerFactionID *int
	err = tx.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM players WHERE id = $1),
		       COALESCE((SELECT character_name FROM players WHERE id = $1), ''),
		       (SELECT faction_id FROM players WHERE id = $1)
	`, req.CustomerPlayerID).Scan(&customerExists, &customerName, &customerFactionID)

	if err != nil || !customerExists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Customer player not found"})
		return
	}

	// Получаем длительность договора из настроек администратора
	var durationMinutes int
	err = tx.QueryRow(`
		SELECT duration_minutes
		FROM contract_duration_settings
		WHERE type = $1
		ORDER BY id DESC
		LIMIT 1
	`, req.ContractType).Scan(&durationMinutes)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Contract duration not configured by admin"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch contract duration"})
		}
		return
	}

	durationSeconds := durationMinutes * 60

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
	var status string
	var signedAt, expiresAt *time.Time

	// Для type2 создаем договор сразу в статусе "signed"
	if req.ContractType == "type2" {
		status = "signed"
		now := time.Now()
		expires := now.Add(time.Duration(durationSeconds) * time.Second)
		signedAt = &now
		expiresAt = &expires

		err = tx.QueryRow(`
			INSERT INTO contracts (
				contract_type,
				customer_player_id,
				executor_player_id,
				customer_faction_id,
				status,
				duration_seconds,
				money_reward_customer,
				money_reward_executor,
				created_at,
				signed_at,
				expires_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			RETURNING id
		`, req.ContractType, req.CustomerPlayerID, *playerID, customerFactionID,
			status, durationSeconds, moneyRewardCustomer, moneyRewardExecutor,
			now, signedAt, expiresAt).Scan(&contractID)
	} else {
		// Для type1 создаем в статусе "pending"
		status = "pending"
		now := time.Now()

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
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING id
		`, req.ContractType, req.CustomerPlayerID, *playerID,
			status, durationSeconds, moneyRewardCustomer, moneyRewardExecutor,
			now).Scan(&contractID)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create contract"})
		return
	}

	// Фиксируем транзакцию
	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	// ВАЖНО: Для type2 создаем таймер автоматического завершения
	if req.ContractType == "type2" && expiresAt != nil {
		h.scheduler.ScheduleContract(contractID, *expiresAt)
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

	// Вычисляем оставшееся время для type2 (он сразу подписан)
	if req.ContractType == "type2" && contract.ExpiresAt != nil {
		now := time.Now()
		if now.Before(*contract.ExpiresAt) {
			remaining := int(contract.ExpiresAt.Sub(now).Seconds())
			contract.TimeRemaining = &remaining
		} else {
			zero := 0
			contract.TimeRemaining = &zero
		}
	}

	c.JSON(http.StatusCreated, contract)
}
