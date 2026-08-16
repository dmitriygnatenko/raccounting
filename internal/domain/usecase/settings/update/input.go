package update

import (
	validation "github.com/go-ozzo/ozzo-validation/v4"

	"raccounting/internal/domain/usecase"
)

// Input is what UpdateSettings needs to change the signed-in user's UI language preference.
type Input struct {
	UserID   uint64
	Language string
}

// Validate rejects a blank/malformed language before any repository write.
func (i Input) Validate() error {
	return validation.ValidateStruct(&i,
		validation.Field(&i.Language, usecase.LanguageRules()...),
	)
}
