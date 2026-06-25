package domain

import "time"

type ProjectStatus string

const (
	ProjectStatusCreated  ProjectStatus = "created"
	ProjectStatusCloning  ProjectStatus = "cloning"
	ProjectStatusBuilding ProjectStatus = "building"
	ProjectStatusRunning  ProjectStatus = "running"
	ProjectStatusError    ProjectStatus = "error"
)

type SourceType string

const (
	SourceTypeLocal  SourceType = "local"
	SourceTypeRemote SourceType = "remote"
)

type Project struct {
	ID              int64         `json:"id"`
	Name            string        `json:"name"`
	SourceType      SourceType    `json:"source_type"`
	RepoURL         string        `json:"repo_url"`
	Branch          string        `json:"branch"`
	Domain          string        `json:"domain"`
	Status          ProjectStatus `json:"status"`
	ContainerID     string        `json:"container_id,omitempty"`
	AgentProviderID *string       `json:"agent_provider_id,omitempty"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
}
