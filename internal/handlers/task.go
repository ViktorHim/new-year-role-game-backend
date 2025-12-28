// internal/handlers/task.go
package handlers

import (
	"database/sql"
	"net/http"
	"new-year-role-game-backend/internal/models"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type TaskHandler struct {
	db              *sql.DB
	goalRaceHandler *GoalRaceHandler
}

func NewTaskHandler(db *sql.DB, goalRaceHandler *GoalRaceHandler) *TaskHandler {
	return &TaskHandler{
		db:              db,
		goalRaceHandler: goalRaceHandler,
	}
}

// GetPlayerTasks возвращает все задачи игрока
func (h *TaskHandler) GetPlayerTasks(c *gin.Context) {
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
			id,
			title,
			description,
			is_completed,
			completed_at,
			created_at
		FROM tasks
		WHERE player_id = $1
		ORDER BY is_completed ASC, created_at DESC
	`, *playerID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tasks"})
		return
	}
	defer rows.Close()

	tasks := make([]models.Task, 0)
	for rows.Next() {
		var task models.Task
		task.PlayerID = *playerID

		err := rows.Scan(
			&task.ID,
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

	if err = rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, models.TasksResponse{Tasks: tasks})
}

// ToggleTaskCompletion отмечает задачу как выполненную или невыполненную
func (h *TaskHandler) ToggleTaskCompletion(c *gin.Context) {
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

	taskIDStr := c.Param("id")
	taskID, err := strconv.Atoi(taskIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID"})
		return
	}

	var req models.CompleteTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if req.IsCompleted == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "is_completed is required"})
		return
	}

	isCompleted := *req.IsCompleted

	// Начинаем транзакцию
	tx, err := h.db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	// Проверяем, что задача принадлежит игроку
	var taskPlayerID int
	var currentCompleted bool
	err = tx.QueryRow(`
		SELECT player_id, is_completed
		FROM tasks
		WHERE id = $1
		FOR UPDATE
	`, taskID).Scan(&taskPlayerID, &currentCompleted)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if taskPlayerID != *playerID {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to modify this task"})
		return
	}

	// Проверяем, что статус действительно меняется
	if currentCompleted == isCompleted {
		action := "completed"
		if !isCompleted {
			action = "incomplete"
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "Task is already " + action})
		return
	}

	// Обновляем статус задачи
	var completedAt *time.Time
	if isCompleted {
		now := time.Now()
		completedAt = &now
	}

	_, err = tx.Exec(`
		UPDATE tasks
		SET is_completed = $1, completed_at = $2
		WHERE id = $3
	`, isCompleted, completedAt, taskID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update task"})
		return
	}

	// Записываем в историю
	action := "completed"
	if !isCompleted {
		action = "uncompleted"
	}

	_, err = tx.Exec(`
		INSERT INTO task_completion_history (task_id, player_id, action)
		VALUES ($1, $2, $3)
	`, taskID, *playerID, action)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record task completion history"})
		return
	}

	// Фиксируем транзакцию
	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	// ВАЖНО: Если задача отмечена как выполненная, проверяем триггеры гонки
	if isCompleted {
		if err := h.goalRaceHandler.CheckTaskCompletionAndTriggerRace(*playerID); err != nil {
			// Логируем ошибку, но не возвращаем ее пользователю
			// так как основное действие (обновление задачи) уже выполнено
			c.JSON(http.StatusOK, gin.H{
				"message": "Task marked as completed",
				"warning": "Failed to check race triggers: " + err.Error(),
			})
			return
		}
	}

	message := "Task marked as completed"
	if !isCompleted {
		message = "Task marked as incomplete"
	}

	c.JSON(http.StatusOK, gin.H{"message": message})
}
