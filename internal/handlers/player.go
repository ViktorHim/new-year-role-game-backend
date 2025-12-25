// internal/handlers/player.go
package handlers

import (
	"database/sql"
	"net/http"
	"new-year-role-game-backend/internal/models"

	"github.com/gin-gonic/gin"
)

type PlayerHandler struct {
	db *sql.DB
}

func NewPlayerHandler(db *sql.DB) *PlayerHandler {
	return &PlayerHandler{db: db}
}

// GetPlayerInfo возвращает информацию о текущем игроке
func (h *PlayerHandler) GetPlayerInfo(c *gin.Context) {
	// Получаем player_id из контекста (установлен в AuthMiddleware)
	playerIDInterface, exists := c.Get("player_id")
	if !exists || playerIDInterface == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Player ID not found in token"})
		return
	}

	playerID := playerIDInterface.(*int)
	if playerID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User is not associated with a player"})
		return
	}

	// Получаем основную информацию об игроке
	var player models.PlayerResponse
	err := h.db.QueryRow(`
		SELECT 
			id,
			character_name,
			role,
			faction_id,
			can_change_faction,
			character_story,
			avatar
		FROM players
		WHERE id = $1
	`, *playerID).Scan(
		&player.ID,
		&player.Name,
		&player.Role,
		&player.FactionID,
		&player.CanChangeFaction,
		&player.Description,
		&player.Avatar,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Player not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Получаем информацию о других игроках
	rows, err := h.db.Query(`
		SELECT description
		FROM info_about_other_players
		WHERE player_id = $1
	`, *playerID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch player info"})
		return
	}
	defer rows.Close()

	player.InfoAboutPlayers = make([]string, 0)
	for rows.Next() {
		var description string
		if err := rows.Scan(&description); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan player info"})
			return
		}
		player.InfoAboutPlayers = append(player.InfoAboutPlayers, description)
	}

	if err = rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, player)
}

// GetPlayerBalance возвращает информацию о балансе игрока
func (h *PlayerHandler) GetPlayerBalance(c *gin.Context) {
	// Получаем player_id из контекста (установлен в AuthMiddleware)
	playerIDInterface, exists := c.Get("player_id")
	if !exists || playerIDInterface == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Player ID not found in token"})
		return
	}

	playerID := playerIDInterface.(*int)
	if playerID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User is not associated with a player"})
		return
	}

	var balance models.BalanceResponse
	err := h.db.QueryRow(`
		SELECT 
			money,
			influence
		FROM players
		WHERE id = $1
	`, *playerID).Scan(
		&balance.Money,
		&balance.Influence,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch player balance info"})
		return
	}

	c.JSON(http.StatusOK, balance)
}

// GetAllPlayers возвращает список всех игроков
func (h *PlayerHandler) GetAllPlayers(c *gin.Context) {
	// Проверяем авторизацию
	playerIDInterface, exists := c.Get("player_id")
	if !exists || playerIDInterface == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Player ID not found in token"})
		return
	}

	playerID := playerIDInterface.(*int)
	if playerID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User is not associated with a player"})
		return
	}

	// Получаем список всех игроков
	rows, err := h.db.Query(`
		SELECT 
			id,
			character_name,
			avatar
		FROM players
		ORDER BY character_name
	`)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch players"})
		return
	}
	defer rows.Close()

	players := make([]models.PlayerListItem, 0)
	for rows.Next() {
		var player models.PlayerListItem
		err := rows.Scan(
			&player.ID,
			&player.Name,
			&player.Avatar,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan player"})
			return
		}
		players = append(players, player)
	}

	if err = rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, models.PlayersListResponse{Players: players})
}
