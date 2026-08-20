package entity

import "time"

// Session is a signed-in user's server-side session record.
type Session struct {
	Token     string    `json:"token"`
	UserID    uint64    `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}
