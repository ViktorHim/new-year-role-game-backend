// internal/models/user.go
package models

type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	PlayerID *int   `json:"player_id,omitempty"`
	IsAdmin  bool   `json:"is_admin"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}
