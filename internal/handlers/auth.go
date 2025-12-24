// /internal/handlers/auth.go
package handlers

import (
	"database/sql"
	"net/http"
	"new-year-role-game-backend/internal/middleware"
	"new-year-role-game-backend/internal/models"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type AuthHandler struct {
	db     *sql.DB
	jwtKey string
}

func NewAuthHandler(db *sql.DB, jwtKey string) *AuthHandler {
	return &AuthHandler{
		db:     db,
		jwtKey: jwtKey,
	}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	var hashedPassword string
	err := h.db.QueryRow(`
		SELECT id, username, password, player_id, is_admin
		FROM users
		WHERE username = $1
	`, req.Username).Scan(&user.ID, &user.Username, &hashedPassword, &user.PlayerID, &user.IsAdmin)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if hashedPassword != req.Password {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	token, err := h.generateToken(user.ID, user.PlayerID, user.IsAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, models.AuthResponse{
		Token: token,
		User:  user,
	})
}

func (h *AuthHandler) generateToken(userID int, playerID *int, isAdmin bool) (string, error) {
	claims := &middleware.Claims{
		UserID:   userID,
		PlayerID: playerID,
		IsAdmin:  isAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.jwtKey))
}
