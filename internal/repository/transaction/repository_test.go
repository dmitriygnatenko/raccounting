package transaction

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"raccounting/internal/domain/entity"
	domainerror "raccounting/internal/domain/error"
	"raccounting/internal/port"
	"raccounting/internal/repository/transaction/mocks"
	storageError "raccounting/internal/storage/error"
	"raccounting/internal/storage/model"
)

// newRepo returns a Repository wired to a fresh MockStorage; any call a test doesn't stub via
// EXPECT() fails it, exactly like an unmet sqlmock expectation would in the adapter tests.
func newRepo(t *testing.T) (*Repository, *mocks.MockStorage) {
	t.Helper()

	mc := gomock.NewController(t)
	t.Cleanup(mc.Finish)

	m := mocks.NewMockStorage(mc)

	return New(m), m
}

// fakeID, fakeAmount, fakeCurrencyCode, fakeMemo and fakeOperationAt are the field-shaped random
// values the tests below bind into mock expectations and returned rows, so a test failure is never
// masked by two cases accidentally sharing a fixture value.
func fakeID() uint64             { return uint64(gofakeit.Number(1, 1_000_000)) }
func fakeAmount() int64          { return int64(gofakeit.Number(1, 1_000_000)) }
func fakeCurrencyCode() string   { return gofakeit.CurrencyShort() }
func fakeMemo() string           { return gofakeit.Sentence() }
func fakeOperationAt() time.Time { return gofakeit.Date().UTC() }

func fakeTransactionModel() model.Transaction {
	return model.Transaction{
		ID:           fakeID(),
		Type:         uint8(entity.TransactionTypeExpense),
		AccountID:    fakeID(),
		CurrencyCode: fakeCurrencyCode(),
		Amount:       fakeAmount(),
		Memo:         fakeMemo(),
		OperationAt:  fakeOperationAt(),
	}
}

// errStub is the sentinel a case uses when it only cares that an error travels through untouched.
var errStub = errors.New(gofakeit.Sentence())

// TestRepository_List covers the row -> entity conversion and plain error propagation.
func TestRepository_List(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		mock         func(m *mocks.MockStorage) []entity.Transaction
		assertResult func(t *testing.T, want, got []entity.Transaction)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "converts every row",
			mock: func(m *mocks.MockStorage) []entity.Transaction {
				rows := []model.Transaction{fakeTransactionModel(), fakeTransactionModel()}
				m.EXPECT().ListTransactions(context.Background()).Return(rows, nil)

				want := make([]entity.Transaction, len(rows))
				for i, row := range rows {
					want[i] = row.ToEntity()
				}

				return want
			},
			assertResult: func(t *testing.T, want, got []entity.Transaction) { require.Equal(t, want, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a storage error is propagated",
			mock: func(m *mocks.MockStorage) []entity.Transaction {
				m.EXPECT().ListTransactions(context.Background()).Return(nil, errStub)

				return nil
			},
			assertResult: func(t *testing.T, want, got []entity.Transaction) { require.Nil(t, got) },
			assertErr:    func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r, m := newRepo(t)
			want := tt.mock(m)

			got, err := r.List(context.Background())
			tt.assertErr(t, err)
			tt.assertResult(t, want, got)
		})
	}
}

