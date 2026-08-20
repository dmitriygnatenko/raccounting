package entity

import (
	"encoding/json"
	"time"
)

// MaxTagNameLength is the maximum accepted length for a tag name — matches the tags.name column
// width (see the adapter migrations).
const MaxTagNameLength = 255

// MaxTagColorLength is the maximum accepted length for a tag color (a hex code, e.g. "#f59e0b") —
// matches the tags.color column width (see the adapter migrations).
const MaxTagColorLength = 10

// Tag is a freeform label a transaction can carry zero or more of (e.g. "vacation", "reimbursable"),
// independent of its category — used to group/sum transactions across categories in the transactions
// list.
type Tag struct {
	ID        uint64
	Name      string
	Color     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// tagJSON is Tag's wire shape.
type tagJSON struct {
	ID        uint64    `json:"id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (t Tag) MarshalJSON() ([]byte, error) {
	return json.Marshal(tagJSON{
		ID:        t.ID,
		Name:      t.Name,
		Color:     t.Color,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	})
}
