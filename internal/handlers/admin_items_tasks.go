// internal/handlers/admin_items_tasks.go
package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type AdminItemsTasksHandler struct {
	db *sql.DB
}

func NewAdminItemsTasksHandler(db *sql.DB) *AdminItemsTasksHandler {
	return &AdminItemsTasksHandler{db: db}
}

// ============================================
// ЗАДАЧИ (TASKS)
// ============================================

type TaskRequest struct {
	PlayerID    int    `json:"player_id" binding:"required"`
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
}

type TaskResponse struct {
	ID          int        `json:"id"`
	PlayerID    int        `json:"player_id"`
	PlayerName  string     `json:"player_name"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	IsCompleted bool       `json:"is_completed"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// GetAllTasks возвращает все задачи
func (h *AdminItemsTasksHandler) GetAllTasks(c *gin.Context) {
	playerIDStr := c.Query("player_id")

	query := `
		SELECT 
			t.id, t.player_id, p.character_name as player_name,
			t.title, t.description, t.is_completed, t.completed_at, t.created_at
		FROM tasks t
		JOIN players p ON t.player_id = p.id
	`

	var rows *sql.Rows
	var err error

	if playerIDStr != "" {
		playerID, err := strconv.Atoi(playerIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid player_id"})
			return
		}
		query += " WHERE t.player_id = $1 ORDER BY t.created_at DESC"
		rows, err = h.db.Query(query, playerID)
	} else {
		query += " ORDER BY t.created_at DESC"
		rows, err = h.db.Query(query)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tasks"})
		return
	}
	defer rows.Close()

	tasks := make([]TaskResponse, 0)
	for rows.Next() {
		var task TaskResponse
		err := rows.Scan(
			&task.ID,
			&task.PlayerID,
			&task.PlayerName,
			&task.Title,
			&task.Description,
			&task.IsCompleted,
			&task.CompletedAt,
			&task.CreatedAt,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan task"})
			return
		}

		tasks = append(tasks, task)
	}

	c.JSON(http.StatusOK, gin.H{"tasks": tasks})
}

// CreateTask создает новую задачу
func (h *AdminItemsTasksHandler) CreateTask(c *gin.Context) {
	var req TaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	var taskID int
	err := h.db.QueryRow(`
		INSERT INTO tasks (player_id, title, description)
		VALUES ($1, $2, $3)
		RETURNING id
	`, req.PlayerID, req.Title, req.Description).Scan(&taskID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create task"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Task created successfully",
		"task_id": taskID,
	})
}

// UpdateTask обновляет задачу
func (h *AdminItemsTasksHandler) UpdateTask(c *gin.Context) {
	taskIDStr := c.Param("id")
	taskID, err := strconv.Atoi(taskIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID"})
		return
	}

	var req TaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	result, err := h.db.Exec(`
		UPDATE tasks
		SET title = $1, description = $2
		WHERE id = $3
	`, req.Title, req.Description, taskID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update task"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Task updated successfully"})
}

// DeleteTask удаляет задачу
func (h *AdminItemsTasksHandler) DeleteTask(c *gin.Context) {
	taskIDStr := c.Param("id")
	taskID, err := strconv.Atoi(taskIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID"})
		return
	}

	result, err := h.db.Exec(`DELETE FROM tasks WHERE id = $1`, taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete task"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Task deleted successfully"})
}

// ============================================
// ПРЕДМЕТЫ (ITEMS)
// ============================================

type ItemRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

type ItemResponse struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	CreatedAt    time.Time `json:"created_at"`
	EffectsCount int       `json:"effects_count"`
}

// GetAllItems возвращает все предметы
func (h *AdminItemsTasksHandler) GetAllItems(c *gin.Context) {
	rows, err := h.db.Query(`
		SELECT 
			i.id, i.name, i.description, i.created_at,
			COUNT(ie.effect_id) as effects_count
		FROM items i
		LEFT JOIN item_effects ie ON i.id = ie.item_id
		GROUP BY i.id, i.name, i.description, i.created_at
		ORDER BY i.name
	`)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch items"})
		return
	}
	defer rows.Close()

	items := make([]ItemResponse, 0)
	for rows.Next() {
		var item ItemResponse
		err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.Description,
			&item.CreatedAt,
			&item.EffectsCount,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan item"})
			return
		}

		items = append(items, item)
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}

