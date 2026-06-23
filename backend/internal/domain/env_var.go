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
