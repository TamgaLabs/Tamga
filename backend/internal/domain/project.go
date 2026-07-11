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
	// SourceTypeCompose marks a project created directly from a pasted
	// compose_yaml (FEAT-029) rather than a git repo - it has no
	// RepoURL/Branch. The deploy engine itself doesn't branch on
	// SourceType (project_service.go's deploy()/Restart() branch on
	// ComposeYAML being non-empty, which is set regardless of
	// SourceType); this value exists so the create handler can tell a
	// compose-only create apart from a git-repo create without a
	// RepoURL-required check misfiring on it, and so the source is
	// self-describing in the API response.
	SourceTypeCompose SourceType = "compose"
)

type Project struct {
	ID          int64         `json:"id"`
	Name        string        `json:"name"`
	SourceType  SourceType    `json:"source_type"`
	RepoURL     string        `json:"repo_url"`
	Branch      string        `json:"branch"`
	Domain      string        `json:"domain"`
	Status      ProjectStatus `json:"status"`
	ContainerID string        `json:"container_id,omitempty"`
	// ComposeYAML is the project's compose definition (FEAT-025/TEST-011's
	// unified compose model). Empty for legacy projects that predate this
	// field, or for any project that hasn't been (re)deployed since.
	ComposeYAML string `json:"compose_yaml,omitempty"`
	// ExposedService is the compose service name project.Domain routes to.
	// Empty means "no explicit override" - the deploy engine falls back to
	// its single-published-port heuristic (see TEST-011 §2c).
	ExposedService string    `json:"exposed_service,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