// GetItem возвращает предмет с эффектами
func (h *AdminItemsTasksHandler) GetItem(c *gin.Context) {
	itemIDStr := c.Param("id")
	itemID, err := strconv.Atoi(itemIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid item ID"})
		return
	}

	// Получаем предмет
	var item ItemResponse
	err = h.db.QueryRow(`
		SELECT id, name, description, created_at
		FROM items
		WHERE id = $1
	`, itemID).Scan(&item.ID, &item.Name, &item.Description, &item.CreatedAt)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch item"})
		return
	}

	// Получаем эффекты
	type EffectInfo struct {
		ID                int     `json:"id"`
		Description       string  `json:"description"`
		EffectType        string  `json:"effect_type"`
		GeneratedResource *string `json:"generated_resource,omitempty"`
		Operation         string  `json:"operation"`
		Value             *int    `json:"value,omitempty"`
		SpawnedItemID     *int    `json:"spawned_item_id,omitempty"`
		SpawnedItemName   *string `json:"spawned_item_name,omitempty"`
		PeriodSeconds     int     `json:"period_seconds"`
	}

	rows, err := h.db.Query(`
		SELECT 
			e.id, e.description, e.effect_type, e.generated_resource, e.operation,
			e.value, e.spawned_item_id, i2.name as spawned_item_name, e.period_seconds
		FROM effects e
		JOIN item_effects ie ON e.id = ie.effect_id
		LEFT JOIN items i2 ON e.spawned_item_id = i2.id
		WHERE ie.item_id = $1
	`, itemID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch effects"})
		return
	}
	defer rows.Close()

	effects := make([]EffectInfo, 0)
	for rows.Next() {
		var effect EffectInfo
		err := rows.Scan(
			&effect.ID,
			&effect.Description,
			&effect.EffectType,
			&effect.GeneratedResource,
			&effect.Operation,
			&effect.Value,
			&effect.SpawnedItemID,
			&effect.SpawnedItemName,
			&effect.PeriodSeconds,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan effect"})
			return
		}

		effects = append(effects, effect)
	}

	c.JSON(http.StatusOK, gin.H{
		"item":    item,
		"effects": effects,
	})
}

// CreateItem создает новый предмет
func (h *AdminItemsTasksHandler) CreateItem(c *gin.Context) {
	var req ItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	var itemID int
	err := h.db.QueryRow(`
		INSERT INTO items (name, description)
		VALUES ($1, $2)
		RETURNING id
	`, req.Name, req.Description).Scan(&itemID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create item"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Item created successfully",
		"item_id": itemID,
	})
}

// UpdateItem обновляет предмет
func (h *AdminItemsTasksHandler) UpdateItem(c *gin.Context) {
	itemIDStr := c.Param("id")
	itemID, err := strconv.Atoi(itemIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid item ID"})
		return
	}

	var req ItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	result, err := h.db.Exec(`
		UPDATE items
		SET name = $1, description = $2
		WHERE id = $3
	`, req.Name, req.Description, itemID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update item"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Item updated successfully"})
}

// DeleteItem удаляет предмет
func (h *AdminItemsTasksHandler) DeleteItem(c *gin.Context) {
	itemIDStr := c.Param("id")
	itemID, err := strconv.Atoi(itemIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid item ID"})
		return
	}

	result, err := h.db.Exec(`DELETE FROM items WHERE id = $1`, itemID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete item"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Item deleted successfully"})
}

// ============================================
// ЭФФЕКТЫ (EFFECTS)
// ============================================

type EffectRequest struct {
	Description       string  `json:"description"`
	EffectType        string  `json:"effect_type" binding:"required,oneof=generate_money generate_influence spawn_item"`
	GeneratedResource *string `json:"generated_resource"`
	Operation         string  `json:"operation" binding:"required,oneof=add mul sub div"`
	Value             *int    `json:"value"`
	SpawnedItemID     *int    `json:"spawned_item_id"`
	PeriodSeconds     int     `json:"period_seconds" binding:"required,min=1"`
}