// TestRepository_FindByID covers the lookup, including the id -> NotFoundError translation.
func TestRepository_FindByID(t *testing.T) {
	t.Parallel()

	id := fakeID()

	tests := []struct {
		name         string
		mock         func(m *mocks.MockStorage) entity.Transaction
		assertResult func(t *testing.T, want, got entity.Transaction)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "finds the transaction",
			mock: func(m *mocks.MockStorage) entity.Transaction {
				row := fakeTransactionModel()
				row.ID = id
				m.EXPECT().FindTransactionByID(context.Background(), id).Return(row, nil)

				return row.ToEntity()
			},
			assertResult: func(t *testing.T, want, got entity.Transaction) { require.Equal(t, want, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "an unknown id becomes a message-less NotFoundError",
			mock: func(m *mocks.MockStorage) entity.Transaction {
				m.EXPECT().FindTransactionByID(context.Background(), id).Return(model.Transaction{}, sql.ErrNoRows)

				return entity.Transaction{}
			},
			assertResult: func(t *testing.T, want, got entity.Transaction) {},
			assertErr: func(t *testing.T, err error) {
				var notFound *domainerror.NotFoundError
				require.ErrorAs(t, err, &notFound)
				require.Empty(t, notFound.Message)
			},
		},
		{
			name: "a storage error is propagated",
			mock: func(m *mocks.MockStorage) entity.Transaction {
				m.EXPECT().FindTransactionByID(context.Background(), id).Return(model.Transaction{}, errStub)

				return entity.Transaction{}
			},
			assertResult: func(t *testing.T, want, got entity.Transaction) {},
			assertErr:    func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r, m := newRepo(t)
			want := tt.mock(m)

			got, err := r.FindByID(context.Background(), id)
			tt.assertErr(t, err)
			tt.assertResult(t, want, got)
		})
	}
}

// TestRepository_Create covers the insert and the overdraw -> ConflictError translation.
func TestRepository_Create(t *testing.T) {
	t.Parallel()

	req := port.TransactionCreateRequest{
		AccountID:    fakeID(),
		Type:         entity.TransactionTypeExpense,
		CurrencyCode: fakeCurrencyCode(),
		Amount:       fakeAmount(),
		Memo:         fakeMemo(),
		OperationAt:  fakeOperationAt(),
	}

	tests := []struct {
		name         string
		mock         func(m *mocks.MockStorage) entity.Transaction
		assertResult func(t *testing.T, want, got entity.Transaction)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "stores the transaction and returns it",
			mock: func(m *mocks.MockStorage) entity.Transaction {
				row := fakeTransactionModel()
				m.EXPECT().CreateTransactionWithBalance(context.Background(), req).Return(row, nil)

				return row.ToEntity()
			},
			assertResult: func(t *testing.T, want, got entity.Transaction) { require.Equal(t, want, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "an overdraw becomes a message-less ConflictError",
			mock: func(m *mocks.MockStorage) entity.Transaction {
				m.EXPECT().
					CreateTransactionWithBalance(context.Background(), req).
					Return(model.Transaction{}, storageError.InsufficientBalanceError)

				return entity.Transaction{}
			},
			assertResult: func(t *testing.T, want, got entity.Transaction) {},
			assertErr: func(t *testing.T, err error) {
				var conflict *domainerror.ConflictError
				require.ErrorAs(t, err, &conflict)
				require.Empty(t, conflict.Message)
			},
		},
		{
			name: "any other storage error is propagated",
			mock: func(m *mocks.MockStorage) entity.Transaction {
				m.EXPECT().CreateTransactionWithBalance(context.Background(), req).Return(model.Transaction{}, errStub)

				return entity.Transaction{}
			},
			assertResult: func(t *testing.T, want, got entity.Transaction) {},
			assertErr:    func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r, m := newRepo(t)
			want := tt.mock(m)

			got, err := r.Create(context.Background(), req)
			tt.assertErr(t, err)
			tt.assertResult(t, want, got)
		})
	}
}

