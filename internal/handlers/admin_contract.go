// internal/handlers/admin_contract.go
package handlers

import (
	"database/sql"
	"net/http"
	"new-year-role-game-backend/internal/models"

	"github.com/gin-gonic/gin"
)

type AdminContractHandler struct {
	db *sql.DB
}

func NewAdminContractHandler(db *sql.DB) *AdminContractHandler {
	return &AdminContractHandler{db: db}
}

// GetContractSettings возвращает текущие настройки наград и штрафов для договоров
func (h *AdminContractHandler) GetContractSettings(c *gin.Context) {
	var settings models.ContractSettingsResponse

	// Получаем настройки Type 1
	err := h.db.QueryRow(`
		SELECT id, money_reward_customer, money_reward_executor, updated_at
		FROM contract_type1_reward_settings
		ORDER BY id DESC
		LIMIT 1
	`).Scan(
		&settings.Type1Rewards.ID,
		&settings.Type1Rewards.MoneyRewardCustomer,
		&settings.Type1Rewards.MoneyRewardExecutor,
		&settings.Type1Rewards.UpdatedAt,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch Type 1 settings"})
		return
	}

	// Получаем настройки Type 2
	err = h.db.QueryRow(`
		SELECT id, money_reward_executor, updated_at
		FROM contract_type2_reward_settings
		ORDER BY id DESC
		LIMIT 1
	`).Scan(
		&settings.Type2Rewards.ID,
		&settings.Type2Rewards.MoneyRewardExecutor,
		&settings.Type2Rewards.UpdatedAt,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch Type 2 settings"})
		return
	}

	// Получаем настройки штрафов
	err = h.db.QueryRow(`
		SELECT id, money_penalty, influence_penalty
		FROM contract_penalty_settings
		ORDER BY id DESC
		LIMIT 1
	`).Scan(
		&settings.Penalties.ID,
		&settings.Penalties.MoneyPenalty,
		&settings.Penalties.InfluencePenalty,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch penalty settings"})
		return
	}

	c.JSON(http.StatusOK, settings)
}

// UpdateContractType1Rewards обновляет настройки наград для Type 1
func (h *AdminContractHandler) UpdateContractType1Rewards(c *gin.Context) {
	var req models.UpdateContractType1RewardsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	_, err := h.db.Exec(`
		UPDATE contract_type1_reward_settings
		SET money_reward_customer = $1,
		    money_reward_executor = $2,
		    updated_at = NOW()
		WHERE id = (SELECT id FROM contract_type1_reward_settings ORDER BY id DESC LIMIT 1)
	`, req.MoneyRewardCustomer, req.MoneyRewardExecutor)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update Type 1 rewards"})
		return
	}

	// Возвращаем обновленные настройки
	var settings models.ContractType1RewardSettings
	err = h.db.QueryRow(`
		SELECT id, money_reward_customer, money_reward_executor, updated_at
		FROM contract_type1_reward_settings
		ORDER BY id DESC
		LIMIT 1
	`).Scan(
		&settings.ID,
		&settings.MoneyRewardCustomer,
		&settings.MoneyRewardExecutor,
		&settings.UpdatedAt,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch updated settings"})
		return
	}

	c.JSON(http.StatusOK, settings)
}

// UpdateContractType2Rewards обновляет настройки наград для Type 2
func (h *AdminContractHandler) UpdateContractType2Rewards(c *gin.Context) {
	var req models.UpdateContractType2RewardsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	_, err := h.db.Exec(`
		UPDATE contract_type2_reward_settings
		SET money_reward_executor = $1,
		    updated_at = NOW()
		WHERE id = (SELECT id FROM contract_type2_reward_settings ORDER BY id DESC LIMIT 1)
	`, req.MoneyRewardExecutor)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update Type 2 rewards"})
		return
	}

	// Возвращаем обновленные настройки
	var settings models.ContractType2RewardSettings
	err = h.db.QueryRow(`
		SELECT id, money_reward_executor, updated_at
		FROM contract_type2_reward_settings
		ORDER BY id DESC
		LIMIT 1
	`).Scan(
		&settings.ID,
		&settings.MoneyRewardExecutor,
		&settings.UpdatedAt,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch updated settings"})
		return
	}

	c.JSON(http.StatusOK, settings)
}

// UpdateContractPenalties обновляет настройки штрафов
func (h *AdminContractHandler) UpdateContractPenalties(c *gin.Context) {
	var req models.UpdateContractPenaltiesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	_, err := h.db.Exec(`
		UPDATE contract_penalty_settings
		SET money_penalty = $1,
		    influence_penalty = $2
		WHERE id = (SELECT id FROM contract_penalty_settings ORDER BY id DESC LIMIT 1)
	`, req.MoneyPenalty, req.InfluencePenalty)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update penalties"})
		return
	}

	// Возвращаем обновленные настройки
	var settings models.ContractPenaltySettings
	err = h.db.QueryRow(`
		SELECT id, money_penalty, influence_penalty
		FROM contract_penalty_settings
		ORDER BY id DESC
		LIMIT 1
	`).Scan(
		&settings.ID,
		&settings.MoneyPenalty,
		&settings.InfluencePenalty,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch updated settings"})
		return
	}

	c.JSON(http.StatusOK, settings)
}
