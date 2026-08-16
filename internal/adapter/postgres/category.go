package postgres

import (
	"context"

	"raccounting/internal/port"
	"raccounting/internal/storage/model"
)

// categoryScanner is satisfied by both *sql.Row and *sql.Rows.
type categoryScanner interface {
	Scan(dest ...any) error
}

// scanCategory reads the category column list (id, name, color, type, status), in the order every
// category query below selects it.
func scanCategory(row categoryScanner) (model.Category, error) {
	var m model.Category

	if err := row.Scan(&m.ID, &m.Name, &m.Color, &m.Type, &m.Status); err != nil {
		return model.Category{}, err
	}

	return m, nil
}

const categoryColumns = `id, name, color, type, status`

// ListCategories returns every category, oldest-created first.
func (s *Storage) ListCategories(ctx context.Context) ([]model.Category, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+categoryColumns+` FROM categories ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []model.Category

	for rows.Next() {
		m, err := scanCategory(rows)
		if err != nil {
			return nil, err
		}

		categories = append(categories, m)
	}

	return categories, rows.Err()
}

// ExistsCategory reports whether a category with this id exists.
func (s *Storage) ExistsCategory(ctx context.Context, id uint64) (bool, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM categories WHERE id = $1`, id).Scan(&n)

	return n > 0, err
}

// CreateCategory inserts a category row and returns its new id.
func (s *Storage) CreateCategory(ctx context.Context, req port.CategoryCreateRequest) (uint64, error) {
	return s.insertReturningID(ctx,
		`INSERT INTO categories (name, type, color) VALUES ($1, $2, $3) RETURNING id`,
		req.Name, uint8(req.Type), req.Color,
	)
}

// UpdateCategory changes name/color/status, returning the full updated row.
func (s *Storage) UpdateCategory(
	ctx context.Context, req port.CategoryUpdateRequest,
) (model.Category, bool, error) {
	res, err := s.DB.ExecContext(ctx,
		`UPDATE categories SET name = $1, color = $2, status = $3, updated_at = CURRENT_TIMESTAMP
		 WHERE id = $4`,
		req.Name, req.Color, uint8(archivedStatus(req.Archived)), req.ID,
	)
	if err != nil {
		return model.Category{}, false, err
	}

	found, err := affected(res)
	if err != nil || !found {
		return model.Category{}, false, err
	}

	row := s.DB.QueryRowContext(ctx, `SELECT `+categoryColumns+` FROM categories WHERE id = $1`, req.ID)

	m, err := scanCategory(row)

	return m, true, err
}

// DeleteCategory removes a category row. found is false if no category with this id existed. A
// FOREIGN KEY violation (still referenced by a transaction) comes back wrapped in
// storageError.ForeignKeyViolationError.
func (s *Storage) DeleteCategory(ctx context.Context, id uint64) (bool, error) {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM categories WHERE id = $1`, id)
	if err != nil {
		return false, wrapForeignKey(err)
	}

	return affected(res)
}