// CreateEffect создает новый эффект
func (h *AdminItemsTasksHandler) CreateEffect(c *gin.Context) {
	var req EffectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Валидация
	if req.EffectType == "spawn_item" && req.SpawnedItemID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "spawned_item_id is required for spawn_item type"})
		return
	}
	if (req.EffectType == "generate_money" || req.EffectType == "generate_influence") &&
		(req.GeneratedResource == nil || req.Value == nil) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "generated_resource and value are required for generate types"})
		return
	}

	var effectID int
	err := h.db.QueryRow(`
		INSERT INTO effects (description, effect_type, generated_resource, operation, value, spawned_item_id, period_seconds)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`, req.Description, req.EffectType, req.GeneratedResource, req.Operation, req.Value, req.SpawnedItemID, req.PeriodSeconds).Scan(&effectID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create effect"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":   "Effect created successfully",
		"effect_id": effectID,
	})
}

// AddEffectToItem добавляет эффект к предмету
func (h *AdminItemsTasksHandler) AddEffectToItem(c *gin.Context) {
	itemIDStr := c.Param("id")
	itemID, err := strconv.Atoi(itemIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid item ID"})
		return
	}

	var req struct {
		EffectID int `json:"effect_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	_, err = h.db.Exec(`
		INSERT INTO item_effects (item_id, effect_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, itemID, req.EffectID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add effect to item"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Effect added to item successfully"})
}

