// internal/handlers/contract_actions_with_scheduler.go
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

type ContractHandlerWithScheduler struct {
	db        *sql.DB
	scheduler *workers.ContractScheduler
}

func NewContractHandlerWithScheduler(db *sql.DB, scheduler *workers.ContractScheduler) *ContractHandlerWithScheduler {
	return &ContractHandlerWithScheduler{
		db:        db,
		scheduler: scheduler,
	}
}

// SignContract - заказчик подписывает договор
func (h *ContractHandlerWithScheduler) SignContract(c *gin.Context) {
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

	contractIDStr := c.Param("id")
	contractID, err := strconv.Atoi(contractIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid contract ID"})
		return
	}

	// Начинаем транзакцию
	tx, err := h.db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	// Получаем информацию о договоре
	var contract struct {
		Status           string
		CustomerPlayerID int
		ExecutorPlayerID int
		DurationSeconds  int
	}

	err = tx.QueryRow(`
		SELECT status, customer_player_id, executor_player_id, duration_seconds
		FROM contracts
		WHERE id = $1
		FOR UPDATE
	`, contractID).Scan(
		&contract.Status,
		&contract.CustomerPlayerID,
		&contract.ExecutorPlayerID,
		&contract.DurationSeconds,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Contract not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Проверяем, что пользователь - заказчик
	if contract.CustomerPlayerID != *playerID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only customer can sign the contract"})
		return
	}

	// Проверяем статус договора
	if contract.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Contract is not in pending status"})
		return
	}

	// Получаем фракцию заказчика и проверяем конфликты
	var customerFactionID *int
	err = tx.QueryRow(`
		SELECT faction_id FROM players WHERE id = $1
	`, *playerID).Scan(&customerFactionID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch customer faction"})
		return
	}

	// Проверяем наличие активных договоров с другими фракциями
	if customerFactionID != nil {
		var conflictingFactionID *int
		err = tx.QueryRow(`
			SELECT DISTINCT p.faction_id
			FROM contracts c
			JOIN players p ON (
				CASE 
					WHEN c.customer_player_id = $1 THEN c.executor_player_id = p.id
					ELSE c.customer_player_id = p.id
				END
			)
			WHERE (c.customer_player_id = $1 OR c.executor_player_id = $1)
			  AND c.status = 'signed'
			  AND p.faction_id IS NOT NULL
			  AND p.faction_id != $2
			LIMIT 1
		`, *playerID, *customerFactionID).Scan(&conflictingFactionID)

		if err != nil && err != sql.ErrNoRows {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check faction conflicts"})
			return
		}

		// Если есть конфликт фракций, применяем штраф
		if conflictingFactionID != nil {
			var moneyPenalty, influencePenalty int
			err = tx.QueryRow(`
				SELECT money_penalty, influence_penalty
				FROM contract_penalty_settings
				ORDER BY id DESC
				LIMIT 1
			`).Scan(&moneyPenalty, &influencePenalty)

			if err != nil && err != sql.ErrNoRows {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch penalty settings"})
				return
			}

			// Снимаем деньги
			if moneyPenalty > 0 {
				_, err = tx.Exec(`
					UPDATE players
					SET money = GREATEST(0, money - $1)
					WHERE id = $2
				`, moneyPenalty, *playerID)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to apply money penalty"})
					return
				}

				_, err = tx.Exec(`
					INSERT INTO money_transactions (from_player_id, amount, transaction_type, reference_id, reference_type, description)
					VALUES ($1, $2, 'contract', $3, 'contract', $4)
				`, *playerID, -moneyPenalty, contractID, "Faction conflict penalty")
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record money penalty"})
					return
				}
			}

			// Снимаем влияние
			if influencePenalty > 0 {
				_, err = tx.Exec(`
					UPDATE players
					SET influence = GREATEST(0, influence - $1)
					WHERE id = $2
				`, influencePenalty, *playerID)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to apply influence penalty"})
					return
				}

				_, err = tx.Exec(`
					INSERT INTO influence_transactions (player_id, amount, transaction_type, reference_id, reference_type, description)
					VALUES ($1, $2, 'contract', $3, 'contract', $4)
				`, *playerID, -influencePenalty, contractID, "Faction conflict penalty")
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record influence penalty"})
					return
				}
			}

			// Записываем штраф
			_, err = tx.Exec(`
				INSERT INTO contract_penalties (player_id, contract_id, violation_type, money_penalty, influence_penalty)
				VALUES ($1, $2, 'faction_conflict', $3, $4)
			`, *playerID, contractID, moneyPenalty, influencePenalty)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record penalty"})
				return
			}
		}
	}

	// Подписываем договор
	now := time.Now()
	expiresAt := now.Add(time.Duration(contract.DurationSeconds) * time.Second)

	_, err = tx.Exec(`
		UPDATE contracts
		SET status = 'signed',
		    signed_at = $1,
		    expires_at = $2,
		    customer_faction_id = $3
		WHERE id = $4
	`, now, expiresAt, customerFactionID, contractID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to sign contract"})
		return
	}

	// Фиксируем транзакцию
	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	// ВАЖНО: Создаём точный таймер для автоматического завершения
	h.scheduler.ScheduleContract(contractID, expiresAt)

	c.JSON(http.StatusOK, gin.H{
		"message":    "Contract signed successfully",
		"signed_at":  now,
		"expires_at": expiresAt,
	})
}

