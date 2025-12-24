// internal/models/faction.go
package models

type FactionMember struct {
	ID            int     `json:"id"`
	CharacterName string  `json:"character_name"`
	Role          string  `json:"role"`
	Influence     int     `json:"influence"`
	Avatar        *string `json:"avatar"`
}

type FactionResponse struct {
	ID                        int              `json:"id"`
	Name                      string           `json:"name"`
	Description               *string          `json:"description"`
	FactionInfluence          int              `json:"faction_influence"`
	TotalInfluence            int              `json:"total_influence"`
	IsCompositionVisibleToAll bool             `json:"is_composition_visible_to_all"`
	LeaderPlayerID            *int             `json:"leader_player_id"`
	IsCurrentPlayerLeader     bool             `json:"is_current_player_leader"`
	IsCurrentPlayerMember     bool             `json:"is_current_player_member"`
	Members                   *[]FactionMember `json:"members,omitempty"` // nil если состав недоступен
}

type FactionsListResponse struct {
	Factions []FactionResponse `json:"factions"`
}

type ChangeFactionRequest struct {
	FactionID int `json:"faction_id" binding:"required"`
}
