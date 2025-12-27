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

	// Создаем schedulers для точных таймеров
	effectsScheduler := workers.NewEffectsScheduler(db)
	contractScheduler := workers.NewContractScheduler(db)
	debtScheduler := workers.NewDebtScheduler(db)

	// Проверяем, активна ли игра, и запускаем schedulers если да
	if isGameActive(db) {
		log.Println("Game is active, starting schedulers and workers...")

		// Запускаем effects scheduler (точные таймеры для эффектов)
		if err := effectsScheduler.Start(); err != nil {
			log.Printf("Warning: Failed to start effects scheduler: %v", err)
		}

		// Запускаем contracts scheduler (точные таймеры для договоров)
		if err := contractScheduler.Start(); err != nil {
			log.Printf("Warning: Failed to start contract scheduler: %v", err)
		}

		if err := debtScheduler.Start(); err != nil {
			log.Printf("Warning: Failed to start debt scheduler: %v", err)
		}

		// // Запускаем workers как fallback (подстраховка)
		// if !effectsWorker.IsRunning() {
		// 	log.Println("Starting effects worker as fallback...")
		// 	go effectsWorker.Start()
		// }

		// if !contractsWorker.IsRunning() {
		// 	log.Println("Starting contracts worker as fallback...")
		// 	go contractsWorker.Start()
		// }
	}

	contractsHandlerWithShedular := handlers.NewContractHandlerWithScheduler(db, contractScheduler)

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
		gameHandler := handlers.NewGameHandler(db)
		protected.GET("/game/status", gameHandler.GetGameStatus)
		protected.Use(middleware.AuthMiddleware(cfg.JWTKey))
		{
			playerHandler := handlers.NewPlayerHandler(db)
			protected.GET("/player/me", playerHandler.GetPlayerInfo)
			protected.GET("/player/balance", playerHandler.GetPlayerBalance)
			protected.GET("/players", playerHandler.GetAllPlayers)

			factionHandler := handlers.NewFactionHandler(db)
			protected.GET("/player/faction", factionHandler.GetPlayerFaction)
			protected.GET("/factions", factionHandler.GetAllFactions)
			protected.PUT("/player/faction", factionHandler.ChangeFaction)

			goalHandler := handlers.NewGoalHandler(db)
			protected.GET("/player/goals", goalHandler.GetPersonalGoals)
			protected.GET("/player/faction/goals", goalHandler.GetFactionGoals)
			protected.PUT("/goals/:id/toggle", goalHandler.ToggleGoalCompletion)

			itemHandler := handlers.NewItemHandlerWithScheduler(db, effectsScheduler)
			protected.GET("/player/inventory", itemHandler.GetPlayerInventory)
			protected.POST("/player/transfer/item", itemHandler.TransferItem)
			protected.POST("/player/transfer/money", itemHandler.TransferMoney)
			protected.GET("/player/items/effects/status", itemHandler.GetItemEffectsStatus)

			abilityHandler := handlers.NewAbilityHandler(db)
			protected.GET("/player/abilities", abilityHandler.GetPlayerAbilities)
			protected.POST("/abilities/:id/use", abilityHandler.UseAbility)

			contractHandler := handlers.NewContractHandler(db)
			protected.GET("/player/contracts", contractHandler.GetPlayerContracts)
			protected.POST("/contracts/create", contractHandler.CreateContract)
			protected.POST("/contracts/:id/sign", contractsHandlerWithShedular.SignContract)

			// Долговые расписки с scheduler
			debtHandler := handlers.NewDebtHandler(db, debtScheduler)
			protected.GET("/player/debts", debtHandler.GetPlayerDebts)
			protected.POST("/debts/create", debtHandler.CreateDebtReceipt)
			protected.POST("/debts/:id/return", debtHandler.ReturnDebt)
		}

		// Admin endpoints - требуют роль администратора
		admin := api.Group("/admin")
		admin.Use(middleware.AuthMiddleware(cfg.JWTKey))
		admin.Use(middleware.AdminMiddleware())
		{
			adminHandler := handlers.NewAdminHandler(db, effectsScheduler, contractScheduler)
			admin.POST("/game/start", adminHandler.StartGame)
			admin.POST("/game/end", adminHandler.EndGame)

			adminContractHandler := handlers.NewAdminContractHandler(db)
			admin.GET("/contracts/settings", adminContractHandler.GetContractSettings)
			admin.PUT("/contracts/type1/rewards", adminContractHandler.UpdateContractType1Rewards)
			admin.PUT("/contracts/type2/rewards", adminContractHandler.UpdateContractType2Rewards)
			admin.PUT("/contracts/penalties", adminContractHandler.UpdateContractPenalties)
			admin.DELETE("/contracts/:id/terminate", contractsHandlerWithShedular.TerminateContract)

			// Настройки долговых расписок
			adminDebtHandler := handlers.NewAdminDebtHandler(db)
			admin.GET("/debts/settings", adminDebtHandler.GetDebtPenaltySettings)
			admin.PUT("/debts/penalties", adminDebtHandler.UpdateDebtPenaltySettings)
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
