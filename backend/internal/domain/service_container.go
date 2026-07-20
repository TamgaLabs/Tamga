package domain

import "time"

// ServiceContainer is one running service/container belonging to a
// project's compose stack (FEAT-025/TEST-011's unified compose model). A
// single-service (folded git-build) project has exactly one row; a
// multi-service compose project has one per declared service.
type ServiceContainer struct {
	ID            int64     `json:"id"`
	ServiceID     int64     `json:"service_id"`
	ContainerID   string    `json:"container_id"`
	ContainerName string    `json:"container_name"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}
