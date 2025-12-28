// internal/handlers/admin_relations.go
package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type AdminRelationsHandler struct {
	db *sql.DB
}

func NewAdminRelationsHandler(db *sql.DB) *AdminRelationsHandler {
	return &AdminRelationsHandler{db: db}
}

// ============================================
// ИНВЕНТАРЬ ИГРОКОВ (player_items)
// ============================================

type PlayerInventoryItem struct {
	ID          int       `json:"id"`
	ItemID      int       `json:"item_id"`
	ItemName    string    `json:"item_name"`
	Description string    `json:"description"`
	AcquiredAt  time.Time `json:"acquired_at"`
}

// GetPlayerInventory получает инвентарь игрока
func (h *AdminRelationsHandler) GetPlayerInventory(c *gin.Context) {
	playerIDStr := c.Param("id")
	playerID, err := strconv.Atoi(playerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid player ID"})
		return
	}

	rows, err := h.db.Query(`
		SELECT 
			pi.id, pi.item_id, i.name as item_name, i.description, pi.acquired_at
		FROM player_items pi
		JOIN items i ON pi.item_id = i.id
		WHERE pi.player_id = $1
		ORDER BY pi.acquired_at DESC
	`, playerID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch inventory"})
		return
	}
	defer rows.Close()

	items := make([]PlayerInventoryItem, 0)
	for rows.Next() {
		var item PlayerInventoryItem
		err := rows.Scan(
			&item.ID,
			&item.ItemID,
			&item.ItemName,
			&item.Description,
			&item.AcquiredAt,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan item"})
			return
		}

		items = append(items, item)
	}

	c.JSON(http.StatusOK, gin.H{
		"player_id": playerID,
		"items":     items,
		"count":     len(items),
	})
}

// AddItemToPlayer добавляет предмет в инвентарь игрока
func (h *AdminRelationsHandler) AddItemToPlayer(c *gin.Context) {
	playerIDStr := c.Param("id")
	playerID, err := strconv.Atoi(playerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid player ID"})
		return
	}

	var req struct {
		ItemID int `json:"item_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Проверяем, что предмет существует
	var itemExists bool
	err = h.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM items WHERE id = $1)`, req.ItemID).Scan(&itemExists)
	if err != nil || !itemExists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Item not found"})
		return
	}

	// Проверяем, что игрок существует
	var playerExists bool
	err = h.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM players WHERE id = $1)`, playerID).Scan(&playerExists)
	if err != nil || !playerExists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Player not found"})
		return
	}

	tx, err := h.db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	// Добавляем предмет в инвентарь
	var inventoryID int
	err = tx.QueryRow(`
		INSERT INTO player_items (player_id, item_id)
		VALUES ($1, $2)
		ON CONFLICT (player_id, item_id) DO NOTHING
		RETURNING id
	`, playerID, req.ItemID).Scan(&inventoryID)

	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Item already in player's inventory or failed to add"})
		return
	}

	// Инициализируем таймеры эффектов для этого предмета
	_, err = tx.Exec(`
		INSERT INTO item_effect_executions (player_id, item_id, effect_id, last_executed_at)
		SELECT $1, $2, e.id, NOW()
		FROM item_effects ie
		JOIN effects e ON ie.effect_id = e.id
		WHERE ie.item_id = $2
		ON CONFLICT (player_id, item_id, effect_id) DO NOTHING
	`, playerID, req.ItemID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize effect timers"})
		return
	}

	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":      "Item added to player's inventory",
		"inventory_id": inventoryID,
	})
}

// RemoveItemFromPlayer удаляет предмет из инвентаря игрока
func (h *AdminRelationsHandler) RemoveItemFromPlayer(c *gin.Context) {
	playerIDStr := c.Param("id")
	playerID, err := strconv.Atoi(playerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid player ID"})
		return
	}

	itemIDStr := c.Param("item_id")
	itemID, err := strconv.Atoi(itemIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid item ID"})
		return
	}

	result, err := h.db.Exec(`
		DELETE FROM player_items
		WHERE player_id = $1 AND item_id = $2
	`, playerID, itemID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove item"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found in player's inventory"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Item removed from player's inventory"})
}

// AddBatchItemsToPlayer массовое добавление предметов игроку
func (h *AdminRelationsHandler) AddBatchItemsToPlayer(c *gin.Context) {
	playerIDStr := c.Param("id")
	playerID, err := strconv.Atoi(playerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid player ID"})
		return
	}

	var req struct {
		ItemIDs []int `json:"item_ids" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	tx, err := h.db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	addedCount := 0
	for _, itemID := range req.ItemIDs {
		var inventoryID int
		err = tx.QueryRow(`
			INSERT INTO player_items (player_id, item_id)
			VALUES ($1, $2)
			ON CONFLICT (player_id, item_id) DO NOTHING
			RETURNING id
		`, playerID, itemID).Scan(&inventoryID)

		if err == nil {
			// Инициализируем таймеры эффектов
			tx.Exec(`
				INSERT INTO item_effect_executions (player_id, item_id, effect_id, last_executed_at)
				SELECT $1, $2, e.id, NOW()
				FROM item_effects ie
				JOIN effects e ON ie.effect_id = e.id
				WHERE ie.item_id = $2
				ON CONFLICT (player_id, item_id, effect_id) DO NOTHING
			`, playerID, itemID)
			addedCount++
		}
	}

	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":     "Items added to player's inventory",
		"added_count": addedCount,
		"total_sent":  len(req.ItemIDs),
	})
}

