// Package updatecredentials is the UpdateCredentials use case: it lets the signed-in user change
// their username and/or password, after confirming their current password — mirrors
// App.api.changeCredentials in the frontend mock.
package updatecredentials

import (
	"context"
	"errors"
	"log/slog"

	domainerror "raccounting/internal/domain/error"
	"raccounting/internal/domain/usecase"
	"raccounting/internal/port"
)

// UseCase implements UpdateCredentials.
type UseCase struct {
	userRepository port.UserRepository
	passwordHasher port.PasswordHasher
}

// New builds a UseCase from its dependencies.
func New(
	userRepository port.UserRepository,
	passwordHasher port.PasswordHasher,
) *UseCase {
	return &UseCase{
		userRepository: userRepository,
		passwordHasher: passwordHasher,
	}
}

// Execute changes the signed-in user's username and/or password, after confirming their current
// password.
func (uc *UseCase) Execute(
	ctx context.Context,
	input Input,
) (Output, error) {
	if err := input.Validate(); err != nil {
		slog.InfoContext(ctx, "update credentials: validation", "error", err)

		return Output{}, domainerror.ToValidationError(err)
	}

	current, err := uc.userRepository.FindByID(ctx, input.User.ID)
	if err != nil {
		slog.ErrorContext(ctx, "update credentials: find user", "error", err)

		return Output{}, errors.New("Failed to verify current password")
	}

	if !uc.passwordHasher.Compare(current.PasswordHash, input.CurrentPassword) {
		slog.InfoContext(ctx, "update credentials: incorrect current password", "user_id", input.User.ID)

		return Output{}, &domainerror.UnauthorizedError{Message: "Incorrect current password"}
	}

	updated := input.User

	if newUsername := usecase.NormalizeUsername(input.NewUsername); newUsername != "" && newUsername != updated.Username {
		if err = uc.userRepository.UpdateUsername(ctx, updated.ID, newUsername); err != nil {
			if domainerror.IsConflictError(err) {
				return Output{}, &domainerror.ConflictError{
					Message: "A user with this username is already registered",
				}
			}

			slog.ErrorContext(ctx, "update credentials: update username", "error", err)

			return Output{}, errors.New("Failed to update username")
		}

		updated.Username = newUsername
	}

	if input.NewPassword != "" {
		hash, hashErr := uc.passwordHasher.Hash(input.NewPassword)
		if hashErr != nil {
			slog.ErrorContext(ctx, "update credentials: hash password", "error", hashErr)

			return Output{}, errors.New("Failed to process password")
		}

		if err = uc.userRepository.UpdatePasswordHash(ctx, updated.ID, hash); err != nil {
			slog.ErrorContext(ctx, "update credentials: update password", "error", err)

			return Output{}, errors.New("Failed to update password")
		}
	}

	return Output{User: updated}, nil
}
