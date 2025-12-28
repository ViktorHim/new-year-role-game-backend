// internal/handlers/goal_race.go
package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"new-year-role-game-backend/internal/models"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type GoalRaceHandler struct {
	db *sql.DB
}

func NewGoalRaceHandler(db *sql.DB) *GoalRaceHandler {
	return &GoalRaceHandler{db: db}
}

// CheckTaskCompletionAndTriggerRace проверяет, выполнил ли игрок достаточно задач для запуска гонки
// Вызывается автоматически при отметке задачи как выполненной
func (h *GoalRaceHandler) CheckTaskCompletionAndTriggerRace(playerID int) error {
	// Получаем количество выполненных задач игрока
	var completedTasksCount int
	err := h.db.QueryRow(`
		SELECT COUNT(*) 
		FROM tasks 
		WHERE player_id = $1 AND is_completed = true
	`, playerID).Scan(&completedTasksCount)

	if err != nil {
		return fmt.Errorf("failed to count completed tasks: %w", err)
	}

	// Проверяем, есть ли активный триггер для этого игрока
	rows, err := h.db.Query(`
		SELECT grt.id, grt.required_tasks_count
		FROM goal_race_triggers grt
		JOIN goal_race_trigger_participants grtp ON grt.id = grtp.trigger_id
		WHERE grtp.player_id = $1 AND grt.is_active = true
	`, playerID)

	if err != nil {
		return fmt.Errorf("failed to fetch triggers: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var triggerID, requiredTasksCount int
		if err := rows.Scan(&triggerID, &requiredTasksCount); err != nil {
			continue
		}

		// Проверяем, достаточно ли выполнено задач
		if completedTasksCount >= requiredTasksCount {
			// Проверяем, не запущена ли уже гонка для этого триггера
			var activeRoundExists bool
			err = h.db.QueryRow(`
				SELECT EXISTS(
					SELECT 1 FROM goal_race_rounds 
					WHERE trigger_id = $1 AND status IN ('pending', 'active')
				)
			`, triggerID).Scan(&activeRoundExists)

			if err == nil && !activeRoundExists {
				// Запускаем первый раунд гонки
				if err := h.startFirstRound(triggerID); err != nil {
					return fmt.Errorf("failed to start first round: %w", err)
				}
			}
		}
	}

	return nil
}

// startFirstRound запускает первый раунд гонки целей
func (h *GoalRaceHandler) startFirstRound(triggerID int) error {
	tx, err := h.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now()

	// Создаем первый раунд
	var roundID int
	err = tx.QueryRow(`
		INSERT INTO goal_race_rounds (trigger_id, round_number, status, started_at)
		VALUES ($1, 1, 'active', $2)
		RETURNING id
	`, triggerID, now).Scan(&roundID)

	if err != nil {
		return fmt.Errorf("failed to create round: %w", err)
	}

	// Получаем участников триггера
	participantRows, err := tx.Query(`
		SELECT player_id 
		FROM goal_race_trigger_participants 
		WHERE trigger_id = $1
	`, triggerID)

	if err != nil {
		return fmt.Errorf("failed to fetch participants: %w", err)
	}

	participants := make([]int, 0)
	for participantRows.Next() {
		var playerID int
		if err := participantRows.Scan(&playerID); err != nil {
			participantRows.Close()
			return err
		}
		participants = append(participants, playerID)
	}
	participantRows.Close()

	if err = participantRows.Err(); err != nil {
		return fmt.Errorf("error reading participants: %w", err)
	}

	// Теперь добавляем участников в раунд
	for _, playerID := range participants {
		_, err = tx.Exec(`
			INSERT INTO goal_race_round_participants (round_id, player_id)
			VALUES ($1, $2)
		`, roundID, playerID)

		if err != nil {
			return fmt.Errorf("failed to add participant to round: %w", err)
		}
	}

	// Создаем цели для участников из предопределенных
	if err := h.assignPredefinedGoals(tx, triggerID, roundID, 1, participants); err != nil {
		return fmt.Errorf("failed to assign goals: %w", err)
	}

	// Скрываем задачи участников
	for _, playerID := range participants {
		_, err = tx.Exec(`
			UPDATE tasks 
			SET is_completed = true 
			WHERE player_id = $1
		`, playerID)

		if err != nil {
			return fmt.Errorf("failed to hide tasks: %w", err)
		}
	}

	return tx.Commit()
}

// assignPredefinedGoals назначает предопределенные цели игрокам в раунде
func (h *GoalRaceHandler) assignPredefinedGoals(tx *sql.Tx, triggerID, roundID, roundNumber int, participants []int) error {
	// Структура для хранения предопределенной цели
	type PredefinedGoalData struct {
		PlayerID        int
		Title           string
		Description     *string
		InfluenceReward int
	}

	// Сначала собираем все предопределенные цели
	predefinedGoals := make([]PredefinedGoalData, 0)

	for _, playerID := range participants {
		predefinedRows, err := tx.Query(`
			SELECT title, description, influence_points_reward
			FROM goal_race_predefined_goals
			WHERE trigger_id = $1 AND round_number = $2 AND player_id = $3
		`, triggerID, roundNumber, playerID)

		if err != nil {
			return fmt.Errorf("failed to fetch predefined goals: %w", err)
		}

		for predefinedRows.Next() {
			var goal PredefinedGoalData
			goal.PlayerID = playerID

			if err := predefinedRows.Scan(&goal.Title, &goal.Description, &goal.InfluenceReward); err != nil {
				predefinedRows.Close()
				return err
			}

			predefinedGoals = append(predefinedGoals, goal)
		}
		predefinedRows.Close()

		if err = predefinedRows.Err(); err != nil {
			return fmt.Errorf("error reading predefined goals: %w", err)
		}
	}

	// Теперь создаем цели и связываем их с раундом
	for _, goal := range predefinedGoals {
		// Создаем цель в таблице goals
		var goalID int
		err := tx.QueryRow(`
			INSERT INTO goals (title, description, goal_type, influence_points_reward, player_id)
			VALUES ($1, $2, 'personal', $3, $4)
			RETURNING id
		`, goal.Title, goal.Description, goal.InfluenceReward, goal.PlayerID).Scan(&goalID)

		if err != nil {
			return fmt.Errorf("failed to create goal: %w", err)
		}

		// Связываем цель с раундом
		_, err = tx.Exec(`
			INSERT INTO goal_race_round_goals (round_id, goal_id, assigned_player_id, is_accessible)
			VALUES ($1, $2, $3, true)
		`, roundID, goalID, goal.PlayerID)

		if err != nil {
			return fmt.Errorf("failed to link goal to round: %w", err)
		}
	}

	return nil
}

// CheckRoundCompletion проверяет, завершил ли игрок все свои цели в раунде
// Вызывается при отметке цели как выполненной
func (h *GoalRaceHandler) CheckRoundCompletion(playerID, goalID int) error {
	// Проверяем, относится ли эта цель к активному раунду гонки
	var roundID, triggerID, roundNumber int
	var totalGoals, completedGoals int

	err := h.db.QueryRow(`
		SELECT 
			grr.id,
			grr.trigger_id,
			grr.round_number,
			COUNT(grrg.id) AS total_goals,
			COUNT(CASE WHEN g.is_completed = true THEN 1 END) AS completed_goals
		FROM goal_race_round_goals grrg
		JOIN goal_race_rounds grr ON grrg.round_id = grr.id
		JOIN goals g ON grrg.goal_id = g.id
		WHERE grrg.assigned_player_id = $1 
			AND grrg.is_accessible = true
			AND grr.status = 'active'
		GROUP BY grr.id, grr.trigger_id, grr.round_number
		HAVING COUNT(grrg.id) = COUNT(CASE WHEN g.is_completed = true THEN 1 END)
	`, playerID).Scan(&roundID, &triggerID, &roundNumber, &totalGoals, &completedGoals)

	if err != nil {
		if err == sql.ErrNoRows {
			// Игрок еще не завершил все цели или это не цель из гонки
			return nil
		}
		return fmt.Errorf("failed to check round completion: %w", err)
	}

	// Игрок завершил все свои цели - начинаем новый раунд
	return h.startNextRound(roundID, triggerID, roundNumber, playerID)
}

// startNextRound завершает текущий раунд и запускает следующий
func (h *GoalRaceHandler) startNextRound(currentRoundID, triggerID, currentRoundNumber, winnerPlayerID int) error {
	tx, err := h.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now()

	// Помечаем текущий раунд как завершенный
	_, err = tx.Exec(`
		UPDATE goal_race_rounds
		SET status = 'completed', completed_at = $1, winner_player_id = $2
		WHERE id = $3
	`, now, winnerPlayerID, currentRoundID)

	if err != nil {
		return fmt.Errorf("failed to complete round: %w", err)
	}

	// Помечаем все цели текущего раунда как недоступные
	_, err = tx.Exec(`
		UPDATE goal_race_round_goals
		SET is_accessible = false, became_inaccessible_at = $1
		WHERE round_id = $2
	`, now, currentRoundID)

	if err != nil {
		return fmt.Errorf("failed to mark goals as inaccessible: %w", err)
	}

	// Получаем участников текущего раунда
	participantRows, err := tx.Query(`
		SELECT player_id 
		FROM goal_race_round_participants 
		WHERE round_id = $1
	`, currentRoundID)

	if err != nil {
		return fmt.Errorf("failed to fetch participants: %w", err)
	}

	participants := make([]int, 0)
	for participantRows.Next() {
		var playerID int
		if err := participantRows.Scan(&playerID); err != nil {
			participantRows.Close()
			return err
		}
		participants = append(participants, playerID)
	}
	participantRows.Close()

	if err = participantRows.Err(); err != nil {
		return fmt.Errorf("error reading participants: %w", err)
	}

	nextRoundNumber := currentRoundNumber + 1

	// Проверяем, есть ли предопределенные цели для следующего раунда
	var hasPredefinedGoals bool
	err = tx.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM goal_race_predefined_goals
			WHERE trigger_id = $1 AND round_number = $2
		)
	`, triggerID, nextRoundNumber).Scan(&hasPredefinedGoals)

	if err != nil || !hasPredefinedGoals {
		// Нет целей для следующего раунда - гонка завершена
		return tx.Commit()
	}

	// Создаем следующий раунд
	var nextRoundID int
	err = tx.QueryRow(`
		INSERT INTO goal_race_rounds (trigger_id, round_number, status, started_at)
		VALUES ($1, $2, 'active', $3)
		RETURNING id
	`, triggerID, nextRoundNumber, now).Scan(&nextRoundID)

	if err != nil {
		return fmt.Errorf("failed to create next round: %w", err)
	}

	// Добавляем участников в новый раунд
	for _, playerID := range participants {
		_, err = tx.Exec(`
			INSERT INTO goal_race_round_participants (round_id, player_id)
			VALUES ($1, $2)
		`, nextRoundID, playerID)

		if err != nil {
			return fmt.Errorf("failed to add participant to next round: %w", err)
		}
	}

	// Назначаем цели для следующего раунда
	if err := h.assignPredefinedGoals(tx, triggerID, nextRoundID, nextRoundNumber, participants); err != nil {
		return fmt.Errorf("failed to assign goals for next round: %w", err)
	}

	return tx.Commit()
}

// GetPlayerRaceProgress возвращает прогресс игрока в текущей гонке
func (h *GoalRaceHandler) GetPlayerRaceProgress(c *gin.Context) {
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

	// Получаем активные раунды для игрока
	rows, err := h.db.Query(`
		SELECT 
			grr.id,
			grr.round_number,
			grr.status,
			grr.started_at,
			COUNT(grrg.id) AS total_goals,
			COUNT(CASE WHEN g.is_completed = true THEN 1 END) AS completed_goals,
			COUNT(CASE WHEN grrg.is_accessible = true THEN 1 END) AS accessible_goals
		FROM goal_race_round_participants grrp
		JOIN goal_race_rounds grr ON grrp.round_id = grr.id
		LEFT JOIN goal_race_round_goals grrg ON grr.id = grrg.round_id AND grrg.assigned_player_id = grrp.player_id
		LEFT JOIN goals g ON grrg.goal_id = g.id
		WHERE grrp.player_id = $1 AND grr.status IN ('pending', 'active')
		GROUP BY grr.id, grr.round_number, grr.status, grr.started_at
		ORDER BY grr.round_number DESC
	`, *playerID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch race progress"})
		return
	}
	defer rows.Close()

	progress := make([]models.RaceProgress, 0)
	for rows.Next() {
		var p models.RaceProgress
		err := rows.Scan(
			&p.RoundID,
			&p.RoundNumber,
			&p.Status,
			&p.StartedAt,
			&p.TotalGoals,
			&p.CompletedGoals,
			&p.AccessibleGoals,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan progress"})
			return
		}

		progress = append(progress, p)
	}

	if err = rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, models.RaceProgressResponse{Progress: progress})
}

// GetActiveRounds возвращает список активных раундов (для админов)
func (h *GoalRaceHandler) GetActiveRounds(c *gin.Context) {
	rows, err := h.db.Query(`
		SELECT 
			grr.id,
			grr.trigger_id,
			grr.round_number,
			grr.status,
			grr.started_at,
			COUNT(DISTINCT grrp.player_id) AS participants_count,
			COUNT(DISTINCT CASE WHEN grrg.is_accessible = true THEN grrg.id END) AS accessible_goals_count
		FROM goal_race_rounds grr
		LEFT JOIN goal_race_round_participants grrp ON grr.id = grrp.round_id
		LEFT JOIN goal_race_round_goals grrg ON grr.id = grrg.round_id
		WHERE grr.status = 'active'
		GROUP BY grr.id, grr.trigger_id, grr.round_number, grr.status, grr.started_at
		ORDER BY grr.started_at DESC
	`)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch active rounds"})
		return
	}
	defer rows.Close()

	rounds := make([]models.ActiveRound, 0)
	for rows.Next() {
		var round models.ActiveRound
		err := rows.Scan(
			&round.ID,
			&round.TriggerID,
			&round.RoundNumber,
			&round.Status,
			&round.StartedAt,
			&round.ParticipantsCount,
			&round.AccessibleGoalsCount,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan round"})
			return
		}

		rounds = append(rounds, round)
	}

	if err = rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, models.ActiveRoundsResponse{Rounds: rounds})
}

// CancelRound отменяет раунд (только для админов)
func (h *GoalRaceHandler) CancelRound(c *gin.Context) {
	roundIDStr := c.Param("id")
	roundID, err := strconv.Atoi(roundIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid round ID"})
		return
	}

	tx, err := h.db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	now := time.Now()

	// Помечаем раунд как отмененный
	_, err = tx.Exec(`
		UPDATE goal_race_rounds
		SET status = 'cancelled', completed_at = $1
		WHERE id = $2
	`, now, roundID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel round"})
		return
	}

	// Помечаем все цели раунда как недоступные
	_, err = tx.Exec(`
		UPDATE goal_race_round_goals
		SET is_accessible = false, became_inaccessible_at = $1
		WHERE round_id = $2
	`, now, roundID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark goals as inaccessible"})
		return
	}

	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Round cancelled successfully"})
}