// CompleteContract - заказчик завершает договор вручную
func (h *ContractHandlerWithScheduler) CompleteContract(c *gin.Context) {
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

	contractIDStr := c.Param("id")
	contractID, err := strconv.Atoi(contractIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid contract ID"})
		return
	}

	// Начинаем транзакцию
	tx, err := h.db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
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
		ExpiresAt           *time.Time
		MoneyRewardCustomer int
		MoneyRewardExecutor int
	}

	err = tx.QueryRow(`
		SELECT status, contract_type, customer_player_id, executor_player_id, 
		       customer_faction_id, expires_at, money_reward_customer, money_reward_executor
		FROM contracts
		WHERE id = $1
		FOR UPDATE
	`, contractID).Scan(
		&contract.Status,
		&contract.ContractType,
		&contract.CustomerPlayerID,
		&contract.ExecutorPlayerID,
		&contract.CustomerFactionID,
		&contract.ExpiresAt,
		&contract.MoneyRewardCustomer,
		&contract.MoneyRewardExecutor,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Contract not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Проверяем, что пользователь - заказчик
	if contract.CustomerPlayerID != *playerID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only customer can complete the contract"})
		return
	}

	// Проверяем статус договора
	if contract.Status != "signed" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Contract is not in signed status"})
		return
	}

	// Проверяем, что срок истёк
	now := time.Now()
	if contract.ExpiresAt == nil || now.Before(*contract.ExpiresAt) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Contract has not expired yet"})
		return
	}

	// Выдаём награды (аналогично scheduler)
	if err := distributeRewards(tx, contractID, contract.ContractType, contract.CustomerPlayerID,
		contract.ExecutorPlayerID, contract.CustomerFactionID, contract.MoneyRewardCustomer,
		contract.MoneyRewardExecutor); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Обновляем статус договора
	_, err = tx.Exec(`
		UPDATE contracts
		SET status = 'completed', completed_at = $1
		WHERE id = $2
	`, now, contractID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to complete contract"})
		return
	}

	// Фиксируем транзакцию
	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	// ВАЖНО: Отменяем таймер, так как договор завершён вручную
	h.scheduler.CancelContract(contractID)

	c.JSON(http.StatusOK, gin.H{
		"message": "Contract completed successfully",
	})
}

// TerminateContract - админ расторгает договор
func (h *ContractHandlerWithScheduler) TerminateContract(c *gin.Context) {
	contractIDStr := c.Param("id")
	contractID, err := strconv.Atoi(contractIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid contract ID"})
		return
	}

	var req models.TerminateContractRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Разрешаем пустое тело запроса
		req.Reason = nil
	}

	// Начинаем транзакцию
	tx, err := h.db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	// Проверяем существование и статус договора
	var status string
	err = tx.QueryRow(`
		SELECT status FROM contracts WHERE id = $1 FOR UPDATE
	`, contractID).Scan(&status)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Contract not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Можно расторгнуть только pending или signed договоры
	if status != "pending" && status != "signed" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Contract cannot be terminated"})
		return
	}

	now := time.Now()
	reason := "Terminated by admin"
	if req.Reason != nil {
		reason = *req.Reason
	}

	// Расторгаем договор
	_, err = tx.Exec(`
		UPDATE contracts
		SET status = 'terminated', terminated_at = $1
		WHERE id = $2
	`, now, contractID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to terminate contract"})
		return
	}

	// Записываем причину расторжения (можно добавить отдельное поле в БД)
	_, err = tx.Exec(`
		INSERT INTO money_transactions (amount, transaction_type, reference_id, reference_type, description)
		VALUES (0, 'contract', $1, 'contract', $2)
	`, contractID, reason)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record termination"})
		return
	}

	// Фиксируем транзакцию
	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	// ВАЖНО: Отменяем таймер
	h.scheduler.CancelContract(contractID)

	c.JSON(http.StatusOK, gin.H{
		"message": "Contract terminated successfully",
		"reason":  reason,
	})
}

