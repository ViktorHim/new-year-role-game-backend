// internal/handlers/admin_goal_race_ext.go
package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// UpdatePredefinedGoal обновляет предопределенную цель
func (h *AdminGoalRaceHandler) UpdatePredefinedGoal(c *gin.Context) {
	goalIDStr := c.Param("id")
	goalID, err := strconv.Atoi(goalIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid goal ID"})
		return
	}

	var req struct {
		Title                 string `json:"title" binding:"required"`
		Description           string `json:"description"`
		InfluencePointsReward int    `json:"influence_points_reward"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	result, err := h.db.Exec(`
		UPDATE goal_race_predefined_goals
		SET title = $1, description = $2, influence_points_reward = $3
		WHERE id = $4
	`, req.Title, req.Description, req.InfluencePointsReward, goalID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update predefined goal"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Predefined goal not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Predefined goal updated successfully"})
}

// DeleteTrigger удаляет триггер гонки целей
func (h *AdminGoalRaceHandler) DeleteTrigger(c *gin.Context) {
	triggerIDStr := c.Param("id")
	triggerID, err := strconv.Atoi(triggerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid trigger ID"})
		return
	}

	result, err := h.db.Exec(`
		DELETE FROM goal_race_triggers
		WHERE id = $1
	`, triggerID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete trigger"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trigger not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Trigger deleted successfully"})
}

// CreateBatchPredefinedGoals массовое создание целей для раундов гонки
func (h *AdminGoalRaceHandler) CreateBatchPredefinedGoals(c *gin.Context) {
	var req struct {
		TriggerID int `json:"trigger_id" binding:"required"`
		Goals     []struct {
			RoundNumber           int    `json:"round_number" binding:"required"`
			PlayerID              int    `json:"player_id" binding:"required"`
			Title                 string `json:"title" binding:"required"`
			Description           string `json:"description"`
			InfluencePointsReward int    `json:"influence_points_reward"`
		} `json:"goals" binding:"required,min=1"`
	}
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

	tx, err := h.db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	createdGoalIDs := make([]int, 0)
	for _, goal := range req.Goals {
		// Проверяем, что игрок существует
		var playerExists bool
		err = tx.QueryRow(`
			SELECT EXISTS(SELECT 1 FROM players WHERE id = $1)
		`, goal.PlayerID).Scan(&playerExists)

		if err != nil || !playerExists {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Player not found: " + strconv.Itoa(goal.PlayerID)})
			return
		}

		var goalID int
		err = tx.QueryRow(`
			INSERT INTO goal_race_predefined_goals (
				trigger_id, round_number, player_id, title, description, influence_points_reward
			)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING id
		`, req.TriggerID, goal.RoundNumber, goal.PlayerID, goal.Title, goal.Description, goal.InfluencePointsReward).Scan(&goalID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create predefined goal"})
			return
		}

		createdGoalIDs = append(createdGoalIDs, goalID)
	}

	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":  "Predefined goals created successfully",
		"goal_ids": createdGoalIDs,
		"count":    len(createdGoalIDs),
	})
}
