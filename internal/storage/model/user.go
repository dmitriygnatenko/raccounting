package model

import (
	"time"

	"raccounting/internal/domain/entity"
)

// User is the shape of a row in the users table.
type User struct {
	ID           uint64
	Username     string
	PasswordHash string
	Settings     UserSettings
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ToEntity converts the stored row into a domain entity.User.
func (m User) ToEntity() entity.User {
	return entity.User{
		ID:           m.ID,
		Username:     m.Username,
		PasswordHash: m.PasswordHash,
		Settings:     m.Settings.ToEntity(),
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
}
