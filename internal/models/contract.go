// internal/models/contract.go
package models

import "time"

// Contract - структура договора
type Contract struct {
	// Базовые поля
	ID                   int        `json:"id"`
	ContractType         string     `json:"contract_type"`
	CustomerPlayerID     int        `json:"customer_player_id"`
	CustomerPlayerName   string     `json:"customer_player_name"`
	CustomerPlayerAvatar *string    `json:"customer_player_avatar"`
	ExecutorPlayerID     int        `json:"executor_player_id"`
	ExecutorPlayerName   string     `json:"executor_player_name"`
	ExecutorPlayerAvatar *string    `json:"executor_player_avatar"`
	CustomerFactionID    *int       `json:"customer_faction_id"`
	CustomerFactionName  *string    `json:"customer_faction_name"`
	Status               string     `json:"status"`
	DurationSeconds      int        `json:"duration_seconds"`
	MoneyRewardCustomer  int        `json:"money_reward_customer"`
	MoneyRewardExecutor  int        `json:"money_reward_executor"`
	CreatedAt            time.Time  `json:"created_at"`
	SignedAt             *time.Time `json:"signed_at"`
	ExpiresAt            *time.Time `json:"expires_at"`
	CompletedAt          *time.Time `json:"completed_at"`
	TerminatedAt         *time.Time `json:"terminated_at"`

	// Вычисляемые поля
	IsCustomer    bool `json:"is_customer"`
	IsExecutor    bool `json:"is_executor"`
	TimeRemaining *int `json:"time_remaining,omitempty"`

	// Возможные действия
	CanSign     bool `json:"can_sign"`
	CanComplete bool `json:"can_complete"`

	// Раскрытая информация (для type2)
	InfoRevealed     bool                   `json:"info_revealed"`                // Был ли раскрыт факт
	RevealedInfoType *string                `json:"revealed_info_type,omitempty"` // "faction", "goal", "item"
	RevealedInfoData map[string]interface{} `json:"revealed_info_data,omitempty"` // Данные раскрытой информации
	RevealedAt       *time.Time             `json:"revealed_at,omitempty"`        // Когда был раскрыт
}

type ContractsResponse struct {
	Contracts []Contract `json:"contracts"`
}

type CreateContractRequest struct {
	ContractType     string `json:"contract_type" binding:"required"` // 'type1' или 'type2'
	CustomerPlayerID int    `json:"customer_player_id" binding:"required"`
	//DurationSeconds  int    `json:"duration_seconds" binding:"required,min=60"` // минимум 1 минута
}

type SignContractRequest struct {
	// Пустое тело - просто подписать
}

type CompleteContractRequest struct {
	// Пустое тело - просто завершить
}

type TerminateContractRequest struct {
	Reason *string `json:"reason,omitempty"` // Причина расторжения
}

// Настройки наград и штрафов для договоров

type ContractType1RewardSettings struct {
	ID                  int       `json:"id"`
	MoneyRewardCustomer int       `json:"money_reward_customer"`
	MoneyRewardExecutor int       `json:"money_reward_executor"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type ContractType2RewardSettings struct {
	ID                  int       `json:"id"`
	MoneyRewardExecutor int       `json:"money_reward_executor"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type ContractPenaltySettings struct {
	ID               int `json:"id"`
	MoneyPenalty     int `json:"money_penalty"`
	InfluencePenalty int `json:"influence_penalty"`
}

type ContractSettingsResponse struct {
	Type1Rewards ContractType1RewardSettings `json:"type1_rewards"`
	Type2Rewards ContractType2RewardSettings `json:"type2_rewards"`
	Penalties    ContractPenaltySettings     `json:"penalties"`
}

type UpdateContractType1RewardsRequest struct {
	MoneyRewardCustomer int `json:"money_reward_customer" binding:"required,min=0"`
	MoneyRewardExecutor int `json:"money_reward_executor" binding:"required,min=0"`
}

type UpdateContractType2RewardsRequest struct {
	MoneyRewardExecutor int `json:"money_reward_executor" binding:"required,min=0"`
}

type UpdateContractPenaltiesRequest struct {
	MoneyPenalty     int `json:"money_penalty" binding:"required,min=0"`
	InfluencePenalty int `json:"influence_penalty" binding:"required,min=0"`
}
