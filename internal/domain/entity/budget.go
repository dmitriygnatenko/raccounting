package entity

import "time"

// Budget is how much is planned to be spent in a category during a given month.
type Budget struct {
	ID         uint64    `json:"id"`
	MonthKey   string    `json:"month_key"`
	CategoryID uint64    `json:"category_id"`
	Amount     int64     `json:"amount"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
