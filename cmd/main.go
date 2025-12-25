// cmd/main.go
package main

import (
	"database/sql"
	"log"
	"new-year-role-game-backend/internal/config"
	"new-year-role-game-backend/internal/database"
	"new-year-role-game-backend/internal/handlers"
	"new-year-role-game-backend/internal/middleware"
	"new-year-role-game-backend/internal/workers"
	"time"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.LoadConfig()

	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	// Создаем worker, но НЕ запускаем его автоматически
	// Worker будет запущен админом через /admin/game/start
	effectsWorker := workers.NewEffectsWorker(db, cfg.EffectsWorkerInterval)

	// Проверяем, активна ли игра, и запускаем worker если да
	if isGameActive(db) && !effectsWorker.IsRunning() {
		log.Println("Game is active, starting effects worker...")
		go effectsWorker.Start()
	}

	r := gin.Default()

	r.Use(middleware.CORS())

	api := r.Group("/api")
	{
		auth := api.Group("/auth")
		{
			authHandler := handlers.NewAuthHandler(db, cfg.JWTKey)
			auth.POST("/login", authHandler.Login)
		}

		protected := api.Group("")
		protected.Use(middleware.AuthMiddleware(cfg.JWTKey))
		{
			playerHandler := handlers.NewPlayerHandler(db)
			protected.GET("/player/me", playerHandler.GetPlayerInfo)
			protected.GET("/player/balance", playerHandler.GetPlayerBalance)

			factionHandler := handlers.NewFactionHandler(db)
			protected.GET("/player/faction", factionHandler.GetPlayerFaction)
			protected.GET("/factions", factionHandler.GetAllFactions)
			protected.PUT("/player/faction", factionHandler.ChangeFaction)

			goalHandler := handlers.NewGoalHandler(db)
			protected.GET("/player/goals", goalHandler.GetPersonalGoals)
			protected.GET("/player/faction/goals", goalHandler.GetFactionGoals)
			protected.PUT("/goals/:id/toggle", goalHandler.ToggleGoalCompletion)

			itemHandler := handlers.NewItemHandler(db)
			protected.GET("/player/inventory", itemHandler.GetPlayerInventory)
			protected.POST("/player/transfer/item", itemHandler.TransferItem)
			protected.POST("/player/transfer/money", itemHandler.TransferMoney)
			protected.GET("/player/items/effects/status", itemHandler.GetItemEffectsStatus)
		}

		// Admin endpoints - требуют роль администратора
		admin := api.Group("/admin")
		admin.Use(middleware.AuthMiddleware(cfg.JWTKey))
		admin.Use(middleware.AdminMiddleware())
		{
			adminHandler := handlers.NewAdminHandler(db, effectsWorker)
			admin.POST("/game/start", adminHandler.StartGame)
			admin.POST("/game/end", adminHandler.EndGame)
			admin.GET("/game/status", adminHandler.GetGameStatus)
		}
	}

	log.Printf("Server starting on port %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}

// isGameActive проверяет, активна ли игра
func isGameActive(db *sql.DB) bool {
	var gameStarted, gameEnded *time.Time
	err := db.QueryRow(`
		SELECT game_started_at, game_ended_at
		FROM game_timeline
		ORDER BY id DESC
		LIMIT 1
	`).Scan(&gameStarted, &gameEnded)

	if err != nil {
		return false
	}

	return gameStarted != nil && gameEnded == nil
}
