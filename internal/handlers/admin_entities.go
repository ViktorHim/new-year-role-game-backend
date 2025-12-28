// internal/handlers/admin_entities.go
package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type AdminEntitiesHandler struct {
	db *sql.DB
}

func NewAdminEntitiesHandler(db *sql.DB) *AdminEntitiesHandler {
	return &AdminEntitiesHandler{db: db}
}

// ============================================
// ФРАКЦИИ (FACTIONS)
// ============================================

type FactionRequest struct {
	Name                       string `json:"name" binding:"required"`
	Description                string `json:"description"`
	FactionInfluence           int    `json:"faction_influence"`
	IsCompositionVisibleToAll bool   `json:"is_composition_visible_to_all"`
	LeaderPlayerID             *int   `json:"leader_player_id"`
}

type FactionResponse struct {
	ID                        int       `json:"id"`
	Name                      string    `json:"name"`
	Description               string    `json:"description"`
	FactionInfluence          int       `json:"faction_influence"`
	IsCompositionVisibleToAll bool      `json:"is_composition_visible_to_all"`
	LeaderPlayerID            *int      `json:"leader_player_id,omitempty"`
	LeaderName                *string   `json:"leader_name,omitempty"`
	MembersCount              int       `json:"members_count"`
	TotalInfluence            int       `json:"total_influence"`
}

// GetAllFactions возвращает все фракции с детальной информацией
func (h *AdminEntitiesHandler) GetAllFactions(c *gin.Context) {
	rows, err := h.db.Query(`
		SELECT 
			f.id, f.name, f.description, f.faction_influence, 
			f.is_composition_visible_to_all, f.leader_player_id,
			p.character_name as leader_name,
			COUNT(DISTINCT p2.id) as members_count,
			COALESCE(SUM(p2.influence), 0) + f.faction_influence as total_influence
		FROM factions f
		LEFT JOIN players p ON f.leader_player_id = p.id
		LEFT JOIN players p2 ON p2.faction_id = f.id
		GROUP BY f.id, f.name, f.description, f.faction_influence, 
		         f.is_composition_visible_to_all, f.leader_player_id, p.character_name
		ORDER BY f.name
	`)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch factions"})
		return
	}
	defer rows.Close()

	factions := make([]FactionResponse, 0)
	for rows.Next() {
		var faction FactionResponse
		err := rows.Scan(
			&faction.ID,
			&faction.Name,
			&faction.Description,
			&faction.FactionInfluence,
			&faction.IsCompositionVisibleToAll,
			&faction.LeaderPlayerID,
			&faction.LeaderName,
			&faction.MembersCount,
			&faction.TotalInfluence,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan faction"})
			return
		}

		factions = append(factions, faction)
	}

	c.JSON(http.StatusOK, gin.H{"factions": factions})
}

// GetFaction возвращает одну фракцию по ID с детальной информацией
func (h *AdminEntitiesHandler) GetFaction(c *gin.Context) {
	factionIDStr := c.Param("id")
	factionID, err := strconv.Atoi(factionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid faction ID"})
		return
	}

	var faction FactionResponse
	err = h.db.QueryRow(`
		SELECT 
			f.id, f.name, f.description, f.faction_influence, 
			f.is_composition_visible_to_all, f.leader_player_id,
			p.character_name as leader_name,
			COUNT(DISTINCT p2.id) as members_count,
			COALESCE(SUM(p2.influence), 0) + f.faction_influence as total_influence
		FROM factions f
		LEFT JOIN players p ON f.leader_player_id = p.id
		LEFT JOIN players p2 ON p2.faction_id = f.id
		WHERE f.id = $1
		GROUP BY f.id, f.name, f.description, f.faction_influence, 
		         f.is_composition_visible_to_all, f.leader_player_id, p.character_name
	`, factionID).Scan(
		&faction.ID,
		&faction.Name,
		&faction.Description,
		&faction.FactionInfluence,
		&faction.IsCompositionVisibleToAll,
		&faction.LeaderPlayerID,
		&faction.LeaderName,
		&faction.MembersCount,
		&faction.TotalInfluence,
	)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Faction not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch faction"})
		return
	}

	c.JSON(http.StatusOK, faction)
}

