package postgres

import (
	"context"
	"database/sql/driver"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	storageError "raccounting/internal/storage/error"
	"raccounting/internal/storage/model"
)

const tagSelectColumns = `id, name, color`

func fakeTagRow() model.Tag {
	return model.Tag{ID: fakeID(), Name: fakeName(), Color: fakeColor()}
}

func tagRowValues(m model.Tag) []driver.Value {
	return []driver.Value{m.ID, m.Name, m.Color}
}

// TestListTags covers the full listing: every row comes back scanned into model.Tag.
func TestListTags(t *testing.T) {
	t.Parallel()

	query := `SELECT ` + tagSelectColumns + ` FROM tags ORDER BY created_at`

	one := fakeTagRow()
	two := fakeTagRow()

	tests := []struct {
		name string
		rows []model.Tag
	}{
		{name: "an empty result yields no rows"},
		{name: "a single tag", rows: []model.Tag{one}},
		{name: "multiple tags", rows: []model.Tag{one, two}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)

			mockRows := sqlmock.NewRows([]string{"id", "name", "color"})
			for _, tag := range tt.rows {
				mockRows.AddRow(tagRowValues(tag)...)
			}

			mock.ExpectQuery(query).WillReturnRows(mockRows)

			got, err := s.ListTags(context.Background())
			require.NoError(t, err)
			require.Equal(t, tt.rows, got)
		})
	}

	t.Run("a driver error is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(query).WillReturnError(errStub)

		_, err := s.ListTags(context.Background())
		require.ErrorIs(t, err, errStub)
	})
}

// TestFindTagsByIDs covers the bulk lookup transaction reads use to hydrate tag ids, including the
// empty-input short circuit (no query issued at all) and the $N placeholder numbering for a
// multi-id IN clause.
func TestFindTagsByIDs(t *testing.T) {
	t.Parallel()

	t.Run("an empty id list issues no query", func(t *testing.T) {
		t.Parallel()

		s, _ := newMock(t)

		got, err := s.FindTagsByIDs(context.Background(), nil)
		require.NoError(t, err)
		require.Nil(t, got)
	})

	one := fakeTagRow()
	two := fakeTagRow()
	ids := []uint64{one.ID, two.ID}

	t.Run("returns every tag among the given ids that exists", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)

		query := `SELECT ` + tagSelectColumns + ` FROM tags WHERE id IN ($1,$2)`
		mock.ExpectQuery(query).WithArgs(ids[0], ids[1]).WillReturnRows(
			sqlmock.NewRows([]string{"id", "name", "color"}).
				AddRow(tagRowValues(one)...).
				AddRow(tagRowValues(two)...),
		)

		got, err := s.FindTagsByIDs(context.Background(), ids)
		require.NoError(t, err)
		require.Equal(t, []model.Tag{one, two}, got)
	})

	t.Run("a driver error is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)

		query := `SELECT ` + tagSelectColumns + ` FROM tags WHERE id IN ($1)`
		mock.ExpectQuery(query).WithArgs(one.ID).WillReturnError(errStub)

		_, err := s.FindTagsByIDs(context.Background(), []uint64{one.ID})
		require.ErrorIs(t, err, errStub)
	})
}

// TestCreateTag covers the insert and the UNIQUE(name) violation it can produce.
func TestCreateTag(t *testing.T) {
	t.Parallel()

	query := `INSERT INTO tags (name, color) VALUES ($1, $2) RETURNING id`
	req := model.TagCreateRequest{Name: fakeName(), Color: fakeColor()}
	wantID := fakeID()

	tests := []struct {
		name         string
		mock         func(mock sqlmock.Sqlmock)
		assertResult func(t *testing.T, got uint64)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "creates the tag",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(query).WithArgs(req.Name, req.Color).
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(wantID))
			},
			assertResult: func(t *testing.T, got uint64) { require.Equal(t, wantID, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a taken name is a unique violation",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(query).WithArgs(req.Name, req.Color).WillReturnError(pgErr(pgUniqueViolation))
			},
			assertResult: func(t *testing.T, got uint64) {},
			assertErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, storageError.UniqueViolationError)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)
			tt.mock(mock)

			got, err := s.CreateTag(context.Background(), req)
			tt.assertErr(t, err)
			tt.assertResult(t, got)
		})
	}
}

// TestUpdateTag covers the update, its found/not-found translation, the follow-up read that produces
// the returned row, and the UNIQUE(name) violation the rename can produce.
func TestUpdateTag(t *testing.T) {
	t.Parallel()

	updateQuery := `UPDATE tags SET name = $1, color = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $3`
	selectQuery := `SELECT ` + tagSelectColumns + ` FROM tags WHERE id = $1`

	req := model.TagUpdateRequest{ID: fakeID(), Name: fakeName(), Color: fakeColor()}
	updated := fakeTagRow()
	updated.ID = req.ID

	tests := []struct {
		name         string
		mock         func(mock sqlmock.Sqlmock)
		assertResult func(t *testing.T, got model.Tag, found bool)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "updates and returns the row",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(updateQuery).WithArgs(req.Name, req.Color, req.ID).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectQuery(selectQuery).WithArgs(req.ID).WillReturnRows(
					sqlmock.NewRows([]string{"id", "name", "color"}).AddRow(tagRowValues(updated)...),
				)
			},
			assertResult: func(t *testing.T, got model.Tag, found bool) {
				require.True(t, found)
				require.Equal(t, updated, got)
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "an unknown id is reported as not found",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(updateQuery).WithArgs(req.Name, req.Color, req.ID).WillReturnResult(sqlmock.NewResult(0, 0))
			},
			assertResult: func(t *testing.T, got model.Tag, found bool) { require.False(t, found) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a taken name is a unique violation",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(updateQuery).WithArgs(req.Name, req.Color, req.ID).WillReturnError(pgErr(pgUniqueViolation))
			},
			assertResult: func(t *testing.T, got model.Tag, found bool) { require.False(t, found) },
			assertErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, storageError.UniqueViolationError)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)
			tt.mock(mock)

			got, found, err := s.UpdateTag(context.Background(), req)
			tt.assertErr(t, err)
			tt.assertResult(t, got, found)
		})
	}
}

// TestDeleteTag covers the delete. Unlike accounts/categories/currencies, deleting a tag has no
// FOREIGN KEY guard to translate — transaction_tags cascades the delete instead (see the migrations).
func TestDeleteTag(t *testing.T) {
	t.Parallel()

	query := `DELETE FROM tags WHERE id = $1`
	id := fakeID()

	tests := []struct {
		name         string
		mock         func(mock sqlmock.Sqlmock)
		assertResult func(t *testing.T, found bool)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "deletes the tag",
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

			found, err := s.DeleteTag(context.Background(), id)
			tt.assertErr(t, err)
			tt.assertResult(t, found)
		})
	}
}
