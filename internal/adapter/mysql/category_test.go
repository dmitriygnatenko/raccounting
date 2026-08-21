package mysql

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"raccounting/internal/domain/entity"
	storageError "raccounting/internal/storage/error"
	"raccounting/internal/storage/model"
)

func fakeCategory() model.Category {
	return model.Category{
		ID:     fakeID(),
		Name:   fakeName(),
		Color:  fakeColor(),
		Type:   uint8(entity.CategoryTypeExpense),
		Status: uint8(entity.AccountStatusActive),
	}
}

func addCategoryRow(rows *sqlmock.Rows, c model.Category) *sqlmock.Rows {
	return rows.AddRow(c.ID, c.Name, c.Color, c.Type, c.Status)
}

// TestListCategories covers the full listing.
func TestListCategories(t *testing.T) {
	t.Parallel()

	query := `SELECT ` + categoryColumns + ` FROM categories ORDER BY created_at`

	t.Run("an empty result yields no rows", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(query).WillReturnRows(sqlmock.NewRows([]string{"id", "name", "color", "type", "status"}))

		got, err := s.ListCategories(context.Background())
		require.NoError(t, err)
		require.Empty(t, got)
	})

	t.Run("multiple rows come back in order", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		c1, c2 := fakeCategory(), fakeCategory()

		rows := sqlmock.NewRows([]string{"id", "name", "color", "type", "status"})
		addCategoryRow(rows, c1)
		addCategoryRow(rows, c2)
		mock.ExpectQuery(query).WillReturnRows(rows)

		got, err := s.ListCategories(context.Background())
		require.NoError(t, err)
		require.Equal(t, []model.Category{c1, c2}, got)
	})

	t.Run("a driver error is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(query).WillReturnError(errStub)

		_, err := s.ListCategories(context.Background())
		require.ErrorIs(t, err, errStub)
	})
}

// TestExistsCategory covers the existence check UpdateCategory-adjacent validation relies on.
func TestExistsCategory(t *testing.T) {
	t.Parallel()

	query := `SELECT COUNT(*) FROM categories WHERE id = ?`
	id := fakeID()

	tests := []struct {
		name         string
		mock         func(mock sqlmock.Sqlmock)
		assertResult func(t *testing.T, got bool)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "an existing category reports true",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(query).WithArgs(id).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
			},
			assertResult: func(t *testing.T, got bool) { require.True(t, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "an unknown id reports false",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(query).WithArgs(id).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			},
			assertResult: func(t *testing.T, got bool) { require.False(t, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a driver error is propagated",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(query).WithArgs(id).WillReturnError(errStub)
			},
			assertResult: func(t *testing.T, got bool) {},
			assertErr:    func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)
			tt.mock(mock)

			got, err := s.ExistsCategory(context.Background(), id)
			tt.assertErr(t, err)
			tt.assertResult(t, got)
		})
	}
}

// TestCreateCategory covers the insert. Unlike tags, a category name isn't unique, so there's no
// constraint violation to wrap here.
func TestCreateCategory(t *testing.T) {
	t.Parallel()

	query := `INSERT INTO categories (name, type, color) VALUES (?, ?, ?)`
	req := model.CategoryCreateRequest{Name: fakeName(), Type: entity.CategoryTypeIncome, Color: fakeColor()}
	wantID := int64(fakeID())

	tests := []struct {
		name         string
		mock         func(mock sqlmock.Sqlmock)
		assertResult func(t *testing.T, got uint64)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "creates the category",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(req.Name, uint8(req.Type), req.Color).
					WillReturnResult(sqlmock.NewResult(wantID, 1))
			},
			assertResult: func(t *testing.T, got uint64) { require.Equal(t, uint64(wantID), got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a driver error is propagated",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(req.Name, uint8(req.Type), req.Color).WillReturnError(errStub)
			},
			assertResult: func(t *testing.T, got uint64) {},
			assertErr:    func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)
			tt.mock(mock)

			got, err := s.CreateCategory(context.Background(), req)
			tt.assertErr(t, err)
			tt.assertResult(t, got)
		})
	}
}

// TestUpdateCategory covers the rename/recolor/archive path, which re-reads the row after a
// successful write, and the not-found case, which skips that re-read.
func TestUpdateCategory(t *testing.T) {
	t.Parallel()

	updateQuery := `UPDATE categories SET name = ?, color = ?, status = ?, updated_at = CURRENT_TIMESTAMP
			 WHERE id = ?`
	selectQuery := `SELECT ` + categoryColumns + ` FROM categories WHERE id = ?`

	req := model.CategoryUpdateRequest{ID: fakeID(), Name: fakeName(), Color: fakeColor(), Archived: false}
	wantRow := fakeCategory()
	wantRow.ID = req.ID

	tests := []struct {
		name         string
		mock         func(mock sqlmock.Sqlmock)
		assertResult func(t *testing.T, got model.Category, found bool)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "updates and re-reads the row",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(updateQuery).
					WithArgs(req.Name, req.Color, uint8(entity.AccountStatusActive), req.ID).
					WillReturnResult(sqlmock.NewResult(0, 1))

				rows := sqlmock.NewRows([]string{"id", "name", "color", "type", "status"})
				mock.ExpectQuery(selectQuery).WithArgs(req.ID).WillReturnRows(addCategoryRow(rows, wantRow))
			},
			assertResult: func(t *testing.T, got model.Category, found bool) {
				require.True(t, found)
				require.Equal(t, wantRow, got)
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "an unknown id is reported as not found, without a re-read",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(updateQuery).
					WithArgs(req.Name, req.Color, uint8(entity.AccountStatusActive), req.ID).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			assertResult: func(t *testing.T, got model.Category, found bool) { require.False(t, found) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a driver error on the update is propagated",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(updateQuery).
					WithArgs(req.Name, req.Color, uint8(entity.AccountStatusActive), req.ID).
					WillReturnError(errStub)
			},
			assertResult: func(t *testing.T, got model.Category, found bool) { require.False(t, found) },
			assertErr:    func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)
			tt.mock(mock)

			got, found, err := s.UpdateCategory(context.Background(), req)
			tt.assertErr(t, err)
			tt.assertResult(t, got, found)
		})
	}
}

// TestDeleteCategory covers the delete and the constraint failure a category still referenced by a
// transaction produces.
func TestDeleteCategory(t *testing.T) {
	t.Parallel()

	query := `DELETE FROM categories WHERE id = ?`
	id := fakeID()

	tests := []struct {
		name         string
		mock         func(mock sqlmock.Sqlmock)
		assertResult func(t *testing.T, found bool)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "deletes an unreferenced category",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(id).WillReturnResult(sqlmock.NewResult(0, 1))
			},
			assertResult: func(t *testing.T, found bool) { require.True(t, found) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "an unknown id is reported as not found",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(id).WillReturnResult(sqlmock.NewResult(0, 0))
			},
			assertResult: func(t *testing.T, found bool) { require.False(t, found) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a category still referenced by a transaction surfaces a foreign key violation",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(id).WillReturnError(mysqlErr(errRowIsReferenced))
			},
			assertResult: func(t *testing.T, found bool) {},
			assertErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, storageError.ForeignKeyViolationError)
			},
		},
		{
			name: "a driver error is propagated",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(id).WillReturnError(errStub)
			},
			assertResult: func(t *testing.T, found bool) {},
			assertErr:    func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)
			tt.mock(mock)

			found, err := s.DeleteCategory(context.Background(), id)
			tt.assertErr(t, err)
			tt.assertResult(t, found)
		})
	}
}
