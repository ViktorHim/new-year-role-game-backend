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
	"strconv"
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

			// Goal Race Handler (создаем первым для использования в других handlers)
			goalRaceHandler := handlers.NewGoalRaceHandler(db)
			goalHandler := handlers.NewGoalHandler(db)
			protected.GET("/player/goals", goalHandler.GetPersonalGoals)
			protected.GET("/player/faction/goals", goalHandler.GetFactionGoals)
			protected.PUT("/goals/:id/toggle", func(c *gin.Context) {
				// Вызываем оригинальный обработчик
				goalHandler.ToggleGoalCompletion(c)

				// Если цель была успешно отмечена как выполненная, проверяем раунд
				if c.Writer.Status() == 200 {
					playerIDInterface, _ := c.Get("player_id")
					playerID := playerIDInterface.(*int)

					goalIDStr := c.Param("id")
					var goalID int
					if _, err := strconv.Atoi(goalIDStr); err == nil {
						// Проверяем завершение раунда в фоне, чтобы не блокировать ответ
						go goalRaceHandler.CheckRoundCompletion(*playerID, goalID)
					}
				}
			})

			itemHandler := handlers.NewItemHandlerWithScheduler(db, effectsScheduler)
			protected.GET("/player/inventory", itemHandler.GetPlayerInventory)
			protected.POST("/player/transfer/item", itemHandler.TransferItem)
			protected.POST("/player/transfer/money", itemHandler.TransferMoney)
			protected.GET("/player/items/effects/status", itemHandler.GetItemEffectsStatus)

			abilityHandler := handlers.NewAbilityHandler(db)
			protected.GET("/player/abilities", abilityHandler.GetPlayerAbilities)
			protected.POST("/abilities/:id/use", abilityHandler.UseAbility)

			contractHandler := handlers.NewContractHandler(db, contractScheduler)
			protected.GET("/player/contracts", contractHandler.GetPlayerContracts)
			protected.POST("/contracts/create", contractHandler.CreateContract)
			protected.POST("/contracts/:id/sign", contractsHandlerWithShedular.SignContract)
			protected.POST("/contracts/:id/reveal", contractsHandlerWithShedular.RevealContractInfo)

			// Долговые расписки с scheduler
			debtHandler := handlers.NewDebtHandler(db, debtScheduler)
			protected.GET("/player/debts", debtHandler.GetPlayerDebts)
			protected.POST("/debts/create", debtHandler.CreateDebtReceipt)
			protected.POST("/debts/:id/return", debtHandler.ReturnDebt)

			// Task Handler с интеграцией гонки целей
			taskHandler := handlers.NewTaskHandler(db, goalRaceHandler)
			protected.GET("/player/tasks", taskHandler.GetPlayerTasks)
			protected.PUT("/tasks/:id/toggle", taskHandler.ToggleTaskCompletion)

			// Goal Race Progress
			protected.GET("/player/race/progress", goalRaceHandler.GetPlayerRaceProgress)
		}

		// Admin endpoints - требуют роль администратора
		admin := api.Group("/admin")
		admin.Use(middleware.AuthMiddleware(cfg.JWTKey))
		admin.Use(middleware.AdminMiddleware())
		{
			// ========================================
			// УПРАВЛЕНИЕ ИГРОЙ
			// ========================================
			adminHandler := handlers.NewAdminHandler(db, effectsScheduler, contractScheduler)
			admin.POST("/game/start", adminHandler.StartGame)
			admin.POST("/game/end", adminHandler.EndGame)
			admin.GET("/game/stats", adminHandler.GetGameStats)

			// ========================================
			// ФРАКЦИИ
			// ========================================
			adminEntitiesHandler := handlers.NewAdminEntitiesHandler(db)
			admin.GET("/factions", adminEntitiesHandler.GetAllFactions)
			admin.GET("/factions/:id", adminEntitiesHandler.GetFaction)
			admin.POST("/factions", adminEntitiesHandler.CreateFaction)
			admin.PUT("/factions/:id", adminEntitiesHandler.UpdateFaction)
			admin.DELETE("/factions/:id", adminEntitiesHandler.DeleteFaction)
			admin.GET("/factions/:id/members", adminEntitiesHandler.GetFactionMembers)

			// ========================================
			// ИГРОКИ
			// ========================================
			admin.GET("/players", adminEntitiesHandler.GetAllPlayers)
			admin.GET("/players/:id", adminEntitiesHandler.GetPlayer)
			admin.POST("/players", adminEntitiesHandler.CreatePlayer)
			admin.PUT("/players/:id", adminEntitiesHandler.UpdatePlayer)
			admin.DELETE("/players/:id", adminEntitiesHandler.DeletePlayer)
			admin.PUT("/players/:id/money", adminEntitiesHandler.UpdatePlayerMoney)
			admin.PUT("/players/:id/influence", adminEntitiesHandler.UpdatePlayerInfluence)

			// ========================================
			// ЦЕЛИ
			// ========================================
			admin.GET("/goals", adminEntitiesHandler.GetAllGoals)
			admin.POST("/goals", adminEntitiesHandler.CreateGoal)
			admin.POST("/goals/batch", adminEntitiesHandler.CreateBatchGoals)
			admin.PUT("/goals/:id", adminEntitiesHandler.UpdateGoal)
			admin.DELETE("/goals/:id", adminEntitiesHandler.DeleteGoal)

			// ========================================
			// ЗАДАЧИ
			// ========================================
			adminItemsTasksHandler := handlers.NewAdminItemsTasksHandler(db)
			admin.GET("/tasks", adminItemsTasksHandler.GetAllTasks)
			admin.POST("/tasks", adminItemsTasksHandler.CreateTask)
			admin.PUT("/tasks/:id", adminItemsTasksHandler.UpdateTask)
			admin.DELETE("/tasks/:id", adminItemsTasksHandler.DeleteTask)

			// ========================================
			// ПРЕДМЕТЫ И ЭФФЕКТЫ
			// ========================================
			admin.GET("/items", adminItemsTasksHandler.GetAllItems)
			admin.GET("/items/:id", adminItemsTasksHandler.GetItem)
			admin.POST("/items", adminItemsTasksHandler.CreateItem)
			admin.PUT("/items/:id", adminItemsTasksHandler.UpdateItem)
			admin.DELETE("/items/:id", adminItemsTasksHandler.DeleteItem)
			admin.POST("/effects", adminItemsTasksHandler.CreateEffect)
			admin.POST("/items/:id/effects", adminItemsTasksHandler.AddEffectToItem)
			admin.DELETE("/items/:id/effects/:effect_id", adminItemsTasksHandler.RemoveEffectFromItem)

			// ========================================
			// СПОСОБНОСТИ
			// ========================================
			admin.GET("/abilities", adminItemsTasksHandler.GetAllAbilities)
			admin.POST("/abilities", adminItemsTasksHandler.CreateAbility)
			admin.PUT("/abilities/:id", adminItemsTasksHandler.UpdateAbility)
			admin.DELETE("/abilities/:id", adminItemsTasksHandler.DeleteAbility)

			// ========================================
			// КОНТРАКТЫ
			// ========================================
			adminMonitoringHandler := handlers.NewAdminMonitoringHandler(db)
			admin.GET("/contracts", adminMonitoringHandler.GetAllContracts)
			admin.GET("/contracts/:id", adminMonitoringHandler.GetContract)
			admin.DELETE("/contracts/:id/terminate", contractsHandlerWithShedular.TerminateContract)

			// Настройки контрактов (уже существуют)
			adminContractHandler := handlers.NewAdminContractHandler(db)
			admin.GET("/contracts/settings", adminContractHandler.GetContractSettings)
			admin.PUT("/contracts/type1/rewards", adminContractHandler.UpdateContractType1Rewards)
			admin.PUT("/contracts/type2/rewards", adminContractHandler.UpdateContractType2Rewards)
			admin.PUT("/contracts/penalties", adminContractHandler.UpdateContractPenalties)

			// Type1 награды по фракциям
			admin.GET("/contracts/type1/faction-rewards", adminMonitoringHandler.GetType1FactionRewards)
			admin.POST("/contracts/type1/faction-rewards", adminMonitoringHandler.SetType1FactionReward)
			admin.DELETE("/contracts/type1/faction-rewards/:faction_id", adminMonitoringHandler.DeleteType1FactionReward)

			// ========================================
			// ДОЛГИ
			// ========================================
			admin.GET("/debts", adminMonitoringHandler.GetAllDebts)
			admin.GET("/debts/:id", adminMonitoringHandler.GetDebt)
			admin.DELETE("/debts/:id", adminMonitoringHandler.DeleteDebt)

			// Настройки долгов (уже существуют)
			adminDebtHandler := handlers.NewAdminDebtHandler(db)
			admin.GET("/debts/settings", adminDebtHandler.GetDebtPenaltySettings)
			admin.PUT("/debts/penalties", adminDebtHandler.UpdateDebtPenaltySettings)

			// ========================================
			// ГОНКА ЦЕЛЕЙ
			// ========================================
			adminGoalRaceHandler := handlers.NewAdminGoalRaceHandler(db)
			admin.GET("/goal-race/triggers", adminGoalRaceHandler.GetTriggers)
			admin.POST("/goal-race/triggers", adminGoalRaceHandler.CreateTrigger)
			admin.PUT("/goal-race/triggers/:id", adminGoalRaceHandler.UpdateTrigger)
			admin.DELETE("/goal-race/triggers/:id", adminGoalRaceHandler.DeleteTrigger)

			admin.GET("/goal-race/predefined-goals", adminGoalRaceHandler.GetPredefinedGoals)
			admin.POST("/goal-race/predefined-goals", adminGoalRaceHandler.CreatePredefinedGoal)
			admin.POST("/goal-race/predefined-goals/batch", adminGoalRaceHandler.CreateBatchPredefinedGoals)
			admin.PUT("/goal-race/predefined-goals/:id", adminGoalRaceHandler.UpdatePredefinedGoal)
			admin.DELETE("/goal-race/predefined-goals/:id", adminGoalRaceHandler.DeletePredefinedGoal)

			admin.GET("/goal-race/history", adminGoalRaceHandler.GetRaceHistory)

			// ========================================
			// МОНИТОРИНГ И ТРАНЗАКЦИИ
			// ========================================
			admin.GET("/transactions", adminMonitoringHandler.GetTransactions)

			// ========================================
			// СВЯЗИ И ЗАВИСИМОСТИ
			// ========================================
			adminRelationsHandler := handlers.NewAdminRelationsHandler(db)

			// Инвентарь игроков
			admin.GET("/players/:id/inventory", adminRelationsHandler.GetPlayerInventory)
			admin.POST("/players/:id/inventory", adminRelationsHandler.AddItemToPlayer)
			admin.POST("/players/:id/inventory/batch", adminRelationsHandler.AddBatchItemsToPlayer)
			admin.DELETE("/players/:id/inventory/:item_id", adminRelationsHandler.RemoveItemFromPlayer)

			// Зависимости целей
			admin.GET("/goals/:id/dependencies", adminRelationsHandler.GetGoalDependencies)
			admin.POST("/goals/:id/dependencies", adminRelationsHandler.AddGoalDependency)
			admin.DELETE("/goals/:id/dependencies/:dependency_id", adminRelationsHandler.DeleteGoalDependency)
			admin.GET("/goals/:id/dependency-unlocks", adminRelationsHandler.GetGoalDependencyUnlocks)

			// Информация о других игроках
			admin.GET("/players/:id/info", adminRelationsHandler.GetPlayerInfo)
			admin.POST("/players/:id/info", adminRelationsHandler.CreatePlayerInfo)
			admin.PUT("/players/:id/info", adminRelationsHandler.UpdatePlayerInfo)
			admin.DELETE("/players/:id/info", adminRelationsHandler.DeletePlayerInfo)

			// Участники триггеров гонки целей
			admin.GET("/goal-race/triggers/:id/participants", adminRelationsHandler.GetTriggerParticipants)
			admin.POST("/goal-race/triggers/:id/participants", adminRelationsHandler.AddTriggerParticipant)
			admin.POST("/goal-race/triggers/:id/participants/batch", adminRelationsHandler.AddBatchTriggerParticipants)
			admin.DELETE("/goal-race/triggers/:id/participants/:player_id", adminRelationsHandler.RemoveTriggerParticipant)

			// История и мониторинг
			admin.GET("/abilities/usage-history", adminRelationsHandler.GetAbilityUsageHistory)
			admin.GET("/players/:id/revealed-info", adminRelationsHandler.GetPlayerRevealedInfo)
			admin.GET("/goal-race/rounds/:id/participants", adminRelationsHandler.GetRoundParticipants)
			admin.GET("/goal-race/rounds/:id/goals", adminRelationsHandler.GetRoundGoals)
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
