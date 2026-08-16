// Package update is the UpdateSettings use case: it changes the signed-in user's UI language
// preference.
package update

import (
	"context"
	"errors"
	"log/slog"

	"raccounting/internal/domain/entity"
	domainerror "raccounting/internal/domain/error"
	"raccounting/internal/port"
)

// UseCase implements UpdateSettings.
type UseCase struct {
	userRepository port.UserRepository
}

// New builds a UseCase from its dependencies.
func New(userRepository port.UserRepository) *UseCase {
	return &UseCase{userRepository: userRepository}
}

// Execute changes the signed-in user's UI language preference.
func (uc *UseCase) Execute(ctx context.Context, input Input) (Output, error) {
	if err := input.Validate(); err != nil {
		slog.InfoContext(ctx, "update settings: validation", "error", err)

		return Output{}, domainerror.ToValidationError(err)
	}

	if err := uc.userRepository.UpdateSettings(ctx, input.UserID, input.Language); err != nil {
		slog.ErrorContext(ctx, "update settings: save", "error", err)

		return Output{}, errors.New("Failed to update settings")
	}

	return Output{Settings: entity.UserSettings{Language: input.Language}}, nil
}
