package sqlite

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"raccounting/internal/storage/model"
)

// fakeBudget returns a random model.Budget, as ListCategoryBudgets scans it (CreatedAt/UpdatedAt are
// left zero — its column list doesn't select them).
func fakeBudget() model.Budget {
	return model.Budget{
		ID:         fakeID(),
		CategoryID: fakeID(),
		MonthKey:   fakeMonthKey(),
		Amount:     fakeAmount(),
	}
}

// TestListCategoryBudgets covers the full listing: every row comes back scanned into model.Budget.
func TestListCategoryBudgets(t *testing.T) {
	t.Parallel()

	query := `SELECT id, category_id, month_key, amount FROM budgets ORDER BY month_key`

	b1, b2 := fakeBudget(), fakeBudget()

	tests := []struct {
		name string
		rows []model.Budget
	}{
		{name: "an empty result yields no rows"},
		{name: "a single budget", rows: []model.Budget{b1}},
		{name: "several budgets", rows: []model.Budget{b1, b2}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)

			mockRows := sqlmock.NewRows([]string{"id", "category_id", "month_key", "amount"})
			for _, b := range tt.rows {
				mockRows.AddRow(b.ID, b.CategoryID, b.MonthKey, b.Amount)
			}

			mock.ExpectQuery(query).WillReturnRows(mockRows)

			got, err := s.ListCategoryBudgets(context.Background())
			require.NoError(t, err)
			require.Equal(t, tt.rows, got)
		})
	}

	t.Run("a driver error is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(query).WillReturnError(errStub)

		_, err := s.ListCategoryBudgets(context.Background())
		require.ErrorIs(t, err, errStub)
	})
}

// TestSetCategoryBudget covers both branches: a positive amount upserts the row, while a
// zero-or-negative amount deletes it instead.
func TestSetCategoryBudget(t *testing.T) {
	t.Parallel()

	upsertQuery := `INSERT INTO budgets (category_id, month_key, amount) VALUES (?, ?, ?)
				 ON CONFLICT (category_id, month_key)
				 DO UPDATE SET amount = excluded.amount, updated_at = CURRENT_TIMESTAMP`
	deleteQuery := `DELETE FROM budgets WHERE category_id = ? AND month_key = ?`

	categoryID := fakeID()
	monthKey := fakeMonthKey()

	tests := []struct {
		name      string
		amount    int64
		mock      func(mock sqlmock.Sqlmock, amount int64)
		assertErr func(t *testing.T, err error)
	}{
		{
			name:   "a positive amount upserts the row",
			amount: fakeAmount(),
			mock: func(mock sqlmock.Sqlmock, amount int64) {
				mock.ExpectExec(upsertQuery).WithArgs(categoryID, monthKey, amount).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name:   "a zero amount deletes the row",
			amount: 0,
			mock: func(mock sqlmock.Sqlmock, amount int64) {
				mock.ExpectExec(deleteQuery).WithArgs(categoryID, monthKey).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name:   "a driver error on the upsert is propagated",
			amount: fakeAmount(),
			mock: func(mock sqlmock.Sqlmock, amount int64) {
				mock.ExpectExec(upsertQuery).WithArgs(categoryID, monthKey, amount).WillReturnError(errStub)
			},
			assertErr: func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
		{
			name:   "a driver error on the delete is propagated",
			amount: 0,
			mock: func(mock sqlmock.Sqlmock, amount int64) {
				mock.ExpectExec(deleteQuery).WithArgs(categoryID, monthKey).WillReturnError(errStub)
			},
			assertErr: func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)
			tt.mock(mock, tt.amount)

			err := s.SetCategoryBudget(context.Background(), model.BudgetSetRequest{
				CategoryID: categoryID,
				MonthKey:   monthKey,
				Amount:     tt.amount,
			})
			tt.assertErr(t, err)
		})
	}
}
