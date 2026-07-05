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
	Command   string       `json:"command,omitempty"`
	Endpoint  string       `json:"endpoint,omitempty"`
	AuthToken string       `json:"-"`
	Env       string       `json:"env,omitempty"`
	IsDefault bool         `json:"is_default"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

type ProviderConfig struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Command string `json:"command"`
}

var DefaultProvider = &AgentProvider{
	ID:        "builtin-opencode",
	Name:      "Opencode (Built-in)",
	Type:      ProviderTypeDocker,
	Image:     "tamga-agent",
	Command:   "opencode acp",
	IsDefault: true,
}
