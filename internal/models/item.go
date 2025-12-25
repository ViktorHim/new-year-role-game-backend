// internal/models/item.go
package models

import "time"

type Effect struct {
	ID                int     `json:"id"`
	Description       *string `json:"description"`
	EffectType        string  `json:"effect_type"`                  // 'generate_money', 'generate_influence', 'spawn_item'
	GeneratedResource *string `json:"generated_resource,omitempty"` // 'money', 'influence'
	Operation         *string `json:"operation,omitempty"`          // 'add', 'mul', 'sub', 'div'
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
