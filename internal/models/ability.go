// internal/models/ability.go
package models

import "time"

type Ability struct {
	ID                      int        `json:"id"`
	PlayerID                int        `json:"player_id"`
	Name                    string     `json:"name"`
	Description             *string    `json:"description"`
	AbilityType             string     `json:"ability_type"` // 'reveal_info', 'add_influence', 'transfer_influence'
	CooldownMinutes         *int       `json:"cooldown_minutes"`
	StartDelayMinutes       *int       `json:"start_delay_minutes"`
	RequiredInfluencePoints *int       `json:"required_influence_points"`
	IsUnlocked              bool       `json:"is_unlocked"`
	
	// Параметры для разных типов способностей
	InfluencePointsToAdd    *int       `json:"influence_points_to_add,omitempty"`
	InfluencePointsToRemove *int       `json:"influence_points_to_remove,omitempty"`
	InfluencePointsToSelf   *int       `json:"influence_points_to_self,omitempty"`
	
	// Статус использования
	LastUsedAt              *time.Time `json:"last_used_at,omitempty"`
	NextAvailableAt         *time.Time `json:"next_available_at,omitempty"`
	CanUseNow               bool       `json:"can_use_now"`
	BlockReason             *string    `json:"block_reason,omitempty"` // Причина блокировки
	
	CreatedAt               time.Time  `json:"created_at"`
}

type AbilitiesResponse struct {
	Abilities []Ability `json:"abilities"`
}

type UseAbilityRequest struct {
	TargetPlayerID *int    `json:"target_player_id,omitempty"` // Для способностей, направленных на других игроков
	InfoCategory   *string `json:"info_category,omitempty"`    // Для reveal_info: 'faction', 'goal', 'item'
}

type RevealedInfoData struct {
	InfoType string      `json:"info_type"` // 'faction', 'goal', 'item'
	Data     interface{} `json:"data"`
}

type UseAbilityResponse struct {
	Message      string            `json:"message"`
	RevealedInfo *RevealedInfoData `json:"revealed_info,omitempty"`
}