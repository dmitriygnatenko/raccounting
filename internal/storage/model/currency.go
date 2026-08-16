package model

import (
	"time"

	"raccounting/internal/domain/entity"
)

// Currency is the shape of a row in the currencies table.
type Currency struct {
	Code      string
	Symbol    string
	Name      string
	Rate      float64
	Default   bool
	Status    uint8
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ToEntity converts the stored row into a domain entity.Currency.
func (m Currency) ToEntity() entity.Currency {
	return entity.Currency{
		Code:      m.Code,
		Symbol:    m.Symbol,
		Name:      m.Name,
		Rate:      m.Rate,
		Default:   m.Default,
		Status:    entity.CurrencyStatus(m.Status),
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}
