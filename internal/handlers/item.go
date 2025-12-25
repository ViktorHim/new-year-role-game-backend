// internal/handlers/item.go
package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"new-year-role-game-backend/internal/models"
	"time"

	"github.com/gin-gonic/gin"
)

type ItemHandler struct {
	db *sql.DB
}

func NewItemHandler(db *sql.DB) *ItemHandler {
	return &ItemHandler{db: db}
}

// GetPlayerInventory возвращает инвентарь игрока
func (h *ItemHandler) GetPlayerInventory(c *gin.Context) {
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

	// Получаем предметы игрока
	rows, err := h.db.Query(`
		SELECT 
			i.id,
			i.name,
			i.description,
			pi.acquired_at
		FROM player_items pi
		JOIN items i ON pi.item_id = i.id
		WHERE pi.player_id = $1
		ORDER BY pi.acquired_at DESC
	`, *playerID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch inventory"})
		return
	}
	defer rows.Close()

	items := make([]models.Item, 0)
	for rows.Next() {
		var item models.Item
		err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.Description,
			&item.AcquiredAt,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan item"})
			return
		}

		// Получаем эффекты для каждого предмета
		effects, err := h.getItemEffects(item.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch item effects"})
			return
		}
		item.Effects = effects

		items = append(items, item)
	}

	if err = rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, models.InventoryResponse{Items: items})
}

// getItemEffects - вспомогательная функция для получения эффектов предмета
func (h *ItemHandler) getItemEffects(itemID int) ([]models.Effect, error) {
	rows, err := h.db.Query(`
		SELECT 
			e.id,
			e.description,
			e.effect_type,
			e.generated_resource,
			e.operation,
			e.value,
			e.spawned_item_id,
			e.period_seconds
		FROM item_effects ie
		JOIN effects e ON ie.effect_id = e.id
		WHERE ie.item_id = $1
		ORDER BY e.id
	`, itemID)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	effects := make([]models.Effect, 0)
	for rows.Next() {
		var effect models.Effect
		err := rows.Scan(
			&effect.ID,
			&effect.Description,
			&effect.EffectType,
			&effect.GeneratedResource,
			&effect.Operation,
			&effect.Value,
			&effect.SpawnedItemID,
			&effect.PeriodSeconds,
		)
		if err != nil {
			return nil, err
		}
		effects = append(effects, effect)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return effects, nil
}

// TransferItem передает предмет другому игроку
func (h *ItemHandler) TransferItem(c *gin.Context) {
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

	var req models.TransferItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Проверяем, что не пытаемся передать себе
	if req.ToPlayerID == *playerID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot transfer item to yourself"})
		return
	}

	// Начинаем транзакцию
	tx, err := h.db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	// Проверяем, что получатель существует
	var recipientExists bool
	err = tx.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM players WHERE id = $1)
	`, req.ToPlayerID).Scan(&recipientExists)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if !recipientExists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Recipient player not found"})
		return
	}

	// Проверяем, что у игрока есть этот предмет
	var hasItem bool
	err = tx.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM player_items 
			WHERE player_id = $1 AND item_id = $2
		)
	`, *playerID, req.ItemID).Scan(&hasItem)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if !hasItem {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found in your inventory"})
		return
	}

	// Получаем название предмета для описания транзакции
	var itemName string
	err = tx.QueryRow(`
		SELECT name FROM items WHERE id = $1
	`, req.ItemID).Scan(&itemName)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch item info"})
		return
	}

	// Удаляем предмет у отправителя
	_, err = tx.Exec(`
		DELETE FROM player_items 
		WHERE player_id = $1 AND item_id = $2
	`, *playerID, req.ItemID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove item from inventory"})
		return
	}

	// Добавляем предмет получателю
	_, err = tx.Exec(`
		INSERT INTO player_items (player_id, item_id)
		VALUES ($1, $2)
		ON CONFLICT (player_id, item_id) DO NOTHING
	`, req.ToPlayerID, req.ItemID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add item to recipient"})
		return
	}

	// УСТАНОВКА ТАЙМЕРОВ: Инициализируем таймеры эффектов для нового владельца
	// Устанавливаем last_executed_at = NOW(), чтобы новый владелец мог использовать
	// эффекты только через period_seconds (не сразу)
	now := time.Now()
	_, err = tx.Exec(`
		INSERT INTO item_effect_executions (player_id, item_id, effect_id, last_executed_at)
		SELECT 
			$1,
			ie.item_id,
			ie.effect_id,
			$2
		FROM item_effects ie
		WHERE ie.item_id = $3
		ON CONFLICT (player_id, item_id, effect_id) 
		DO UPDATE SET last_executed_at = $2
	`, req.ToPlayerID, now, req.ItemID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize effect timers for recipient"})
		return
	}

	// Записываем транзакцию
	_, err = tx.Exec(`
		INSERT INTO item_transactions (from_player_id, to_player_id, item_id, transaction_type, description)
		VALUES ($1, $2, $3, 'transfer', $4)
	`, *playerID, req.ToPlayerID, req.ItemID, "Item transfer: "+itemName)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record transaction"})
		return
	}

	// Фиксируем транзакцию
	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Item transferred successfully",
		"item_id":      req.ItemID,
		"to_player_id": req.ToPlayerID,
	})
}

