package config

import (
	"log"
	"os"
)

type Config struct {
	DatabaseURL string
	JWTKey      string
	Port        string
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

	return &Config{
		DatabaseURL: databaseURL,
		JWTKey:      jwtKey,
		Port:        port,
	}
}
