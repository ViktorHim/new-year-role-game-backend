// internal/handlers/contract_reveal.go
package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"new-year-role-game-backend/internal/models"
	"strconv"

	"github.com/gin-gonic/gin"
)

// RevealContractInfo раскрывает факт о заказчике для договора type2
func (h *ContractHandlerWithScheduler) RevealContractInfo(c *gin.Context) {
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

	// Парсим тело запроса
	var req models.RevealContractInfoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Проверяем категорию
	if req.InfoCategory != "faction" && req.InfoCategory != "goal" && req.InfoCategory != "item" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "info_category must be 'faction', 'goal', or 'item'"})
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
		ContractType     string
		CustomerPlayerID int
		ExecutorPlayerID int
	}

	err = tx.QueryRow(`
		SELECT status, contract_type, customer_player_id, executor_player_id
		FROM contracts
		WHERE id = $1
		FOR UPDATE
	`, contractID).Scan(
		&contract.Status,
		&contract.ContractType,
		&contract.CustomerPlayerID,
		&contract.ExecutorPlayerID,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Contract not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Проверяем, что это договор type2
	if contract.ContractType != "type2" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only type2 contracts allow info reveal"})
		return
	}

	// Проверяем, что пользователь - исполнитель
	if contract.ExecutorPlayerID != *playerID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only executor can reveal contract info"})
		return
	}

	// Проверяем статус договора (должен быть completed)
	if contract.Status != "completed" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Contract must be completed to reveal info"})
		return
	}

	// Проверяем, что факт еще не раскрыт для этого договора
	var alreadyRevealed bool
	err = tx.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM revealed_info 
			WHERE contract_id = $1
		)
	`, contractID).Scan(&alreadyRevealed)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check revealed info"})
		return
	}

	if alreadyRevealed {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Info has already been revealed for this contract"})
		return
	}

	// Раскрываем информацию
	revealedData, err := h.executeRevealContractInfo(
		tx,
		*playerID,
		contract.CustomerPlayerID,
		req.InfoCategory,
		contractID,
	)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Фиксируем транзакцию
	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	c.JSON(http.StatusOK, models.RevealContractInfoResponse{
		Message:      "Info revealed successfully",
		RevealedInfo: revealedData,
	})
}

// executeRevealContractInfo выполняет раскрытие информации о заказчике
func (h *ContractHandlerWithScheduler) executeRevealContractInfo(
	tx *sql.Tx,
	executorPlayerID,
	customerPlayerID int,
	infoCategory string,
	contractID int,
) (*models.ContractRevealedInfoData, error) {

	// Проверяем, что заказчик существует
	var customerExists bool
	err := tx.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM players WHERE id = $1)
	`, customerPlayerID).Scan(&customerExists)

	if err != nil || !customerExists {
		return nil, fmt.Errorf("Customer player not found")
	}

	// Раскрываем информацию в зависимости от категории
	var revealedData models.ContractRevealedInfoData
	var revealedJSON []byte

	switch infoCategory {
	case "faction":
		var factionID *int
		var factionName *string
		err = tx.QueryRow(`
			SELECT p.faction_id, f.name
			FROM players p
			LEFT JOIN factions f ON p.faction_id = f.id
			WHERE p.id = $1
		`, customerPlayerID).Scan(&factionID, &factionName)

		if err != nil {
			return nil, fmt.Errorf("Failed to fetch faction info")
		}

		revealedData.InfoType = "faction"
		if factionID != nil && factionName != nil {
			revealedData.Data = map[string]interface{}{
				"faction_id":   *factionID,
				"faction_name": *factionName,
			}
		} else {
			revealedData.Data = map[string]interface{}{
				"faction_id":   nil,
				"faction_name": "Нейтральный",
			}
		}

	case "goal":
		// Выбираем случайную личную цель заказчика
		var goalID int
		var goalTitle string
		var goalDescription *string
		err = tx.QueryRow(`
			SELECT id, title, description
			FROM goals
			WHERE player_id = $1 AND goal_type = 'personal'
			ORDER BY RANDOM()
			LIMIT 1
		`, customerPlayerID).Scan(&goalID, &goalTitle, &goalDescription)

		if err != nil {
			if err == sql.ErrNoRows {
				return nil, fmt.Errorf("Customer has no personal goals")
			}
			return nil, fmt.Errorf("Failed to fetch goal info")
		}

		revealedData.InfoType = "goal"
		revealedData.Data = map[string]interface{}{
			"goal_id":          goalID,
			"goal_title":       goalTitle,
			"goal_description": goalDescription,
		}

	case "item":
		// Выбираем случайный предмет заказчика
		var itemID int
		var itemName string
		var itemDescription *string
		err = tx.QueryRow(`
			SELECT i.id, i.name, i.description
			FROM player_items pi
			JOIN items i ON pi.item_id = i.id
			WHERE pi.player_id = $1
			ORDER BY RANDOM()
			LIMIT 1
		`, customerPlayerID).Scan(&itemID, &itemName, &itemDescription)

		if err != nil {
			if err == sql.ErrNoRows {
				return nil, fmt.Errorf("Customer has no items")
			}
			return nil, fmt.Errorf("Failed to fetch item info")
		}

		revealedData.InfoType = "item"
		revealedData.Data = map[string]interface{}{
			"item_id":          itemID,
			"item_name":        itemName,
			"item_description": itemDescription,
		}

	default:
		return nil, fmt.Errorf("Invalid info_category. Must be: faction, goal, or item")
	}

	// Сериализуем данные в JSON для сохранения в БД
	revealedJSON, err = json.Marshal(revealedData.Data)
	if err != nil {
		return nil, fmt.Errorf("Failed to serialize revealed data")
	}

	// Сохраняем раскрытую информацию
	_, err = tx.Exec(`
		INSERT INTO revealed_info (revealer_player_id, target_player_id, info_type, revealed_data, contract_id)
		VALUES ($1, $2, $3, $4, $5)
	`, executorPlayerID, customerPlayerID, infoCategory, revealedJSON, contractID)

	if err != nil {
		return nil, fmt.Errorf("Failed to save revealed info")
	}

	return &revealedData, nil
}
