// internal/handlers/faction.go
package handlers

import "database/sql"

type FactionHandler struct {
	db     *sql.DB
	jwtKey string
}

func NewFactionHandler(db *sql.DB, jwtKey string) *FactionHandler {
	return &FactionHandler{
		db:     db,
		jwtKey: jwtKey,
	}
}