// CreateFaction создает новую фракцию
func (h *AdminEntitiesHandler) CreateFaction(c *gin.Context) {
	var req FactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	var factionID int
	err := h.db.QueryRow(`
		INSERT INTO factions (name, description, faction_influence, is_composition_visible_to_all, leader_player_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, req.Name, req.Description, req.FactionInfluence, req.IsCompositionVisibleToAll, req.LeaderPlayerID).Scan(&factionID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create faction"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Faction created successfully",
		"faction_id": factionID,
	})
}

// UpdateFaction обновляет фракцию
func (h *AdminEntitiesHandler) UpdateFaction(c *gin.Context) {
	factionIDStr := c.Param("id")
	factionID, err := strconv.Atoi(factionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid faction ID"})
		return
	}

	var req FactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	result, err := h.db.Exec(`
		UPDATE factions
		SET name = $1, description = $2, faction_influence = $3, 
		    is_composition_visible_to_all = $4, leader_player_id = $5
		WHERE id = $6
	`, req.Name, req.Description, req.FactionInfluence, req.IsCompositionVisibleToAll, 
	   req.LeaderPlayerID, factionID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update faction"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Faction not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Faction updated successfully"})
}

// DeleteFaction удаляет фракцию
func (h *AdminEntitiesHandler) DeleteFaction(c *gin.Context) {
	factionIDStr := c.Param("id")
	factionID, err := strconv.Atoi(factionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid faction ID"})
		return
	}

	result, err := h.db.Exec(`DELETE FROM factions WHERE id = $1`, factionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete faction"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Faction not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Faction deleted successfully"})
}

// GetFactionMembers возвращает участников фракции
func (h *AdminEntitiesHandler) GetFactionMembers(c *gin.Context) {
	factionIDStr := c.Param("id")
	factionID, err := strconv.Atoi(factionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid faction ID"})
		return
	}

	rows, err := h.db.Query(`
		SELECT id, character_name, role, money, influence, can_change_faction
		FROM players
		WHERE faction_id = $1
		ORDER BY character_name
	`, factionID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch faction members"})
		return
	}
	defer rows.Close()

	type Member struct {
		ID               int    `json:"id"`
		CharacterName    string `json:"character_name"`
		Role             string `json:"role"`
		Money            int    `json:"money"`
		Influence        int    `json:"influence"`
		CanChangeFaction bool   `json:"can_change_faction"`
	}

	members := make([]Member, 0)
	for rows.Next() {
		var member Member
		err := rows.Scan(
			&member.ID,
			&member.CharacterName,
			&member.Role,
			&member.Money,
			&member.Influence,
			&member.CanChangeFaction,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan member"})
			return
		}

		members = append(members, member)
	}

	c.JSON(http.StatusOK, gin.H{"members": members})
}

// ============================================
// ИГРОКИ (PLAYERS)
// ============================================

type PlayerRequest struct {
	CharacterName    string  `json:"character_name" binding:"required"`
	Password         string  `json:"password" binding:"required"`
	CharacterStory   string  `json:"character_story"`
	Role             string  `json:"role" binding:"required"`
	Money            int     `json:"money"`
	Influence        int     `json:"influence"`
	FactionID        *int    `json:"faction_id"`
	CanChangeFaction bool    `json:"can_change_faction"`
	Avatar           *string `json:"avatar"`
}

type PlayerResponse struct {
	ID               int       `json:"id"`
	CharacterName    string    `json:"character_name"`
	CharacterStory   string    `json:"character_story"`
	Role             string    `json:"role"`
	Money            int       `json:"money"`
	Influence        int       `json:"influence"`
	FactionID        *int      `json:"faction_id,omitempty"`
	FactionName      *string   `json:"faction_name,omitempty"`
	CanChangeFaction bool      `json:"can_change_faction"`
	Avatar           *string   `json:"avatar,omitempty"`
}

// GetAllPlayers возвращает всех игроков
func (h *AdminEntitiesHandler) GetAllPlayers(c *gin.Context) {
	rows, err := h.db.Query(`
		SELECT 
			p.id, p.character_name, p.character_story, p.role, p.money, 
			p.influence, p.faction_id, f.name as faction_name, 
			p.can_change_faction, p.avatar
		FROM players p
		LEFT JOIN factions f ON p.faction_id = f.id
		ORDER BY p.character_name
	`)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch players"})
		return
	}
	defer rows.Close()

	players := make([]PlayerResponse, 0)
	for rows.Next() {
		var player PlayerResponse
		err := rows.Scan(
			&player.ID,
			&player.CharacterName,
			&player.CharacterStory,
			&player.Role,
			&player.Money,
			&player.Influence,
			&player.FactionID,
			&player.FactionName,
			&player.CanChangeFaction,
			&player.Avatar,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan player"})
			return
		}

		players = append(players, player)
	}

	c.JSON(http.StatusOK, gin.H{"players": players})
}

// GetPlayer возвращает одного игрока по ID
func (h *AdminEntitiesHandler) GetPlayer(c *gin.Context) {
	playerIDStr := c.Param("id")
	playerID, err := strconv.Atoi(playerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid player ID"})
		return
	}

	var player PlayerResponse
	err = h.db.QueryRow(`
		SELECT 
			p.id, p.character_name, p.character_story, p.role, p.money, 
			p.influence, p.faction_id, f.name as faction_name, 
			p.can_change_faction, p.avatar
		FROM players p
		LEFT JOIN factions f ON p.faction_id = f.id
		WHERE p.id = $1
	`, playerID).Scan(
		&player.ID,
		&player.CharacterName,
		&player.CharacterStory,
		&player.Role,
		&player.Money,
		&player.Influence,
		&player.FactionID,
		&player.FactionName,
		&player.CanChangeFaction,
		&player.Avatar,
	)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Player not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch player"})
		return
	}

	c.JSON(http.StatusOK, player)
}

// CreatePlayer создает нового игрока
func (h *AdminEntitiesHandler) CreatePlayer(c *gin.Context) {
	var req PlayerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	var playerID int
	err := h.db.QueryRow(`
		INSERT INTO players (character_name, password, character_story, role, money, 
		                     influence, faction_id, can_change_faction, avatar)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`, req.CharacterName, req.Password, req.CharacterStory, req.Role, req.Money,
	   req.Influence, req.FactionID, req.CanChangeFaction, req.Avatar).Scan(&playerID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create player"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":   "Player created successfully",
		"player_id": playerID,
	})
}

// UpdatePlayer обновляет игрока
func (h *AdminEntitiesHandler) UpdatePlayer(c *gin.Context) {
	playerIDStr := c.Param("id")
	playerID, err := strconv.Atoi(playerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid player ID"})
		return
	}

	var req PlayerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	result, err := h.db.Exec(`
		UPDATE players
		SET character_name = $1, password = $2, character_story = $3, role = $4,
		    money = $5, influence = $6, faction_id = $7, can_change_faction = $8, avatar = $9
		WHERE id = $10
	`, req.CharacterName, req.Password, req.CharacterStory, req.Role, req.Money,
	   req.Influence, req.FactionID, req.CanChangeFaction, req.Avatar, playerID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update player"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Player not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Player updated successfully"})
}

// DeletePlayer удаляет игрока
func (h *AdminEntitiesHandler) DeletePlayer(c *gin.Context) {
	playerIDStr := c.Param("id")
	playerID, err := strconv.Atoi(playerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid player ID"})
		return
	}

	result, err := h.db.Exec(`DELETE FROM players WHERE id = $1`, playerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete player"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Player not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Player deleted successfully"})
}

// UpdatePlayerMoney изменяет деньги игрока
func (h *AdminEntitiesHandler) UpdatePlayerMoney(c *gin.Context) {
	playerIDStr := c.Param("id")
	playerID, err := strconv.Atoi(playerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid player ID"})
		return
	}

	var req struct {
		Money int `json:"money" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	result, err := h.db.Exec(`UPDATE players SET money = $1 WHERE id = $2`, req.Money, playerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update player money"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Player not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Player money updated successfully"})
}

