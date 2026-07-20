package domain

import "time"

// Seal is Tamga's persisted top-level deployment identity. Source, checkout,
// configuration, and build state belong to its child Projects.
type Seal struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
