package domain

import "time"

type EnvVar struct {
	ID        int64     `json:"id"`
	ProjectID int64     `json:"project_id"`
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ServiceEnvVar is a database-owned environment value for one supported
// Compose service. Global EnvVar values remain separate and are applied first
// when a stack is deployed.
type ServiceEnvVar struct {
	ID          int64     `json:"id"`
	ProjectID   int64     `json:"project_id"`
	ServiceName string    `json:"service_name"`
	Key         string    `json:"key"`
	Value       string    `json:"value"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
