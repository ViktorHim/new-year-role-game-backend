// internal/models/item.go
package models

import "time"

type Effect struct {
	ID                int     `json:"id"`
	Description       *string `json:"description"`
	EffectType        string  `json:"effect_type"` // 'generate_money', 'generate_influence', 'spawn_item'
	GeneratedResource *string `json:"generated_resource,omitempty"` // 'money', 'influence'
	Operation         *string `json:"operation,omitempty"` // 'add', 'mul', 'sub', 'div'
	Value             *int    `json:"value,omitempty"`
	SpawnedItemID     *int    `json:"spawned_item_id,omitempty"`
	PeriodSeconds     int     `json:"period_seconds"`
}

type Item struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	AcquiredAt  time.Time `json:"acquired_at"`
	Effects     []Effect  `json:"effects"`
}

type InventoryResponse struct {
	Items []Item `json:"items"`
}

type TransferItemRequest struct {
	ToPlayerID int `json:"to_player_id" binding:"required"`
	ItemID     int `json:"item_id" binding:"required"`
}

type TransferMoneyRequest struct {
	ToPlayerID int `json:"to_player_id" binding:"required"`
	Amount     int `json:"amount" binding:"required,min=1"`
}

type EffectExecution struct {
	ItemID            int        `json:"item_id"`
	ItemName          string     `json:"item_name"`
	EffectID          int        `json:"effect_id"`
	EffectDescription *string    `json:"effect_description"`
	EffectType        string     `json:"effect_type"`
	Executed          bool       `json:"executed"`
	Result            *string    `json:"result,omitempty"`
	NextAvailableAt   *time.Time `json:"next_available_at,omitempty"`
}

type ExecuteEffectsResponse struct {
	MoneyGenerated     int               `json:"money_generated"`
	InfluenceGenerated int               `json:"influence_generated"`
	ItemsSpawned       int               `json:"items_spawned"`
	ExecutedEffects    []EffectExecution `json:"executed_effects"`
}

type EffectStatus struct {
	ItemID            int        `json:"item_id"`
	ItemName          string     `json:"item_name"`
	EffectID          int        `json:"effect_id"`
	EffectDescription *string    `json:"effect_description"`
	EffectType        string     `json:"effect_type"`
	PeriodSeconds     int        `json:"period_seconds"`
	LastExecutedAt    *time.Time `json:"last_executed_at"`
	NextAvailableAt   *time.Time `json:"next_available_at"`
	CanExecuteNow     bool       `json:"can_execute_now"`
}

type ItemEffectsStatusResponse struct {
	Effects []EffectStatus `json:"effects"`
}