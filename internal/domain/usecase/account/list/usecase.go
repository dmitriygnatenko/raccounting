// Package list is the ListAccounts use case: it returns every account belonging to the signed-in
// user.
package list

import (
	"context"
	"errors"
	"log/slog"

	"raccounting/internal/domain/entity"
	"raccounting/internal/port"
)

// UseCase implements ListAccounts.
type UseCase struct {
	accountRepository port.AccountRepository
}

// New builds a UseCase from its dependencies.
func New(
	accountRepository port.AccountRepository,
) *UseCase {
	return &UseCase{
		accountRepository: accountRepository,
	}
}

// Execute returns every account.
func (uc *UseCase) Execute(ctx context.Context) (Output, error) {
	accounts, err := uc.accountRepository.List(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "list accounts: load", "error", err)

		return Output{}, errors.New("Failed to load accounts")
	}

	if accounts == nil {
		accounts = []entity.Account{}
	}

	return Output{Accounts: accounts}, nil
}
