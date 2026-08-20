package budget

import (
	"context"
	"errors"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"raccounting/internal/domain/entity"
	"raccounting/internal/port"
	"raccounting/internal/repository/budget/mocks"
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

// fakeID, fakeMonthKey and fakeAmount are the field-shaped random values the tests below bind into
// mock expectations and returned rows, so a test failure is never masked by two cases accidentally
// sharing a fixture value.
func fakeID() uint64       { return uint64(gofakeit.Number(1, 1_000_000)) }
func fakeMonthKey() string { return gofakeit.Date().Format("2006-01") }
func fakeAmount() int64    { return int64(gofakeit.Number(0, 1_000_000)) }

func fakeBudgetModel() model.Budget {
	return model.Budget{
		ID:         fakeID(),
		MonthKey:   fakeMonthKey(),
		CategoryID: fakeID(),
		Amount:     fakeAmount(),
	}
}

// errStub is the sentinel a case uses when it only cares that an error travels through untouched.
var errStub = errors.New(gofakeit.Sentence())

// TestRepository_List covers the row -> entity conversion and plain error propagation.
func TestRepository_List(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		mock         func(m *mocks.MockStorage) []entity.Budget
		assertResult func(t *testing.T, want, got []entity.Budget)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "converts every row",
			mock: func(m *mocks.MockStorage) []entity.Budget {
				rows := []model.Budget{fakeBudgetModel(), fakeBudgetModel()}
				m.EXPECT().ListCategoryBudgets(context.Background()).Return(rows, nil)

				want := make([]entity.Budget, len(rows))
				for i, row := range rows {
					want[i] = row.ToEntity()
				}

				return want
			},
			assertResult: func(t *testing.T, want, got []entity.Budget) { require.Equal(t, want, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a storage error is propagated",
			mock: func(m *mocks.MockStorage) []entity.Budget {
				m.EXPECT().ListCategoryBudgets(context.Background()).Return(nil, errStub)

				return nil
			},
			assertResult: func(t *testing.T, want, got []entity.Budget) { require.Nil(t, got) },
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

// TestRepository_Set covers the plain delegation to storage.
func TestRepository_Set(t *testing.T) {
	t.Parallel()

	req := port.BudgetSetRequest{
		CategoryID: fakeID(),
		MonthKey:   fakeMonthKey(),
		Amount:     fakeAmount(),
	}

	tests := []struct {
		name      string
		mock      func(m *mocks.MockStorage)
		assertErr func(t *testing.T, err error)
	}{
		{
			name: "delegates to storage",
			mock: func(m *mocks.MockStorage) {
				m.EXPECT().SetCategoryBudget(context.Background(), req).Return(nil)
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a storage error is propagated",
			mock: func(m *mocks.MockStorage) {
				m.EXPECT().SetCategoryBudget(context.Background(), req).Return(errStub)
			},
			assertErr: func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r, m := newRepo(t)
			tt.mock(m)

			err := r.Set(context.Background(), req)
			tt.assertErr(t, err)
		})
	}
}
