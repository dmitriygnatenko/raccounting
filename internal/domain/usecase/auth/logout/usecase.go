// Package logout is the LogoutUser use case: it ends a session. It has no output.go — Execute takes
// a bare token and returns nothing; it's best-effort by design, an absent or already-gone token is
// not an error, and the caller always ends up logged out client-side regardless.
package logout

import (
	"context"
	"log/slog"

	domainError "raccounting/internal/domain/error"
	"raccounting/internal/port"
)

// UseCase implements LogoutUser.
type UseCase struct {
	sessionRepository port.SessionRepository
}

// New builds a UseCase from its dependencies.
func New(
	sessionRepository port.SessionRepository,
) *UseCase {
	return &UseCase{
		sessionRepository: sessionRepository,
	}
}

// Execute deletes the session behind token, if any.
func (uc *UseCase) Execute(
	ctx context.Context,
	input Input,
) {
	if input.Token == "" {
		slog.InfoContext(ctx, "logout: empty token")

		return
	}

	err := uc.sessionRepository.Delete(ctx, input.Token)
	if err != nil && !domainError.IsNotFoundError(err) {
		slog.ErrorContext(ctx, "logout: delete token", "error", err)
	}
}