// distributeRewards - общая функция выдачи наград
func distributeRewards(tx *sql.Tx, contractID int, contractType string,
	customerPlayerID, executorPlayerID int, customerFactionID *int,
	moneyRewardCustomer, moneyRewardExecutor int) error {

	if contractType == "type1" {
		// Type 1: заказчик получает деньги + предмет, исполнитель получает деньги

		// Даём деньги заказчику
		if moneyRewardCustomer > 0 {
			_, err := tx.Exec(`
				UPDATE players SET money = money + $1 WHERE id = $2
			`, moneyRewardCustomer, customerPlayerID)
			if err != nil {
				return fmt.Errorf("failed to give money to customer: %w", err)
			}

			_, err = tx.Exec(`
				INSERT INTO money_transactions (to_player_id, amount, transaction_type, reference_id, reference_type, description)
				VALUES ($1, $2, 'contract', $3, 'contract', $4)
			`, customerPlayerID, moneyRewardCustomer, contractID,
				fmt.Sprintf("Contract %d completion reward", contractID))
			if err != nil {
				return fmt.Errorf("failed to record customer money transaction: %w", err)
			}
		}

		// Даём деньги исполнителю
		if moneyRewardExecutor > 0 {
			_, err := tx.Exec(`
				UPDATE players SET money = money + $1 WHERE id = $2
			`, moneyRewardExecutor, executorPlayerID)
			if err != nil {
				return fmt.Errorf("failed to give money to executor: %w", err)
			}

			_, err = tx.Exec(`
				INSERT INTO money_transactions (to_player_id, amount, transaction_type, reference_id, reference_type, description)
				VALUES ($1, $2, 'contract', $3, 'contract', $4)
			`, executorPlayerID, moneyRewardExecutor, contractID,
				fmt.Sprintf("Contract %d completion reward", contractID))
			if err != nil {
				return fmt.Errorf("failed to record executor money transaction: %w", err)
			}
		}

		// Даём предмет заказчику (если у него есть фракция)
		if customerFactionID != nil {
			var itemID *int
			err := tx.QueryRow(`
				SELECT customer_item_reward_id
				FROM contract_type1_settings
				WHERE faction_id = $1
			`, *customerFactionID).Scan(&itemID)

			if err != nil && err != sql.ErrNoRows {
				return fmt.Errorf("failed to fetch item reward settings: %w", err)
			}

			if itemID != nil && *itemID > 0 {
				_, err = tx.Exec(`
					INSERT INTO player_items (player_id, item_id)
					VALUES ($1, $2)
					ON CONFLICT (player_id, item_id) DO NOTHING
				`, customerPlayerID, *itemID)
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
				`, customerPlayerID, *itemID)
				if err != nil {
					return fmt.Errorf("failed to initialize item effect timers: %w", err)
				}

				_, err = tx.Exec(`
					INSERT INTO item_transactions (to_player_id, item_id, transaction_type, reference_id, reference_type, description)
					VALUES ($1, $2, 'contract', $3, 'contract', $4)
				`, customerPlayerID, *itemID, contractID,
					fmt.Sprintf("Contract %d completion reward", contractID))
				if err != nil {
					return fmt.Errorf("failed to record item transaction: %w", err)
				}
			}
		}

	} else if contractType == "type2" {
		// Type 2: исполнитель получает деньги

		if moneyRewardExecutor > 0 {
			_, err := tx.Exec(`
				UPDATE players SET money = money + $1 WHERE id = $2
			`, moneyRewardExecutor, executorPlayerID)
			if err != nil {
				return fmt.Errorf("failed to give money to executor: %w", err)
			}

			_, err = tx.Exec(`
				INSERT INTO money_transactions (to_player_id, amount, transaction_type, reference_id, reference_type, description)
				VALUES ($1, $2, 'contract', $3, 'contract', $4)
			`, executorPlayerID, moneyRewardExecutor, contractID,
				fmt.Sprintf("Contract %d completion reward", contractID))
			if err != nil {
				return fmt.Errorf("failed to record executor money transaction: %w", err)
			}
		}
	}

	return nil
}
