// Package login is the LoginUser use case: raccounting is single-user, so this both verifies a
// username/password pair and, the very first time it's ever called against an empty database,
// auto-provisions the one account it will check from then on — mirroring App.api.login in the
// existing frontend mock exactly.
package login

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"raccounting/internal/domain/entity"
	domainerror "raccounting/internal/domain/error"
	"raccounting/internal/domain/usecase"
	"raccounting/internal/port"
)

const sessionDuration = 30 * 24 * time.Hour

// incorrectCredentials is returned for both an unknown/mismatched username and a wrong password —
// the two are deliberately not distinguished to callers.
var incorrectCredentials = &domainerror.UnauthorizedError{
	Message: "Incorrect username or password",
}

// UseCase implements LoginUser.
type UseCase struct {
	userRepository    port.UserRepository
	sessionRepository port.SessionRepository
	passwordHasher    port.PasswordHasher
	tokenGenerator    port.TokenGenerator
}

// New builds a UseCase from its dependencies.
func New(
	userRepository port.UserRepository,
	sessionRepository port.SessionRepository,
	passwordHasher port.PasswordHasher,
	tokenGenerator port.TokenGenerator,
) *UseCase {
	return &UseCase{
		userRepository:    userRepository,
		sessionRepository: sessionRepository,
		passwordHasher:    passwordHasher,
		tokenGenerator:    tokenGenerator,
	}
}

// Execute verifies the given credentials against the single account, auto-provisioning it first if
// none exists yet, and starts a new session on success.
func (uc *UseCase) Execute(
	ctx context.Context,
	input Input,
) (Output, error) {
	if err := input.Validate(); err != nil {
		slog.InfoContext(ctx, "login: validation", "error", err)

		return Output{}, domainerror.ToValidationError(err)
	}

	username := usecase.NormalizeUsername(input.Username)

	user, err := uc.resolveUser(ctx, username, input.Password)
	if err != nil {
		return Output{}, err
	}

	if user.Settings.Language == "" && input.Language != "" {
		uc.saveLanguage(ctx, &user, input.Language)
	}

	token, err := uc.tokenGenerator.NewToken()
	if err != nil {
		slog.ErrorContext(ctx, "login: failed to generate a token", "error", err)

		return Output{}, errors.New("Failed to generate a token")
	}

	session := entity.Session{
		Token:     token,
		UserID:    user.ID,
		ExpiresAt: time.Now().UTC().Add(sessionDuration),
	}
	if err = uc.sessionRepository.Create(ctx, session); err != nil {
		slog.ErrorContext(ctx, "login: failed to create a session", "error", err)

		return Output{}, errors.New("Failed to start a session")
	}

	return Output{
		User:    user.Public(),
		Session: session,
	}, nil
}

// resolveUser returns the single account, auto-provisioning it (with the given credentials) the
// first time this is ever called against an empty database.
func (uc *UseCase) resolveUser(
	ctx context.Context, username, password string,
) (entity.User, error) {
	count, err := uc.userRepository.Count(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "login: count users", "error", err)

		return entity.User{}, errors.New("Failed to check for an existing account")
	}

	if count == 0 {
		return uc.provisionUser(ctx, username, password)
	}

	user, err := uc.userRepository.FindByUsername(ctx, username)
	if err != nil {
		if !domainerror.IsNotFoundError(err) {
			slog.ErrorContext(ctx, "login: find user", "error", err)
		}

		slog.InfoContext(ctx, "login: user is not found")

		return entity.User{}, incorrectCredentials
	}

	if !uc.passwordHasher.Compare(user.PasswordHash, password) {
		slog.InfoContext(ctx, "login: incorrect password")

		return entity.User{}, incorrectCredentials
	}

	return user, nil
}

// saveLanguage persists language as user's UI language preference and reflects it onto user, so the
// response the caller builds from it is already up to date. It's best-effort: a failure here
// shouldn't fail the login itself, just leave the language unsaved for next time.
func (uc *UseCase) saveLanguage(ctx context.Context, user *entity.User, language string) {
	if err := uc.userRepository.UpdateSettings(ctx, user.ID, language); err != nil {
		slog.ErrorContext(ctx, "login: save language", "error", err)

		return
	}

	user.Settings = entity.UserSettings{Language: language}
}

// provisionUser creates the single account on the very first successful login.
func (uc *UseCase) provisionUser(
	ctx context.Context, username, password string,
) (entity.User, error) {
	hash, err := uc.passwordHasher.Hash(password)
	if err != nil {
		slog.ErrorContext(ctx, "login: hash password", "error", err)

		return entity.User{}, errors.New("Failed to process password")
	}

	id, err := uc.userRepository.Create(ctx, port.UserCreateRequest{
		Username:     username,
		PasswordHash: hash,
	})
	if err != nil {
		slog.ErrorContext(ctx, "login: provision account", "error", err)

		return entity.User{}, errors.New("Failed to create account")
	}

	slog.InfoContext(ctx, "login: auto-provisioned the account", "username", username)

	return entity.User{
		ID:           id,
		Username:     username,
		PasswordHash: hash,
	}, nil
}
