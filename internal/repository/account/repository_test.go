package account

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"raccounting/internal/domain/entity"
	domainerror "raccounting/internal/domain/error"
	"raccounting/internal/port"
	"raccounting/internal/repository/account/mocks"
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

// fakeID, fakeName, fakeCurrencyCode and fakeBalance are the field-shaped random values the tests
// below bind into mock expectations and returned rows, so a test failure is never masked by two
// cases accidentally sharing a fixture value.
func fakeID() uint64                      { return uint64(gofakeit.Number(1, 1_000_000)) }
func fakeName() string                    { return gofakeit.Word() }
func fakeCurrencyCode() string            { return gofakeit.CurrencyShort() }
func fakeBalance() int64                  { return int64(gofakeit.Number(0, 1_000_000)) }
func fakeAccountType() entity.AccountType { return entity.AccountTypeCash }

func fakeAccountModel() model.Account {
	return model.Account{
		ID:           fakeID(),
		Name:         fakeName(),
		Type:         uint8(fakeAccountType()),
		CurrencyCode: fakeCurrencyCode(),
		Balance:      fakeBalance(),
		Status:       uint8(entity.AccountStatusActive),
	}
}

// errStub is the sentinel a case uses when it only cares that an error travels through untouched.
var errStub = errors.New(gofakeit.Sentence())

