// cmd/main.go
package main

import (
	"log"
	"new-year-role-game-backend/internal/config"
	"new-year-role-game-backend/internal/database"
	"new-year-role-game-backend/internal/handlers"
	"new-year-role-game-backend/internal/middleware"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.LoadConfig()

	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

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
		}
	}

	log.Printf("Server starting on port %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