// UpdatePlayerInfluence изменяет очки влияния игрока
func (h *AdminEntitiesHandler) UpdatePlayerInfluence(c *gin.Context) {
	playerIDStr := c.Param("id")
	playerID, err := strconv.Atoi(playerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid player ID"})
		return
	}

	var req struct {
		Influence int `json:"influence" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	result, err := h.db.Exec(`UPDATE players SET influence = $1 WHERE id = $2`, req.Influence, playerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update player influence"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Player not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Player influence updated successfully"})
}

// ============================================
// ЦЕЛИ (GOALS)
// ============================================

type GoalRequest struct {
	Title                  string `json:"title" binding:"required"`
	Description            string `json:"description"`
	GoalType               string `json:"goal_type" binding:"required,oneof=personal faction"`
	InfluencePointsReward  int    `json:"influence_points_reward"`
	PlayerID               *int   `json:"player_id"`
	FactionID              *int   `json:"faction_id"`
}

type GoalResponse struct {
	ID                     int       `json:"id"`
	Title                  string    `json:"title"`
	Description            string    `json:"description"`
	GoalType               string    `json:"goal_type"`
	InfluencePointsReward  int       `json:"influence_points_reward"`
	PlayerID               *int      `json:"player_id,omitempty"`
	PlayerName             *string   `json:"player_name,omitempty"`
	FactionID              *int      `json:"faction_id,omitempty"`
	FactionName            *string   `json:"faction_name,omitempty"`
	IsCompleted            bool      `json:"is_completed"`
	CompletedAt            *time.Time `json:"completed_at,omitempty"`
	CreatedAt              time.Time `json:"created_at"`
}

// GetAllGoals возвращает все цели
func (h *AdminEntitiesHandler) GetAllGoals(c *gin.Context) {
	goalType := c.Query("type") // personal, faction, или пустая строка для всех

	query := `
		SELECT 
			g.id, g.title, g.description, g.goal_type, g.influence_points_reward,
			g.player_id, p.character_name as player_name,
			g.faction_id, f.name as faction_name,
			g.is_completed, g.completed_at, g.created_at
		FROM goals g
		LEFT JOIN players p ON g.player_id = p.id
		LEFT JOIN factions f ON g.faction_id = f.id
	`

	var rows *sql.Rows
	var err error

	if goalType != "" {
		query += " WHERE g.goal_type = $1 ORDER BY g.created_at DESC"
		rows, err = h.db.Query(query, goalType)
	} else {
		query += " ORDER BY g.created_at DESC"
		rows, err = h.db.Query(query)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch goals"})
		return
	}
	defer rows.Close()

	goals := make([]GoalResponse, 0)
	for rows.Next() {
		var goal GoalResponse
		err := rows.Scan(
			&goal.ID,
			&goal.Title,
			&goal.Description,
			&goal.GoalType,
			&goal.InfluencePointsReward,
			&goal.PlayerID,
			&goal.PlayerName,
			&goal.FactionID,
			&goal.FactionName,
			&goal.IsCompleted,
			&goal.CompletedAt,
			&goal.CreatedAt,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan goal"})
			return
		}

		goals = append(goals, goal)
	}

	c.JSON(http.StatusOK, gin.H{"goals": goals})
}

// CreateGoal создает новую цель
func (h *AdminEntitiesHandler) CreateGoal(c *gin.Context) {
	var req GoalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Валидация
	if req.GoalType == "personal" && req.PlayerID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "player_id is required for personal goals"})
		return
	}
	if req.GoalType == "faction" && req.FactionID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "faction_id is required for faction goals"})
		return
	}

	var goalID int
	err := h.db.QueryRow(`
		INSERT INTO goals (title, description, goal_type, influence_points_reward, player_id, faction_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`, req.Title, req.Description, req.GoalType, req.InfluencePointsReward, req.PlayerID, req.FactionID).Scan(&goalID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create goal"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Goal created successfully",
		"goal_id": goalID,
	})
}

// UpdateGoal обновляет цель
func (h *AdminEntitiesHandler) UpdateGoal(c *gin.Context) {
	goalIDStr := c.Param("id")
	goalID, err := strconv.Atoi(goalIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid goal ID"})
		return
	}

	var req GoalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	result, err := h.db.Exec(`
		UPDATE goals
		SET title = $1, description = $2, influence_points_reward = $3
		WHERE id = $4
	`, req.Title, req.Description, req.InfluencePointsReward, goalID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update goal"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Goal not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Goal updated successfully"})
}

// DeleteGoal удаляет цель
func (h *AdminEntitiesHandler) DeleteGoal(c *gin.Context) {
	goalIDStr := c.Param("id")
	goalID, err := strconv.Atoi(goalIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid goal ID"})
		return
	}

	result, err := h.db.Exec(`DELETE FROM goals WHERE id = $1`, goalID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete goal"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Goal not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Goal deleted successfully"})
}

// CreateBatchGoals массовое создание целей
func (h *AdminEntitiesHandler) CreateBatchGoals(c *gin.Context) {
	var req struct {
		Goals []GoalRequest `json:"goals" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	tx, err := h.db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	createdGoalIDs := make([]int, 0)
	for _, goal := range req.Goals {
		// Валидация
		if goal.GoalType == "personal" && goal.PlayerID == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "player_id is required for personal goals"})
			return
		}
		if goal.GoalType == "faction" && goal.FactionID == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "faction_id is required for faction goals"})
			return
		}

		var goalID int
		err := tx.QueryRow(`
			INSERT INTO goals (title, description, goal_type, influence_points_reward, player_id, faction_id)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING id
		`, goal.Title, goal.Description, goal.GoalType, goal.InfluencePointsReward, goal.PlayerID, goal.FactionID).Scan(&goalID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create goal"})
			return
		}

		createdGoalIDs = append(createdGoalIDs, goalID)
	}

	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":  "Goals created successfully",
		"goal_ids": createdGoalIDs,
		"count":    len(createdGoalIDs),
	})
}