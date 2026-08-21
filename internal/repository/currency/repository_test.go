package currency

import (
	"context"
	"errors"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"raccounting/internal/domain/entity"
	domainerror "raccounting/internal/domain/error"
	"raccounting/internal/port"
	"raccounting/internal/repository/currency/mocks"
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

// fakeCode, fakeSymbol, fakeName and fakeRate are the field-shaped random values the tests below
// bind into mock expectations and returned rows, so a test failure is never masked by two cases
// accidentally sharing a fixture value.
func fakeCode() string   { return gofakeit.CurrencyShort() }
func fakeSymbol() string { return gofakeit.Currency().Short }
func fakeName() string   { return gofakeit.Word() }
func fakeRate() float64  { return gofakeit.Price(0.01, 1000) }

func fakeCurrencyModel() model.Currency {
	return model.Currency{
		Code:    fakeCode(),
		Symbol:  fakeSymbol(),
		Name:    fakeName(),
		Rate:    fakeRate(),
		Default: gofakeit.Bool(),
		Status:  uint8(entity.CurrencyStatusActive),
	}
}

// errStub is the sentinel a case uses when it only cares that an error travels through untouched.
var errStub = errors.New(gofakeit.Sentence())

// TestRepository_List covers the row -> entity conversion and plain error propagation.
func TestRepository_List(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		mock         func(m *mocks.MockStorage) []entity.Currency
		assertResult func(t *testing.T, want, got []entity.Currency)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "converts every row",
			mock: func(m *mocks.MockStorage) []entity.Currency {
				rows := []model.Currency{fakeCurrencyModel(), fakeCurrencyModel()}
				m.EXPECT().ListCurrencies(context.Background()).Return(rows, nil)

				want := make([]entity.Currency, len(rows))
				for i, row := range rows {
					want[i] = row.ToEntity()
				}

				return want
			},
			assertResult: func(t *testing.T, want, got []entity.Currency) { require.Equal(t, want, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a storage error is propagated",
			mock: func(m *mocks.MockStorage) []entity.Currency {
				m.EXPECT().ListCurrencies(context.Background()).Return(nil, errStub)

				return nil
			},
			assertResult: func(t *testing.T, want, got []entity.Currency) { require.Nil(t, got) },
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

// TestRepository_Exists covers the plain delegation to storage.
func TestRepository_Exists(t *testing.T) {
	t.Parallel()

	code := fakeCode()

	tests := []struct {
		name         string
		mock         func(m *mocks.MockStorage)
		assertResult func(t *testing.T, got bool)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "delegates to storage",
			mock: func(m *mocks.MockStorage) {
				m.EXPECT().ExistsCurrency(context.Background(), code).Return(true, nil)
			},
			assertResult: func(t *testing.T, got bool) { require.True(t, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a storage error is propagated",
			mock: func(m *mocks.MockStorage) {
				m.EXPECT().ExistsCurrency(context.Background(), code).Return(false, errStub)
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

			got, err := r.Exists(context.Background(), code)
			tt.assertErr(t, err)
			tt.assertResult(t, got)
		})
	}
}

// TestRepository_Create covers the insert and the code-collision -> ConflictError translation.
func TestRepository_Create(t *testing.T) {
	t.Parallel()

	req := port.CurrencyCreateRequest{
		Code:    fakeCode(),
		Symbol:  fakeSymbol(),
		Name:    fakeName(),
		Rate:    fakeRate(),
		Default: gofakeit.Bool(),
	}
	storageReq := model.CurrencyCreateRequest{
		Code:    req.Code,
		Symbol:  req.Symbol,
		Name:    req.Name,
		Rate:    req.Rate,
		Default: req.Default,
	}

	tests := []struct {
		name         string
		mock         func(m *mocks.MockStorage) entity.Currency
		assertResult func(t *testing.T, want, got entity.Currency)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "stores the currency and returns it",
			mock: func(m *mocks.MockStorage) entity.Currency {
				m.EXPECT().CreateCurrency(context.Background(), storageReq).Return(nil)

				return entity.Currency{
					Code:    req.Code,
					Symbol:  req.Symbol,
					Name:    req.Name,
					Rate:    req.Rate,
					Default: req.Default,
					Status:  entity.CurrencyStatusActive,
				}
			},
			assertResult: func(t *testing.T, want, got entity.Currency) { require.Equal(t, want, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a taken code becomes a message-less ConflictError",
			mock: func(m *mocks.MockStorage) entity.Currency {
				m.EXPECT().
					CreateCurrency(context.Background(), storageReq).
					Return(storageError.UniqueViolationError)

				return entity.Currency{}
			},
			assertResult: func(t *testing.T, want, got entity.Currency) {},
			assertErr: func(t *testing.T, err error) {
				var conflict *domainerror.ConflictError
				require.ErrorAs(t, err, &conflict)
				require.Empty(t, conflict.Message)
			},
		},
		{
			name: "any other storage error is propagated",
			mock: func(m *mocks.MockStorage) entity.Currency {
				m.EXPECT().CreateCurrency(context.Background(), storageReq).Return(errStub)

				return entity.Currency{}
			},
			assertResult: func(t *testing.T, want, got entity.Currency) {},
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

	req := port.CurrencyUpdateRequest{
		Code:     fakeCode(),
		Symbol:   fakeSymbol(),
		Name:     fakeName(),
		Rate:     fakeRate(),
		Default:  gofakeit.Bool(),
		Archived: gofakeit.Bool(),
	}
	storageReq := model.CurrencyUpdateRequest{
		Code:     req.Code,
		Symbol:   req.Symbol,
		Name:     req.Name,
		Rate:     req.Rate,
		Default:  req.Default,
		Archived: req.Archived,
	}

	tests := []struct {
		name         string
		mock         func(m *mocks.MockStorage) entity.Currency
		assertResult func(t *testing.T, want, got entity.Currency)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "updates the currency",
			mock: func(m *mocks.MockStorage) entity.Currency {
				row := fakeCurrencyModel()
				row.Code = req.Code
				m.EXPECT().UpdateCurrency(context.Background(), storageReq).Return(row, true, nil)

				return row.ToEntity()
			},
			assertResult: func(t *testing.T, want, got entity.Currency) { require.Equal(t, want, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "an unknown code becomes a message-less NotFoundError",
			mock: func(m *mocks.MockStorage) entity.Currency {
				m.EXPECT().UpdateCurrency(context.Background(), storageReq).Return(model.Currency{}, false, nil)

				return entity.Currency{}
			},
			assertResult: func(t *testing.T, want, got entity.Currency) {},
			assertErr: func(t *testing.T, err error) {
				var notFound *domainerror.NotFoundError
				require.ErrorAs(t, err, &notFound)
				require.Empty(t, notFound.Message)
			},
		},
		{
			name: "a storage error is propagated",
			mock: func(m *mocks.MockStorage) entity.Currency {
				m.EXPECT().UpdateCurrency(context.Background(), storageReq).Return(model.Currency{}, false, errStub)

				return entity.Currency{}
			},
			assertResult: func(t *testing.T, want, got entity.Currency) {},
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

	code := fakeCode()

	tests := []struct {
		name      string
		mock      func(m *mocks.MockStorage)
		assertErr func(t *testing.T, err error)
	}{
		{
			name: "deletes the currency",
			mock: func(m *mocks.MockStorage) {
				m.EXPECT().DeleteCurrency(context.Background(), code).Return(true, nil)
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a foreign key violation becomes a message-less ConflictError",
			mock: func(m *mocks.MockStorage) {
				m.EXPECT().DeleteCurrency(context.Background(), code).
					Return(false, storageError.ForeignKeyViolationError)
			},
			assertErr: func(t *testing.T, err error) {
				var conflict *domainerror.ConflictError
				require.ErrorAs(t, err, &conflict)
				require.Empty(t, conflict.Message)
			},
		},
		{
			name: "an unknown code becomes a message-less NotFoundError",
			mock: func(m *mocks.MockStorage) {
				m.EXPECT().DeleteCurrency(context.Background(), code).Return(false, nil)
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
				m.EXPECT().DeleteCurrency(context.Background(), code).Return(false, errStub)
			},
			assertErr: func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r, m := newRepo(t)
			tt.mock(m)

			err := r.Delete(context.Background(), code)
			tt.assertErr(t, err)
		})
	}
}
