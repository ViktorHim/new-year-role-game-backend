// internal/handlers/admin.go
package handlers

import (
	"database/sql"
	"net/http"
	"new-year-role-game-backend/internal/models"
	"new-year-role-game-backend/internal/workers"
	"time"

	"github.com/gin-gonic/gin"
)

type AdminHandler struct {
	db            *sql.DB
	effectsWorker *workers.EffectsWorker
}

func NewAdminHandler(db *sql.DB, effectsWorker *workers.EffectsWorker) *AdminHandler {
	return &AdminHandler{
		db:            db,
		effectsWorker: effectsWorker,
	}
}

// StartGame начинает игру
func (h *AdminHandler) StartGame(c *gin.Context) {
	// Проверяем, не запущена ли уже игра
	var gameStarted, gameEnded *time.Time
	err := h.db.QueryRow(`
		SELECT game_started_at, game_ended_at
		FROM game_timeline
		ORDER BY id DESC
		LIMIT 1
	`).Scan(&gameStarted, &gameEnded)

	if err != nil && err != sql.ErrNoRows {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Если есть активная игра
	if err == nil && gameStarted != nil && gameEnded == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Game is already running"})
		return
	}

	// Начинаем транзакцию
	tx, err := h.db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	now := time.Now()

	// Создаем новую запись о начале игры
	_, err = tx.Exec(`
		INSERT INTO game_timeline (game_started_at)
		VALUES ($1)
	`, now)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start game"})
		return
	}

	// Инициализируем таймеры эффектов для всех предметов игроков
	// Устанавливаем last_executed_at = NOW(), чтобы первое выполнение произошло через period_seconds
	_, err = tx.Exec(`
		INSERT INTO item_effect_executions (player_id, item_id, effect_id, last_executed_at)
		SELECT 
			pi.player_id,
			pi.item_id,
			e.id,
			$1
		FROM player_items pi
		JOIN item_effects ie ON pi.item_id = ie.item_id
		JOIN effects e ON ie.effect_id = e.id
		ON CONFLICT (player_id, item_id, effect_id) 
		DO UPDATE SET last_executed_at = $1
	`, now)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize effect timers"})
		return
	}

	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	// Запускаем worker если он не запущен
	if !h.effectsWorker.IsRunning() {
		go h.effectsWorker.Start()
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "Game started successfully",
		"started_at":     now,
		"worker_running": h.effectsWorker.IsRunning(),
	})
}

// EndGame завершает игру
func (h *AdminHandler) EndGame(c *gin.Context) {
	// Проверяем, запущена ли игра
	var gameID int
	var gameStarted, gameEnded *time.Time
	err := h.db.QueryRow(`
		SELECT id, game_started_at, game_ended_at
		FROM game_timeline
		ORDER BY id DESC
		LIMIT 1
	`).Scan(&gameID, &gameStarted, &gameEnded)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No game has been started"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Если игра уже завершена
	if gameEnded != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Game is already ended"})
		return
	}

	// Если игра не начата
	if gameStarted == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Game has not been started"})
		return
	}

	now := time.Now()

	// Завершаем игру
	_, err = h.db.Exec(`
		UPDATE game_timeline
		SET game_ended_at = $1
		WHERE id = $2
	`, now, gameID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to end game"})
		return
	}

	// Останавливаем worker (он остановится при следующей проверке)
	// Worker сам проверяет статус игры и не будет выполнять эффекты

	c.JSON(http.StatusOK, gin.H{
		"message":    "Game ended successfully",
		"started_at": gameStarted,
		"ended_at":   now,
		"duration":   now.Sub(*gameStarted).String(),
	})
}

// GetGameStatus возвращает текущий статус игры
func (h *AdminHandler) GetGameStatus(c *gin.Context) {
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
				Status:        "not_started",
				WorkerRunning: h.effectsWorker.IsRunning(),
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
		Status:        status,
		StartedAt:     gameStarted,
		EndedAt:       gameEnded,
		Duration:      duration,
		WorkerRunning: h.effectsWorker.IsRunning(),
	})
}
