package domain

import "time"

// SealServiceRoute exposes one exact domain owned by a selected Seal service.
type SealServiceRoute struct {
	ID        int64     `json:"id"`
	SealID    int64     `json:"seal_id"`
	ServiceID int64     `json:"service_id"`
	Domain    string    `json:"domain"`
	CreatedAt time.Time `json:"created_at"`
}