// ============================================
// ЗАВИСИМОСТИ ЦЕЛЕЙ (goal_dependencies)
// ============================================

type GoalDependency struct {
	ID                        int       `json:"id"`
	GoalID                    int       `json:"goal_id"`
	DependencyType            string    `json:"dependency_type"`
	RequiredGoalID            *int      `json:"required_goal_id,omitempty"`
	RequiredGoalTitle         *string   `json:"required_goal_title,omitempty"`
	InfluencePlayerID         *int      `json:"influence_player_id,omitempty"`
	InfluencePlayerName       *string   `json:"influence_player_name,omitempty"`
	RequiredInfluencePoints   *int      `json:"required_influence_points,omitempty"`
	IsVisibleBeforeCompletion bool      `json:"is_visible_before_completion"`
	CreatedAt                 time.Time `json:"created_at"`
}

// GetGoalDependencies получает зависимости цели
func (h *AdminRelationsHandler) GetGoalDependencies(c *gin.Context) {
	goalIDStr := c.Param("id")
	goalID, err := strconv.Atoi(goalIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid goal ID"})
		return
	}

	rows, err := h.db.Query(`
		SELECT 
			gd.id, gd.goal_id, gd.dependency_type, gd.required_goal_id, g.title as required_goal_title,
			gd.influence_player_id, p.character_name as influence_player_name, 
			gd.required_influence_points, gd.is_visible_before_completion, gd.created_at
		FROM goal_dependencies gd
		LEFT JOIN goals g ON gd.required_goal_id = g.id
		LEFT JOIN players p ON gd.influence_player_id = p.id
		WHERE gd.goal_id = $1
		ORDER BY gd.created_at
	`, goalID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch dependencies"})
		return
	}
	defer rows.Close()

	dependencies := make([]GoalDependency, 0)
	for rows.Next() {
		var dep GoalDependency
		err := rows.Scan(
			&dep.ID,
			&dep.GoalID,
			&dep.DependencyType,
			&dep.RequiredGoalID,
			&dep.RequiredGoalTitle,
			&dep.InfluencePlayerID,
			&dep.InfluencePlayerName,
			&dep.RequiredInfluencePoints,
			&dep.IsVisibleBeforeCompletion,
			&dep.CreatedAt,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan dependency"})
			return
		}

		dependencies = append(dependencies, dep)
	}

	c.JSON(http.StatusOK, gin.H{
		"goal_id":      goalID,
		"dependencies": dependencies,
		"count":        len(dependencies),
	})
}

