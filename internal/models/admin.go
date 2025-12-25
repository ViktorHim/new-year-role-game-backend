// internal/models/admin.go
package models

import "time"

type GameStatusResponse struct {
	Status        string     `json:"status"` // 'not_started', 'running', 'ended'
	StartedAt     *time.Time `json:"started_at,omitempty"`
	EndedAt       *time.Time `json:"ended_at,omitempty"`
	Duration      *string    `json:"duration,omitempty"`
	WorkerRunning bool       `json:"worker_running"`
}
