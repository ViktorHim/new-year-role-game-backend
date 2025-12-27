// internal/handlers/admin_final.go
package handlers

import (
	"database/sql"
	"net/http"
	"new-year-role-game-backend/internal/workers"
	"time"

	"github.com/gin-gonic/gin"
)

type AdminHandler struct {
	db                *sql.DB
	effectsScheduler  *workers.EffectsScheduler
	contractScheduler *workers.ContractScheduler
}

func NewAdminHandler(db *sql.DB, effectsScheduler *workers.EffectsScheduler,
	contractScheduler *workers.ContractScheduler) *AdminHandler {
	return &AdminHandler{
		db:                db,
		effectsScheduler:  effectsScheduler,
		contractScheduler: contractScheduler,
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

	// Запускаем schedulers (основные системы с точными таймерами)
	var schedulerErrors []string

	if err := h.effectsScheduler.Start(); err != nil {
		schedulerErrors = append(schedulerErrors, "effects: "+err.Error())
	}

	if err := h.contractScheduler.Start(); err != nil {
		schedulerErrors = append(schedulerErrors, "contracts: "+err.Error())
	}

	// Запускаем workers как fallback (подстраховка)
	// if !h.effectsWorker.IsRunning() {
	// 	go h.effectsWorker.Start()
	// }

	// if !h.contractsWorker.IsRunning() {
	// 	go h.contractsWorker.Start()
	// }

	response := gin.H{
		"message":    "Game started successfully",
		"started_at": now,
		"schedulers": gin.H{
			"effects_scheduled":   h.effectsScheduler.GetScheduledCount(),
			"contracts_scheduled": h.contractScheduler.GetScheduledCount(),
		},
		// "workers": gin.H{
		// 	"effects_running":   h.effectsWorker.IsRunning(),
		// 	"contracts_running": h.contractsWorker.IsRunning(),
		// },
	}

	if len(schedulerErrors) > 0 {
		response["scheduler_warnings"] = schedulerErrors
	}

	c.JSON(http.StatusOK, response)
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

	// Останавливаем все schedulers
	h.effectsScheduler.Stop()
	h.contractScheduler.Stop()

	c.JSON(http.StatusOK, gin.H{
		"message":    "Game ended successfully",
		"started_at": gameStarted,
		"ended_at":   now,
		"duration":   now.Sub(*gameStarted).String(),
	})
}

// GetGameStats возвращает детальную статистику игры
func (h *AdminHandler) GetGameStats(c *gin.Context) {
	stats := gin.H{
		"schedulers": gin.H{
			"effects_scheduled":   h.effectsScheduler.GetScheduledCount(),
			"contracts_scheduled": h.contractScheduler.GetScheduledCount(),
		},
		// "workers": gin.H{
		// 	"effects_running":   h.effectsWorker.IsRunning(),
		// 	"contracts_running": h.contractsWorker.IsRunning(),
		// },
	}

	// Статистика по договорам
	var contractStats struct {
		Pending    int
		Signed     int
		Completed  int
		Terminated int
	}

	err := h.db.QueryRow(`
		SELECT 
			COUNT(*) FILTER (WHERE status = 'pending') as pending,
			COUNT(*) FILTER (WHERE status = 'signed') as signed,
			COUNT(*) FILTER (WHERE status = 'completed') as completed,
			COUNT(*) FILTER (WHERE status = 'terminated') as terminated
		FROM contracts
	`).Scan(&contractStats.Pending, &contractStats.Signed, &contractStats.Completed, &contractStats.Terminated)

	if err == nil {
		stats["contracts"] = contractStats
	}

	// Количество истекших, но не завершённых договоров (должно быть 0 при работающем scheduler)
	var expiredContracts int
	err = h.db.QueryRow(`
		SELECT COUNT(*)
		FROM contracts
		WHERE status = 'signed' AND expires_at <= NOW()
	`).Scan(&expiredContracts)

	if err == nil {
		stats["expired_contracts"] = expiredContracts
		if expiredContracts > 0 {
			stats["warning"] = "There are expired contracts not completed yet"
		}
	}

	// Статистика по игрокам и предметам
	var playerStats struct {
		TotalPlayers     int
		PlayersWithItems int
		TotalItems       int
		TotalEffects     int
	}

	err = h.db.QueryRow(`
		SELECT 
			(SELECT COUNT(*) FROM players) as total_players,
			(SELECT COUNT(DISTINCT player_id) FROM player_items) as players_with_items,
			(SELECT COUNT(*) FROM player_items) as total_items,
			(SELECT COUNT(*) FROM item_effect_executions) as total_effects
	`).Scan(
		&playerStats.TotalPlayers,
		&playerStats.PlayersWithItems,
		&playerStats.TotalItems,
		&playerStats.TotalEffects,
	)

	if err == nil {
		stats["players"] = playerStats
	}

	// Информация об игре
	var gameInfo struct {
		Status    string
		StartedAt *time.Time
		EndedAt   *time.Time
		Duration  *string
	}

	var gameStarted, gameEnded *time.Time
	err = h.db.QueryRow(`
		SELECT game_started_at, game_ended_at
		FROM game_timeline
		ORDER BY id DESC
		LIMIT 1
	`).Scan(&gameStarted, &gameEnded)

	if err == nil {
		gameInfo.StartedAt = gameStarted
		gameInfo.EndedAt = gameEnded

		if gameStarted != nil && gameEnded == nil {
			gameInfo.Status = "running"
			duration := time.Since(*gameStarted).String()
			gameInfo.Duration = &duration
		} else if gameStarted != nil && gameEnded != nil {
			gameInfo.Status = "ended"
			duration := gameEnded.Sub(*gameStarted).String()
			gameInfo.Duration = &duration
		} else {
			gameInfo.Status = "not_started"
		}

		stats["game"] = gameInfo
	}

	c.JSON(http.StatusOK, stats)
}
