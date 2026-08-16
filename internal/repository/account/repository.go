// Package account implements port.AccountRepository on top of the accounts table.
package account

import (
	"context"
	"database/sql"
	"errors"

	"raccounting/internal/domain/entity"
	domainerror "raccounting/internal/domain/error"
	"raccounting/internal/port"
	storageError "raccounting/internal/storage/error"
	"raccounting/internal/storage/model"
)

const notFoundMessage = "Account not found"

// inUseMessage is what Delete reports when a transaction still references the account.
const inUseMessage = "This account is in use — remove its transactions first"

// insufficientBalanceMessage is what Create reports for a negative opening balance.
const insufficientBalanceMessage = "Opening balance can't be negative"

// Storage is the slice of the DB adapter this repository uses — the accounts table and nothing
// else.
type Storage interface {
	ListAccounts(ctx context.Context) ([]model.Account, error)
	// FindAccountByID returns sql.ErrNoRows when no account with this id exists.
	FindAccountByID(ctx context.Context, id uint64) (model.Account, error)
	CreateAccount(ctx context.Context, req port.AccountCreateRequest) (id uint64, err error)
	// UpdateAccount changes name/type/currency/status, returning the full updated row. found is
	// false if no account with this id exists.
	UpdateAccount(ctx context.Context, req port.AccountUpdateRequest) (row model.Account, found bool, err error)
	// DeleteAccount removes an account row. found is false if no account with this id existed. A
	// FOREIGN KEY violation (the account is still referenced by a transaction) comes back wrapped in
	// storageError.ForeignKeyViolationError.
	DeleteAccount(ctx context.Context, id uint64) (found bool, err error)
}

// Repository implements port.AccountRepository.
type Repository struct {
	storage Storage
}

// New builds a Repository against s.
func New(s Storage) *Repository {
	return &Repository{storage: s}
}

// List returns every account.
func (r *Repository) List(ctx context.Context) ([]entity.Account, error) {
	rows, err := r.storage.ListAccounts(ctx)
	if err != nil {
		return nil, err
	}

	accounts := make([]entity.Account, len(rows))
	for i, row := range rows {
		accounts[i] = row.ToEntity()
	}

	return accounts, nil
}

// FindByID looks up an account by id.
func (r *Repository) FindByID(ctx context.Context, id uint64) (entity.Account, error) {
	row, err := r.storage.FindAccountByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Account{}, &domainerror.NotFoundError{Message: notFoundMessage}
		}

		return entity.Account{}, err
	}

	return row.ToEntity(), nil
}

// Create inserts a new account and returns it. A negative opening balance is reported as a
// *domainerror.ConflictError (accounts.balance can never go negative).
func (r *Repository) Create(ctx context.Context, req port.AccountCreateRequest) (entity.Account, error) {
	id, err := r.storage.CreateAccount(ctx, req)
	if err != nil {
		if errors.Is(err, storageError.InsufficientBalanceError) {
			return entity.Account{}, &domainerror.ConflictError{Message: insufficientBalanceMessage}
		}

		return entity.Account{}, err
	}

	return entity.Account{
		ID:           id,
		Name:         req.Name,
		Type:         req.Type,
		CurrencyCode: req.CurrencyCode,
		Balance:      req.Balance,
		Status:       entity.AccountStatusActive,
	}, nil
}

// Update changes an existing account's name/type/currency/archived flag.
func (r *Repository) Update(ctx context.Context, req port.AccountUpdateRequest) (entity.Account, error) {
	row, found, err := r.storage.UpdateAccount(ctx, req)
	if err != nil {
		return entity.Account{}, err
	}

	if !found {
		return entity.Account{}, &domainerror.NotFoundError{Message: notFoundMessage}
	}

	return row.ToEntity(), nil
}

// Delete removes an account, reporting a *domainerror.NotFoundError if it doesn't exist, or a
// *domainerror.ConflictError if it's still referenced by a transaction.
func (r *Repository) Delete(ctx context.Context, id uint64) error {
	found, err := r.storage.DeleteAccount(ctx, id)
	if err != nil {
		if errors.Is(err, storageError.ForeignKeyViolationError) {
			return &domainerror.ConflictError{Message: inUseMessage}
		}

		return err
	}

	if !found {
		return &domainerror.NotFoundError{Message: notFoundMessage}
	}

	return nil
}
