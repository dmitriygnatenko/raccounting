package create

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"raccounting/internal/domain/entity"
	"raccounting/internal/port"
)

// fakeAccountRepository is a hand-rolled test double for port.AccountRepository.
type fakeAccountRepository struct {
	createFn func(ctx context.Context, req port.AccountCreateRequest) (entity.Account, error)
}

func (f *fakeAccountRepository) List(context.Context) ([]entity.Account, error) {
	panic("not stubbed")
}

func (f *fakeAccountRepository) FindByID(context.Context, uint64) (entity.Account, error) {
	panic("not stubbed")
}

func (f *fakeAccountRepository) Create(ctx context.Context, req port.AccountCreateRequest) (entity.Account, error) {
	return f.createFn(ctx, req)
}

func (f *fakeAccountRepository) Update(context.Context, port.AccountUpdateRequest) (entity.Account, error) {
	panic("not stubbed")
}

func (f *fakeAccountRepository) Delete(context.Context, uint64) error {
	panic("not stubbed")
}

// fakeCurrencyRepository is a hand-rolled test double for port.CurrencyRepository.
type fakeCurrencyRepository struct {
	existsFn func(ctx context.Context, code string) (bool, error)
}

func (f *fakeCurrencyRepository) List(context.Context) ([]entity.Currency, error) {
	panic("not stubbed")
}

func (f *fakeCurrencyRepository) Exists(ctx context.Context, code string) (bool, error) {
	return f.existsFn(ctx, code)
}

func (f *fakeCurrencyRepository) Create(context.Context, port.CurrencyCreateRequest) (entity.Currency, error) {
	panic("not stubbed")
}

func (f *fakeCurrencyRepository) Update(context.Context, port.CurrencyUpdateRequest) (entity.Currency, error) {
	panic("not stubbed")
}

func (f *fakeCurrencyRepository) Delete(context.Context, string) error {
	panic("not stubbed")
}

func newUseCase() (*UseCase, *fakeAccountRepository, *fakeCurrencyRepository) {
	accounts := &fakeAccountRepository{}
	currencies := &fakeCurrencyRepository{}

	return New(accounts, currencies), accounts, currencies
}

var errStub = errors.New("stub failure")

func TestUseCase_Execute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		mock         func(accounts *fakeAccountRepository, currencies *fakeCurrencyRepository) Input
		assertResult func(t *testing.T, got Output)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "blank name fails validation before any lookup",
			mock: func(*fakeAccountRepository, *fakeCurrencyRepository) Input {
				return Input{Name: "", Type: uint8(entity.AccountTypeCash), CurrencyCode: "RUB"}
			},
			assertResult: func(t *testing.T, got Output) { require.Equal(t, Output{}, got) },
			assertErr: func(t *testing.T, err error) {
				require.EqualError(t, err, "Please enter an account name")
			},
		},
		{
			name: "overlong name fails validation",
			mock: func(*fakeAccountRepository, *fakeCurrencyRepository) Input {
				return Input{Name: strings.Repeat("a", entity.MaxAccountNameLength+1), Type: uint8(entity.AccountTypeCash), CurrencyCode: "RUB"}
			},
			assertResult: func(t *testing.T, got Output) { require.Equal(t, Output{}, got) },
			assertErr: func(t *testing.T, err error) {
				require.EqualError(t, err, "Account name must be at most 255 characters")
			},
		},
		{
			name: "unset type fails validation",
			mock: func(*fakeAccountRepository, *fakeCurrencyRepository) Input {
				return Input{Name: "Cash", Type: 0, CurrencyCode: "RUB"}
			},
			assertResult: func(t *testing.T, got Output) { require.Equal(t, Output{}, got) },
			assertErr: func(t *testing.T, err error) {
				require.EqualError(t, err, "Please choose an account type")
			},
		},
		{
			name: "unknown currency is rejected",
			mock: func(_ *fakeAccountRepository, currencies *fakeCurrencyRepository) Input {
				currencies.existsFn = func(context.Context, string) (bool, error) { return false, nil }

				return Input{Name: "Cash", Type: uint8(entity.AccountTypeCash), CurrencyCode: "XYZ"}
			},
			assertResult: func(t *testing.T, got Output) { require.Equal(t, Output{}, got) },
			assertErr: func(t *testing.T, err error) {
				require.EqualError(t, err, "Currency not found")
			},
		},
		{
			name: "currency check storage failure is reported",
			mock: func(_ *fakeAccountRepository, currencies *fakeCurrencyRepository) Input {
				currencies.existsFn = func(context.Context, string) (bool, error) { return false, errStub }

				return Input{Name: "Cash", Type: uint8(entity.AccountTypeCash), CurrencyCode: "RUB"}
			},
			assertResult: func(t *testing.T, got Output) { require.Equal(t, Output{}, got) },
			assertErr: func(t *testing.T, err error) {
				require.EqualError(t, err, "Failed to verify currency")
			},
		},
		{
			name: "account creation failure is reported",
			mock: func(accounts *fakeAccountRepository, currencies *fakeCurrencyRepository) Input {
				currencies.existsFn = func(context.Context, string) (bool, error) { return true, nil }
				accounts.createFn = func(context.Context, port.AccountCreateRequest) (entity.Account, error) {
					return entity.Account{}, errStub
				}

				return Input{Name: "Cash", Type: uint8(entity.AccountTypeCash), CurrencyCode: "RUB"}
			},
			assertResult: func(t *testing.T, got Output) { require.Equal(t, Output{}, got) },
			assertErr: func(t *testing.T, err error) {
				require.EqualError(t, err, "Failed to save account")
			},
		},
		{
			name: "trims the name and creates the account with its opening balance",
			mock: func(accounts *fakeAccountRepository, currencies *fakeCurrencyRepository) Input {
				currencies.existsFn = func(_ context.Context, code string) (bool, error) {
					require.Equal(t, "RUB", code)
					return true, nil
				}
				accounts.createFn = func(_ context.Context, req port.AccountCreateRequest) (entity.Account, error) {
					require.Equal(t, "Cash", req.Name)
					require.Equal(t, int64(500), req.Balance)

					return entity.Account{ID: 1, Name: req.Name, Type: req.Type, CurrencyCode: req.CurrencyCode, Balance: req.Balance}, nil
				}

				return Input{Name: "  Cash  ", Type: uint8(entity.AccountTypeCash), CurrencyCode: "RUB", Balance: 500}
			},
			assertResult: func(t *testing.T, got Output) {
				require.Equal(t, "Cash", got.Account.Name)
				require.Equal(t, int64(500), got.Account.Balance)
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "zero opening balance is allowed",
			mock: func(accounts *fakeAccountRepository, currencies *fakeCurrencyRepository) Input {
				currencies.existsFn = func(context.Context, string) (bool, error) { return true, nil }
				accounts.createFn = func(_ context.Context, req port.AccountCreateRequest) (entity.Account, error) {
					require.Equal(t, int64(0), req.Balance)

					return entity.Account{ID: 1, Name: req.Name, Type: req.Type, CurrencyCode: req.CurrencyCode, Balance: req.Balance}, nil
				}

				return Input{Name: "Cash", Type: uint8(entity.AccountTypeCash), CurrencyCode: "RUB", Balance: 0}
			},
			assertResult: func(t *testing.T, got Output) {
				require.Equal(t, int64(0), got.Account.Balance)
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc, accounts, currencies := newUseCase()
			input := tt.mock(accounts, currencies)

			got, err := uc.Execute(context.Background(), input)
			tt.assertErr(t, err)
			tt.assertResult(t, got)
		})
	}
}
