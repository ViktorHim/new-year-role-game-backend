// internal/handlers/admin_debt.go
package handlers

import (
	"database/sql"
	"net/http"
	"new-year-role-game-backend/internal/models"

	"github.com/gin-gonic/gin"
)

type AdminDebtHandler struct {
	db *sql.DB
}

func NewAdminDebtHandler(db *sql.DB) *AdminDebtHandler {
	return &AdminDebtHandler{db: db}
}

// GetDebtPenaltySettings возвращает текущие настройки штрафов для долговых расписок
func (h *AdminDebtHandler) GetDebtPenaltySettings(c *gin.Context) {
	var settings models.DebtPenaltySettings

	err := h.db.QueryRow(`
		SELECT id, penalty_influence_points
		FROM debt_penalty_settings
		ORDER BY id DESC
		LIMIT 1
	`).Scan(&settings.ID, &settings.PenaltyInfluencePoints)

	if err != nil {
		if err == sql.ErrNoRows {
			// Если нет настроек, создаём дефолтные
			_, err = h.db.Exec(`
				INSERT INTO debt_penalty_settings (penalty_influence_points)
				VALUES (0)
			`)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create default settings"})
				return
			}

			// Получаем созданные настройки
			err = h.db.QueryRow(`
				SELECT id, penalty_influence_points
				FROM debt_penalty_settings
				ORDER BY id DESC
				LIMIT 1
			`).Scan(&settings.ID, &settings.PenaltyInfluencePoints)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch settings"})
				return
			}
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch penalty settings"})
			return
		}
	}

	c.JSON(http.StatusOK, settings)
}

// UpdateDebtPenaltySettings обновляет настройки штрафов
func (h *AdminDebtHandler) UpdateDebtPenaltySettings(c *gin.Context) {
	var req models.UpdateDebtPenaltyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Проверяем, есть ли настройки
	var settingsExist bool
	err := h.db.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM debt_penalty_settings)
	`).Scan(&settingsExist)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if !settingsExist {
		// Создаём новые настройки
		_, err = h.db.Exec(`
			INSERT INTO debt_penalty_settings (penalty_influence_points)
			VALUES ($1)
		`, req.PenaltyInfluencePoints)
	} else {
		// Обновляем существующие
		_, err = h.db.Exec(`
			UPDATE debt_penalty_settings
			SET penalty_influence_points = $1
			WHERE id = (SELECT id FROM debt_penalty_settings ORDER BY id DESC LIMIT 1)
		`, req.PenaltyInfluencePoints)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update penalty settings"})
		return
	}

	// Возвращаем обновленные настройки
	var settings models.DebtPenaltySettings
	err = h.db.QueryRow(`
		SELECT id, penalty_influence_points
		FROM debt_penalty_settings
		ORDER BY id DESC
		LIMIT 1
	`).Scan(&settings.ID, &settings.PenaltyInfluencePoints)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch updated settings"})
		return
	}

	c.JSON(http.StatusOK, settings)
}
