package mysql

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/brianvoe/gofakeit/v7"
	"github.com/stretchr/testify/require"

	"raccounting/internal/storage/model"
)

func fakeMonthKey() string { return gofakeit.Date().Format("2006-01") }

// TestListCategoryBudgets covers the full listing.
func TestListCategoryBudgets(t *testing.T) {
	t.Parallel()

	query := `SELECT id, category_id, month_key, amount FROM budgets ORDER BY month_key`

	t.Run("an empty result yields no rows", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(query).WillReturnRows(sqlmock.NewRows([]string{"id", "category_id", "month_key", "amount"}))

		got, err := s.ListCategoryBudgets(context.Background())
		require.NoError(t, err)
		require.Empty(t, got)
	})

	t.Run("multiple rows come back in order", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		b1 := model.Budget{ID: fakeID(), CategoryID: fakeID(), MonthKey: fakeMonthKey(), Amount: fakeAmount()}
		b2 := model.Budget{ID: fakeID(), CategoryID: fakeID(), MonthKey: fakeMonthKey(), Amount: fakeAmount()}

		rows := sqlmock.NewRows([]string{"id", "category_id", "month_key", "amount"}).
			AddRow(b1.ID, b1.CategoryID, b1.MonthKey, b1.Amount).
			AddRow(b2.ID, b2.CategoryID, b2.MonthKey, b2.Amount)
		mock.ExpectQuery(query).WillReturnRows(rows)

		got, err := s.ListCategoryBudgets(context.Background())
		require.NoError(t, err)
		require.Equal(t, []model.Budget{b1, b2}, got)
	})

	t.Run("a driver error is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(query).WillReturnError(errStub)

		_, err := s.ListCategoryBudgets(context.Background())
		require.ErrorIs(t, err, errStub)
	})
}

// TestSetCategoryBudget covers both branches: a positive amount upserts, and a zero/negative amount
// deletes the row instead.
func TestSetCategoryBudget(t *testing.T) {
	t.Parallel()

	upsertQuery := `INSERT INTO budgets (category_id, month_key, amount) VALUES (?, ?, ?)
				 ON DUPLICATE KEY UPDATE amount = VALUES(amount), updated_at = CURRENT_TIMESTAMP`
	deleteQuery := `DELETE FROM budgets WHERE category_id = ? AND month_key = ?`

	categoryID := fakeID()
	monthKey := fakeMonthKey()

	tests := []struct {
		name      string
		req       model.BudgetSetRequest
		mock      func(mock sqlmock.Sqlmock, req model.BudgetSetRequest)
		assertErr func(t *testing.T, err error)
	}{
		{
			name: "a positive amount upserts the row",
			req:  model.BudgetSetRequest{CategoryID: categoryID, MonthKey: monthKey, Amount: fakeAmount()},
			mock: func(mock sqlmock.Sqlmock, req model.BudgetSetRequest) {
				mock.ExpectExec(upsertQuery).WithArgs(req.CategoryID, req.MonthKey, req.Amount).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a zero amount deletes the row",
			req:  model.BudgetSetRequest{CategoryID: categoryID, MonthKey: monthKey, Amount: 0},
			mock: func(mock sqlmock.Sqlmock, req model.BudgetSetRequest) {
				mock.ExpectExec(deleteQuery).WithArgs(req.CategoryID, req.MonthKey).WillReturnResult(sqlmock.NewResult(0, 1))
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a driver error on the upsert is propagated",
			req:  model.BudgetSetRequest{CategoryID: categoryID, MonthKey: monthKey, Amount: fakeAmount()},
			mock: func(mock sqlmock.Sqlmock, req model.BudgetSetRequest) {
				mock.ExpectExec(upsertQuery).WithArgs(req.CategoryID, req.MonthKey, req.Amount).WillReturnError(errStub)
			},
			assertErr: func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
		{
			name: "a driver error on the delete is propagated",
			req:  model.BudgetSetRequest{CategoryID: categoryID, MonthKey: monthKey, Amount: 0},
			mock: func(mock sqlmock.Sqlmock, req model.BudgetSetRequest) {
				mock.ExpectExec(deleteQuery).WithArgs(req.CategoryID, req.MonthKey).WillReturnError(errStub)
			},
			assertErr: func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)
			tt.mock(mock, tt.req)

			err := s.SetCategoryBudget(context.Background(), tt.req)
			tt.assertErr(t, err)
		})
	}
}
