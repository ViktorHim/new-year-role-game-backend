// internal/models/goal_race.go
package models

import "time"

// Task представляет задачу игрока
type Task struct {
	ID          int        `json:"id"`
	PlayerID    int        `json:"player_id"`
	Title       string     `json:"title"`
	Description *string    `json:"description"`
	IsCompleted bool       `json:"is_completed"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// TasksResponse список задач
type TasksResponse struct {
	Tasks []Task `json:"tasks"`
}

// CompleteTaskRequest запрос на изменение статуса задачи
type CompleteTaskRequest struct {
	IsCompleted *bool `json:"is_completed" binding:"required"`
}

// RaceProgress прогресс игрока в гонке
type RaceProgress struct {
	RoundID         int       `json:"round_id"`
	RoundNumber     int       `json:"round_number"`
	Status          string    `json:"status"`
	StartedAt       time.Time `json:"started_at"`
	TotalGoals      int       `json:"total_goals"`
	CompletedGoals  int       `json:"completed_goals"`
	AccessibleGoals int       `json:"accessible_goals"`
}

// RaceProgressResponse ответ с прогрессом в гонке
type RaceProgressResponse struct {
	Progress []RaceProgress `json:"progress"`
}

// ActiveRound активный раунд гонки
type ActiveRound struct {
	ID                   int       `json:"id"`
	TriggerID            int       `json:"trigger_id"`
	RoundNumber          int       `json:"round_number"`
	Status               string    `json:"status"`
	StartedAt            time.Time `json:"started_at"`
	ParticipantsCount    int       `json:"participants_count"`
	AccessibleGoalsCount int       `json:"accessible_goals_count"`
}

// ActiveRoundsResponse список активных раундов
type ActiveRoundsResponse struct {
	Rounds []ActiveRound `json:"rounds"`
}

// GoalRaceTrigger триггер для запуска гонки
type GoalRaceTrigger struct {
	ID                 int       `json:"id"`
	Name               string    `json:"name"`
	Description        *string   `json:"description"`
	RequiredTasksCount int       `json:"required_tasks_count"`
	IsActive           bool      `json:"is_active"`
	CreatedAt          time.Time `json:"created_at"`
}

// PredefinedGoal предопределенная цель для раунда
type PredefinedGoal struct {
	ID                    int       `json:"id"`
	TriggerID             int       `json:"trigger_id"`
	RoundNumber           int       `json:"round_number"`
	PlayerID              int       `json:"player_id"`
	Title                 string    `json:"title"`
	Description           *string   `json:"description"`
	InfluencePointsReward int       `json:"influence_points_reward"`
	CreatedAt             time.Time `json:"created_at"`
}

// CreatePredefinedGoalRequest запрос на создание предопределенной цели
type CreatePredefinedGoalRequest struct {
	TriggerID             int     `json:"trigger_id" binding:"required"`
	RoundNumber           int     `json:"round_number" binding:"required,min=1"`
	PlayerID              int     `json:"player_id" binding:"required"`
	Title                 string  `json:"title" binding:"required"`
	Description           *string `json:"description"`
	InfluencePointsReward int     `json:"influence_points_reward" binding:"min=0"`
}

// CreateTriggerRequest запрос на создание триггера
type CreateTriggerRequest struct {
	Name               string  `json:"name" binding:"required"`
	Description        *string `json:"description"`
	RequiredTasksCount int     `json:"required_tasks_count" binding:"required,min=1"`
	ParticipantIDs     []int   `json:"participant_ids" binding:"required,min=2"`
}

// UpdateTriggerRequest запрос на обновление триггера
type UpdateTriggerRequest struct {
	IsActive *bool `json:"is_active" binding:"required"`
}
