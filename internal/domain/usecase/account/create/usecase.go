// Package create is the CreateAccount use case.
package create

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"raccounting/internal/domain/entity"
	domainError "raccounting/internal/domain/error"
	"raccounting/internal/port"
)

// UseCase implements CreateAccount.
type UseCase struct {
	accountRepository  port.AccountRepository
	currencyRepository port.CurrencyRepository
}

// New builds a UseCase from its dependencies.
func New(
	accountRepository port.AccountRepository,
	currencyRepository port.CurrencyRepository,
) *UseCase {
	return &UseCase{
		accountRepository:  accountRepository,
		currencyRepository: currencyRepository,
	}
}

// Execute validates and creates a new account, verifying its currency exists first.
func (uc *UseCase) Execute(
	ctx context.Context,
	input Input,
) (Output, error) {
	if err := input.Validate(); err != nil {
		slog.InfoContext(ctx, "create account: validation", "error", err)

		return Output{}, domainError.ToValidationError(err)
	}

	name := strings.TrimSpace(input.Name)

	exists, err := uc.currencyRepository.Exists(ctx, input.CurrencyCode)
	if err != nil {
		slog.ErrorContext(ctx, "create account: verify currency", "error", err)

		return Output{}, errors.New("Failed to verify currency")
	}

	if !exists {
		slog.InfoContext(ctx, "create account: currency not found", "code", input.CurrencyCode)

		return Output{}, &domainError.ValidationError{Message: "Currency not found"}
	}

	accountType, _ := entity.ParseAccountType(input.Type)

	account, err := uc.accountRepository.Create(ctx, port.AccountCreateRequest{
		Name:         name,
		Type:         accountType,
		CurrencyCode: input.CurrencyCode,
		Balance:      input.Balance,
	})
	if err != nil {
		if domainError.IsConflictError(err) {
			return Output{}, &domainError.ConflictError{Message: "Opening balance can't be negative"}
		}

		slog.ErrorContext(ctx, "create account: save", "error", err)

		return Output{}, errors.New("Failed to save account")
	}

	return Output{Account: account}, nil
}
