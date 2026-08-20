package create

import (
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"raccounting/internal/domain/usecase"
)

// Input is what CreateTag needs to create a new tag.
type Input struct {
	Name  string
	Color string
}

// Validate rejects structurally invalid input before any repository lookup.
func (i Input) Validate() error {
	i.Name = strings.TrimSpace(i.Name)

	return validation.ValidateStruct(&i,
		validation.Field(&i.Name, usecase.TagNameRules()...),
		validation.Field(&i.Color, usecase.TagColorRules()...),
	)
}
