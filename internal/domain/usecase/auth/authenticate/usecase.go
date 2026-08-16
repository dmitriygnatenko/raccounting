// Package authenticate is the AuthenticateSession use case: it resolves a session token into the
// user it belongs to, used by the requireAuth middleware.
package authenticate

import (
	"context"
	"log/slog"
	"time"

	domainError "raccounting/internal/domain/error"
	"raccounting/internal/port"
)

// unauthorized is returned for every failure mode below (missing token, no matching/expired
// session, dangling user row).
var unauthorized = &domainError.UnauthorizedError{
	Message: "Authentication required",
}

// UseCase implements AuthenticateSession.
type UseCase struct {
	sessionRepository port.SessionRepository
	userRepository    port.UserRepository
}

// New builds a UseCase from its dependencies.
func New(
	sessionRepository port.SessionRepository,
	userRepository port.UserRepository,
) *UseCase {
	return &UseCase{
		sessionRepository: sessionRepository,
		userRepository:    userRepository,
	}
}

// Execute resolves token into the user it belongs to, failing with unauthorized for a
// missing/expired/dangling session.
func (uc *UseCase) Execute(
	ctx context.Context,
	input Input,
) (Output, error) {
	if input.Token == "" {
		return Output{}, unauthorized
	}

	session, err := uc.sessionRepository.FindByToken(ctx, input.Token)
	if err != nil {
		if !domainError.IsNotFoundError(err) {
			slog.ErrorContext(ctx, "authenticate: find by token", "error", err)
		}

		return Output{}, unauthorized
	}

	if session.ExpiresAt.Before(time.Now().UTC()) {
		slog.InfoContext(ctx, "authenticate: expired token")

		if err = uc.sessionRepository.Delete(ctx, input.Token); err != nil {
			slog.ErrorContext(ctx, "authenticate: delete expired token", "error", err)
		}

		return Output{}, unauthorized
	}

	user, err := uc.userRepository.FindByID(ctx, session.UserID)
	if err != nil {
		slog.ErrorContext(ctx, "authenticate: find user", "id", session.UserID, "error", err)

		return Output{}, unauthorized
	}

	return Output{
		User: user.Public(),
	}, nil
}
