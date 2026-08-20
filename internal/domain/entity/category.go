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

// CategoryTypes returns every known category type, in a stable order — used to build the "must be
// one of these" validation rule. The wire format is the bare numeric value (see categoryJSON); the
// frontend (web/js/data/categories.js) keeps its own App.CategoryType constants matching these.
func CategoryTypes() []CategoryType {
	return []CategoryType{
		CategoryTypeIncome,
		CategoryTypeExpense,
	}
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

// categoryJSON is Category's wire shape: Type as its bare numeric value (the frontend keeps its own
// App.CategoryType constants, see CategoryTypes), Status collapsed to a boolean (the frontend only
// ever asks "is this archived?").
type categoryJSON struct {
	ID        uint64    `json:"id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	Type      uint8     `json:"type"`
	Archived  bool      `json:"archived"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (c Category) MarshalJSON() ([]byte, error) {
	return json.Marshal(categoryJSON{
		ID:        c.ID,
		Name:      c.Name,
		Color:     c.Color,
		Type:      uint8(c.Type),
		Archived:  c.Status == CategoryStatusArchived,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	})
}

// UnmarshalJSON is categoryJSON's inverse — used to read a Category back out of a Backup file (see
// entity.Backup).
func (c *Category) UnmarshalJSON(data []byte) error {
	var j categoryJSON
	if err := json.Unmarshal(data, &j); err != nil {
		return err
	}

	status := CategoryStatusActive
	if j.Archived {
		status = CategoryStatusArchived
	}

	*c = Category{
		ID:        j.ID,
		Name:      j.Name,
		Color:     j.Color,
		Type:      CategoryType(j.Type),
		Status:    status,
		CreatedAt: j.CreatedAt,
		UpdatedAt: j.UpdatedAt,
	}

	return nil
}
