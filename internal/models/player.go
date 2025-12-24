// internal/models/player.go
package models

type PlayerResponse struct {
	ID               int      `json:"id"`
	Name             string   `json:"name"`
	Role             string   `json:"role"`
	FactionID        *int     `json:"faction_id"`
	CanChangeFaction bool     `json:"can_change_faction"`
	Description      string   `json:"description"`
	InfoAboutPlayers []string `json:"info_about_players"`
	Avatar           *string  `json:"avatar"`
}

type BalanceResponse struct {
	Money     int `json:"money"`
	Influence int `json:"influence"`
}
