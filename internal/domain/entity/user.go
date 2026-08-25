package entity

import "time"

const MinUsernameLength = 3

// MaxUsernameLength is the maximum accepted length for a username — matches the users.username
// column width (see the adapter migrations).
const MaxUsernameLength = 255

const MinPasswordLength = 4

type User struct {
	ID           uint64       `json:"id"`
	Username     string       `json:"username"`
	PasswordHash string       `json:"-"`
	Settings     UserSettings `json:"settings"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

type UserSettings struct {
	Language string `json:"language,omitempty"`
	Theme    string `json:"theme,omitempty"`
}

type PublicUser struct {
	ID       uint64       `json:"id"`
	Username string       `json:"username"`
	Settings UserSettings `json:"settings"`
}

func (u User) Public() PublicUser {
	return PublicUser{
		ID:       u.ID,
		Username: u.Username,
		Settings: u.Settings,
	}
}
