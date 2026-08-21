package tag

import (
	"context"
	"errors"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"raccounting/internal/domain/entity"
	domainerror "raccounting/internal/domain/error"
	"raccounting/internal/port"
	"raccounting/internal/repository/tag/mocks"
	storageError "raccounting/internal/storage/error"
	"raccounting/internal/storage/model"
)

// newRepo returns a Repository wired to a fresh MockStorage; any call a test doesn't stub via
// EXPECT() fails it, exactly like an unmet sqlmock expectation would in the adapter tests.
func newRepo(t *testing.T) (*Repository, *mocks.MockStorage) {
	t.Helper()

	mc := gomock.NewController(t)
	t.Cleanup(mc.Finish)

	m := mocks.NewMockStorage(mc)

	return New(m), m
}

// fakeID, fakeName and fakeColor are the field-shaped random values the tests below bind into mock
// expectations and returned rows, so a test failure is never masked by two cases accidentally
// sharing a fixture value.
func fakeID() uint64    { return uint64(gofakeit.Number(1, 1_000_000)) }
func fakeName() string  { return gofakeit.Word() }
func fakeColor() string { return gofakeit.HexColor() }

func fakeTagModel() model.Tag {
	return model.Tag{
		ID:    fakeID(),
		Name:  fakeName(),
		Color: fakeColor(),
	}
}

// errStub is the sentinel a case uses when it only cares that an error travels through untouched.
var errStub = errors.New(gofakeit.Sentence())

// TestRepository_List covers the row -> entity conversion and plain error propagation.
func TestRepository_List(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		mock         func(m *mocks.MockStorage) []entity.Tag
		assertResult func(t *testing.T, want, got []entity.Tag)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "converts every row",
			mock: func(m *mocks.MockStorage) []entity.Tag {
				rows := []model.Tag{fakeTagModel(), fakeTagModel()}
				m.EXPECT().ListTags(context.Background()).Return(rows, nil)

				want := make([]entity.Tag, len(rows))
				for i, row := range rows {
					want[i] = row.ToEntity()
				}

				return want
			},
			assertResult: func(t *testing.T, want, got []entity.Tag) { require.Equal(t, want, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a storage error is propagated",
			mock: func(m *mocks.MockStorage) []entity.Tag {
				m.EXPECT().ListTags(context.Background()).Return(nil, errStub)

				return nil
			},
			assertResult: func(t *testing.T, want, got []entity.Tag) { require.Nil(t, got) },
			assertErr:    func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r, m := newRepo(t)
			want := tt.mock(m)

			got, err := r.List(context.Background())
			tt.assertErr(t, err)
			tt.assertResult(t, want, got)
		})
	}
}

// TestRepository_FindByIDs covers the row -> entity conversion and plain error propagation.
func TestRepository_FindByIDs(t *testing.T) {
	t.Parallel()

	ids := []uint64{fakeID(), fakeID()}

	tests := []struct {
		name         string
		mock         func(m *mocks.MockStorage) []entity.Tag
		assertResult func(t *testing.T, want, got []entity.Tag)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "converts every matching row",
			mock: func(m *mocks.MockStorage) []entity.Tag {
				rows := []model.Tag{fakeTagModel()}
				m.EXPECT().FindTagsByIDs(context.Background(), ids).Return(rows, nil)

				want := make([]entity.Tag, len(rows))
				for i, row := range rows {
					want[i] = row.ToEntity()
				}

				return want
			},
			assertResult: func(t *testing.T, want, got []entity.Tag) { require.Equal(t, want, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a storage error is propagated",
			mock: func(m *mocks.MockStorage) []entity.Tag {
				m.EXPECT().FindTagsByIDs(context.Background(), ids).Return(nil, errStub)

				return nil
			},
			assertResult: func(t *testing.T, want, got []entity.Tag) { require.Nil(t, got) },
			assertErr:    func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r, m := newRepo(t)
			want := tt.mock(m)

			got, err := r.FindByIDs(context.Background(), ids)
			tt.assertErr(t, err)
			tt.assertResult(t, want, got)
		})
	}
}