// TestRepository_List covers the row -> entity conversion and plain error propagation.
func TestRepository_List(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		mock         func(m *mocks.MockStorage) []entity.Account
		assertResult func(t *testing.T, want, got []entity.Account)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "converts every row",
			mock: func(m *mocks.MockStorage) []entity.Account {
				rows := []model.Account{fakeAccountModel(), fakeAccountModel()}
				m.EXPECT().ListAccounts(context.Background()).Return(rows, nil)

				want := make([]entity.Account, len(rows))
				for i, row := range rows {
					want[i] = row.ToEntity()
				}

				return want
			},
			assertResult: func(t *testing.T, want, got []entity.Account) { require.Equal(t, want, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a storage error is propagated",
			mock: func(m *mocks.MockStorage) []entity.Account {
				m.EXPECT().ListAccounts(context.Background()).Return(nil, errStub)

				return nil
			},
			assertResult: func(t *testing.T, want, got []entity.Account) { require.Nil(t, got) },
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
		mock         func(m *mocks.MockStorage) entity.Account
		assertResult func(t *testing.T, want, got entity.Account)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "finds the account",
			mock: func(m *mocks.MockStorage) entity.Account {
				row := fakeAccountModel()
				row.ID = id
				m.EXPECT().FindAccountByID(context.Background(), id).Return(row, nil)

				return row.ToEntity()
			},
			assertResult: func(t *testing.T, want, got entity.Account) { require.Equal(t, want, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "an unknown id becomes a message-less NotFoundError",
			mock: func(m *mocks.MockStorage) entity.Account {
				m.EXPECT().FindAccountByID(context.Background(), id).Return(model.Account{}, sql.ErrNoRows)

				return entity.Account{}
			},
			assertResult: func(t *testing.T, want, got entity.Account) {},
			assertErr: func(t *testing.T, err error) {
				var notFound *domainerror.NotFoundError
				require.ErrorAs(t, err, &notFound)
				require.Empty(t, notFound.Message)
			},
		},
		{
			name: "a storage error is propagated",
			mock: func(m *mocks.MockStorage) entity.Account {
				m.EXPECT().FindAccountByID(context.Background(), id).Return(model.Account{}, errStub)

				return entity.Account{}
			},
			assertResult: func(t *testing.T, want, got entity.Account) {},
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

// TestRepository_Create covers the insert and the negative-balance -> ConflictError translation.
func TestRepository_Create(t *testing.T) {
	t.Parallel()

	req := port.AccountCreateRequest{
		Name:         fakeName(),
		Type:         fakeAccountType(),
		CurrencyCode: fakeCurrencyCode(),
		Balance:      fakeBalance(),
	}

	tests := []struct {
		name         string
		mock         func(m *mocks.MockStorage) entity.Account
		assertResult func(t *testing.T, want, got entity.Account)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "stores the account and returns it",
			mock: func(m *mocks.MockStorage) entity.Account {
				id := fakeID()
				m.EXPECT().CreateAccount(context.Background(), req).Return(id, nil)

				return entity.Account{
					ID:           id,
					Name:         req.Name,
					Type:         req.Type,
					CurrencyCode: req.CurrencyCode,
					Balance:      req.Balance,
					Status:       entity.AccountStatusActive,
				}
			},
			assertResult: func(t *testing.T, want, got entity.Account) { require.Equal(t, want, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a negative opening balance becomes a message-less ConflictError",
			mock: func(m *mocks.MockStorage) entity.Account {
				m.EXPECT().
					CreateAccount(context.Background(), req).
					Return(uint64(0), storageError.InsufficientBalanceError)

				return entity.Account{}
			},
			assertResult: func(t *testing.T, want, got entity.Account) {},
			assertErr: func(t *testing.T, err error) {
				var conflict *domainerror.ConflictError
				require.ErrorAs(t, err, &conflict)
				require.Empty(t, conflict.Message)
			},
		},
		{
			name: "any other storage error is propagated",
			mock: func(m *mocks.MockStorage) entity.Account {
				m.EXPECT().CreateAccount(context.Background(), req).Return(uint64(0), errStub)

				return entity.Account{}
			},
			assertResult: func(t *testing.T, want, got entity.Account) {},
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

// TestRepository_Update covers the rewrite and the found -> NotFoundError translation.
func TestRepository_Update(t *testing.T) {
	t.Parallel()

	req := port.AccountUpdateRequest{
		ID:           fakeID(),
		Name:         fakeName(),
		Type:         fakeAccountType(),
		CurrencyCode: fakeCurrencyCode(),
		Archived:     gofakeit.Bool(),
	}

	tests := []struct {
		name         string
		mock         func(m *mocks.MockStorage) entity.Account
		assertResult func(t *testing.T, want, got entity.Account)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "updates the account",
			mock: func(m *mocks.MockStorage) entity.Account {
				row := fakeAccountModel()
				row.ID = req.ID
				m.EXPECT().UpdateAccount(context.Background(), req).Return(row, true, nil)

				return row.ToEntity()
			},
			assertResult: func(t *testing.T, want, got entity.Account) { require.Equal(t, want, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "an unknown id becomes a message-less NotFoundError",
			mock: func(m *mocks.MockStorage) entity.Account {
				m.EXPECT().UpdateAccount(context.Background(), req).Return(model.Account{}, false, nil)

				return entity.Account{}
			},
			assertResult: func(t *testing.T, want, got entity.Account) {},
			assertErr: func(t *testing.T, err error) {
				var notFound *domainerror.NotFoundError
				require.ErrorAs(t, err, &notFound)
				require.Empty(t, notFound.Message)
			},
		},
		{
			name: "a storage error is propagated",
			mock: func(m *mocks.MockStorage) entity.Account {
				m.EXPECT().UpdateAccount(context.Background(), req).Return(model.Account{}, false, errStub)

				return entity.Account{}
			},
			assertResult: func(t *testing.T, want, got entity.Account) {},
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

// TestRepository_Delete covers the removal and both error translations: FK violation -> ConflictError,
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
			name: "deletes the account",
			mock: func(m *mocks.MockStorage) {
				m.EXPECT().DeleteAccount(context.Background(), id).Return(true, nil)
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a foreign key violation becomes a message-less ConflictError",
			mock: func(m *mocks.MockStorage) {
				m.EXPECT().DeleteAccount(context.Background(), id).
					Return(false, storageError.ForeignKeyViolationError)
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
				m.EXPECT().DeleteAccount(context.Background(), id).Return(false, nil)
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
				m.EXPECT().DeleteAccount(context.Background(), id).Return(false, errStub)
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
