package domain

import "time"

type User struct {
	ID           int64     `json:"id"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}
