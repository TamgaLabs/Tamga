package domain

import "time"

type AgentTaskStatus string

const (
	AgentTaskStatusPending    AgentTaskStatus = "pending"
	AgentTaskStatusProcessing AgentTaskStatus = "processing"
	AgentTaskStatusCompleted  AgentTaskStatus = "completed"
	AgentTaskStatusFailed     AgentTaskStatus = "failed"
)

type AgentTask struct {
	ID          string          `json:"id"`
	ProjectID   int64           `json:"project_id"`
	SessionID   *string         `json:"session_id,omitempty"`
	Message     string          `json:"message"`
	Status      AgentTaskStatus `json:"status"`
	Response    string          `json:"response"`
	Diff        string          `json:"diff"`
	CreatedAt   time.Time       `json:"created_at"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
}

type AgentSession struct {
	ID        string    `json:"id"`
	ProjectID int64     `json:"project_id"`
	DirID     string    `json:"dir_id,omitempty"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
