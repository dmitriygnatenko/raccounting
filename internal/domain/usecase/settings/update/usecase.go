// Package update is the UpdateSettings use case: it changes the signed-in user's UI settings
// (language, theme).
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

// Execute changes the signed-in user's UI settings (language, theme).
func (uc *UseCase) Execute(ctx context.Context, input Input) (Output, error) {
	if err := input.Validate(); err != nil {
		slog.InfoContext(ctx, "update settings: validation", "error", err)

		return Output{}, domainerror.ToValidationError(err)
	}

	settings := entity.UserSettings{Language: input.Language, Theme: input.Theme}

	if err := uc.userRepository.UpdateSettings(ctx, input.UserID, settings); err != nil {
		slog.ErrorContext(ctx, "update settings: save", "error", err)

		return Output{}, errors.New("Failed to update settings")
	}

	return Output{Settings: settings}, nil
}
