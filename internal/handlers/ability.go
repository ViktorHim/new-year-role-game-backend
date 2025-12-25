// internal/handlers/ability.go
package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"new-year-role-game-backend/internal/models"
	"time"

	"github.com/gin-gonic/gin"
)

type AbilityHandler struct {
	db *sql.DB
}

func NewAbilityHandler(db *sql.DB) *AbilityHandler {
	return &AbilityHandler{db: db}
}

// GetPlayerAbilities возвращает все уникальные способности игрока
func (h *AbilityHandler) GetPlayerAbilities(c *gin.Context) {
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

	// Получаем текущее влияние игрока для проверки разблокировки
	var currentInfluence int
	err := h.db.QueryRow(`
		SELECT influence FROM players WHERE id = $1
	`, *playerID).Scan(&currentInfluence)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch player influence"})
		return
	}

	// Получаем время начала игры для проверки start_delay
	var gameStartedAt *time.Time
	err = h.db.QueryRow(`
		SELECT game_started_at
		FROM game_timeline
		ORDER BY id DESC
		LIMIT 1
	`).Scan(&gameStartedAt)

	if err != nil && err != sql.ErrNoRows {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch game timeline"})
		return
	}

	// Получаем способности игрока с информацией о последнем использовании
	rows, err := h.db.Query(`
		SELECT 
			a.id,
			a.player_id,
			a.name,
			a.description,
			a.ability_type,
			a.cooldown_minutes,
			a.start_delay_minutes,
			a.required_influence_points,
			a.is_unlocked,
			a.influence_points_to_add,
			a.influence_points_to_remove,
			a.influence_points_to_self,
			a.created_at,
			au.used_at AS last_used_at
		FROM abilities a
		LEFT JOIN LATERAL (
			SELECT used_at
			FROM ability_usage
			WHERE ability_id = a.id AND player_id = a.player_id
			ORDER BY used_at DESC
			LIMIT 1
		) au ON true
		WHERE a.player_id = $1
		ORDER BY a.created_at
	`, *playerID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch abilities"})
		return
	}
	defer rows.Close()

	abilities := make([]models.Ability, 0)
	now := time.Now()

	for rows.Next() {
		var ability models.Ability
		var lastUsedAt *time.Time

		err := rows.Scan(
			&ability.ID,
			&ability.PlayerID,
			&ability.Name,
			&ability.Description,
			&ability.AbilityType,
			&ability.CooldownMinutes,
			&ability.StartDelayMinutes,
			&ability.RequiredInfluencePoints,
			&ability.IsUnlocked,
			&ability.InfluencePointsToAdd,
			&ability.InfluencePointsToRemove,
			&ability.InfluencePointsToSelf,
			&ability.CreatedAt,
			&lastUsedAt,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan ability"})
			return
		}

		ability.LastUsedAt = lastUsedAt

		// Определяем, можно ли использовать способность сейчас
		canUse, blockReason, nextAvailable := h.checkAbilityAvailability(
			&ability,
			currentInfluence,
			gameStartedAt,
			lastUsedAt,
			now,
		)

		ability.CanUseNow = canUse
		ability.BlockReason = blockReason
		ability.NextAvailableAt = nextAvailable

		abilities = append(abilities, ability)
	}

	if err = rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, models.AbilitiesResponse{Abilities: abilities})
}

// checkAbilityAvailability проверяет, можно ли использовать способность сейчас
func (h *AbilityHandler) checkAbilityAvailability(
	ability *models.Ability,
	currentInfluence int,
	gameStartedAt *time.Time,
	lastUsedAt *time.Time,
	now time.Time,
) (canUse bool, blockReason *string, nextAvailable *time.Time) {

	// Проверка 1: Способность не разблокирована
	if !ability.IsUnlocked {
		// Проверяем условия разблокировки
		if ability.RequiredInfluencePoints != nil {
			if currentInfluence < *ability.RequiredInfluencePoints {
				reason := "Требуется больше очков влияния для разблокировки"
				return false, &reason, nil
			}
			// Условие выполнено, но способность еще не разблокирована в БД
			// (это должно обновляться автоматически, но пока считаем заблокированной)
			reason := "Способность заблокирована"
			return false, &reason, nil
		}
		reason := "Способность заблокирована"
		return false, &reason, nil
	}

	// Проверка 2: Задержка от начала игры
	if ability.StartDelayMinutes != nil && gameStartedAt != nil {
		delayDuration := time.Duration(*ability.StartDelayMinutes) * time.Minute
		availableAt := gameStartedAt.Add(delayDuration)

		if now.Before(availableAt) {
			reason := "Способность станет доступна позже"
			return false, &reason, &availableAt
		}
	}

	// Проверка 3: Cooldown
	if ability.CooldownMinutes != nil && lastUsedAt != nil {
		cooldownDuration := time.Duration(*ability.CooldownMinutes) * time.Minute
		availableAt := lastUsedAt.Add(cooldownDuration)

		if now.Before(availableAt) {
			reason := "Способность на перезарядке"
			return false, &reason, &availableAt
		}
	}

	// Все проверки пройдены
	return true, nil, nil
}

