package domain

import "time"

type ProjectStatus string

const (
	ProjectStatusCreated       ProjectStatus = "created"
	ProjectStatusCloning       ProjectStatus = "cloning"
	ProjectStatusBuilding      ProjectStatus = "building"
	ProjectStatusRunning       ProjectStatus = "running"
	ProjectStatusError         ProjectStatus = "error"
	ProjectStatusConfiguring   ProjectStatus = "configuring"
	ProjectStatusCloneFailed   ProjectStatus = "clone_failed"
	ProjectStatusBuildFailed   ProjectStatus = "build_failed"
	ProjectStatusReadyToDeploy ProjectStatus = "ready_to_deploy"
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
	ExposedService string           `json:"exposed_service,omitempty"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
	Sources        []*ProjectSource `json:"sources,omitempty"`
	ConfigRevision int64            `json:"config_revision"`
	BuildRevision  int64            `json:"build_revision"`
}

// ProjectRoute is one explicitly public compose service. Domains are unique
// across all projects; services without a route remain private.
type ProjectRoute struct {
	ID        int64  `json:"id"`
	ProjectID int64  `json:"project_id"`
	Service   string `json:"service"`
	Domain    string `json:"domain"`
}

type ProjectSourceStatus string

const (
	ProjectSourceStatusPending     ProjectSourceStatus = "pending"
	ProjectSourceStatusCloning     ProjectSourceStatus = "cloning"
	ProjectSourceStatusReady       ProjectSourceStatus = "ready"
	ProjectSourceStatusCloneFailed ProjectSourceStatus = "clone_failed"
)

// ProjectSource is an independently cloned source tree owned by a project.
// WorkspacePath is relative to the project workspace; `.` is the primary
// source and additional sources live below sources/<safe-name>.
type ProjectSource struct {
	ID            int64               `json:"id"`
	ProjectID     int64               `json:"project_id"`
	DisplayName   string              `json:"display_name"`
	RemoteURL     string              `json:"remote_url,omitempty"`
	Branch        string              `json:"branch,omitempty"`
	WorkspacePath string              `json:"workspace_path"`
	Status        ProjectSourceStatus `json:"status"`
	ErrorSummary  string              `json:"error_summary,omitempty"`
	CreatedAt     time.Time           `json:"created_at"`
	UpdatedAt     time.Time           `json:"updated_at"`
}
