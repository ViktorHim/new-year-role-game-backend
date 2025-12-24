// internal/handlers/faction.go
package handlers

import (
	"database/sql"
	"net/http"
	"new-year-role-game-backend/internal/models"

	"github.com/gin-gonic/gin"
)

type FactionHandler struct {
	db *sql.DB
}

func NewFactionHandler(db *sql.DB) *FactionHandler {
	return &FactionHandler{db: db}
}

// GetPlayerFaction возвращает информацию о фракции текущего игрока
func (h *FactionHandler) GetPlayerFaction(c *gin.Context) {
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

	// Получаем faction_id игрока
	var factionID *int
	err := h.db.QueryRow(`
		SELECT faction_id
		FROM players
		WHERE id = $1
	`, *playerID).Scan(&factionID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Если игрок не состоит во фракции
	if factionID == nil {
		c.JSON(http.StatusOK, gin.H{"faction": nil})
		return
	}

	// Получаем информацию о фракции
	faction, err := h.getFactionInfo(*factionID, playerID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Faction not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch faction info"})
		return
	}

	c.JSON(http.StatusOK, faction)
}

// GetAllFactions возвращает список всех фракций
func (h *FactionHandler) GetAllFactions(c *gin.Context) {
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

	// Получаем faction_id текущего игрока
	var currentPlayerFactionID *int
	err := h.db.QueryRow(`
		SELECT faction_id
		FROM players
		WHERE id = $1
	`, *playerID).Scan(&currentPlayerFactionID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Получаем список всех фракций
	rows, err := h.db.Query(`
		SELECT 
			f.id,
			f.name,
			f.description,
			f.faction_influence,
			f.is_composition_visible_to_all,
			f.leader_player_id,
			fti.total_influence
		FROM factions f
		LEFT JOIN faction_total_influence fti ON f.id = fti.faction_id
		ORDER BY f.name
	`)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch factions"})
		return
	}
	defer rows.Close()

	factions := make([]models.FactionResponse, 0)
	for rows.Next() {
		var faction models.FactionResponse
		var totalInfluence *int

		err := rows.Scan(
			&faction.ID,
			&faction.Name,
			&faction.Description,
			&faction.FactionInfluence,
			&faction.IsCompositionVisibleToAll,
			&faction.LeaderPlayerID,
			&totalInfluence,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan faction"})
			return
		}

		// Устанавливаем total_influence
		if totalInfluence != nil {
			faction.TotalInfluence = *totalInfluence
		} else {
			faction.TotalInfluence = faction.FactionInfluence
		}

		// Проверяем, является ли текущий игрок членом фракции
		faction.IsCurrentPlayerMember = currentPlayerFactionID != nil && *currentPlayerFactionID == faction.ID

		// Проверяем, является ли текущий игрок лидером фракции
		faction.IsCurrentPlayerLeader = faction.LeaderPlayerID != nil &&
			*faction.LeaderPlayerID == *playerID

		// Определяем, показывать ли состав фракции
		canSeeComposition := faction.IsCompositionVisibleToAll || faction.IsCurrentPlayerMember

		if canSeeComposition {
			members, err := h.getFactionMembers(faction.ID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch faction members"})
				return
			}
			faction.Members = &members
		}

		factions = append(factions, faction)
	}

	if err = rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, models.FactionsListResponse{Factions: factions})
}

