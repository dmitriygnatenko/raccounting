package postgres

import (
	"context"
	"database/sql/driver"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/brianvoe/gofakeit/v7"
	"github.com/stretchr/testify/require"

	"raccounting/internal/domain/entity"
	storageError "raccounting/internal/storage/error"
	"raccounting/internal/storage/model"
)

const categorySelectColumns = `id, name, color, type, status`

func fakeCategoryType() entity.CategoryType {
	types := entity.CategoryTypes()
	return types[gofakeit.Number(0, len(types)-1)]
}

func fakeCategoryRow() model.Category {
	return model.Category{
		ID:     fakeID(),
		Name:   fakeName(),
		Color:  fakeColor(),
		Type:   uint8(fakeCategoryType()),
		Status: uint8(entity.CategoryStatusActive),
	}
}

func categoryRowValues(m model.Category) []driver.Value {
	return []driver.Value{m.ID, m.Name, m.Color, m.Type, m.Status}
}

// TestListCategories covers the full listing: every row comes back scanned into model.Category.
func TestListCategories(t *testing.T) {
	t.Parallel()

	query := `SELECT ` + categorySelectColumns + ` FROM categories ORDER BY created_at`

	one := fakeCategoryRow()
	two := fakeCategoryRow()

	tests := []struct {
		name string
		rows []model.Category
	}{
		{name: "an empty result yields no rows"},
		{name: "a single category", rows: []model.Category{one}},
		{name: "multiple categories", rows: []model.Category{one, two}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)

			mockRows := sqlmock.NewRows([]string{"id", "name", "color", "type", "status"})
			for _, c := range tt.rows {
				mockRows.AddRow(categoryRowValues(c)...)
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

// TestExistsCategory covers the existence check the transaction/budget create paths validate a
// referenced category id against.
func TestExistsCategory(t *testing.T) {
	t.Parallel()

	query := `SELECT COUNT(*) FROM categories WHERE id = $1`
	id := fakeID()

	tests := []struct {
		name         string
		mock         func(mock sqlmock.Sqlmock)
		assertResult func(t *testing.T, got bool)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "an existing id is found",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(query).WithArgs(id).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
			},
			assertResult: func(t *testing.T, got bool) { require.True(t, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "an unknown id is not found",
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

// TestCreateCategory covers the insert, returning the generated id via the RETURNING clause.
func TestCreateCategory(t *testing.T) {
	t.Parallel()

	query := `INSERT INTO categories (name, type, color) VALUES ($1, $2, $3) RETURNING id`
	req := model.CategoryCreateRequest{Name: fakeName(), Type: fakeCategoryType(), Color: fakeColor()}
	wantID := fakeID()

	tests := []struct {
		name         string
		mock         func(mock sqlmock.Sqlmock)
		assertResult func(t *testing.T, got uint64)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "creates the category",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(query).WithArgs(req.Name, uint8(req.Type), req.Color).
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(wantID))
			},
			assertResult: func(t *testing.T, got uint64) { require.Equal(t, wantID, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a driver error is propagated",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(query).WithArgs(req.Name, uint8(req.Type), req.Color).WillReturnError(errStub)
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

// TestUpdateCategory covers the update, its found/not-found translation, and the follow-up read that
// produces the returned row.
func TestUpdateCategory(t *testing.T) {
	t.Parallel()

	updateQuery := `UPDATE categories SET name = $1, color = $2, status = $3, updated_at = CURRENT_TIMESTAMP
		 WHERE id = $4`
	selectQuery := `SELECT ` + categorySelectColumns + ` FROM categories WHERE id = $1`

	req := model.CategoryUpdateRequest{ID: fakeID(), Name: fakeName(), Color: fakeColor(), Archived: false}
	updated := fakeCategoryRow()
	updated.ID = req.ID

	tests := []struct {
		name         string
		mock         func(mock sqlmock.Sqlmock)
		assertResult func(t *testing.T, got model.Category, found bool)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "updates and returns the row",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(updateQuery).
					WithArgs(req.Name, req.Color, uint8(entity.CategoryStatusActive), req.ID).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectQuery(selectQuery).WithArgs(req.ID).WillReturnRows(
					sqlmock.NewRows([]string{"id", "name", "color", "type", "status"}).
						AddRow(categoryRowValues(updated)...),
				)
			},
			assertResult: func(t *testing.T, got model.Category, found bool) {
				require.True(t, found)
				require.Equal(t, updated, got)
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "an unknown id is reported as not found",
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

// TestDeleteCategory covers the delete and the FOREIGN KEY violation a still-referenced category
// produces.
func TestDeleteCategory(t *testing.T) {
	t.Parallel()

	query := `DELETE FROM categories WHERE id = $1`
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
			name: "a category still referenced by a transaction surfaces the driver's foreign key error",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(id).WillReturnError(pgErr(pgForeignKeyViolation))
			},
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
			tt.mock(mock)

			found, err := s.DeleteCategory(context.Background(), id)
			tt.assertErr(t, err)
			tt.assertResult(t, found)
		})
	}
}
