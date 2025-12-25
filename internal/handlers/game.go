// internal/handlers/faction.go
package handlers

import (
	"database/sql"
	"net/http"
	"new-year-role-game-backend/internal/models"
	"time"

	"github.com/gin-gonic/gin"
)

type GameHandler struct {
	db *sql.DB
}

func NewGameHandler(db *sql.DB) *GameHandler {
	return &GameHandler{
		db: db,
	}
}

// GetGameStatus возвращает текущий статус игры
func (h *GameHandler) GetGameStatus(c *gin.Context) {
	var gameStarted, gameEnded *time.Time
	err := h.db.QueryRow(`
		SELECT game_started_at, game_ended_at
		FROM game_timeline
		ORDER BY id DESC
		LIMIT 1
	`).Scan(&gameStarted, &gameEnded)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusOK, models.GameStatusResponse{
				Status: "not_started",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	var status string
	var duration *string

	if gameStarted != nil && gameEnded == nil {
		status = "running"
		d := time.Since(*gameStarted).String()
		duration = &d
	} else if gameStarted != nil && gameEnded != nil {
		status = "ended"
		d := gameEnded.Sub(*gameStarted).String()
		duration = &d
	} else {
		status = "not_started"
	}

	c.JSON(http.StatusOK, models.GameStatusResponse{
		Status:    status,
		StartedAt: gameStarted,
		EndedAt:   gameEnded,
		Duration:  duration,
	})
}
