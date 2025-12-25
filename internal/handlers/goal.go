// internal/handlers/goal.go (версия с поддержкой unlocks)
package handlers

import (
	"database/sql"
	"net/http"
	"new-year-role-game-backend/internal/models"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type GoalHandler struct {
	db *sql.DB
}

func NewGoalHandler(db *sql.DB) *GoalHandler {
	return &GoalHandler{db: db}
}

// GetPersonalGoals возвращает личные цели игрока (только видимые)
func (h *GoalHandler) GetPersonalGoals(c *gin.Context) {
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

	// Получаем видимые личные цели игрока с информацией о блокировке
	rows, err := h.db.Query(`
		SELECT 
			g.id,
			g.title,
			g.description,
			g.goal_type,
			g.influence_points_reward,
			g.player_id,
			g.is_completed,
			g.completed_at,
			g.created_at,
			pvg.is_visible,
			pvg.is_locked
		FROM goals g
		LEFT JOIN player_visible_goals pvg ON g.id = pvg.id
		WHERE g.goal_type = 'personal' AND g.player_id = $1
		ORDER BY g.is_completed ASC, g.created_at ASC
	`, *playerID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch personal goals"})
		return
	}
	defer rows.Close()

	goals := make([]models.GoalWithLockStatus, 0)
	for rows.Next() {
		var goal models.GoalWithLockStatus
		err := rows.Scan(
			&goal.ID,
			&goal.Title,
			&goal.Description,
			&goal.GoalType,
			&goal.InfluencePointsReward,
			&goal.PlayerID,
			&goal.IsCompleted,
			&goal.CompletedAt,
			&goal.CreatedAt,
			&goal.IsVisible,
			&goal.IsLocked,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan goal"})
			return
		}

		// Показываем только видимые цели
		if goal.IsVisible {
			// Получаем информацию о зависимостях (как выполненных, так и невыполненных)
			dependencies, err := h.getGoalDependencies(goal.ID)
			if err == nil && len(dependencies) > 0 {
				goal.Dependencies = dependencies
			}
			goals = append(goals, goal)
		}
	}

	if err = rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, models.PersonalGoalsResponseWithLock{Goals: goals})
}

// getGoalDependencies возвращает информацию о зависимостях цели с учётом unlocks
func (h *GoalHandler) getGoalDependencies(goalID int) ([]models.GoalDependency, error) {
	rows, err := h.db.Query(`
		SELECT 
			gd.id,
			gd.dependency_type,
			gd.required_goal_id,
			rg.title AS required_goal_title,
			rg.is_completed AS required_goal_completed,
			gd.influence_player_id,
			p.character_name AS influence_player_name,
			p.influence AS current_influence,
			gd.required_influence_points,
			gdu.unlocked_at,
			CASE 
				WHEN gdu.id IS NOT NULL THEN true
				WHEN gd.dependency_type = 'goal_completion' AND rg.is_completed = true THEN true
				WHEN gd.dependency_type = 'influence_threshold' AND p.influence >= gd.required_influence_points THEN true
				ELSE false
			END AS is_satisfied
		FROM goal_dependencies gd
		LEFT JOIN goals rg ON gd.required_goal_id = rg.id
		LEFT JOIN players p ON gd.influence_player_id = p.id
		LEFT JOIN goal_dependency_unlocks gdu ON gd.id = gdu.dependency_id AND gd.goal_id = gdu.goal_id
		WHERE gd.goal_id = $1
		ORDER BY gd.created_at
	`, goalID)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	dependencies := make([]models.GoalDependency, 0)
	for rows.Next() {
		var dep models.GoalDependency
		var dependencyID int
		var requiredGoalID *int
		var requiredGoalTitle *string
		var requiredGoalCompleted *bool
		var influencePlayerID *int
		var influencePlayerName *string
		var currentInfluence *int
		var requiredInfluence *int
		var unlockedAt *time.Time
		var isSatisfied bool

		err := rows.Scan(
			&dependencyID,
			&dep.DependencyType,
			&requiredGoalID,
			&requiredGoalTitle,
			&requiredGoalCompleted,
			&influencePlayerID,
			&influencePlayerName,
			&currentInfluence,
			&requiredInfluence,
			&unlockedAt,
			&isSatisfied,
		)

		if err != nil {
			return nil, err
		}

		dep.IsSatisfied = isSatisfied
		dep.UnlockedAt = unlockedAt

		// Заполняем данные в зависимости от типа
		if dep.DependencyType == "goal_completion" {
			dep.RequiredGoalID = requiredGoalID
			dep.RequiredGoalTitle = requiredGoalTitle
			dep.RequiredGoalCompleted = requiredGoalCompleted
		} else if dep.DependencyType == "influence_threshold" {
			dep.InfluencePlayerID = influencePlayerID
			dep.InfluencePlayerName = influencePlayerName
			dep.CurrentInfluence = currentInfluence
			dep.RequiredInfluence = requiredInfluence
		}

		dependencies = append(dependencies, dep)
	}

	return dependencies, nil
}

// GetFactionGoals возвращает командные цели фракции игрока
func (h *GoalHandler) GetFactionGoals(c *gin.Context) {
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
		c.JSON(http.StatusOK, models.FactionGoalsResponse{Goals: []models.Goal{}})
		return
	}

	// Получаем командные цели фракции
	rows, err := h.db.Query(`
		SELECT 
			id,
			title,
			description,
			goal_type,
			influence_points_reward,
			faction_id,
			is_completed,
			completed_at,
			created_at
		FROM goals
		WHERE goal_type = 'faction' AND faction_id = $1
		ORDER BY is_completed ASC, created_at ASC
	`, *factionID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch faction goals"})
		return
	}
	defer rows.Close()

	goals := make([]models.Goal, 0)
	for rows.Next() {
		var goal models.Goal
		goal.IsVisible = true // Командные цели всегда видимы членам фракции

		err := rows.Scan(
			&goal.ID,
			&goal.Title,
			&goal.Description,
			&goal.GoalType,
			&goal.InfluencePointsReward,
			&goal.FactionID,
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

	if err = rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, models.FactionGoalsResponse{Goals: goals})
}