// TestRepository_Update covers the rewrite and both error translations: overdraw -> ConflictError,
// found=false -> NotFoundError.
func TestRepository_Update(t *testing.T) {
	t.Parallel()

	req := port.TransactionUpdateRequest{
		ID:           fakeID(),
		AccountID:    fakeID(),
		Type:         entity.TransactionTypeExpense,
		CurrencyCode: fakeCurrencyCode(),
		Amount:       fakeAmount(),
		Memo:         fakeMemo(),
		OperationAt:  fakeOperationAt(),
	}

	tests := []struct {
		name         string
		mock         func(m *mocks.MockStorage) entity.Transaction
		assertResult func(t *testing.T, want, got entity.Transaction)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "updates the transaction",
			mock: func(m *mocks.MockStorage) entity.Transaction {
				row := fakeTransactionModel()
				row.ID = req.ID
				m.EXPECT().UpdateTransactionWithBalance(context.Background(), req).Return(row, true, nil)

				return row.ToEntity()
			},
			assertResult: func(t *testing.T, want, got entity.Transaction) { require.Equal(t, want, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "an overdraw becomes a message-less ConflictError",
			mock: func(m *mocks.MockStorage) entity.Transaction {
				m.EXPECT().
					UpdateTransactionWithBalance(context.Background(), req).
					Return(model.Transaction{}, false, storageError.InsufficientBalanceError)

				return entity.Transaction{}
			},
			assertResult: func(t *testing.T, want, got entity.Transaction) {},
			assertErr: func(t *testing.T, err error) {
				var conflict *domainerror.ConflictError
				require.ErrorAs(t, err, &conflict)
				require.Empty(t, conflict.Message)
			},
		},
		{
			name: "an unknown id becomes a message-less NotFoundError",
			mock: func(m *mocks.MockStorage) entity.Transaction {
				m.EXPECT().UpdateTransactionWithBalance(context.Background(), req).Return(model.Transaction{}, false, nil)

				return entity.Transaction{}
			},
			assertResult: func(t *testing.T, want, got entity.Transaction) {},
			assertErr: func(t *testing.T, err error) {
				var notFound *domainerror.NotFoundError
				require.ErrorAs(t, err, &notFound)
				require.Empty(t, notFound.Message)
			},
		},
		{
			name: "a storage error is propagated",
			mock: func(m *mocks.MockStorage) entity.Transaction {
				m.EXPECT().UpdateTransactionWithBalance(context.Background(), req).Return(model.Transaction{}, false, errStub)

				return entity.Transaction{}
			},
			assertResult: func(t *testing.T, want, got entity.Transaction) {},
			assertErr:    func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r, m := newRepo(t)
			want := tt.mock(m)

			got, err := r.Update(context.Background(), req)
			tt.assertErr(t, err)
			tt.assertResult(t, want, got)
		})
	}
}

// TestRepository_Delete covers the removal and both error translations: overdraw -> ConflictError,
// found=false -> NotFoundError.
func TestRepository_Delete(t *testing.T) {
	t.Parallel()

	id := fakeID()

	tests := []struct {
		name      string
		mock      func(m *mocks.MockStorage)
		assertErr func(t *testing.T, err error)
	}{
		{
			name: "deletes the transaction",
			mock: func(m *mocks.MockStorage) {
				m.EXPECT().DeleteTransactionWithBalance(context.Background(), id).Return(true, nil)
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "an overdraw becomes a message-less ConflictError",
			mock: func(m *mocks.MockStorage) {
				m.EXPECT().DeleteTransactionWithBalance(context.Background(), id).
					Return(false, storageError.InsufficientBalanceError)
			},
			assertErr: func(t *testing.T, err error) {
				var conflict *domainerror.ConflictError
				require.ErrorAs(t, err, &conflict)
				require.Empty(t, conflict.Message)
			},
		},
		{
			name: "an unknown id becomes a message-less NotFoundError",
			mock: func(m *mocks.MockStorage) {
				m.EXPECT().DeleteTransactionWithBalance(context.Background(), id).Return(false, nil)
			},
			assertErr: func(t *testing.T, err error) {
				var notFound *domainerror.NotFoundError
				require.ErrorAs(t, err, &notFound)
				require.Empty(t, notFound.Message)
			},
		},
		{
			name: "any other storage error is propagated",
			mock: func(m *mocks.MockStorage) {
				m.EXPECT().DeleteTransactionWithBalance(context.Background(), id).Return(false, errStub)
			},
			assertErr: func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r, m := newRepo(t)
			tt.mock(m)

			err := r.Delete(context.Background(), id)
			tt.assertErr(t, err)
		})
	}
}

