package postgres

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"raccounting/internal/storage/model"
)

func fakeBudgetRow() model.Budget {
	return model.Budget{ID: fakeID(), CategoryID: fakeID(), MonthKey: fakeMonthKey(), Amount: fakeAmount()}
}

// TestListCategoryBudgets covers the full listing: every row comes back scanned into model.Budget.
func TestListCategoryBudgets(t *testing.T) {
	t.Parallel()

	query := `SELECT id, category_id, month_key, amount FROM budgets ORDER BY month_key`

	one := fakeBudgetRow()
	two := fakeBudgetRow()

	tests := []struct {
		name string
		rows []model.Budget
	}{
		{name: "an empty result yields no rows"},
		{name: "a single budget", rows: []model.Budget{one}},
		{name: "multiple budgets", rows: []model.Budget{one, two}},
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

// TestSetCategoryBudget covers both branches of the upsert-or-delete: a positive Amount upserts via
// ON CONFLICT, and Amount <= 0 deletes the row instead.
func TestSetCategoryBudget(t *testing.T) {
	t.Parallel()

	upsertQuery := `INSERT INTO budgets (category_id, month_key, amount) VALUES ($1, $2, $3)
				 ON CONFLICT (category_id, month_key)
				 DO UPDATE SET amount = excluded.amount, updated_at = CURRENT_TIMESTAMP`
	deleteQuery := `DELETE FROM budgets WHERE category_id = $1 AND month_key = $2`

	categoryID := fakeID()
	monthKey := fakeMonthKey()
	amount := fakeAmount()

	tests := []struct {
		name      string
		req       model.BudgetSetRequest
		mock      func(mock sqlmock.Sqlmock)
		assertErr func(t *testing.T, err error)
	}{
		{
			name: "a positive amount upserts the row",
			req:  model.BudgetSetRequest{CategoryID: categoryID, MonthKey: monthKey, Amount: amount},
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(upsertQuery).WithArgs(categoryID, monthKey, amount).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a zero amount deletes the row",
			req:  model.BudgetSetRequest{CategoryID: categoryID, MonthKey: monthKey, Amount: 0},
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(deleteQuery).WithArgs(categoryID, monthKey).WillReturnResult(sqlmock.NewResult(0, 1))
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a negative amount also deletes the row",
			req:  model.BudgetSetRequest{CategoryID: categoryID, MonthKey: monthKey, Amount: -1},
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(deleteQuery).WithArgs(categoryID, monthKey).WillReturnResult(sqlmock.NewResult(0, 1))
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a driver error on upsert is propagated",
			req:  model.BudgetSetRequest{CategoryID: categoryID, MonthKey: monthKey, Amount: amount},
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(upsertQuery).WithArgs(categoryID, monthKey, amount).WillReturnError(errStub)
			},
			assertErr: func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
		{
			name: "a driver error on delete is propagated",
			req:  model.BudgetSetRequest{CategoryID: categoryID, MonthKey: monthKey, Amount: 0},
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(deleteQuery).WithArgs(categoryID, monthKey).WillReturnError(errStub)
			},
			assertErr: func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)
			tt.mock(mock)

			err := s.SetCategoryBudget(context.Background(), tt.req)
			tt.assertErr(t, err)
		})
	}
}
