package domain

import "time"

// ServiceEnvVar is a database-owned environment value for one service.
type ServiceEnvVar struct {
	ID        int64     `json:"id"`
	ServiceID int64     `json:"service_id"`
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
