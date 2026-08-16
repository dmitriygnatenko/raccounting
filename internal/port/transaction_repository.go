package port

import (
	"context"
	"time"

	"raccounting/internal/domain/entity"
)

// TransactionCreateRequest bundles the TransactionRepository.Create parameters. Creating a
// transaction also adjusts its account's balance, atomically, in the same DB transaction.
// CurrencyCode is the account's currency at the time of the transaction (resolved by the use case,
// not supplied by the caller).
type TransactionCreateRequest struct {
	AccountID    uint64
	CategoryID   *uint64
	Type         entity.TransactionType
	CurrencyCode string
	Amount       int64
	Memo         string
	OperationAt  time.Time
}

// TransactionUpdateRequest bundles the TransactionRepository.Update parameters. Updating a
// transaction reverses its old balance effect and applies the new one, atomically, in the same DB
// transaction — even when AccountID changed.
type TransactionUpdateRequest struct {
	ID           uint64
	AccountID    uint64
	CategoryID   *uint64
	Type         entity.TransactionType
	CurrencyCode string
	Amount       int64
	Memo         string
	OperationAt  time.Time
}

// TransactionRepository persists Transactions, keeping each transaction's account balance in sync
// as a side effect of Create/Update/Delete.
// Transfer legs (Type == entity.TransactionTypeTransfer) are managed through
// TransferRepository instead, not through this interface's Create/Update.
type TransactionRepository interface {
	List(ctx context.Context) ([]entity.Transaction, error)
	// FindByID returns a *domainerror.NotFoundError if no transaction with this id exists.
	FindByID(ctx context.Context, id uint64) (entity.Transaction, error)
	Create(ctx context.Context, req TransactionCreateRequest) (entity.Transaction, error)
	// Update returns a *domainerror.NotFoundError if no transaction with this id exists.
	Update(ctx context.Context, req TransactionUpdateRequest) (entity.Transaction, error)
	// Delete returns a *domainerror.NotFoundError if no transaction with this id exists.
	Delete(ctx context.Context, id uint64) error
}
