// internal/handlers/admin_goal_race.go
package handlers

import (
	"database/sql"
	"net/http"
	"new-year-role-game-backend/internal/models"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type AdminGoalRaceHandler struct {
	db *sql.DB
}

func NewAdminGoalRaceHandler(db *sql.DB) *AdminGoalRaceHandler {
	return &AdminGoalRaceHandler{db: db}
}

// CreateTrigger создает новый триггер гонки целей
func (h *AdminGoalRaceHandler) CreateTrigger(c *gin.Context) {
	var req models.CreateTriggerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Проверяем минимальное количество участников
	if len(req.ParticipantIDs) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least 2 participants are required"})
		return
	}

	tx, err := h.db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	// Создаем триггер
	var triggerID int
	err = tx.QueryRow(`
		INSERT INTO goal_race_triggers (name, description, required_tasks_count, is_active)
		VALUES ($1, $2, $3, true)
		RETURNING id
	`, req.Name, req.Description, req.RequiredTasksCount).Scan(&triggerID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create trigger"})
		return
	}

	// Добавляем участников
	for _, playerID := range req.ParticipantIDs {
		// Проверяем, что игрок существует
		var exists bool
		err = tx.QueryRow(`
			SELECT EXISTS(SELECT 1 FROM players WHERE id = $1)
		`, playerID).Scan(&exists)

		if err != nil || !exists {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Player not found: " + strconv.Itoa(playerID)})
			return
		}

		// Добавляем участника
		_, err = tx.Exec(`
			INSERT INTO goal_race_trigger_participants (trigger_id, player_id)
			VALUES ($1, $2)
		`, triggerID, playerID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add participant"})
			return
		}
	}

	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	// Получаем созданный триггер
	var trigger models.GoalRaceTrigger
	err = h.db.QueryRow(`
		SELECT id, name, description, required_tasks_count, is_active, created_at
		FROM goal_race_triggers
		WHERE id = $1
	`, triggerID).Scan(
		&trigger.ID,
		&trigger.Name,
		&trigger.Description,
		&trigger.RequiredTasksCount,
		&trigger.IsActive,
		&trigger.CreatedAt,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch created trigger"})
		return
	}

	c.JSON(http.StatusCreated, trigger)
}

// GetTriggers возвращает список всех триггеров
func (h *AdminGoalRaceHandler) GetTriggers(c *gin.Context) {
	rows, err := h.db.Query(`
		SELECT id, name, description, required_tasks_count, is_active, created_at
		FROM goal_race_triggers
		ORDER BY created_at DESC
	`)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch triggers"})
		return
	}
	defer rows.Close()

	triggers := make([]models.GoalRaceTrigger, 0)
	for rows.Next() {
		var trigger models.GoalRaceTrigger
		err := rows.Scan(
			&trigger.ID,
			&trigger.Name,
			&trigger.Description,
			&trigger.RequiredTasksCount,
			&trigger.IsActive,
			&trigger.CreatedAt,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan trigger"})
			return
		}

		triggers = append(triggers, trigger)
	}

	if err = rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"triggers": triggers})
}

// UpdateTrigger обновляет триггер (активация/деактивация)
func (h *AdminGoalRaceHandler) UpdateTrigger(c *gin.Context) {
	triggerIDStr := c.Param("id")
	triggerID, err := strconv.Atoi(triggerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid trigger ID"})
		return
	}

	var req models.UpdateTriggerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if req.IsActive == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "is_active is required"})
		return
	}

	_, err = h.db.Exec(`
		UPDATE goal_race_triggers
		SET is_active = $1
		WHERE id = $2
	`, *req.IsActive, triggerID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update trigger"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Trigger updated successfully"})
}

// CreatePredefinedGoal создает предопределенную цель для раунда
func (h *AdminGoalRaceHandler) CreatePredefinedGoal(c *gin.Context) {
	var req models.CreatePredefinedGoalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Проверяем, что триггер существует
	var triggerExists bool
	err := h.db.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM goal_race_triggers WHERE id = $1)
	`, req.TriggerID).Scan(&triggerExists)

	if err != nil || !triggerExists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Trigger not found"})
		return
	}

	// Проверяем, что игрок существует
	var playerExists bool
	err = h.db.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM players WHERE id = $1)
	`, req.PlayerID).Scan(&playerExists)

	if err != nil || !playerExists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Player not found"})
		return
	}

	// Создаем предопределенную цель
	var goalID int
	err = h.db.QueryRow(`
		INSERT INTO goal_race_predefined_goals (
			trigger_id, round_number, player_id, title, description, influence_points_reward
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`, req.TriggerID, req.RoundNumber, req.PlayerID, req.Title, req.Description, req.InfluencePointsReward).Scan(&goalID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create predefined goal"})
		return
	}

	// Получаем созданную цель
	var goal models.PredefinedGoal
	err = h.db.QueryRow(`
		SELECT id, trigger_id, round_number, player_id, title, description, influence_points_reward, created_at
		FROM goal_race_predefined_goals
		WHERE id = $1
	`, goalID).Scan(
		&goal.ID,
		&goal.TriggerID,
		&goal.RoundNumber,
		&goal.PlayerID,
		&goal.Title,
		&goal.Description,
		&goal.InfluencePointsReward,
		&goal.CreatedAt,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch created goal"})
		return
	}

	c.JSON(http.StatusCreated, goal)
}