// AddGoalDependency добавляет зависимость к цели
func (h *AdminRelationsHandler) AddGoalDependency(c *gin.Context) {
	goalIDStr := c.Param("id")
	goalID, err := strconv.Atoi(goalIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid goal ID"})
		return
	}

	var req struct {
		DependencyType            string `json:"dependency_type" binding:"required,oneof=goal_completion influence_threshold"`
		RequiredGoalID            *int   `json:"required_goal_id"`
		InfluencePlayerID         *int   `json:"influence_player_id"`
		RequiredInfluencePoints   *int   `json:"required_influence_points"`
		IsVisibleBeforeCompletion bool   `json:"is_visible_before_completion"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Валидация в зависимости от типа
	if req.DependencyType == "goal_completion" && req.RequiredGoalID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "required_goal_id is required for goal_completion type"})
		return
	}
	if req.DependencyType == "influence_threshold" &&
		(req.InfluencePlayerID == nil || req.RequiredInfluencePoints == nil) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "influence_player_id and required_influence_points are required for influence_threshold type"})
		return
	}

	var dependencyID int
	err = h.db.QueryRow(`
		INSERT INTO goal_dependencies (
			goal_id, dependency_type, required_goal_id, influence_player_id,
			required_influence_points, is_visible_before_completion
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`, goalID, req.DependencyType, req.RequiredGoalID, req.InfluencePlayerID,
		req.RequiredInfluencePoints, req.IsVisibleBeforeCompletion).Scan(&dependencyID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add dependency"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":       "Dependency added successfully",
		"dependency_id": dependencyID,
	})
}

// DeleteGoalDependency удаляет зависимость цели
func (h *AdminRelationsHandler) DeleteGoalDependency(c *gin.Context) {
	goalIDStr := c.Param("id")
	_, err := strconv.Atoi(goalIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid goal ID"})
		return
	}

	dependencyIDStr := c.Param("dependency_id")
	dependencyID, err := strconv.Atoi(dependencyIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid dependency ID"})
		return
	}

	result, err := h.db.Exec(`
		DELETE FROM goal_dependencies
		WHERE id = $1
	`, dependencyID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete dependency"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Dependency not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Dependency deleted successfully"})
}

// GetGoalDependencyUnlocks получает историю разблокировок зависимостей цели
func (h *AdminRelationsHandler) GetGoalDependencyUnlocks(c *gin.Context) {
	goalIDStr := c.Param("id")
	goalID, err := strconv.Atoi(goalIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid goal ID"})
		return
	}

	rows, err := h.db.Query(`
		SELECT 
			gdu.id, gdu.dependency_id, gd.dependency_type, gdu.player_id, 
			p.character_name as player_name, gdu.unlocked_at
		FROM goal_dependency_unlocks gdu
		JOIN goal_dependencies gd ON gdu.dependency_id = gd.id
		JOIN players p ON gdu.player_id = p.id
		WHERE gdu.goal_id = $1
		ORDER BY gdu.unlocked_at DESC
	`, goalID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch unlocks"})
		return
	}
	defer rows.Close()

	type Unlock struct {
		ID             int       `json:"id"`
		DependencyID   int       `json:"dependency_id"`
		DependencyType string    `json:"dependency_type"`
		PlayerID       int       `json:"player_id"`
		PlayerName     string    `json:"player_name"`
		UnlockedAt     time.Time `json:"unlocked_at"`
	}

	unlocks := make([]Unlock, 0)
	for rows.Next() {
		var unlock Unlock
		err := rows.Scan(
			&unlock.ID,
			&unlock.DependencyID,
			&unlock.DependencyType,
			&unlock.PlayerID,
			&unlock.PlayerName,
			&unlock.UnlockedAt,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan unlock"})
			return
		}

		unlocks = append(unlocks, unlock)
	}

	c.JSON(http.StatusOK, gin.H{
		"goal_id": goalID,
		"unlocks": unlocks,
		"count":   len(unlocks),
	})
}

// ============================================
// ИНФОРМАЦИЯ О ДРУГИХ ИГРОКАХ (info_about_other_players)
// ============================================

type PlayerInfo struct {
	ID          int    `json:"id"`
	PlayerID    int    `json:"player_id"`
	PlayerName  string `json:"player_name"`
	Description string `json:"description"`
}

// GetPlayerInfo получает информацию о другом игроке
func (h *AdminRelationsHandler) GetPlayerInfo(c *gin.Context) {
	playerIDStr := c.Param("id")
	playerID, err := strconv.Atoi(playerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid player ID"})
		return
	}

	var info PlayerInfo
	err = h.db.QueryRow(`
		SELECT i.id, i.player_id, p.character_name as player_name, i.description
		FROM info_about_other_players i
		JOIN players p ON i.player_id = p.id
		WHERE i.player_id = $1
	`, playerID).Scan(&info.ID, &info.PlayerID, &info.PlayerName, &info.Description)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "No info found for this player"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch player info"})
		return
	}

	c.JSON(http.StatusOK, info)
}

// CreatePlayerInfo создает информацию о игроке
func (h *AdminRelationsHandler) CreatePlayerInfo(c *gin.Context) {
	playerIDStr := c.Param("id")
	playerID, err := strconv.Atoi(playerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid player ID"})
		return
	}

	var req struct {
		Description string `json:"description" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	var infoID int
	err = h.db.QueryRow(`
		INSERT INTO info_about_other_players (player_id, description)
		VALUES ($1, $2)
		RETURNING id
	`, playerID, req.Description).Scan(&infoID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create player info"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Player info created successfully",
		"info_id": infoID,
	})
}

// UpdatePlayerInfo обновляет информацию о игроке
func (h *AdminRelationsHandler) UpdatePlayerInfo(c *gin.Context) {
	playerIDStr := c.Param("id")
	playerID, err := strconv.Atoi(playerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid player ID"})
		return
	}

	var req struct {
		Description string `json:"description" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	result, err := h.db.Exec(`
		UPDATE info_about_other_players
		SET description = $1
		WHERE player_id = $2
	`, req.Description, playerID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update player info"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Player info not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Player info updated successfully"})
}

// DeletePlayerInfo удаляет информацию о игроке
func (h *AdminRelationsHandler) DeletePlayerInfo(c *gin.Context) {
	playerIDStr := c.Param("id")
	playerID, err := strconv.Atoi(playerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid player ID"})
		return
	}

	result, err := h.db.Exec(`
		DELETE FROM info_about_other_players
		WHERE player_id = $1
	`, playerID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete player info"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Player info not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Player info deleted successfully"})
}

// ============================================
// УЧАСТНИКИ ТРИГГЕРОВ ГОНКИ (goal_race_trigger_participants)
// ============================================

type TriggerParticipant struct {
	ID         int       `json:"id"`
	TriggerID  int       `json:"trigger_id"`
	PlayerID   int       `json:"player_id"`
	PlayerName string    `json:"player_name"`
	CreatedAt  time.Time `json:"created_at"`
}

// GetTriggerParticipants получает участников триггера
func (h *AdminRelationsHandler) GetTriggerParticipants(c *gin.Context) {
	triggerIDStr := c.Param("id")
	triggerID, err := strconv.Atoi(triggerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid trigger ID"})
		return
	}

	rows, err := h.db.Query(`
		SELECT 
			tp.id, tp.trigger_id, tp.player_id, p.character_name as player_name, tp.created_at
		FROM goal_race_trigger_participants tp
		JOIN players p ON tp.player_id = p.id
		WHERE tp.trigger_id = $1
		ORDER BY tp.created_at
	`, triggerID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch participants"})
		return
	}
	defer rows.Close()

	participants := make([]TriggerParticipant, 0)
	for rows.Next() {
		var participant TriggerParticipant
		err := rows.Scan(
			&participant.ID,
			&participant.TriggerID,
			&participant.PlayerID,
			&participant.PlayerName,
			&participant.CreatedAt,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan participant"})
			return
		}

		participants = append(participants, participant)
	}

	c.JSON(http.StatusOK, gin.H{
		"trigger_id":   triggerID,
		"participants": participants,
		"count":        len(participants),
	})
}

// AddTriggerParticipant добавляет участника к триггеру
func (h *AdminRelationsHandler) AddTriggerParticipant(c *gin.Context) {
	triggerIDStr := c.Param("id")
	triggerID, err := strconv.Atoi(triggerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid trigger ID"})
		return
	}

	var req struct {
		PlayerID int `json:"player_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	var participantID int
	err = h.db.QueryRow(`
		INSERT INTO goal_race_trigger_participants (trigger_id, player_id)
		VALUES ($1, $2)
		ON CONFLICT (trigger_id, player_id) DO NOTHING
		RETURNING id
	`, triggerID, req.PlayerID).Scan(&participantID)

	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Participant already added or failed to add"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":        "Participant added successfully",
		"participant_id": participantID,
	})
}

