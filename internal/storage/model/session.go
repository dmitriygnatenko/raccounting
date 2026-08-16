package model

import (
	"time"

	"raccounting/internal/domain/entity"
)

// Session is the shape of a row in the sessions table.
type Session struct {
	Token     string
	UserID    uint64
	ExpiresAt time.Time
	CreatedAt time.Time
}

// ToEntity converts the stored row into a domain entity.Session.
func (m Session) ToEntity() entity.Session {
	return entity.Session{
		Token:     m.Token,
		UserID:    m.UserID,
		ExpiresAt: m.ExpiresAt,
		CreatedAt: m.CreatedAt,
	}
}