// ToggleGoalCompletion отмечает цель как выполненную или невыполненную
func (h *GoalHandler) ToggleGoalCompletion(c *gin.Context) {
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

	goalIDStr := c.Param("id")
	goalID, err := strconv.Atoi(goalIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid goal ID"})
		return
	}

	var req models.CompleteGoalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Проверяем, что is_completed был передан
	if req.IsCompleted == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "is_completed is required"})
		return
	}

	isCompleted := *req.IsCompleted

	// Начинаем транзакцию
	tx, err := h.db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	// Получаем информацию о цели
	var goal models.Goal
	var currentCompleted bool
	err = tx.QueryRow(`
		SELECT 
			id,
			title,
			goal_type,
			influence_points_reward,
			player_id,
			faction_id,
			is_completed
		FROM goals
		WHERE id = $1
		FOR UPDATE
	`, goalID).Scan(
		&goal.ID,
		&goal.Title,
		&goal.GoalType,
		&goal.InfluencePointsReward,
		&goal.PlayerID,
		&goal.FactionID,
		&currentCompleted,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Goal not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Проверяем права доступа
	if goal.GoalType == "personal" {
		// Личную цель может выполнять только владелец
		if goal.PlayerID == nil || *goal.PlayerID != *playerID {
			c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to modify this goal"})
			return
		}

		// Проверяем, не заблокирована ли цель (если пытаемся отметить как выполненную)
		if isCompleted && !currentCompleted {
			var isLocked bool
			err = tx.QueryRow(`
				SELECT is_locked FROM player_visible_goals WHERE id = $1
			`, goalID).Scan(&isLocked)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check goal status"})
				return
			}

			if isLocked {
				c.JSON(http.StatusForbidden, gin.H{"error": "Goal is locked. Complete required dependencies first."})
				return
			}
		}
	} else if goal.GoalType == "faction" {
		// Командную цель может выполнять только лидер фракции
		var isLeader bool
		err = tx.QueryRow(`
			SELECT EXISTS(
				SELECT 1 FROM factions 
				WHERE id = $1 AND leader_player_id = $2
			)
		`, goal.FactionID, *playerID).Scan(&isLeader)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}

		if !isLeader {
			c.JSON(http.StatusForbidden, gin.H{"error": "Only faction leader can complete faction goals"})
			return
		}
	}

	// Проверяем, что статус действительно меняется
	if currentCompleted == isCompleted {
		action := "completed"
		if !isCompleted {
			action = "incomplete"
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "Goal is already " + action})
		return
	}

	// Обновляем статус цели
	var completedAt *time.Time
	if isCompleted {
		now := time.Now()
		completedAt = &now
	}

	_, err = tx.Exec(`
		UPDATE goals
		SET is_completed = $1, completed_at = $2
		WHERE id = $3
	`, isCompleted, completedAt, goalID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update goal"})
		return
	}

	// Начисляем или снимаем очки влияния
	var influenceChange int
	var action string
	if isCompleted {
		influenceChange = goal.InfluencePointsReward
		action = "completed"
	} else {
		influenceChange = -goal.InfluencePointsReward - 5
		action = "uncompleted"
	}

	// Обновляем влияние игрока (для личных целей) или фракции (для командных)
	if goal.GoalType == "personal" {
		_, err = tx.Exec(`
			UPDATE players
			SET influence = influence + $1
			WHERE id = $2
		`, influenceChange, *playerID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update player influence"})
			return
		}

		// Записываем транзакцию влияния
		_, err = tx.Exec(`
			INSERT INTO influence_transactions (player_id, amount, transaction_type, reference_id, reference_type, description)
			VALUES ($1, $2, 'goal', $3, 'goal', $4)
		`, *playerID, influenceChange, goalID, "Goal "+action+": "+goal.Title)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record influence transaction"})
			return
		}

		// ВАЖНО: После изменения influence срабатывает триггер unlock_goal_dependencies_on_influence_change
		// который автоматически разблокирует зависимости других целей если нужно
	} else if goal.GoalType == "faction" {
		_, err = tx.Exec(`
			UPDATE factions
			SET faction_influence = faction_influence + $1
			WHERE id = $2
		`, influenceChange, *goal.FactionID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update faction influence"})
			return
		}

		// Записываем транзакцию влияния (для отслеживания)
		_, err = tx.Exec(`
			INSERT INTO influence_transactions (player_id, amount, transaction_type, reference_id, reference_type, description)
			VALUES ($1, $2, 'goal', $3, 'goal', $4)
		`, *playerID, influenceChange, goalID, "Faction goal "+action+": "+goal.Title)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record influence transaction"})
			return
		}
	}

	// Записываем в историю выполнения целей
	_, err = tx.Exec(`
		INSERT INTO goal_completion_history (goal_id, player_id, action, influence_change)
		VALUES ($1, $2, $3, $4)
	`, goalID, *playerID, action, influenceChange)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record goal completion history"})
		return
	}

	// ВАЖНО: После обновления is_completed срабатывает триггер unlock_goal_dependencies_on_goal_completion
	// который автоматически разблокирует зависимости других целей от этой цели

	// Фиксируем транзакцию
	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	message := "Goal marked as completed"
	if !isCompleted {
		message = "Goal marked as incomplete"
	}

	c.JSON(http.StatusOK, gin.H{
		"message":          message,
		"influence_change": influenceChange,
	})
}