// TransferMoney переводит деньги другому игроку
func (h *ItemHandler) TransferMoney(c *gin.Context) {
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

	var req models.TransferMoneyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Проверяем, что не пытаемся перевести себе
	if req.ToPlayerID == *playerID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot transfer money to yourself"})
		return
	}

	// Начинаем транзакцию
	tx, err := h.db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	// Проверяем баланс отправителя
	var senderMoney int
	var senderName string
	err = tx.QueryRow(`
		SELECT money, character_name 
		FROM players 
		WHERE id = $1
		FOR UPDATE
	`, *playerID).Scan(&senderMoney, &senderName)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if senderMoney < req.Amount {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Insufficient funds"})
		return
	}

	// Проверяем, что получатель существует и блокируем его запись
	var recipientExists bool
	var recipientName string
	err = tx.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM players WHERE id = $1), 
		       COALESCE((SELECT character_name FROM players WHERE id = $1), '')
		FROM players WHERE id = $2
		FOR UPDATE
	`, req.ToPlayerID, req.ToPlayerID).Scan(&recipientExists, &recipientName)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if !recipientExists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Recipient player not found"})
		return
	}

	// Снимаем деньги у отправителя
	_, err = tx.Exec(`
		UPDATE players
		SET money = money - $1
		WHERE id = $2
	`, req.Amount, *playerID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to deduct money"})
		return
	}

	// Добавляем деньги получателю
	_, err = tx.Exec(`
		UPDATE players
		SET money = money + $1
		WHERE id = $2
	`, req.Amount, req.ToPlayerID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add money to recipient"})
		return
	}

	// Записываем транзакцию
	description := fmt.Sprintf("%s transferred %d money to %s", senderName, req.Amount, recipientName)
	_, err = tx.Exec(`
		INSERT INTO money_transactions (from_player_id, to_player_id, amount, transaction_type, description)
		VALUES ($1, $2, $3, 'transfer', $4)
	`, *playerID, req.ToPlayerID, req.Amount, description)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record transaction"})
		return
	}

	// Фиксируем транзакцию
	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	// Получаем обновленный баланс
	var newBalance int
	err = h.db.QueryRow(`
		SELECT money FROM players WHERE id = $1
	`, *playerID).Scan(&newBalance)

	if err != nil {
		// Транзакция уже зафиксирована, просто не возвращаем новый баланс
		c.JSON(http.StatusOK, gin.H{
			"message":      "Money transferred successfully",
			"amount":       req.Amount,
			"to_player_id": req.ToPlayerID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Money transferred successfully",
		"amount":       req.Amount,
		"to_player_id": req.ToPlayerID,
		"new_balance":  newBalance,
	})
}

// GetItemEffectsStatus возвращает статус всех эффектов предметов игрока
func (h *ItemHandler) GetItemEffectsStatus(c *gin.Context) {
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

	rows, err := h.db.Query(`
		SELECT 
			i.id,
			i.name,
			e.id,
			e.description,
			e.effect_type,
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
		WHERE pi.player_id = $1
		ORDER BY i.id, e.id
	`, *playerID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch effects status"})
		return
	}
	defer rows.Close()

	effects := make([]models.EffectStatus, 0)
	now := time.Now()

	for rows.Next() {
		var status models.EffectStatus

		err := rows.Scan(
			&status.ItemID,
			&status.ItemName,
			&status.EffectID,
			&status.EffectDescription,
			&status.EffectType,
			&status.PeriodSeconds,
			&status.LastExecutedAt,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan effect status"})
			return
		}

		// Вычисляем, можно ли выполнить эффект сейчас
		if status.LastExecutedAt == nil {
			status.CanExecuteNow = true
		} else {
			nextExecution := status.LastExecutedAt.Add(time.Duration(status.PeriodSeconds) * time.Second)
			status.NextAvailableAt = &nextExecution
			status.CanExecuteNow = now.After(nextExecution) || now.Equal(nextExecution)
		}

		effects = append(effects, status)
	}

	if err = rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, models.ItemEffectsStatusResponse{Effects: effects})
}

// calculateEffectValue вычисляет значение эффекта с учетом операции
func (h *ItemHandler) calculateEffectValue(value int, operation string) int {
	// Пока что поддерживаем только операцию 'add'
	// В будущем можно добавить 'mul', 'sub', 'div' с базовым значением
	switch operation {
	case "add":
		return value
	case "mul":
		// Для умножения нужно базовое значение, пока возвращаем как есть
		return value
	case "sub":
		return -value
	case "div":
		// Для деления нужно базовое значение, пока возвращаем как есть
		return value
	default:
		return value
	}
}
