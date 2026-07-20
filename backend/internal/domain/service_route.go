package domain

import "time"

// ServiceRoute exposes one exact domain owned by a Service.
type ServiceRoute struct {
	ID        int64     `json:"id"`
	ServiceID int64     `json:"service_id"`
	Domain    string    `json:"domain"`
	CreatedAt time.Time `json:"created_at"`
}