// UseAbility использует уникальную способность
func (h *AbilityHandler) UseAbility(c *gin.Context) {
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

	// Получаем ID способности из URL
	abilityIDStr := c.Param("id")
	var abilityID int
	if _, err := fmt.Sscanf(abilityIDStr, "%d", &abilityID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ability ID"})
		return
	}

	// Парсим тело запроса
	var req models.UseAbilityRequest
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

	// Получаем информацию о способности
	var ability models.Ability
	var lastUsedAt *time.Time
	err = tx.QueryRow(`
		SELECT 
			a.id,
			a.player_id,
			a.name,
			a.ability_type,
			a.cooldown_minutes,
			a.start_delay_minutes,
			a.required_influence_points,
			a.is_unlocked,
			a.influence_points_to_add,
			a.influence_points_to_remove,
			a.influence_points_to_self,
			au.used_at
		FROM abilities a
		LEFT JOIN LATERAL (
			SELECT used_at
			FROM ability_usage
			WHERE ability_id = a.id AND player_id = a.player_id
			ORDER BY used_at DESC
			LIMIT 1
		) au ON true
		WHERE a.id = $1 AND a.player_id = $2
		FOR UPDATE OF a
	`, abilityID, *playerID).Scan(
		&ability.ID,
		&ability.PlayerID,
		&ability.Name,
		&ability.AbilityType,
		&ability.CooldownMinutes,
		&ability.StartDelayMinutes,
		&ability.RequiredInfluencePoints,
		&ability.IsUnlocked,
		&ability.InfluencePointsToAdd,
		&ability.InfluencePointsToRemove,
		&ability.InfluencePointsToSelf,
		&lastUsedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Ability not found or does not belong to you"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Получаем текущее влияние и время начала игры для проверки доступности
	var currentInfluence int
	var gameStartedAt *time.Time

	err = tx.QueryRow(`
		SELECT p.influence, gt.game_started_at
		FROM players p
		LEFT JOIN LATERAL (
			SELECT game_started_at
			FROM game_timeline
			ORDER BY id DESC
			LIMIT 1
		) gt ON true
		WHERE p.id = $1
	`, *playerID).Scan(&currentInfluence, &gameStartedAt)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch player data"})
		return
	}

	// Проверяем доступность способности
	canUse, blockReason, _ := h.checkAbilityAvailability(
		&ability,
		currentInfluence,
		gameStartedAt,
		lastUsedAt,
		time.Now(),
	)

	if !canUse {
		message := "Ability cannot be used"
		if blockReason != nil {
			message = *blockReason
		}
		c.JSON(http.StatusForbidden, gin.H{"error": message})
		return
	}

	// Выполняем способность в зависимости от типа
	var response models.UseAbilityResponse

	switch ability.AbilityType {
	case "reveal_info":
		if req.TargetPlayerID == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "target_player_id is required for reveal_info"})
			return
		}
		if req.InfoCategory == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "info_category is required for reveal_info (faction, goal, or item)"})
			return
		}

		_, revealedInfo, err := h.executeRevealInfo(tx, *playerID, *req.TargetPlayerID, *req.InfoCategory, abilityID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		response.Message = "Information revealed successfully"
		response.RevealedInfo = revealedInfo

	case "add_influence":
		if req.TargetPlayerID == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "target_player_id is required for add_influence"})
			return
		}
		if ability.InfluencePointsToAdd == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid ability configuration"})
			return
		}

		_, err := h.executeAddInfluence(tx, *playerID, *req.TargetPlayerID, *ability.InfluencePointsToAdd, abilityID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		response.Message = fmt.Sprintf("Successfully added %d influence points to target player", *ability.InfluencePointsToAdd)

	case "transfer_influence":
		if req.TargetPlayerID == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "target_player_id is required for transfer_influence"})
			return
		}
		if ability.InfluencePointsToRemove == nil || ability.InfluencePointsToSelf == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid ability configuration"})
			return
		}

		_, err := h.executeTransferInfluence(tx, *playerID, *req.TargetPlayerID, *ability.InfluencePointsToRemove, *ability.InfluencePointsToSelf, abilityID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		response.Message = fmt.Sprintf("Successfully transferred influence: removed %d from target, added %d to yourself", *ability.InfluencePointsToRemove, *ability.InfluencePointsToSelf)

	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Unknown ability type"})
		return
	}

	// Фиксируем транзакцию
	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	c.JSON(http.StatusOK, response)
}

