package postgres

import (
	"context"

	"raccounting/internal/storage/model"
)

// ListCategoryBudgets returns every category budget.
func (s *Storage) ListCategoryBudgets(ctx context.Context) ([]model.Budget, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT id, category_id, month_key, amount FROM budgets ORDER BY month_key`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var budgets []model.Budget

	for rows.Next() {
		var m model.Budget
		if err := rows.Scan(&m.ID, &m.CategoryID, &m.MonthKey, &m.Amount); err != nil {
			return nil, err
		}

		budgets = append(budgets, m)
	}

	return budgets, rows.Err()
}

// SetCategoryBudget upserts the (categoryId, monthKey) row when Amount > 0, and deletes it
// otherwise.
func (s *Storage) SetCategoryBudget(ctx context.Context, req model.BudgetSetRequest) error {
	if req.Amount > 0 {
		_, err := s.DB.ExecContext(ctx,
			`INSERT INTO budgets (category_id, month_key, amount) VALUES ($1, $2, $3)
			 ON CONFLICT (category_id, month_key)
			 DO UPDATE SET amount = excluded.amount, updated_at = CURRENT_TIMESTAMP`,
			req.CategoryID, req.MonthKey, req.Amount,
		)

		return err
	}

	_, err := s.DB.ExecContext(ctx,
		`DELETE FROM budgets WHERE category_id = $1 AND month_key = $2`,
		req.CategoryID, req.MonthKey,
	)

	return err
}
