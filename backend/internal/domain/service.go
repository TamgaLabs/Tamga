package domain

import "time"

// Service is one deployable unit owned by a Project.
type Service struct {
	ID           int64     `json:"id"`
	ProjectID    int64     `json:"project_id"`
	Name         string    `json:"name"`
	BuildContext string    `json:"build_context"`
	InternalPort int       `json:"internal_port"`
	Dependencies []string  `json:"dependencies"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