// TestRepository_Create covers the insert and the unique-name -> ConflictError translation.
func TestRepository_Create(t *testing.T) {
	t.Parallel()

	req := port.TagCreateRequest{
		Name:  fakeName(),
		Color: fakeColor(),
	}
	storageReq := model.TagCreateRequest{Name: req.Name, Color: req.Color}

	tests := []struct {
		name         string
		mock         func(m *mocks.MockStorage) entity.Tag
		assertResult func(t *testing.T, want, got entity.Tag)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "stores the tag and returns it",
			mock: func(m *mocks.MockStorage) entity.Tag {
				id := fakeID()
				m.EXPECT().CreateTag(context.Background(), storageReq).Return(id, nil)

				return entity.Tag{ID: id, Name: req.Name, Color: req.Color}
			},
			assertResult: func(t *testing.T, want, got entity.Tag) { require.Equal(t, want, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a name collision becomes a message-less ConflictError",
			mock: func(m *mocks.MockStorage) entity.Tag {
				m.EXPECT().CreateTag(context.Background(), storageReq).
					Return(uint64(0), storageError.UniqueViolationError)

				return entity.Tag{}
			},
			assertResult: func(t *testing.T, want, got entity.Tag) {},
			assertErr: func(t *testing.T, err error) {
				var conflict *domainerror.ConflictError
				require.ErrorAs(t, err, &conflict)
				require.Empty(t, conflict.Message)
			},
		},
		{
			name: "any other storage error is propagated",
			mock: func(m *mocks.MockStorage) entity.Tag {
				m.EXPECT().CreateTag(context.Background(), storageReq).Return(uint64(0), errStub)

				return entity.Tag{}
			},
			assertResult: func(t *testing.T, want, got entity.Tag) {},
			assertErr:    func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r, m := newRepo(t)
			want := tt.mock(m)

			got, err := r.Create(context.Background(), req)
			tt.assertErr(t, err)
			tt.assertResult(t, want, got)
		})
	}
}

// TestRepository_Update covers the rewrite, the found -> NotFoundError translation, and the
// unique-name -> ConflictError translation.
func TestRepository_Update(t *testing.T) {
	t.Parallel()

	req := port.TagUpdateRequest{
		ID:    fakeID(),
		Name:  fakeName(),
		Color: fakeColor(),
	}
	storageReq := model.TagUpdateRequest{ID: req.ID, Name: req.Name, Color: req.Color}

	tests := []struct {
		name         string
		mock         func(m *mocks.MockStorage) entity.Tag
		assertResult func(t *testing.T, want, got entity.Tag)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "updates the tag",
			mock: func(m *mocks.MockStorage) entity.Tag {
				row := fakeTagModel()
				row.ID = req.ID
				m.EXPECT().UpdateTag(context.Background(), storageReq).Return(row, true, nil)

				return row.ToEntity()
			},
			assertResult: func(t *testing.T, want, got entity.Tag) { require.Equal(t, want, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "an unknown id becomes a message-less NotFoundError",
			mock: func(m *mocks.MockStorage) entity.Tag {
				m.EXPECT().UpdateTag(context.Background(), storageReq).Return(model.Tag{}, false, nil)

				return entity.Tag{}
			},
			assertResult: func(t *testing.T, want, got entity.Tag) {},
			assertErr: func(t *testing.T, err error) {
				var notFound *domainerror.NotFoundError
				require.ErrorAs(t, err, &notFound)
				require.Empty(t, notFound.Message)
			},
		},
		{
			name: "a name collision becomes a message-less ConflictError",
			mock: func(m *mocks.MockStorage) entity.Tag {
				m.EXPECT().UpdateTag(context.Background(), storageReq).
					Return(model.Tag{}, false, storageError.UniqueViolationError)

				return entity.Tag{}
			},
			assertResult: func(t *testing.T, want, got entity.Tag) {},
			assertErr: func(t *testing.T, err error) {
				var conflict *domainerror.ConflictError
				require.ErrorAs(t, err, &conflict)
				require.Empty(t, conflict.Message)
			},
		},
		{
			name: "any other storage error is propagated",
			mock: func(m *mocks.MockStorage) entity.Tag {
				m.EXPECT().UpdateTag(context.Background(), storageReq).Return(model.Tag{}, false, errStub)

				return entity.Tag{}
			},
			assertResult: func(t *testing.T, want, got entity.Tag) {},
			assertErr:    func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r, m := newRepo(t)
			want := tt.mock(m)

			got, err := r.Update(context.Background(), req)
			tt.assertErr(t, err)
			tt.assertResult(t, want, got)
		})
	}
}

// TestRepository_Delete covers the removal and the found=false -> NotFoundError translation.
func TestRepository_Delete(t *testing.T) {
	t.Parallel()

	id := fakeID()

	tests := []struct {
		name      string
		mock      func(m *mocks.MockStorage)
		assertErr func(t *testing.T, err error)
	}{
		{
			name: "deletes the tag",
			mock: func(m *mocks.MockStorage) {
				m.EXPECT().DeleteTag(context.Background(), id).Return(true, nil)
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "an unknown id becomes a message-less NotFoundError",
			mock: func(m *mocks.MockStorage) {
				m.EXPECT().DeleteTag(context.Background(), id).Return(false, nil)
			},
			assertErr: func(t *testing.T, err error) {
				var notFound *domainerror.NotFoundError
				require.ErrorAs(t, err, &notFound)
				require.Empty(t, notFound.Message)
			},
		},
		{
			name: "any other storage error is propagated",
			mock: func(m *mocks.MockStorage) {
				m.EXPECT().DeleteTag(context.Background(), id).Return(false, errStub)
			},
			assertErr: func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r, m := newRepo(t)
			tt.mock(m)

			err := r.Delete(context.Background(), id)
			tt.assertErr(t, err)
		})
	}
}
