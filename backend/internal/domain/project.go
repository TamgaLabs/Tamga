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
	// SourceTypeEmpty marks a Seal with no repository or compose source yet.
	// It is persisted so an empty Seal never masquerades as a cloneable remote
	// project while it is being configured.
	SourceTypeEmpty  SourceType = "empty"
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
	ID              int64         `json:"id"`
	SealID          int64         `json:"seal_id"`
	Name            string        `json:"name"`
	SourceType      SourceType    `json:"source_type"`
	RepoURL         string        `json:"repo_url"`
	Branch          string        `json:"branch"`
	ComposeYAML     string        `json:"compose_yaml,omitempty"`
	ConfigAuthority string        `json:"config_authority"`
	Status          ProjectStatus `json:"status"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
	ConfigRevision  int64         `json:"config_revision"`
	BuildRevision   int64         `json:"build_revision"`
}
