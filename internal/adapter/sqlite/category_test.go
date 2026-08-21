package sqlite

import (
	"context"
	"database/sql"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"raccounting/internal/domain/entity"
	storageError "raccounting/internal/storage/error"
	"raccounting/internal/storage/model"
)

const categorySelectQuery = `SELECT id, name, color, type, status FROM categories`

var categoryColumnNames = []string{"id", "name", "color", "type", "status"}

// fakeCategory returns a random model.Category, as scanCategory would produce it (CreatedAt/UpdatedAt
// are left zero — categoryColumns doesn't select them).
func fakeCategory() model.Category {
	return model.Category{
		ID:     fakeID(),
		Name:   fakeName(),
		Color:  fakeColor(),
		Type:   uint8(entity.CategoryTypeExpense),
		Status: uint8(entity.CategoryStatusActive),
	}
}

func addCategoryRow(rows *sqlmock.Rows, m model.Category) *sqlmock.Rows {
	return rows.AddRow(m.ID, m.Name, m.Color, m.Type, m.Status)
}

// TestListCategories covers the full listing: every row comes back scanned into model.Category.
func TestListCategories(t *testing.T) {
	t.Parallel()

	query := categorySelectQuery + ` ORDER BY created_at`

	c1, c2 := fakeCategory(), fakeCategory()

	tests := []struct {
		name string
		rows []model.Category
	}{
		{name: "an empty result yields no rows"},
		{name: "a single category", rows: []model.Category{c1}},
		{name: "several categories", rows: []model.Category{c1, c2}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)

			mockRows := sqlmock.NewRows(categoryColumnNames)
			for _, c := range tt.rows {
				addCategoryRow(mockRows, c)
			}

			mock.ExpectQuery(query).WillReturnRows(mockRows)

			got, err := s.ListCategories(context.Background())
			require.NoError(t, err)
			require.Equal(t, tt.rows, got)
		})
	}

	t.Run("a driver error is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(query).WillReturnError(errStub)

		_, err := s.ListCategories(context.Background())
		require.ErrorIs(t, err, errStub)
	})
}

// TestExistsCategory covers the referential check used before pointing a transaction/budget at a
// category id.
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
			name: "an existing id reports true",
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
			name:         "a driver error is propagated",
			mock:         func(mock sqlmock.Sqlmock) { mock.ExpectQuery(query).WithArgs(id).WillReturnError(errStub) },
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

// TestCreateCategory covers the insert and id generation.
func TestCreateCategory(t *testing.T) {
	t.Parallel()

	query := `INSERT INTO categories (name, type, color) VALUES (?, ?, ?)`

	req := model.CategoryCreateRequest{
		Name:  fakeName(),
		Type:  entity.CategoryTypeExpense,
		Color: fakeColor(),
	}
	wantID := int64(fakeID())

	tests := []struct {
		name         string
		mock         func(mock sqlmock.Sqlmock)
		assertResult func(t *testing.T, got uint64)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "inserts the category",
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

			id, err := s.CreateCategory(context.Background(), req)
			tt.assertErr(t, err)
			tt.assertResult(t, id)
		})
	}
}

// TestUpdateCategory covers the update, which re-reads the full row on success, and the not-found
// case, where the follow-up read never happens.
func TestUpdateCategory(t *testing.T) {
	t.Parallel()

	updateQuery := `UPDATE categories SET name = ?, color = ?, status = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`
	selectQuery := categorySelectQuery + ` WHERE id = ?`

	req := model.CategoryUpdateRequest{
		ID:       fakeID(),
		Name:     fakeName(),
		Color:    fakeColor(),
		Archived: false,
	}
	want := fakeCategory()
	want.ID = req.ID

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
					WithArgs(req.Name, req.Color, uint8(entity.CategoryStatusActive), req.ID).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectQuery(selectQuery).WithArgs(req.ID).WillReturnRows(
					addCategoryRow(sqlmock.NewRows(categoryColumnNames), want),
				)
			},
			assertResult: func(t *testing.T, got model.Category, found bool) {
				require.True(t, found)
				require.Equal(t, want, got)
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "an unknown id is reported as not found, no re-read",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(updateQuery).
					WithArgs(req.Name, req.Color, uint8(entity.CategoryStatusActive), req.ID).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			assertResult: func(t *testing.T, got model.Category, found bool) { require.False(t, found) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a driver error on the update is propagated",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(updateQuery).
					WithArgs(req.Name, req.Color, uint8(entity.CategoryStatusActive), req.ID).
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
		res          sql.Result
		mockErr      error
		assertResult func(t *testing.T, found bool)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name:         "deletes an unreferenced category",
			res:          sqlmock.NewResult(0, 1),
			assertResult: func(t *testing.T, found bool) { require.True(t, found) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name:         "an unknown id is reported as not found",
			res:          sqlmock.NewResult(0, 0),
			assertResult: func(t *testing.T, found bool) { require.False(t, found) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name:         "a category still referenced by a transaction is a foreign key violation",
			mockErr:      sqliteForeignKeyErr(),
			assertResult: func(t *testing.T, found bool) {},
			assertErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, storageError.ForeignKeyViolationError)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)

			exp := mock.ExpectExec(query).WithArgs(id)
			if tt.mockErr != nil {
				exp.WillReturnError(tt.mockErr)
			} else {
				exp.WillReturnResult(tt.res)
			}

			found, err := s.DeleteCategory(context.Background(), id)
			tt.assertErr(t, err)
			tt.assertResult(t, found)
		})
	}
}
