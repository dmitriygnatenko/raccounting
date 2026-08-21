package create

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"raccounting/internal/domain/entity"
	domainerror "raccounting/internal/domain/error"
	"raccounting/internal/port"
)

// fakeAccountRepository is a hand-rolled test double for port.AccountRepository — only FindByID is
// exercised here.
type fakeAccountRepository struct {
	findByIDFn func(ctx context.Context, id uint64) (entity.Account, error)
}

func (f *fakeAccountRepository) List(context.Context) ([]entity.Account, error) {
	panic("not stubbed")
}

func (f *fakeAccountRepository) FindByID(ctx context.Context, id uint64) (entity.Account, error) {
	return f.findByIDFn(ctx, id)
}

func (f *fakeAccountRepository) Create(context.Context, port.AccountCreateRequest) (entity.Account, error) {
	panic("not stubbed")
}

func (f *fakeAccountRepository) Update(context.Context, port.AccountUpdateRequest) (entity.Account, error) {
	panic("not stubbed")
}

func (f *fakeAccountRepository) Delete(context.Context, uint64) error {
	panic("not stubbed")
}

// fakeTransactionRepository is a hand-rolled test double for port.TransactionRepository — only
// CreateTransfer is exercised here.
type fakeTransactionRepository struct {
	createFn func(ctx context.Context, req port.TransferCreateRequest) (port.TransferResult, error)
}

func (f *fakeTransactionRepository) List(context.Context) ([]entity.Transaction, error) {
	panic("not stubbed")
}

func (f *fakeTransactionRepository) ListFiltered(
	context.Context, port.TransactionListFilter,
) (port.TransactionListResult, error) {
	panic("not stubbed")
}

func (f *fakeTransactionRepository) Usage(context.Context) (port.TransactionUsage, error) {
	panic("not stubbed")
}

func (f *fakeTransactionRepository) FindByID(context.Context, uint64) (entity.Transaction, error) {
	panic("not stubbed")
}

func (f *fakeTransactionRepository) Create(
	context.Context, port.TransactionCreateRequest,
) (entity.Transaction, error) {
	panic("not stubbed")
}

func (f *fakeTransactionRepository) Update(
	context.Context, port.TransactionUpdateRequest,
) (entity.Transaction, error) {
	panic("not stubbed")
}

func (f *fakeTransactionRepository) Delete(context.Context, uint64) error {
	panic("not stubbed")
}

func (f *fakeTransactionRepository) CreateTransfer(
	ctx context.Context, req port.TransferCreateRequest,
) (port.TransferResult, error) {
	return f.createFn(ctx, req)
}

func (f *fakeTransactionRepository) DeleteTransfer(context.Context, uint64) (bool, error) {
	panic("not stubbed")
}

var errStub = errors.New("stub failure")

