package mysql

import (
	"context"
	"strings"

	"raccounting/internal/port"
	"raccounting/internal/storage/model"
)

// tagScanner is satisfied by both *sql.Row and *sql.Rows.
type tagScanner interface {
	Scan(dest ...any) error
}

// scanTag reads the tag column list (id, name, color), in the order every tag query below selects
// it.
func scanTag(row tagScanner) (model.Tag, error) {
	var m model.Tag

	if err := row.Scan(&m.ID, &m.Name, &m.Color); err != nil {
		return model.Tag{}, err
	}

	return m, nil
}

const tagColumns = `id, name, color`

// ListTags returns every tag, oldest-created first.
func (s *Storage) ListTags(ctx context.Context) ([]model.Tag, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+tagColumns+` FROM tags ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []model.Tag

	for rows.Next() {
		m, err := scanTag(rows)
		if err != nil {
			return nil, err
		}

		tags = append(tags, m)
	}

	return tags, rows.Err()
}

// FindTagsByIDs returns every tag among ids that exists.
func (s *Storage) FindTagsByIDs(ctx context.Context, ids []uint64) ([]model.Tag, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))

	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	query := `SELECT ` + tagColumns + ` FROM tags WHERE id IN (` + strings.Join(placeholders, ",") + `)`

	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []model.Tag

	for rows.Next() {
		m, err := scanTag(rows)
		if err != nil {
			return nil, err
		}

		tags = append(tags, m)
	}

	return tags, rows.Err()
}

// CreateTag inserts a tag row and returns its new id. A UNIQUE(name) violation comes back wrapped
// in storageError.UniqueViolationError.
func (s *Storage) CreateTag(ctx context.Context, req port.TagCreateRequest) (uint64, error) {
	id, err := s.insertReturningID(ctx,
		`INSERT INTO tags (name, color) VALUES (?, ?)`,
		req.Name, req.Color,
	)

	return id, wrapUnique(err)
}

// UpdateTag changes name/color, returning the full updated row. A UNIQUE(name) violation comes
// back wrapped in storageError.UniqueViolationError.
func (s *Storage) UpdateTag(
	ctx context.Context, req port.TagUpdateRequest,
) (model.Tag, bool, error) {
	res, err := s.DB.ExecContext(ctx,
		`UPDATE tags SET name = ?, color = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		req.Name, req.Color, req.ID,
	)
	if err != nil {
		return model.Tag{}, false, wrapUnique(err)
	}

	found, err := affected(res)
	if err != nil || !found {
		return model.Tag{}, false, err
	}

	row := s.DB.QueryRowContext(ctx, `SELECT `+tagColumns+` FROM tags WHERE id = ?`, req.ID)

	m, err := scanTag(row)

	return m, true, err
}

// DeleteTag removes a tag row. found is false if no tag with this id existed. Any transaction that
// had this tag loses it (see the transaction_tags ON DELETE CASCADE in the migrations).
func (s *Storage) DeleteTag(ctx context.Context, id uint64) (bool, error) {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM tags WHERE id = ?`, id)
	if err != nil {
		return false, err
	}

	return affected(res)
}
