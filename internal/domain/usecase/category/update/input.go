package update

import (
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"raccounting/internal/domain/usecase"
)

// Input is what UpdateCategory needs to rename/recolor/archive an existing category. Type can't be
// changed this way (matches the frontend, which never lets a category switch between expense/income).
type Input struct {
	ID       uint64
	Name     string
	Color    string
	Archived bool
}

// Validate rejects structurally invalid input before any repository lookup.
func (i Input) Validate() error {
	i.Name = strings.TrimSpace(i.Name)

	return validation.ValidateStruct(&i,
		validation.Field(&i.Name, usecase.CategoryNameRules()...),
		validation.Field(&i.Color, usecase.CategoryColorRules()...),
	)
}