// getFactionInfo - вспомогательная функция для получения информации о фракции
func (h *FactionHandler) getFactionInfo(factionID int, playerID *int) (*models.FactionResponse, error) {
	var faction models.FactionResponse
	var totalInfluence *int

	err := h.db.QueryRow(`
		SELECT 
			f.id,
			f.name,
			f.description,
			f.faction_influence,
			f.is_composition_visible_to_all,
			f.leader_player_id,
			fti.total_influence
		FROM factions f
		LEFT JOIN faction_total_influence fti ON f.id = fti.faction_id
		WHERE f.id = $1
	`, factionID).Scan(
		&faction.ID,
		&faction.Name,
		&faction.Description,
		&faction.FactionInfluence,
		&faction.IsCompositionVisibleToAll,
		&faction.LeaderPlayerID,
		&totalInfluence,
	)

	if err != nil {
		return nil, err
	}

	// Устанавливаем total_influence
	if totalInfluence != nil {
		faction.TotalInfluence = *totalInfluence
	} else {
		faction.TotalInfluence = faction.FactionInfluence
	}

	// Игрок является членом своей фракции
	faction.IsCurrentPlayerMember = true

	// Проверяем, является ли игрок лидером
	faction.IsCurrentPlayerLeader = faction.LeaderPlayerID != nil &&
		playerID != nil &&
		*faction.LeaderPlayerID == *playerID

	// Игрок всегда видит состав своей фракции
	members, err := h.getFactionMembers(factionID)
	if err != nil {
		return nil, err
	}
	faction.Members = &members

	return &faction, nil
}

// getFactionMembers - вспомогательная функция для получения членов фракции
func (h *FactionHandler) getFactionMembers(factionID int) ([]models.FactionMember, error) {
	rows, err := h.db.Query(`
		SELECT 
			id,
			character_name,
			role,
			influence,
			avatar
		FROM players
		WHERE faction_id = $1
		ORDER BY influence DESC, character_name
	`, factionID)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := make([]models.FactionMember, 0)
	for rows.Next() {
		var member models.FactionMember
		err := rows.Scan(
			&member.ID,
			&member.CharacterName,
			&member.Role,
			&member.Influence,
			&member.Avatar,
		)
		if err != nil {
			return nil, err
		}
		members = append(members, member)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return members, nil
}

// ChangeFaction позволяет игроку вступить во фракцию или сменить фракцию
func (h *FactionHandler) ChangeFaction(c *gin.Context) {
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

	var req models.ChangeFactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Начинаем транзакцию
	tx, err := h.db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	// Получаем текущую информацию об игроке
	var currentFactionID *int
	var canChangeFaction bool
	err = tx.QueryRow(`
		SELECT faction_id, can_change_faction
		FROM players
		WHERE id = $1
		FOR UPDATE
	`, *playerID).Scan(&currentFactionID, &canChangeFaction)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch player info"})
		return
	}

	// Проверяем, что целевая фракция существует
	var factionExists bool
	err = tx.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM factions WHERE id = $1)
	`, req.FactionID).Scan(&factionExists)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if !factionExists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Faction not found"})
		return
	}

	// Проверяем, что игрок не пытается "сменить" фракцию на ту же самую
	if currentFactionID != nil && *currentFactionID == req.FactionID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "You are already in this faction"})
		return
	}

	// Логика проверки прав на смену фракции
	if currentFactionID == nil {
		// Игрок нейтральный (без фракции)
		// Может вступить во фракцию, если can_change_faction = true
		// (это флаг используется для отслеживания одноразового вступления)
		if !canChangeFaction {
			c.JSON(http.StatusForbidden, gin.H{"error": "You have already used your one-time faction join"})
			return
		}
	} else {
		// Игрок уже во фракции
		// Может сменить фракцию, только если can_change_faction = true
		if !canChangeFaction {
			c.JSON(http.StatusForbidden, gin.H{"error": "You cannot change your faction"})
			return
		}
	}

	// Обновляем фракцию игрока и снимаем возможность смены
	_, err = tx.Exec(`
		UPDATE players
		SET faction_id = $1, can_change_faction = false
		WHERE id = $2
	`, req.FactionID, *playerID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update faction"})
		return
	}

	// Фиксируем транзакцию
	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	// Получаем обновленную информацию о фракции
	faction, err := h.getFactionInfo(req.FactionID, playerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch updated faction info"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Faction changed successfully",
		"faction": faction,
	})
}
