package model

import (
	"time"

	"raccounting/internal/domain/entity"
)

// Category is the shape of a row in the categories table.
type Category struct {
	ID        uint64
	Name      string
	Color     string
	Type      uint8
	Status    uint8
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ToEntity converts the stored row into a domain entity.Category.
func (m Category) ToEntity() entity.Category {
	return entity.Category{
		ID:        m.ID,
		Name:      m.Name,
		Color:     m.Color,
		Type:      entity.CategoryType(m.Type),
		Status:    entity.CategoryStatus(m.Status),
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}
