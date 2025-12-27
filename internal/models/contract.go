// internal/models/contract.go
package models

import "time"

type Contract struct {
	ID                   int        `json:"id"`
	ContractType         string     `json:"contract_type"` // 'type1', 'type2'
	CustomerPlayerID     int        `json:"customer_player_id"`
	CustomerPlayerName   string     `json:"customer_player_name"`
	CustomerPlayerAvatar *string    `json:"customer_player_avatar"`
	ExecutorPlayerID     int        `json:"executor_player_id"`
	ExecutorPlayerName   string     `json:"executor_player_name"`
	ExecutorPlayerAvatar *string    `json:"executor_player_avatar"`
	CustomerFactionID    *int       `json:"customer_faction_id"`
	CustomerFactionName  *string    `json:"customer_faction_name"`
	Status               string     `json:"status"` // 'pending', 'signed', 'completed', 'terminated'
	DurationSeconds      int        `json:"duration_seconds"`
	MoneyRewardCustomer  int        `json:"money_reward_customer"`
	MoneyRewardExecutor  int        `json:"money_reward_executor"`
	CreatedAt            time.Time  `json:"created_at"`
	SignedAt             *time.Time `json:"signed_at,omitempty"`
	ExpiresAt            *time.Time `json:"expires_at,omitempty"`
	CompletedAt          *time.Time `json:"completed_at,omitempty"`
	TerminatedAt         *time.Time `json:"terminated_at,omitempty"`

	// Дополнительные поля для удобства клиента
	IsCustomer    bool `json:"is_customer"`              // true если текущий игрок - заказчик
	IsExecutor    bool `json:"is_executor"`              // true если текущий игрок - исполнитель
	TimeRemaining *int `json:"time_remaining,omitempty"` // секунды до истечения (для signed)
	CanSign       bool `json:"can_sign"`                 // можно ли подписать (для pending)
	CanComplete   bool `json:"can_complete"`             // можно ли завершить (для signed)
}

type ContractsResponse struct {
	Contracts []Contract `json:"contracts"`
}

type CreateContractRequest struct {
	ContractType     string `json:"contract_type" binding:"required"` // 'type1' или 'type2'
	CustomerPlayerID int    `json:"customer_player_id" binding:"required"`
	DurationSeconds  int    `json:"duration_seconds" binding:"required,min=60"` // минимум 1 минута
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
