// Package currency implements port.CurrencyRepository on top of the currencies table.
package currency

import (
	"context"
	"errors"

	"raccounting/internal/domain/entity"
	domainerror "raccounting/internal/domain/error"
	"raccounting/internal/port"
	storageError "raccounting/internal/storage/error"
	"raccounting/internal/storage/model"
)

//go:generate go tool mockgen -source=repository.go -destination=mocks/storage_mock.go -package=mocks

// Storage is the slice of the mysql adapter this repository uses — the currencies table and nothing
// else.
type Storage interface {
	ListCurrencies(ctx context.Context) ([]model.Currency, error)
	// ExistsCurrency reports whether a currency with this code exists.
	ExistsCurrency(ctx context.Context, code string) (bool, error)
	// CreateCurrency inserts a currency row. A taken code comes back wrapped in
	// storageError.UniqueViolationError.
	CreateCurrency(ctx context.Context, req model.CurrencyCreateRequest) error
	// UpdateCurrency changes symbol/name/rate/default/archived, returning the full updated row.
	// found is false if no currency with this code exists.
	UpdateCurrency(
		ctx context.Context,
		req model.CurrencyUpdateRequest,
	) (row model.Currency, found bool, err error)
	// DeleteCurrency removes a currency row. found is false if no currency with this code existed. A
	// FOREIGN KEY violation (the currency is still referenced by an account) comes back wrapped in
	// storageError.ForeignKeyViolationError.
	DeleteCurrency(ctx context.Context, code string) (found bool, err error)
}

// Repository implements port.CurrencyRepository.
type Repository struct {
	storage Storage
}

// New builds a Repository against s.
func New(s Storage) *Repository {
	return &Repository{storage: s}
}

// List returns every currency.
func (r *Repository) List(ctx context.Context) ([]entity.Currency, error) {
	rows, err := r.storage.ListCurrencies(ctx)
	if err != nil {
		return nil, err
	}

	currencies := make([]entity.Currency, len(rows))
	for i, row := range rows {
		currencies[i] = row.ToEntity()
	}

	return currencies, nil
}

// Exists reports whether a currency with this code exists.
func (r *Repository) Exists(ctx context.Context, code string) (bool, error) {
	return r.storage.ExistsCurrency(ctx, code)
}

// Create inserts a new currency and returns it. A code collision is reported as a message-less
// *domainerror.ConflictError — the use case supplies the message.
func (r *Repository) Create(
	ctx context.Context, req port.CurrencyCreateRequest,
) (entity.Currency, error) {
	if err := r.storage.CreateCurrency(ctx, model.CurrencyCreateRequest{
		Code:    req.Code,
		Symbol:  req.Symbol,
		Name:    req.Name,
		Rate:    req.Rate,
		Default: req.Default,
	}); err != nil {
		if errors.Is(err, storageError.UniqueViolationError) {
			return entity.Currency{}, &domainerror.ConflictError{}
		}

		return entity.Currency{}, err
	}

	return entity.Currency{
		Code:    req.Code,
		Symbol:  req.Symbol,
		Name:    req.Name,
		Rate:    req.Rate,
		Default: req.Default,
		Status:  entity.CurrencyStatusActive,
	}, nil
}

// Update changes an existing currency's symbol/name/rate/default/archived flag. An unknown code is
// reported as a message-less *domainerror.NotFoundError — the use case supplies the message.
func (r *Repository) Update(
	ctx context.Context, req port.CurrencyUpdateRequest,
) (entity.Currency, error) {
	row, found, err := r.storage.UpdateCurrency(ctx, model.CurrencyUpdateRequest{
		Code:     req.Code,
		Symbol:   req.Symbol,
		Name:     req.Name,
		Rate:     req.Rate,
		Default:  req.Default,
		Archived: req.Archived,
	})
	if err != nil {
		return entity.Currency{}, err
	}

	if !found {
		return entity.Currency{}, &domainerror.NotFoundError{}
	}

	return row.ToEntity(), nil
}

// Delete removes a currency, reporting a *domainerror.NotFoundError if it doesn't exist, or a
// *domainerror.ConflictError if it's still referenced by an account. Both are message-less — the
// use case supplies the message.
func (r *Repository) Delete(ctx context.Context, code string) error {
	found, err := r.storage.DeleteCurrency(ctx, code)
	if err != nil {
		if errors.Is(err, storageError.ForeignKeyViolationError) {
			return &domainerror.ConflictError{}
		}

		return err
	}

	if !found {
		return &domainerror.NotFoundError{}
	}

	return nil
}
