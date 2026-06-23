package domain

import "time"

type DeploymentStatus string

const (
	DeploymentStatusPending  DeploymentStatus = "pending"
	DeploymentStatusRunning  DeploymentStatus = "running"
	DeploymentStatusSuccess  DeploymentStatus = "success"
	DeploymentStatusFailed   DeploymentStatus = "failed"
)

type Deployment struct {
	ID        int64            `json:"id"`
	ProjectID int64            `json:"project_id"`
	Status    DeploymentStatus `json:"status"`
	CommitSHA string           `json:"commit_sha,omitempty"`
	Logs      string           `json:"logs,omitempty"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
}
