package create

import (
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"raccounting/internal/domain/usecase"
)

// Input is what CreateCategory needs to create a new category. Type is the bare numeric
// entity.CategoryType value — see entity.CategoryTypes.
type Input struct {
	Name  string
	Type  uint8
	Color string
}

// Validate rejects structurally invalid input before any repository lookup.
func (i Input) Validate() error {
	i.Name = strings.TrimSpace(i.Name)

	return validation.ValidateStruct(&i,
		validation.Field(&i.Name, usecase.CategoryNameRules()...),
		validation.Field(&i.Type, usecase.CategoryTypeRules()...),
		validation.Field(&i.Color, usecase.CategoryColorRules()...),
	)
}
