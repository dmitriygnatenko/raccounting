package mysql

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	storageError "raccounting/internal/storage/error"
	"raccounting/internal/storage/model"
)

func fakeTag() model.Tag {
	return model.Tag{ID: fakeID(), Name: fakeName(), Color: fakeColor()}
}

func addTagRow(rows *sqlmock.Rows, tag model.Tag) *sqlmock.Rows {
	return rows.AddRow(tag.ID, tag.Name, tag.Color)
}

// TestListTags covers the full listing.
func TestListTags(t *testing.T) {
	t.Parallel()

	query := `SELECT ` + tagColumns + ` FROM tags ORDER BY created_at`

	t.Run("an empty result yields no rows", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(query).WillReturnRows(sqlmock.NewRows([]string{"id", "name", "color"}))

		got, err := s.ListTags(context.Background())
		require.NoError(t, err)
		require.Empty(t, got)
	})

	t.Run("multiple rows come back in order", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		tag1, tag2 := fakeTag(), fakeTag()

		rows := sqlmock.NewRows([]string{"id", "name", "color"})
		addTagRow(rows, tag1)
		addTagRow(rows, tag2)
		mock.ExpectQuery(query).WillReturnRows(rows)

		got, err := s.ListTags(context.Background())
		require.NoError(t, err)
		require.Equal(t, []model.Tag{tag1, tag2}, got)
	})

	t.Run("a driver error is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(query).WillReturnError(errStub)

		_, err := s.ListTags(context.Background())
		require.ErrorIs(t, err, errStub)
	})
}

// TestFindTagsByIDs covers the batch lookup, including the empty-ids short circuit that skips the
// query entirely.
func TestFindTagsByIDs(t *testing.T) {
	t.Parallel()

	t.Run("no ids skips the query", func(t *testing.T) {
		t.Parallel()

		s, _ := newMock(t)

		got, err := s.FindTagsByIDs(context.Background(), nil)
		require.NoError(t, err)
		require.Nil(t, got)
	})

	t.Run("returns every tag among the given ids that exists", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		tag1, tag2 := fakeTag(), fakeTag()
		query := `SELECT ` + tagColumns + ` FROM tags WHERE id IN (?,?)`

		rows := sqlmock.NewRows([]string{"id", "name", "color"})
		addTagRow(rows, tag1)
		addTagRow(rows, tag2)
		mock.ExpectQuery(query).WithArgs(tag1.ID, tag2.ID).WillReturnRows(rows)

		got, err := s.FindTagsByIDs(context.Background(), []uint64{tag1.ID, tag2.ID})
		require.NoError(t, err)
		require.Equal(t, []model.Tag{tag1, tag2}, got)
	})

	t.Run("a driver error is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		id := fakeID()
		query := `SELECT ` + tagColumns + ` FROM tags WHERE id IN (?)`
		mock.ExpectQuery(query).WithArgs(id).WillReturnError(errStub)

		_, err := s.FindTagsByIDs(context.Background(), []uint64{id})
		require.ErrorIs(t, err, errStub)
	})
}

// TestCreateTag covers the insert and the UNIQUE(name) violation.
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
			name: "creates the tag",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(req.Name, req.Color).WillReturnResult(sqlmock.NewResult(wantID, 1))
			},
			assertResult: func(t *testing.T, got uint64) { require.Equal(t, uint64(wantID), got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a taken name is a unique violation",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(req.Name, req.Color).WillReturnError(mysqlErr(errDuplicateEntry))
			},
			assertResult: func(t *testing.T, got uint64) {},
			assertErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, storageError.UniqueViolationError)
			},
		},
		{
			name: "a driver error is propagated",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(req.Name, req.Color).WillReturnError(errStub)
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

			got, err := s.CreateTag(context.Background(), req)
			tt.assertErr(t, err)
			tt.assertResult(t, got)
		})
	}
}

// TestUpdateTag covers the rename/recolor, which re-reads the row after a successful write, the
// UNIQUE(name) violation (wrapped directly on the UPDATE, unlike account/category), and the
// not-found case.
func TestUpdateTag(t *testing.T) {
	t.Parallel()

	updateQuery := `UPDATE tags SET name = ?, color = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	selectQuery := `SELECT ` + tagColumns + ` FROM tags WHERE id = ?`

	req := model.TagUpdateRequest{ID: fakeID(), Name: fakeName(), Color: fakeColor()}
	wantRow := fakeTag()
	wantRow.ID = req.ID

	t.Run("updates and re-reads the row", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectExec(updateQuery).WithArgs(req.Name, req.Color, req.ID).WillReturnResult(sqlmock.NewResult(0, 1))

		rows := sqlmock.NewRows([]string{"id", "name", "color"})
		mock.ExpectQuery(selectQuery).WithArgs(req.ID).WillReturnRows(addTagRow(rows, wantRow))

		got, found, err := s.UpdateTag(context.Background(), req)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, wantRow, got)
	})

	t.Run("a taken name is a unique violation, from the update itself", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectExec(updateQuery).WithArgs(req.Name, req.Color, req.ID).WillReturnError(mysqlErr(errDuplicateEntry))

		_, found, err := s.UpdateTag(context.Background(), req)
		require.ErrorIs(t, err, storageError.UniqueViolationError)
		require.False(t, found)
	})

	t.Run("an unknown id is reported as not found, without a re-read", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectExec(updateQuery).WithArgs(req.Name, req.Color, req.ID).WillReturnResult(sqlmock.NewResult(0, 0))

		_, found, err := s.UpdateTag(context.Background(), req)
		require.NoError(t, err)
		require.False(t, found)
	})
}

// TestDeleteTag covers the delete. Unlike account/category/currency, there's no
// wrapForeignKey call here: a deleted tag's transaction_tags rows cascade away (ON DELETE CASCADE
// in the migrations), so there's no constraint to violate.
func TestDeleteTag(t *testing.T) {
	t.Parallel()

	query := `DELETE FROM tags WHERE id = ?`
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
