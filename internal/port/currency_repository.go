package port

import (
	"context"

	"raccounting/internal/domain/entity"
)

// CurrencyCreateRequest bundles the CurrencyRepository.Create parameters.
type CurrencyCreateRequest struct {
	Code    string
	Symbol  string
	Name    string
	Rate    float64
	Default bool
}

// CurrencyUpdateRequest bundles the CurrencyRepository.Update parameters.
type CurrencyUpdateRequest struct {
	Code     string
	Symbol   string
	Name     string
	Rate     float64
	Default  bool
	Archived bool
}

// CurrencyRepository persists Currencies
type CurrencyRepository interface {
	List(ctx context.Context) ([]entity.Currency, error)
	// Exists reports whether a currency with this code exists — used to validate an account's
	// currency before saving it.
	Exists(ctx context.Context, code string) (bool, error)
	// Create returns a *domainerror.ConflictError if a currency with this code already exists.
	Create(ctx context.Context, req CurrencyCreateRequest) (entity.Currency, error)
	// Update returns a *domainerror.NotFoundError if no currency with this code exists.
	Update(ctx context.Context, req CurrencyUpdateRequest) (entity.Currency, error)
	// Delete returns a *domainerror.NotFoundError if no currency with this code exists, or a
	// *domainerror.ConflictError if it is still referenced by an account.
	Delete(ctx context.Context, code string) error
}