// GetPredefinedGoals возвращает предопределенные цели для триггера
func (h *AdminGoalRaceHandler) GetPredefinedGoals(c *gin.Context) {
	triggerIDStr := c.Query("trigger_id")
	if triggerIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "trigger_id query parameter is required"})
		return
	}

	triggerID, err := strconv.Atoi(triggerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid trigger_id"})
		return
	}

	rows, err := h.db.Query(`
		SELECT 
			g.id, g.trigger_id, g.round_number, g.player_id, g.title, g.description, 
			g.influence_points_reward, g.created_at, p.character_name
		FROM goal_race_predefined_goals g
		JOIN players p ON g.player_id = p.id
		WHERE g.trigger_id = $1
		ORDER BY g.round_number, g.player_id
	`, triggerID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch predefined goals"})
		return
	}
	defer rows.Close()

	type PredefinedGoalWithPlayer struct {
		models.PredefinedGoal
		PlayerName string `json:"player_name"`
	}

	goals := make([]PredefinedGoalWithPlayer, 0)
	for rows.Next() {
		var goal PredefinedGoalWithPlayer
		err := rows.Scan(
			&goal.ID,
			&goal.TriggerID,
			&goal.RoundNumber,
			&goal.PlayerID,
			&goal.Title,
			&goal.Description,
			&goal.InfluencePointsReward,
			&goal.CreatedAt,
			&goal.PlayerName,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan goal"})
			return
		}

		goals = append(goals, goal)
	}

	if err = rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"predefined_goals": goals})
}

// DeletePredefinedGoal удаляет предопределенную цель
func (h *AdminGoalRaceHandler) DeletePredefinedGoal(c *gin.Context) {
	goalIDStr := c.Param("id")
	goalID, err := strconv.Atoi(goalIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid goal ID"})
		return
	}

	result, err := h.db.Exec(`
		DELETE FROM goal_race_predefined_goals
		WHERE id = $1
	`, goalID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete predefined goal"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Predefined goal not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Predefined goal deleted successfully"})
}

// GetRaceHistory возвращает историю раундов гонки
func (h *AdminGoalRaceHandler) GetRaceHistory(c *gin.Context) {
	triggerIDStr := c.Query("trigger_id")

	query := `
		SELECT 
			grr.id,
			grr.trigger_id,
			grr.round_number,
			grr.status,
			grr.started_at,
			grr.completed_at,
			grr.winner_player_id,
			p.character_name AS winner_name
		FROM goal_race_rounds grr
		LEFT JOIN players p ON grr.winner_player_id = p.id
	`

	var rows *sql.Rows
	var err error

	if triggerIDStr != "" {
		triggerID, err := strconv.Atoi(triggerIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid trigger_id"})
			return
		}
		query += " WHERE grr.trigger_id = $1 ORDER BY grr.started_at DESC"
		rows, err = h.db.Query(query, triggerID)
	} else {
		query += " ORDER BY grr.started_at DESC"
		rows, err = h.db.Query(query)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch race history"})
		return
	}
	defer rows.Close()

	type RoundHistory struct {
		ID             int        `json:"id"`
		TriggerID      int        `json:"trigger_id"`
		RoundNumber    int        `json:"round_number"`
		Status         string     `json:"status"`
		StartedAt      *time.Time `json:"started_at"`
		CompletedAt    *time.Time `json:"completed_at,omitempty"`
		WinnerPlayerID *int       `json:"winner_player_id,omitempty"`
		WinnerName     *string    `json:"winner_name,omitempty"`
	}

	history := make([]RoundHistory, 0)
	for rows.Next() {
		var round RoundHistory
		err := rows.Scan(
			&round.ID,
			&round.TriggerID,
			&round.RoundNumber,
			&round.Status,
			&round.StartedAt,
			&round.CompletedAt,
			&round.WinnerPlayerID,
			&round.WinnerName,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan round"})
			return
		}

		history = append(history, round)
	}

	if err = rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"race_history": history})
}
