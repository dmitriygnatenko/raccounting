// Package transaction implements port.TransactionRepository on top of the transactions table. Every
// write here also adjusts the affected account's balance, atomically, inside the same DB
// transaction — see the DB adapters' *WithBalance methods.
package transaction

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

const notFoundMessage = "Transaction not found"

// insufficientBalanceMessage is what Create/Update/Delete report when applying the transaction's
// balance effect would take its account negative.
const insufficientBalanceMessage = "This would overdraw the account"

// Storage is the slice of the DB adapter this repository uses — the transactions table, plus the
// balance side effect on accounts.
type Storage interface {
	ListTransactions(ctx context.Context) ([]model.Transaction, error)
	// FindTransactionByID returns sql.ErrNoRows when no transaction with this id exists.
	FindTransactionByID(ctx context.Context, id uint64) (model.Transaction, error)
	// CreateTransactionWithBalance inserts a transaction row and adjusts its account's balance,
	// atomically, in one DB transaction.
	CreateTransactionWithBalance(ctx context.Context, req port.TransactionCreateRequest) (model.Transaction, error)
	// UpdateTransactionWithBalance reverses the transaction's old balance effect and applies the new
	// one — even across an account change — atomically, in one DB transaction. found is false if no
	// transaction with this id exists.
	UpdateTransactionWithBalance(
		ctx context.Context, req port.TransactionUpdateRequest,
	) (row model.Transaction, found bool, err error)
	// DeleteTransactionWithBalance removes a transaction row and reverses its balance effect,
	// atomically, in one DB transaction. found is false if no transaction with this id existed.
	DeleteTransactionWithBalance(ctx context.Context, id uint64) (found bool, err error)
}

// Repository implements port.TransactionRepository.
type Repository struct {
	storage Storage
}

// New builds a Repository against s.
func New(s Storage) *Repository {
	return &Repository{storage: s}
}

// List returns every transaction.
func (r *Repository) List(ctx context.Context) ([]entity.Transaction, error) {
	rows, err := r.storage.ListTransactions(ctx)
	if err != nil {
		return nil, err
	}

	transactions := make([]entity.Transaction, len(rows))
	for i, row := range rows {
		transactions[i] = row.ToEntity()
	}

	return transactions, nil
}

// FindByID looks up a transaction by id.
func (r *Repository) FindByID(ctx context.Context, id uint64) (entity.Transaction, error) {
	row, err := r.storage.FindTransactionByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Transaction{}, &domainerror.NotFoundError{Message: notFoundMessage}
		}

		return entity.Transaction{}, err
	}

	return row.ToEntity(), nil
}

// Create inserts a new transaction and adjusts its account's balance. A negative resulting balance
// is reported as a *domainerror.ConflictError.
func (r *Repository) Create(ctx context.Context, req port.TransactionCreateRequest) (entity.Transaction, error) {
	row, err := r.storage.CreateTransactionWithBalance(ctx, req)
	if err != nil {
		if errors.Is(err, storageError.InsufficientBalanceError) {
			return entity.Transaction{}, &domainerror.ConflictError{Message: insufficientBalanceMessage}
		}

		return entity.Transaction{}, err
	}

	return row.ToEntity(), nil
}

// Update changes an existing transaction, reversing its old balance effect and applying the new
// one. A negative resulting balance is reported as a *domainerror.ConflictError.
func (r *Repository) Update(ctx context.Context, req port.TransactionUpdateRequest) (entity.Transaction, error) {
	row, found, err := r.storage.UpdateTransactionWithBalance(ctx, req)
	if err != nil {
		if errors.Is(err, storageError.InsufficientBalanceError) {
			return entity.Transaction{}, &domainerror.ConflictError{Message: insufficientBalanceMessage}
		}

		return entity.Transaction{}, err
	}

	if !found {
		return entity.Transaction{}, &domainerror.NotFoundError{Message: notFoundMessage}
	}

	return row.ToEntity(), nil
}

// Delete removes a transaction and reverses its balance effect. A negative resulting balance is
// reported as a *domainerror.ConflictError.
func (r *Repository) Delete(ctx context.Context, id uint64) error {
	found, err := r.storage.DeleteTransactionWithBalance(ctx, id)
	if err != nil {
		if errors.Is(err, storageError.InsufficientBalanceError) {
			return &domainerror.ConflictError{Message: insufficientBalanceMessage}
		}

		return err
	}

	if !found {
		return &domainerror.NotFoundError{Message: notFoundMessage}
	}

	return nil
}
