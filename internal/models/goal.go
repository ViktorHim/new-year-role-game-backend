// internal/models/goal.go
package models

import "time"

type Goal struct {
	ID                    int        `json:"id"`
	Title                 string     `json:"title"`
	Description           *string    `json:"description"`
	GoalType              string     `json:"goal_type"` // 'personal' или 'faction'
	InfluencePointsReward int        `json:"influence_points_reward"`
	PlayerID              *int       `json:"player_id,omitempty"`
	FactionID             *int       `json:"faction_id,omitempty"`
	IsCompleted           bool       `json:"is_completed"`
	CompletedAt           *time.Time `json:"completed_at,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	IsVisible             bool       `json:"is_visible"` // для скрытых целей
}

// GoalWithLockStatus расширяет Goal информацией о блокировке и зависимостях
type GoalWithLockStatus struct {
	ID                    int              `json:"id"`
	Title                 string           `json:"title"`
	Description           *string          `json:"description"`
	GoalType              string           `json:"goal_type"`
	InfluencePointsReward int              `json:"influence_points_reward"`
	PlayerID              *int             `json:"player_id,omitempty"`
	FactionID             *int             `json:"faction_id,omitempty"`
	IsCompleted           bool             `json:"is_completed"`
	CompletedAt           *time.Time       `json:"completed_at,omitempty"`
	CreatedAt             time.Time        `json:"created_at"`
	IsVisible             bool             `json:"is_visible"`
	IsLocked              bool             `json:"is_locked"`              // true = видна, но заблокирована
	Dependencies          []GoalDependency `json:"dependencies,omitempty"` // все зависимости (выполненные и невыполненные)
}

// GoalDependency описывает зависимость цели
type GoalDependency struct {
	DependencyType string     `json:"dependency_type"`       // 'goal_completion' или 'influence_threshold'
	IsSatisfied    bool       `json:"is_satisfied"`          // выполнена ли зависимость (или разблокирована навсегда)
	UnlockedAt     *time.Time `json:"unlocked_at,omitempty"` // когда была разблокирована (если была)

	// Для зависимости от выполнения другой цели
	RequiredGoalID        *int    `json:"required_goal_id,omitempty"`
	RequiredGoalTitle     *string `json:"required_goal_title,omitempty"`
	RequiredGoalCompleted *bool   `json:"required_goal_completed,omitempty"`

	// Для зависимости от очков влияния другого игрока
	InfluencePlayerID   *int    `json:"influence_player_id,omitempty"`
	InfluencePlayerName *string `json:"influence_player_name,omitempty"`
	CurrentInfluence    *int    `json:"current_influence,omitempty"`
	RequiredInfluence   *int    `json:"required_influence,omitempty"`
}

type PersonalGoalsResponse struct {
	Goals []Goal `json:"goals"`
}

type PersonalGoalsResponseWithLock struct {
	Goals []GoalWithLockStatus `json:"goals"`
}

type FactionGoalsResponse struct {
	Goals []Goal `json:"goals"`
}

type CompleteGoalRequest struct {
	IsCompleted *bool `json:"is_completed" binding:"required"`
}