func TestUseCase_Execute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		mock         func(transfers *fakeTransactionRepository, accounts *fakeAccountRepository) Input
		assertResult func(t *testing.T, got Output)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "zero amount fails validation before any lookup",
			mock: func(*fakeTransactionRepository, *fakeAccountRepository) Input {
				return Input{FromAccountID: 1, ToAccountID: 2, Amount: 0, Date: "2024-01-15"}
			},
			assertResult: func(t *testing.T, got Output) { require.Equal(t, Output{}, got) },
			assertErr: func(t *testing.T, err error) {
				require.EqualError(t, err, "Transfer amount must be greater than zero")
			},
		},
		{
			name: "same source and destination account is rejected",
			mock: func(*fakeTransactionRepository, *fakeAccountRepository) Input {
				return Input{FromAccountID: 1, ToAccountID: 1, Amount: 100, Date: "2024-01-15"}
			},
			assertResult: func(t *testing.T, got Output) { require.Equal(t, Output{}, got) },
			assertErr: func(t *testing.T, err error) {
				require.EqualError(t, err, "Choose two different accounts")
			},
		},
		{
			name: "unknown source account is rejected",
			mock: func(_ *fakeTransactionRepository, accounts *fakeAccountRepository) Input {
				accounts.findByIDFn = func(_ context.Context, id uint64) (entity.Account, error) {
					require.Equal(t, uint64(1), id)
					return entity.Account{}, &domainerror.NotFoundError{Message: "Account not found"}
				}

				return Input{FromAccountID: 1, ToAccountID: 2, Amount: 100, Date: "2024-01-15"}
			},
			assertResult: func(t *testing.T, got Output) { require.Equal(t, Output{}, got) },
			assertErr: func(t *testing.T, err error) {
				require.EqualError(t, err, "Source account not found")
			},
		},
		{
			name: "unknown destination account is rejected",
			mock: func(_ *fakeTransactionRepository, accounts *fakeAccountRepository) Input {
				accounts.findByIDFn = func(_ context.Context, id uint64) (entity.Account, error) {
					if id == 1 {
						return entity.Account{ID: 1, Name: "Cash", CurrencyCode: "RUB"}, nil
					}

					return entity.Account{}, &domainerror.NotFoundError{Message: "Account not found"}
				}

				return Input{FromAccountID: 1, ToAccountID: 2, Amount: 100, Date: "2024-01-15"}
			},
			assertResult: func(t *testing.T, got Output) { require.Equal(t, Output{}, got) },
			assertErr: func(t *testing.T, err error) {
				require.EqualError(t, err, "Destination account not found")
			},
		},
		{
			name: "creates a same-currency transfer, defaulting ToAmount and Rate",
			mock: func(transfers *fakeTransactionRepository, accounts *fakeAccountRepository) Input {
				accounts.findByIDFn = func(_ context.Context, id uint64) (entity.Account, error) {
					if id == 1 {
						return entity.Account{ID: 1, Name: "Card", CurrencyCode: "RUB"}, nil
					}

					return entity.Account{ID: 2, Name: "Savings", CurrencyCode: "RUB"}, nil
				}
				transfers.createFn = func(_ context.Context, req port.TransferCreateRequest) (port.TransferResult, error) {
					require.Equal(t, uint64(1), req.FromAccountID)
					require.Equal(t, "RUB", req.FromCurrencyCode)
					require.Equal(t, uint64(2), req.ToAccountID)
					require.Equal(t, "RUB", req.ToCurrencyCode)
					require.Equal(t, int64(100), req.Amount)
					require.Equal(t, int64(100), req.CreditAmount)
					require.Equal(t, 1.0, req.Rate)

					return port.TransferResult{
						LegFrom: entity.Transaction{ID: 10, AccountID: 1, Amount: -10000},
						LegTo:   entity.Transaction{ID: 11, AccountID: 2, Amount: 10000},
					}, nil
				}

				return Input{FromAccountID: 1, ToAccountID: 2, Amount: 100, Date: "2024-01-15"}
			},
			assertResult: func(t *testing.T, got Output) {
				require.Equal(t, uint64(10), got.LegFrom.ID)
				require.Equal(t, int64(-10000), got.LegFrom.Amount)
				require.Equal(t, uint64(11), got.LegTo.ID)
				require.Equal(t, int64(10000), got.LegTo.Amount)
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "creates a cross-currency transfer honoring ToAmount and Rate",
			mock: func(transfers *fakeTransactionRepository, accounts *fakeAccountRepository) Input {
				accounts.findByIDFn = func(_ context.Context, id uint64) (entity.Account, error) {
					if id == 1 {
						return entity.Account{ID: 1, Name: "RUB card", CurrencyCode: "RUB"}, nil
					}

					return entity.Account{ID: 2, Name: "USD account", CurrencyCode: "USD"}, nil
				}

				toAmount := int64(1100)
				rate := 90.0

				transfers.createFn = func(_ context.Context, req port.TransferCreateRequest) (port.TransferResult, error) {
					require.Equal(t, int64(1000), req.Amount)
					require.Equal(t, int64(1100), req.CreditAmount)
					require.Equal(t, 90.0, req.Rate)

					return port.TransferResult{}, nil
				}

				return Input{
					FromAccountID: 1, ToAccountID: 2, Amount: 1000, ToAmount: &toAmount,
					Rate: &rate, Date: "2024-01-15",
				}
			},
			assertResult: func(t *testing.T, got Output) { require.Equal(t, Output{}, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "save failure is reported",
			mock: func(transfers *fakeTransactionRepository, accounts *fakeAccountRepository) Input {
				accounts.findByIDFn = func(context.Context, uint64) (entity.Account, error) {
					return entity.Account{ID: 1, Name: "Cash", CurrencyCode: "RUB"}, nil
				}
				transfers.createFn = func(context.Context, port.TransferCreateRequest) (port.TransferResult, error) {
					return port.TransferResult{}, errStub
				}

				return Input{FromAccountID: 1, ToAccountID: 2, Amount: 100, Date: "2024-01-15"}
			},
			assertResult: func(t *testing.T, got Output) { require.Equal(t, Output{}, got) },
			assertErr: func(t *testing.T, err error) {
				require.EqualError(t, err, "Failed to save transfer")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			transfers := &fakeTransactionRepository{}
			accounts := &fakeAccountRepository{}

			uc := New(transfers, accounts)
			input := tt.mock(transfers, accounts)

			got, err := uc.Execute(context.Background(), input)
			tt.assertErr(t, err)
			tt.assertResult(t, got)
		})
	}
}