// TestRepository_CreateTransfer covers inserting both legs and the overdraw -> ConflictError
// translation.
func TestRepository_CreateTransfer(t *testing.T) {
	t.Parallel()

	req := port.TransferCreateRequest{
		FromAccountID:    fakeID(),
		FromCurrencyCode: fakeCurrencyCode(),
		ToAccountID:      fakeID(),
		ToCurrencyCode:   fakeCurrencyCode(),
		Amount:           fakeAmount(),
		CreditAmount:     fakeAmount(),
		Rate:             gofakeit.Price(0.01, 100),
		OperationAt:      fakeOperationAt(),
		Memo:             fakeMemo(),
	}

	tests := []struct {
		name         string
		mock         func(m *mocks.MockStorage) port.TransferResult
		assertResult func(t *testing.T, want, got port.TransferResult)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "stores both legs and returns them",
			mock: func(m *mocks.MockStorage) port.TransferResult {
				legFrom, legTo := fakeTransactionModel(), fakeTransactionModel()
				m.EXPECT().CreateTransferWithBalance(context.Background(), req).Return(legFrom, legTo, nil)

				return port.TransferResult{LegFrom: legFrom.ToEntity(), LegTo: legTo.ToEntity()}
			},
			assertResult: func(t *testing.T, want, got port.TransferResult) { require.Equal(t, want, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "an overdraw on either leg becomes a message-less ConflictError",
			mock: func(m *mocks.MockStorage) port.TransferResult {
				m.EXPECT().
					CreateTransferWithBalance(context.Background(), req).
					Return(model.Transaction{}, model.Transaction{}, storageError.InsufficientBalanceError)

				return port.TransferResult{}
			},
			assertResult: func(t *testing.T, want, got port.TransferResult) {},
			assertErr: func(t *testing.T, err error) {
				var conflict *domainerror.ConflictError
				require.ErrorAs(t, err, &conflict)
				require.Empty(t, conflict.Message)
			},
		},
		{
			name: "any other storage error is propagated",
			mock: func(m *mocks.MockStorage) port.TransferResult {
				m.EXPECT().
					CreateTransferWithBalance(context.Background(), req).
					Return(model.Transaction{}, model.Transaction{}, errStub)

				return port.TransferResult{}
			},
			assertResult: func(t *testing.T, want, got port.TransferResult) {},
			assertErr:    func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r, m := newRepo(t)
			want := tt.mock(m)

			got, err := r.CreateTransfer(context.Background(), req)
			tt.assertErr(t, err)
			tt.assertResult(t, want, got)
		})
	}
}

// TestRepository_DeleteTransfer covers removing both legs and the overdraw -> ConflictError
// translation.
func TestRepository_DeleteTransfer(t *testing.T) {
	t.Parallel()

	id := fakeID()

	tests := []struct {
		name         string
		mock         func(m *mocks.MockStorage)
		assertResult func(t *testing.T, got bool)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "deletes both legs",
			mock: func(m *mocks.MockStorage) {
				m.EXPECT().DeleteTransferWithBalance(context.Background(), id).Return(true, nil)
			},
			assertResult: func(t *testing.T, got bool) { require.True(t, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "an overdraw on either leg becomes a message-less ConflictError",
			mock: func(m *mocks.MockStorage) {
				m.EXPECT().DeleteTransferWithBalance(context.Background(), id).
					Return(false, storageError.InsufficientBalanceError)
			},
			assertResult: func(t *testing.T, got bool) { require.False(t, got) },
			assertErr: func(t *testing.T, err error) {
				var conflict *domainerror.ConflictError
				require.ErrorAs(t, err, &conflict)
				require.Empty(t, conflict.Message)
			},
		},
		{
			name: "found is passed through untouched for an unknown id",
			mock: func(m *mocks.MockStorage) {
				m.EXPECT().DeleteTransferWithBalance(context.Background(), id).Return(false, nil)
			},
			assertResult: func(t *testing.T, got bool) { require.False(t, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "any other storage error is propagated",
			mock: func(m *mocks.MockStorage) {
				m.EXPECT().DeleteTransferWithBalance(context.Background(), id).Return(false, errStub)
			},
			assertResult: func(t *testing.T, got bool) { require.False(t, got) },
			assertErr:    func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r, m := newRepo(t)
			tt.mock(m)

			got, err := r.DeleteTransfer(context.Background(), id)
			tt.assertErr(t, err)
			tt.assertResult(t, got)
		})
	}
}
