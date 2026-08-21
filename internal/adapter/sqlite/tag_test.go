package sqlite

import (
	"context"
	"database/sql"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	storageError "raccounting/internal/storage/error"
	"raccounting/internal/storage/model"
)

const tagSelectQuery = `SELECT id, name, color FROM tags`

var tagColumnNames = []string{"id", "name", "color"}

// fakeTag returns a random model.Tag, as scanTag would produce it (CreatedAt/UpdatedAt are left
// zero — tagColumns doesn't select them).
func fakeTag() model.Tag {
	return model.Tag{ID: fakeID(), Name: fakeName(), Color: fakeColor()}
}

func addTagRow(rows *sqlmock.Rows, m model.Tag) *sqlmock.Rows {
	return rows.AddRow(m.ID, m.Name, m.Color)
}

// TestListTags covers the full listing: every row comes back scanned into model.Tag.
func TestListTags(t *testing.T) {
	t.Parallel()

	query := tagSelectQuery + ` ORDER BY created_at`

	tag1, tag2 := fakeTag(), fakeTag()

	tests := []struct {
		name string
		rows []model.Tag
	}{
		{name: "an empty result yields no rows"},
		{name: "a single tag", rows: []model.Tag{tag1}},
		{name: "several tags", rows: []model.Tag{tag1, tag2}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)

			mockRows := sqlmock.NewRows(tagColumnNames)
			for _, tag := range tt.rows {
				addTagRow(mockRows, tag)
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

// TestFindTagsByIDs covers the bulk lookup used to hydrate a transaction's tags: a nil/empty id
// slice short-circuits without touching the database, and the placeholder list grows with the id
// count.
func TestFindTagsByIDs(t *testing.T) {
	t.Parallel()

	tag1, tag2 := fakeTag(), fakeTag()

	t.Run("an empty id slice short-circuits without a query", func(t *testing.T) {
		t.Parallel()

		s, _ := newMock(t)

		got, err := s.FindTagsByIDs(context.Background(), nil)
		require.NoError(t, err)
		require.Nil(t, got)
	})

	t.Run("a single id", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)

		query := tagSelectQuery + ` WHERE id IN (?)`
		mock.ExpectQuery(query).WithArgs(tag1.ID).WillReturnRows(
			addTagRow(sqlmock.NewRows(tagColumnNames), tag1),
		)

		got, err := s.FindTagsByIDs(context.Background(), []uint64{tag1.ID})
		require.NoError(t, err)
		require.Equal(t, []model.Tag{tag1}, got)
	})

	t.Run("several ids grow the placeholder list", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)

		query := tagSelectQuery + ` WHERE id IN (?,?)`
		mock.ExpectQuery(query).WithArgs(tag1.ID, tag2.ID).WillReturnRows(
			addTagRow(addTagRow(sqlmock.NewRows(tagColumnNames), tag1), tag2),
		)

		got, err := s.FindTagsByIDs(context.Background(), []uint64{tag1.ID, tag2.ID})
		require.NoError(t, err)
		require.Equal(t, []model.Tag{tag1, tag2}, got)
	})

	t.Run("a driver error is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)

		query := tagSelectQuery + ` WHERE id IN (?)`
		mock.ExpectQuery(query).WithArgs(tag1.ID).WillReturnError(errStub)

		_, err := s.FindTagsByIDs(context.Background(), []uint64{tag1.ID})
		require.ErrorIs(t, err, errStub)
	})
}

// TestCreateTag covers the insert and the one error the repositories act on: a taken name, which has
// to arrive as storageError.UniqueViolationError.
func TestCreateTag(t *testing.T) {
	t.Parallel()

	query := `INSERT INTO tags (name, color) VALUES (?, ?)`
	req := model.TagCreateRequest{Name: fakeName(), Color: fakeColor()}
	wantID := int64(fakeID())

	tests := []struct {
		name         string
		mock         func(mock sqlmock.Sqlmock)
		assertResult func(t *testing.T, got uint64)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "inserts the tag",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(req.Name, req.Color).WillReturnResult(sqlmock.NewResult(wantID, 1))
			},
			assertResult: func(t *testing.T, got uint64) { require.Equal(t, uint64(wantID), got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a taken name is a unique violation",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(req.Name, req.Color).WillReturnError(sqliteUniqueErr())
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

			id, err := s.CreateTag(context.Background(), req)
			tt.assertErr(t, err)
			tt.assertResult(t, id)
		})
	}
}

// TestUpdateTag covers the rename, which re-reads the full row on success, a taken name (a unique
// violation surfaced directly from the UPDATE, unlike currency/tag creates which surface it from an
// INSERT), and the not-found case, where the follow-up read never happens.
func TestUpdateTag(t *testing.T) {
	t.Parallel()

	updateQuery := `UPDATE tags SET name = ?, color = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	selectQuery := tagSelectQuery + ` WHERE id = ?`

	req := model.TagUpdateRequest{ID: fakeID(), Name: fakeName(), Color: fakeColor()}
	want := fakeTag()
	want.ID = req.ID

	tests := []struct {
		name         string
		mock         func(mock sqlmock.Sqlmock)
		assertResult func(t *testing.T, got model.Tag, found bool)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "renames and re-reads the row",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(updateQuery).WithArgs(req.Name, req.Color, req.ID).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectQuery(selectQuery).WithArgs(req.ID).WillReturnRows(
					addTagRow(sqlmock.NewRows(tagColumnNames), want),
				)
			},
			assertResult: func(t *testing.T, got model.Tag, found bool) {
				require.True(t, found)
				require.Equal(t, want, got)
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "an unknown id is reported as not found, no re-read",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(updateQuery).WithArgs(req.Name, req.Color, req.ID).WillReturnResult(sqlmock.NewResult(0, 0))
			},
			assertResult: func(t *testing.T, got model.Tag, found bool) { require.False(t, found) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a taken name is a unique violation",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(updateQuery).WithArgs(req.Name, req.Color, req.ID).WillReturnError(sqliteUniqueErr())
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

// TestDeleteTag covers the delete. Unlike accounts/categories/currencies, this has no foreign key
// wrapping — any transaction that had this tag just loses it via ON DELETE CASCADE.
func TestDeleteTag(t *testing.T) {
	t.Parallel()

	query := `DELETE FROM tags WHERE id = ?`
	id := fakeID()

	tests := []struct {
		name         string
		res          sql.Result
		mockErr      error
		assertResult func(t *testing.T, found bool)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name:         "deletes the tag",
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
			name:         "a driver error is propagated",
			mockErr:      errStub,
			assertResult: func(t *testing.T, found bool) { require.False(t, found) },
			assertErr:    func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
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

			found, err := s.DeleteTag(context.Background(), id)
			tt.assertErr(t, err)
			tt.assertResult(t, found)
		})
	}
}