// RemoveTriggerParticipant удаляет участника из триггера
func (h *AdminRelationsHandler) RemoveTriggerParticipant(c *gin.Context) {
	triggerIDStr := c.Param("id")
	triggerID, err := strconv.Atoi(triggerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid trigger ID"})
		return
	}

	playerIDStr := c.Param("player_id")
	playerID, err := strconv.Atoi(playerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid player ID"})
		return
	}

	result, err := h.db.Exec(`
		DELETE FROM goal_race_trigger_participants
		WHERE trigger_id = $1 AND player_id = $2
	`, triggerID, playerID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove participant"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Participant not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Participant removed successfully"})
}

// AddBatchTriggerParticipants массовое добавление участников к триггеру
func (h *AdminRelationsHandler) AddBatchTriggerParticipants(c *gin.Context) {
	triggerIDStr := c.Param("id")
	triggerID, err := strconv.Atoi(triggerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid trigger ID"})
		return
	}

	var req struct {
		PlayerIDs []int `json:"player_ids" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	tx, err := h.db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	addedCount := 0
	for _, playerID := range req.PlayerIDs {
		var participantID int
		err = tx.QueryRow(`
			INSERT INTO goal_race_trigger_participants (trigger_id, player_id)
			VALUES ($1, $2)
			ON CONFLICT (trigger_id, player_id) DO NOTHING
			RETURNING id
		`, triggerID, playerID).Scan(&participantID)

		if err == nil {
			addedCount++
		}
	}

	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":     "Participants added successfully",
		"added_count": addedCount,
		"total_sent":  len(req.PlayerIDs),
	})
}

// ============================================
// ИСТОРИЯ И МОНИТОРИНГ
// ============================================

// GetAbilityUsageHistory получает историю использования всех способностей
func (h *AdminRelationsHandler) GetAbilityUsageHistory(c *gin.Context) {
	limit := 100
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 500 {
			limit = l
		}
	}

	playerIDStr := c.Query("player_id")
	abilityIDStr := c.Query("ability_id")

	query := `
		SELECT 
			au.id, au.player_id, p.character_name as player_name,
			au.ability_id, a.name as ability_name, a.ability_type,
			au.target_player_id, p2.character_name as target_player_name,
			au.info_category, au.used_at
		FROM ability_usage au
		JOIN players p ON au.player_id = p.id
		JOIN abilities a ON au.ability_id = a.id
		LEFT JOIN players p2 ON au.target_player_id = p2.id
	`

	where := make([]string, 0)
	args := make([]interface{}, 0)
	argNum := 1

	if playerIDStr != "" {
		where = append(where, "au.player_id = $"+strconv.Itoa(argNum))
		playerID, _ := strconv.Atoi(playerIDStr)
		args = append(args, playerID)
		argNum++
	}

	if abilityIDStr != "" {
		where = append(where, "au.ability_id = $"+strconv.Itoa(argNum))
		abilityID, _ := strconv.Atoi(abilityIDStr)
		args = append(args, abilityID)
		argNum++
	}

	if len(where) > 0 {
		query += " WHERE " + where[0]
		for i := 1; i < len(where); i++ {
			query += " AND " + where[i]
		}
	}

	query += " ORDER BY au.used_at DESC LIMIT $" + strconv.Itoa(argNum)
	args = append(args, limit)

	rows, err := h.db.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch usage history"})
		return
	}
	defer rows.Close()

	type UsageRecord struct {
		ID               int       `json:"id"`
		PlayerID         int       `json:"player_id"`
		PlayerName       string    `json:"player_name"`
		AbilityID        int       `json:"ability_id"`
		AbilityName      string    `json:"ability_name"`
		AbilityType      string    `json:"ability_type"`
		TargetPlayerID   *int      `json:"target_player_id,omitempty"`
		TargetPlayerName *string   `json:"target_player_name,omitempty"`
		InfoCategory     *string   `json:"info_category,omitempty"`
		UsedAt           time.Time `json:"used_at"`
	}

	records := make([]UsageRecord, 0)
	for rows.Next() {
		var record UsageRecord
		err := rows.Scan(
			&record.ID,
			&record.PlayerID,
			&record.PlayerName,
			&record.AbilityID,
			&record.AbilityName,
			&record.AbilityType,
			&record.TargetPlayerID,
			&record.TargetPlayerName,
			&record.InfoCategory,
			&record.UsedAt,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan record"})
			return
		}

		records = append(records, record)
	}

	c.JSON(http.StatusOK, gin.H{
		"usage_history": records,
		"count":         len(records),
	})
}

// GetPlayerRevealedInfo получает раскрытую информацию о игроке
func (h *AdminRelationsHandler) GetPlayerRevealedInfo(c *gin.Context) {
	playerIDStr := c.Param("id")
	playerID, err := strconv.Atoi(playerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid player ID"})
		return
	}

	rows, err := h.db.Query(`
		SELECT 
			ri.id, ri.revealer_player_id, p.character_name as revealer_name,
			ri.info_type, ri.revealed_data, ri.revealed_at
		FROM revealed_info ri
		JOIN players p ON ri.revealer_player_id = p.id
		WHERE ri.target_player_id = $1
		ORDER BY ri.revealed_at DESC
	`, playerID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch revealed info"})
		return
	}
	defer rows.Close()

	type RevealedInfo struct {
		ID               int       `json:"id"`
		RevealerPlayerID int       `json:"revealer_player_id"`
		RevealerName     string    `json:"revealer_name"`
		InfoType         string    `json:"info_type"`
		RevealedData     string    `json:"revealed_data"`
		RevealedAt       time.Time `json:"revealed_at"`
	}

	infos := make([]RevealedInfo, 0)
	for rows.Next() {
		var info RevealedInfo
		err := rows.Scan(
			&info.ID,
			&info.RevealerPlayerID,
			&info.RevealerName,
			&info.InfoType,
			&info.RevealedData,
			&info.RevealedAt,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan info"})
			return
		}

		infos = append(infos, info)
	}

	c.JSON(http.StatusOK, gin.H{
		"player_id":     playerID,
		"revealed_info": infos,
		"count":         len(infos),
	})
}

// GetRoundParticipants получает участников раунда гонки
func (h *AdminRelationsHandler) GetRoundParticipants(c *gin.Context) {
	roundIDStr := c.Param("id")
	roundID, err := strconv.Atoi(roundIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid round ID"})
		return
	}

	rows, err := h.db.Query(`
		SELECT 
			rp.id, rp.round_id, rp.player_id, p.character_name as player_name, rp.joined_at
		FROM goal_race_round_participants rp
		JOIN players p ON rp.player_id = p.id
		WHERE rp.round_id = $1
		ORDER BY rp.joined_at
	`, roundID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch participants"})
		return
	}
	defer rows.Close()

	type RoundParticipant struct {
		ID         int       `json:"id"`
		RoundID    int       `json:"round_id"`
		PlayerID   int       `json:"player_id"`
		PlayerName string    `json:"player_name"`
		JoinedAt   time.Time `json:"joined_at"`
	}

	participants := make([]RoundParticipant, 0)
	for rows.Next() {
		var participant RoundParticipant
		err := rows.Scan(
			&participant.ID,
			&participant.RoundID,
			&participant.PlayerID,
			&participant.PlayerName,
			&participant.JoinedAt,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan participant"})
			return
		}

		participants = append(participants, participant)
	}

	c.JSON(http.StatusOK, gin.H{
		"round_id":     roundID,
		"participants": participants,
		"count":        len(participants),
	})
}

// GetRoundGoals получает цели раунда гонки
func (h *AdminRelationsHandler) GetRoundGoals(c *gin.Context) {
	roundIDStr := c.Param("id")
	roundID, err := strconv.Atoi(roundIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid round ID"})
		return
	}

	rows, err := h.db.Query(`
		SELECT 
			rg.id, rg.round_id, rg.goal_id, g.title, g.description,
			rg.assigned_player_id, p.character_name as player_name,
			rg.is_accessible, g.is_completed, rg.assigned_at, rg.became_inaccessible_at
		FROM goal_race_round_goals rg
		JOIN goals g ON rg.goal_id = g.id
		JOIN players p ON rg.assigned_player_id = p.id
		WHERE rg.round_id = $1
		ORDER BY rg.assigned_at
	`, roundID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch goals"})
		return
	}
	defer rows.Close()

	type RoundGoal struct {
		ID                   int        `json:"id"`
		RoundID              int        `json:"round_id"`
		GoalID               int        `json:"goal_id"`
		Title                string     `json:"title"`
		Description          string     `json:"description"`
		AssignedPlayerID     int        `json:"assigned_player_id"`
		PlayerName           string     `json:"player_name"`
		IsAccessible         bool       `json:"is_accessible"`
		IsCompleted          bool       `json:"is_completed"`
		AssignedAt           time.Time  `json:"assigned_at"`
		BecameInaccessibleAt *time.Time `json:"became_inaccessible_at,omitempty"`
	}

	goals := make([]RoundGoal, 0)
	for rows.Next() {
		var goal RoundGoal
		err := rows.Scan(
			&goal.ID,
			&goal.RoundID,
			&goal.GoalID,
			&goal.Title,
			&goal.Description,
			&goal.AssignedPlayerID,
			&goal.PlayerName,
			&goal.IsAccessible,
			&goal.IsCompleted,
			&goal.AssignedAt,
			&goal.BecameInaccessibleAt,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan goal"})
			return
		}

		goals = append(goals, goal)
	}

	c.JSON(http.StatusOK, gin.H{
		"round_id": roundID,
		"goals":    goals,
		"count":    len(goals),
	})
}