// executeRevealInfo выполняет способность раскрытия информации
func (h *AbilityHandler) executeRevealInfo(tx *sql.Tx, playerID, targetPlayerID int, infoCategory string, abilityID int) (int, *models.RevealedInfoData, error) {
	// Проверяем, что целевой игрок существует
	var targetExists bool
	err := tx.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM players WHERE id = $1)
	`, targetPlayerID).Scan(&targetExists)

	if err != nil || !targetExists {
		return 0, nil, fmt.Errorf("Target player not found")
	}

	// Записываем использование способности
	var usageID int
	err = tx.QueryRow(`
		INSERT INTO ability_usage (player_id, ability_id, target_player_id, info_category, used_at)
		VALUES ($1, $2, $3, $4, NOW())
		RETURNING id
	`, playerID, abilityID, targetPlayerID, infoCategory).Scan(&usageID)

	if err != nil {
		return 0, nil, fmt.Errorf("Failed to record ability usage")
	}

	// Раскрываем информацию в зависимости от категории
	var revealedData models.RevealedInfoData
	var revealedJSON []byte

	switch infoCategory {
	case "faction":
		var factionID *int
		var factionName *string
		err = tx.QueryRow(`
			SELECT p.faction_id, f.name
			FROM players p
			LEFT JOIN factions f ON p.faction_id = f.id
			WHERE p.id = $1
		`, targetPlayerID).Scan(&factionID, &factionName)

		if err != nil {
			return 0, nil, fmt.Errorf("Failed to fetch faction info")
		}

		revealedData.InfoType = "faction"
		if factionID != nil && factionName != nil {
			revealedData.Data = map[string]interface{}{
				"faction_id":   *factionID,
				"faction_name": *factionName,
			}
		} else {
			revealedData.Data = map[string]interface{}{
				"faction_id":   nil,
				"faction_name": "Нейтральный",
			}
		}

	case "goal":
		// Выбираем случайную личную цель целевого игрока
		var goalID int
		var goalTitle string
		var goalDescription *string
		err = tx.QueryRow(`
			SELECT id, title, description
			FROM goals
			WHERE player_id = $1 AND goal_type = 'personal'
			ORDER BY RANDOM()
			LIMIT 1
		`, targetPlayerID).Scan(&goalID, &goalTitle, &goalDescription)

		if err != nil {
			if err == sql.ErrNoRows {
				return 0, nil, fmt.Errorf("Target player has no personal goals")
			}
			return 0, nil, fmt.Errorf("Failed to fetch goal info")
		}

		revealedData.InfoType = "goal"
		revealedData.Data = map[string]interface{}{
			"goal_id":          goalID,
			"goal_title":       goalTitle,
			"goal_description": goalDescription,
		}

	case "item":
		// Выбираем случайный предмет целевого игрока
		var itemID int
		var itemName string
		var itemDescription *string
		err = tx.QueryRow(`
			SELECT i.id, i.name, i.description
			FROM player_items pi
			JOIN items i ON pi.item_id = i.id
			WHERE pi.player_id = $1
			ORDER BY RANDOM()
			LIMIT 1
		`, targetPlayerID).Scan(&itemID, &itemName, &itemDescription)

		if err != nil {
			if err == sql.ErrNoRows {
				return 0, nil, fmt.Errorf("Target player has no items")
			}
			return 0, nil, fmt.Errorf("Failed to fetch item info")
		}

		revealedData.InfoType = "item"
		revealedData.Data = map[string]interface{}{
			"item_id":          itemID,
			"item_name":        itemName,
			"item_description": itemDescription,
		}

	default:
		return 0, nil, fmt.Errorf("Invalid info_category. Must be: faction, goal, or item")
	}

	// Сериализуем данные в JSON для сохранения в БД
	revealedJSON, err = json.Marshal(revealedData.Data)
	if err != nil {
		return 0, nil, fmt.Errorf("Failed to serialize revealed data")
	}

	// Сохраняем раскрытую информацию
	_, err = tx.Exec(`
		INSERT INTO revealed_info (revealer_player_id, target_player_id, info_type, revealed_data, ability_usage_id)
		VALUES ($1, $2, $3, $4, $5)
	`, playerID, targetPlayerID, infoCategory, revealedJSON, usageID)

	if err != nil {
		return 0, nil, fmt.Errorf("Failed to save revealed info")
	}

	return usageID, &revealedData, nil
}

// executeAddInfluence выполняет способность начисления влияния
func (h *AbilityHandler) executeAddInfluence(tx *sql.Tx, playerID, targetPlayerID, points, abilityID int) (int, error) {
	// Проверяем, что целевой игрок существует
	var targetExists bool
	err := tx.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM players WHERE id = $1)
	`, targetPlayerID).Scan(&targetExists)

	if err != nil || !targetExists {
		return 0, fmt.Errorf("Target player not found")
	}

	// Записываем использование способности
	var usageID int
	err = tx.QueryRow(`
		INSERT INTO ability_usage (player_id, ability_id, target_player_id, used_at)
		VALUES ($1, $2, $3, NOW())
		RETURNING id
	`, playerID, abilityID, targetPlayerID).Scan(&usageID)

	if err != nil {
		return 0, fmt.Errorf("Failed to record ability usage")
	}

	// Начисляем влияние целевому игроку
	_, err = tx.Exec(`
		UPDATE players
		SET influence = influence + $1
		WHERE id = $2
	`, points, targetPlayerID)

	if err != nil {
		return 0, fmt.Errorf("Failed to add influence")
	}

	// Записываем транзакцию влияния
	_, err = tx.Exec(`
		INSERT INTO influence_transactions (player_id, amount, transaction_type, reference_id, reference_type, description)
		VALUES ($1, $2, 'ability', $3, 'ability', $4)
	`, targetPlayerID, points, abilityID, fmt.Sprintf("Received %d influence from ability", points))

	if err != nil {
		return 0, fmt.Errorf("Failed to record influence transaction")
	}

	return usageID, nil
}

