package login

import (
	validation "github.com/go-ozzo/ozzo-validation/v4"

	"raccounting/internal/domain/usecase"
)

// Input is what LoginUser needs to verify credentials (or auto-provision the single account) and
// start a session. Language is the frontend's current UI language — saved as the user's preference
// the first time they ever log in with one on record, and otherwise ignored (see UseCase.Execute).
type Input struct {
	Username string
	Password string
	Language string
}

// Validate rejects structurally invalid credentials before any repository lookup. Language, if
// given, must be well-formed — but a blank one is fine, since older frontends won't send it.
func (i Input) Validate() error {
	return validation.ValidateStruct(&i,
		validation.Field(&i.Username, usecase.UsernameRules()...),
		validation.Field(&i.Password, usecase.PasswordRules()...),
		validation.Field(&i.Language, validation.When(i.Language != "", usecase.LanguageRules()...)),
	)
}
