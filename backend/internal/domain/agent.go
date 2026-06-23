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
	Message     string          `json:"message"`
	Status      AgentTaskStatus `json:"status"`
	Response    string          `json:"response,omitempty"`
	Diff        string          `json:"diff,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
}
