package port

import (
	"context"

	"raccounting/internal/domain/entity"
)

// AccountCreateRequest bundles the AccountRepository.Create parameters.
type AccountCreateRequest struct {
	Name         string
	Type         entity.AccountType
	CurrencyCode string
	Balance      int64
}

// AccountUpdateRequest bundles the AccountRepository.Update parameters. Balance is deliberately
// absent — it is only ever changed by the transaction/transfer use cases, inside their own DB
// transaction.
type AccountUpdateRequest struct {
	ID           uint64
	Name         string
	Type         entity.AccountType
	CurrencyCode string
	Archived     bool
}

// AccountRepository persists Accounts
type AccountRepository interface {
	List(ctx context.Context) ([]entity.Account, error)
	// FindByID returns a *domainerror.NotFoundError if no account with this id exists.
	FindByID(ctx context.Context, id uint64) (entity.Account, error)
	Create(ctx context.Context, req AccountCreateRequest) (entity.Account, error)
	// Update returns a *domainerror.NotFoundError if no account with this id exists.
	Update(ctx context.Context, req AccountUpdateRequest) (entity.Account, error)
	// Delete returns a *domainerror.NotFoundError if no account with this id exists, or a
	// *domainerror.ConflictError if it is still referenced by a transaction.
	Delete(ctx context.Context, id uint64) error
}
