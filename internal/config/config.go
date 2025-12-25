// internal/config/config.go
package config

import (
	"log"
	"os"
	"strconv"
)

type Config struct {
	DatabaseURL           string
	JWTKey                string
	Port                  string
	EffectsWorkerInterval int // в секундах
}

func LoadConfig() *Config {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	jwtKey := os.Getenv("JWT_KEY")
	if jwtKey == "" {
		log.Fatal("JWT_KEY environment variable is required")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Интервал выполнения эффектов предметов (по умолчанию 60 секунд)
	effectsWorkerInterval := 60
	if envInterval := os.Getenv("EFFECTS_WORKER_INTERVAL"); envInterval != "" {
		if interval, err := strconv.Atoi(envInterval); err == nil && interval > 0 {
			effectsWorkerInterval = interval
		}
	}

	return &Config{
		DatabaseURL:           databaseURL,
		JWTKey:                jwtKey,
		Port:                  port,
		EffectsWorkerInterval: effectsWorkerInterval,
	}
}
