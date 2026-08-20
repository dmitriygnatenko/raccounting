// Package model holds the row-shaped structs each driver adapter (internal/adapter/mysql, postgres,
// sqlite) scans a table's columns into, plus each one's ToEntity method converting it to its
// internal/domain/entity counterpart. Enum-like columns (Type, Status, ...) are kept as their raw
// uint8 here and only decoded into the entity's named type on that conversion, so this package
// stays free of domain behavior.
package model

import (
	"time"

	"raccounting/internal/domain/entity"
)

// Account is the shape of a row in the accounts table.
type Account struct {
	ID           uint64
	Name         string
	Type         uint8
	CurrencyCode string
	Balance      int64
	Status       uint8
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ToEntity converts the stored row into a domain entity.Account.
func (m Account) ToEntity() entity.Account {
	return entity.Account{
		ID:           m.ID,
		Name:         m.Name,
		Type:         entity.AccountType(m.Type),
		CurrencyCode: m.CurrencyCode,
		Balance:      m.Balance,
		Status:       entity.AccountStatus(m.Status),
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
}