// RemoveEffectFromItem удаляет эффект у предмета
func (h *AdminItemsTasksHandler) RemoveEffectFromItem(c *gin.Context) {
	itemIDStr := c.Param("id")
	itemID, err := strconv.Atoi(itemIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid item ID"})
		return
	}

	effectIDStr := c.Param("effect_id")
	effectID, err := strconv.Atoi(effectIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid effect ID"})
		return
	}

	result, err := h.db.Exec(`
		DELETE FROM item_effects
		WHERE item_id = $1 AND effect_id = $2
	`, itemID, effectID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove effect from item"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Effect not found for this item"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Effect removed from item successfully"})
}

// ============================================
// СПОСОБНОСТИ (ABILITIES)
// ============================================

type AbilityRequest struct {
	PlayerID                int    `json:"player_id" binding:"required"`
	Name                    string `json:"name" binding:"required"`
	Description             string `json:"description"`
	AbilityType             string `json:"ability_type" binding:"required,oneof=reveal_info add_influence transfer_influence"`
	CooldownMinutes         *int   `json:"cooldown_minutes"`
	StartDelayMinutes       *int   `json:"start_delay_minutes"`
	RequiredInfluencePoints *int   `json:"required_influence_points"`
	IsUnlocked              bool   `json:"is_unlocked"`
	InfluencePointsToAdd    *int   `json:"influence_points_to_add"`
	InfluencePointsToRemove *int   `json:"influence_points_to_remove"`
	InfluencePointsToSelf   *int   `json:"influence_points_to_self"`
}

type AbilityResponse struct {
	ID                      int       `json:"id"`
	PlayerID                int       `json:"player_id"`
	PlayerName              string    `json:"player_name"`
	Name                    string    `json:"name"`
	Description             string    `json:"description"`
	AbilityType             string    `json:"ability_type"`
	CooldownMinutes         *int      `json:"cooldown_minutes,omitempty"`
	StartDelayMinutes       *int      `json:"start_delay_minutes,omitempty"`
	RequiredInfluencePoints *int      `json:"required_influence_points,omitempty"`
	IsUnlocked              bool      `json:"is_unlocked"`
	InfluencePointsToAdd    *int      `json:"influence_points_to_add,omitempty"`
	InfluencePointsToRemove *int      `json:"influence_points_to_remove,omitempty"`
	InfluencePointsToSelf   *int      `json:"influence_points_to_self,omitempty"`
	CreatedAt               time.Time `json:"created_at"`
}

// GetAllAbilities возвращает все способности
func (h *AdminItemsTasksHandler) GetAllAbilities(c *gin.Context) {
	playerIDStr := c.Query("player_id")

	query := `
		SELECT 
			a.id, a.player_id, p.character_name as player_name, a.name, a.description,
			a.ability_type, a.cooldown_minutes, a.start_delay_minutes, 
			a.required_influence_points, a.is_unlocked, a.influence_points_to_add,
			a.influence_points_to_remove, a.influence_points_to_self, a.created_at
		FROM abilities a
		JOIN players p ON a.player_id = p.id
	`

	var rows *sql.Rows
	var err error

	if playerIDStr != "" {
		playerID, err := strconv.Atoi(playerIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid player_id"})
			return
		}
		query += " WHERE a.player_id = $1 ORDER BY a.created_at DESC"
		rows, err = h.db.Query(query, playerID)
	} else {
		query += " ORDER BY a.created_at DESC"
		rows, err = h.db.Query(query)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch abilities"})
		return
	}
	defer rows.Close()

	abilities := make([]AbilityResponse, 0)
	for rows.Next() {
		var ability AbilityResponse
		err := rows.Scan(
			&ability.ID,
			&ability.PlayerID,
			&ability.PlayerName,
			&ability.Name,
			&ability.Description,
			&ability.AbilityType,
			&ability.CooldownMinutes,
			&ability.StartDelayMinutes,
			&ability.RequiredInfluencePoints,
			&ability.IsUnlocked,
			&ability.InfluencePointsToAdd,
			&ability.InfluencePointsToRemove,
			&ability.InfluencePointsToSelf,
			&ability.CreatedAt,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan ability"})
			return
		}

		abilities = append(abilities, ability)
	}

	c.JSON(http.StatusOK, gin.H{"abilities": abilities})
}

// CreateAbility создает новую способность
func (h *AdminItemsTasksHandler) CreateAbility(c *gin.Context) {
	var req AbilityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Валидация
	if req.AbilityType == "add_influence" && req.InfluencePointsToAdd == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "influence_points_to_add is required for add_influence type"})
		return
	}
	if req.AbilityType == "transfer_influence" &&
		(req.InfluencePointsToRemove == nil || req.InfluencePointsToSelf == nil) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "influence_points_to_remove and influence_points_to_self are required for transfer_influence type"})
		return
	}

	var abilityID int
	err := h.db.QueryRow(`
		INSERT INTO abilities (
			player_id, name, description, ability_type, cooldown_minutes, 
			start_delay_minutes, required_influence_points, is_unlocked,
			influence_points_to_add, influence_points_to_remove, influence_points_to_self
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id
	`, req.PlayerID, req.Name, req.Description, req.AbilityType, req.CooldownMinutes,
		req.StartDelayMinutes, req.RequiredInfluencePoints, req.IsUnlocked,
		req.InfluencePointsToAdd, req.InfluencePointsToRemove, req.InfluencePointsToSelf).Scan(&abilityID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create ability"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Ability created successfully",
		"ability_id": abilityID,
	})
}

// UpdateAbility обновляет способность
func (h *AdminItemsTasksHandler) UpdateAbility(c *gin.Context) {
	abilityIDStr := c.Param("id")
	abilityID, err := strconv.Atoi(abilityIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ability ID"})
		return
	}

	var req AbilityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	result, err := h.db.Exec(`
		UPDATE abilities
		SET name = $1, description = $2, cooldown_minutes = $3, 
		    start_delay_minutes = $4, required_influence_points = $5, is_unlocked = $6
		WHERE id = $7
	`, req.Name, req.Description, req.CooldownMinutes, req.StartDelayMinutes,
		req.RequiredInfluencePoints, req.IsUnlocked, abilityID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update ability"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ability not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Ability updated successfully"})
}

// DeleteAbility удаляет способность
func (h *AdminItemsTasksHandler) DeleteAbility(c *gin.Context) {
	abilityIDStr := c.Param("id")
	abilityID, err := strconv.Atoi(abilityIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ability ID"})
		return
	}

	result, err := h.db.Exec(`DELETE FROM abilities WHERE id = $1`, abilityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete ability"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ability not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Ability deleted successfully"})
}
