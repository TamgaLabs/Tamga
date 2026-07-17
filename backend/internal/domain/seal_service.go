package domain

import "time"

// SealService is one deployable service owned by a Seal and built from one of
// its owned repository checkouts. Dependencies name sibling Seal services.
type SealService struct {
	ID           int64     `json:"id"`
	SealID       int64     `json:"seal_id"`
	RepositoryID int64     `json:"repository_id"`
	Name         string    `json:"name"`
	BuildContext string    `json:"build_context"`
	InternalPort int       `json:"internal_port"`
	Dependencies []string  `json:"dependencies"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
