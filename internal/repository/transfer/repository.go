// Package transfer implements port.TransferRepository on top of the transactions table: a transfer
// is two linked rows pointing at each other's id via transfer_transaction_id, created/deleted
// together with both accounts' balances kept in sync, atomically, inside a single DB transaction —
// see the DB adapters.
package transfer

import (
	"context"
	"errors"

	domainerror "raccounting/internal/domain/error"
	"raccounting/internal/port"
	storageError "raccounting/internal/storage/error"
	"raccounting/internal/storage/model"
)

// insufficientBalanceMessage is what Create/Delete report when applying a transfer's balance effect
// would take one of its accounts negative.
const insufficientBalanceMessage = "This would overdraw the account"

// Storage is the slice of the DB adapter this repository uses.
type Storage interface {
	// CreateTransferWithBalance inserts both transfer legs and adjusts both accounts' balances,
	// atomically, in one DB transaction.
	CreateTransferWithBalance(
		ctx context.Context, req port.TransferCreateRequest,
	) (legFrom, legTo model.Transaction, err error)
	// DeleteTransferWithBalance removes both legs of the transfer that id belongs to and reverses
	// their balance effects, atomically, in one DB transaction. found is false if no transaction
	// with this id existed.
	DeleteTransferWithBalance(ctx context.Context, id uint64) (found bool, err error)
}

// Repository implements port.TransferRepository.
type Repository struct {
	storage Storage
}

// New builds a Repository against s.
func New(s Storage) *Repository {
	return &Repository{storage: s}
}

// Create inserts a new transfer's two legs and adjusts both accounts' balances. A negative
// resulting balance is reported as a *domainerror.ConflictError.
func (r *Repository) Create(ctx context.Context, req port.TransferCreateRequest) (port.TransferResult, error) {
	legFrom, legTo, err := r.storage.CreateTransferWithBalance(ctx, req)
	if err != nil {
		if errors.Is(err, storageError.InsufficientBalanceError) {
			return port.TransferResult{}, &domainerror.ConflictError{Message: insufficientBalanceMessage}
		}

		return port.TransferResult{}, err
	}

	return port.TransferResult{
		LegFrom: legFrom.ToEntity(),
		LegTo:   legTo.ToEntity(),
	}, nil
}

// Delete removes both legs of a transfer and reverses their balance effects. A negative resulting
// balance is reported as a *domainerror.ConflictError.
func (r *Repository) Delete(ctx context.Context, id uint64) (bool, error) {
	found, err := r.storage.DeleteTransferWithBalance(ctx, id)
	if err != nil && errors.Is(err, storageError.InsufficientBalanceError) {
		return false, &domainerror.ConflictError{Message: insufficientBalanceMessage}
	}

	return found, err
}
