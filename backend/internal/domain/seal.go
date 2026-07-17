package domain

import "time"

// Seal is Tamga's persisted top-level deployment identity.
type Seal struct {
	ID              int64         `json:"id"`
	Name            string        `json:"name"`
	SourceType      SourceType    `json:"source_type"`
	RepoURL         string        `json:"repo_url"`
	Branch          string        `json:"branch"`
	Domain          string        `json:"domain"`
	Status          ProjectStatus `json:"status"`
	ContainerID     string        `json:"container_id,omitempty"`
	ComposeYAML     string        `json:"compose_yaml,omitempty"`
	ConfigAuthority string        `json:"config_authority"`
	ExposedService  string        `json:"exposed_service,omitempty"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
	ConfigRevision  int64         `json:"config_revision"`
	BuildRevision   int64         `json:"build_revision"`
}