// executeTransferInfluence выполняет способность переноса влияния
func (h *AbilityHandler) executeTransferInfluence(tx *sql.Tx, playerID, targetPlayerID, pointsToRemove, pointsToSelf, abilityID int) (int, error) {
	// Проверяем, что целевой игрок существует
	var targetExists bool
	var targetInfluence int
	err := tx.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM players WHERE id = $1), 
		       COALESCE((SELECT influence FROM players WHERE id = $1), 0)
	`, targetPlayerID).Scan(&targetExists, &targetInfluence)

	if err != nil || !targetExists {
		return 0, fmt.Errorf("Target player not found")
	}

	// Записываем использование способности
	var usageID int
	err = tx.QueryRow(`
		INSERT INTO ability_usage (player_id, ability_id, target_player_id, used_at)
		VALUES ($1, $2, $3, NOW())
		RETURNING id
	`, playerID, abilityID, targetPlayerID).Scan(&usageID)

	if err != nil {
		return 0, fmt.Errorf("Failed to record ability usage")
	}

	// Снимаем влияние у целевого игрока
	_, err = tx.Exec(`
		UPDATE players
		SET influence = GREATEST(0, influence - $1)
		WHERE id = $2
	`, pointsToRemove, targetPlayerID)

	if err != nil {
		return 0, fmt.Errorf("Failed to remove influence from target")
	}

	// Начисляем влияние себе
	_, err = tx.Exec(`
		UPDATE players
		SET influence = influence + $1
		WHERE id = $2
	`, pointsToSelf, playerID)

	if err != nil {
		return 0, fmt.Errorf("Failed to add influence to self")
	}

	// Записываем транзакции влияния
	actualRemoved := pointsToRemove
	if targetInfluence < pointsToRemove {
		actualRemoved = targetInfluence
	}

	_, err = tx.Exec(`
		INSERT INTO influence_transactions (player_id, amount, transaction_type, reference_id, reference_type, description)
		VALUES 
			($1, $2, 'ability', $3, 'ability', $4),
			($5, $6, 'ability', $3, 'ability', $7)
	`, targetPlayerID, -actualRemoved, abilityID, fmt.Sprintf("Lost %d influence from ability", actualRemoved),
		playerID, pointsToSelf, fmt.Sprintf("Gained %d influence from ability", pointsToSelf))

	if err != nil {
		return 0, fmt.Errorf("Failed to record influence transactions")
	}

	return usageID, nil
}
