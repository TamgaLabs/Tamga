package domain

import "time"

type ProviderType string

const (
	ProviderTypeDocker ProviderType = "docker"
)

type AgentProvider struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Type      ProviderType `json:"type"`
	Image     string       `json:"image,omitempty"`
	Env       string       `json:"env,omitempty"`
	IsDefault bool         `json:"is_default"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

var DefaultProvider = &AgentProvider{
	ID:        "builtin-opencode",
	Name:      "Opencode (Built-in)",
	Type:      ProviderTypeDocker,
	Image:     "tamga-agent",
	IsDefault: true,
}
