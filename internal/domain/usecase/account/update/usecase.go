// Package update is the UpdateAccount use case: it renames/retypes/archives an existing account.
// Its balance can't be changed this way — only the transaction/transfer use cases touch it.
package update

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"raccounting/internal/domain/entity"
	domainError "raccounting/internal/domain/error"
	"raccounting/internal/port"
)

// UseCase implements UpdateAccount.
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

// Execute updates an existing account's name/type/currency/archived flag.
func (uc *UseCase) Execute(
	ctx context.Context,
	input Input,
) (Output, error) {
	if err := input.Validate(); err != nil {
		slog.InfoContext(ctx, "update account: validation", "error", err)

		return Output{}, domainError.ToValidationError(err)
	}

	name := strings.TrimSpace(input.Name)

	exists, err := uc.currencyRepository.Exists(ctx, input.CurrencyCode)
	if err != nil {
		slog.ErrorContext(ctx, "update account: verify currency", "error", err)

		return Output{}, errors.New("Failed to verify currency")
	}

	if !exists {
		slog.InfoContext(ctx, "update account: currency not found", "code", input.CurrencyCode)

		return Output{}, &domainError.ValidationError{Message: "Currency not found"}
	}

	accountType, _ := entity.ParseAccountType(input.Type)

	account, err := uc.accountRepository.Update(ctx, port.AccountUpdateRequest{
		ID:           input.ID,
		Name:         name,
		Type:         accountType,
		CurrencyCode: input.CurrencyCode,
		Archived:     input.Archived,
	})
	if err != nil {
		if domainError.IsNotFoundError(err) {
			return Output{}, &domainError.NotFoundError{Message: "Account not found"}
		}

		slog.ErrorContext(ctx, "update account: save", "id", input.ID, "error", err)

		return Output{}, errors.New("Failed to update account")
	}

	return Output{Account: account}, nil
}
