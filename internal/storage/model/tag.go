package model

import (
	"time"

	"raccounting/internal/domain/entity"
)

// Tag is the shape of a row in the tags table.
type Tag struct {
	ID        uint64
	Name      string
	Color     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ToEntity converts the stored row into a domain entity.Tag.
func (m Tag) ToEntity() entity.Tag {
	return entity.Tag{
		ID:        m.ID,
		Name:      m.Name,
		Color:     m.Color,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}
