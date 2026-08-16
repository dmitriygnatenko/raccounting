package entity

import (
	"encoding/json"
	"time"
)

// MaxCategoryNameLength is the maximum accepted length for a category name — matches the
// categories.name column width (see the adapter migrations).
const MaxCategoryNameLength = 255

// MaxCategoryColorLength is the maximum accepted length for a category color (a hex code, e.g.
// "#f59e0b") — matches the categories.color column width (see the adapter migrations).
const MaxCategoryColorLength = 10

type CategoryType uint8

const (
	CategoryTypeIncome CategoryType = iota + 1
	CategoryTypeExpense
)

var categoryTypeNames = map[CategoryType]string{
	CategoryTypeIncome:  "income",
	CategoryTypeExpense: "expense",
}

// String returns the wire-format name for t, or "" if t is not a known category type.
func (t CategoryType) String() string {
	return categoryTypeNames[t]
}

// ParseCategoryType parses a wire-format category type string ("income"/"expense"). ok is false
// for an unrecognized value.
func ParseCategoryType(s string) (CategoryType, bool) {
	for t, name := range categoryTypeNames {
		if name == s {
			return t, true
		}
	}

	return 0, false
}

type CategoryStatus uint8

const (
	CategoryStatusActive CategoryStatus = iota + 1
	CategoryStatusArchived
)

// Category groups transactions for reporting/budgeting (e.g. "Groceries", "Salary").
type Category struct {
	ID        uint64
	Name      string
	Color     string
	Type      CategoryType
	Status    CategoryStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}

// categoryJSON is Category's wire shape: Type as its string name, Status collapsed to a boolean
// (the frontend only ever asks "is this archived?").
type categoryJSON struct {
	ID        uint64    `json:"id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	Type      string    `json:"type"`
	Archived  bool      `json:"archived"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (c Category) MarshalJSON() ([]byte, error) {
	return json.Marshal(categoryJSON{
		ID:        c.ID,
		Name:      c.Name,
		Color:     c.Color,
		Type:      c.Type.String(),
		Archived:  c.Status == CategoryStatusArchived,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	})
}
