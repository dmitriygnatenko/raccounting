package updatecredentials

import (
	validation "github.com/go-ozzo/ozzo-validation/v4"

	"raccounting/internal/domain/entity"
	"raccounting/internal/domain/usecase"
)

// Input is what UpdateCredentials needs to change the signed-in user's username and/or password.
// NewUsername/NewPassword are optional — a blank one leaves that field unchanged, matching
// App.api.changeCredentials in the frontend mock.
type Input struct {
	User            entity.PublicUser
	CurrentPassword string
	NewUsername     string
	NewPassword     string
}

// Validate rejects a blank current password before any repository lookup, and a non-blank new
// password that's too short.
func (i Input) Validate() error {
	return validation.ValidateStruct(&i,
		validation.Field(&i.CurrentPassword, validation.Required.Error("Please enter your current password")),
		validation.Field(&i.NewPassword,
			validation.When(i.NewPassword != "", usecase.PasswordRules()...),
		),
	)
}
