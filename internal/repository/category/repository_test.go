package category

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
	"raccounting/internal/repository/category/mocks"
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

func fakeCategoryModel() model.Category {
	return model.Category{
		ID:     fakeID(),
		Name:   fakeName(),
		Color:  fakeColor(),
		Type:   uint8(entity.CategoryTypeExpense),
		Status: uint8(entity.CategoryStatusActive),
	}
}

// errStub is the sentinel a case uses when it only cares that an error travels through untouched.
var errStub = errors.New(gofakeit.Sentence())

// TestRepository_List covers the row -> entity conversion and plain error propagation.
func TestRepository_List(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		mock         func(m *mocks.MockStorage) []entity.Category
		assertResult func(t *testing.T, want, got []entity.Category)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "converts every row",
			mock: func(m *mocks.MockStorage) []entity.Category {
				rows := []model.Category{fakeCategoryModel(), fakeCategoryModel()}
				m.EXPECT().ListCategories(context.Background()).Return(rows, nil)

				want := make([]entity.Category, len(rows))
				for i, row := range rows {
					want[i] = row.ToEntity()
				}

				return want
			},
			assertResult: func(t *testing.T, want, got []entity.Category) { require.Equal(t, want, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a storage error is propagated",
			mock: func(m *mocks.MockStorage) []entity.Category {
				m.EXPECT().ListCategories(context.Background()).Return(nil, errStub)

				return nil
			},
			assertResult: func(t *testing.T, want, got []entity.Category) { require.Nil(t, got) },
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

// TestRepository_Exists covers the plain delegation to storage.
func TestRepository_Exists(t *testing.T) {
	t.Parallel()

	id := fakeID()

	tests := []struct {
		name         string
		mock         func(m *mocks.MockStorage)
		assertResult func(t *testing.T, got bool)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "delegates to storage",
			mock: func(m *mocks.MockStorage) {
				m.EXPECT().ExistsCategory(context.Background(), id).Return(true, nil)
			},
			assertResult: func(t *testing.T, got bool) { require.True(t, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a storage error is propagated",
			mock: func(m *mocks.MockStorage) {
				m.EXPECT().ExistsCategory(context.Background(), id).Return(false, errStub)
			},
			assertResult: func(t *testing.T, got bool) { require.False(t, got) },
			assertErr:    func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r, m := newRepo(t)
			tt.mock(m)

			got, err := r.Exists(context.Background(), id)
			tt.assertErr(t, err)
			tt.assertResult(t, got)
		})
	}
}

// TestRepository_Create covers the insert and its plain error propagation — category creation has
// no error type of its own to translate.
func TestRepository_Create(t *testing.T) {
	t.Parallel()

	req := port.CategoryCreateRequest{
		Name:  fakeName(),
		Type:  entity.CategoryTypeExpense,
		Color: fakeColor(),
	}

	tests := []struct {
		name         string
		mock         func(m *mocks.MockStorage) entity.Category
		assertResult func(t *testing.T, want, got entity.Category)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "stores the category and returns it",
			mock: func(m *mocks.MockStorage) entity.Category {
				id := fakeID()
				m.EXPECT().CreateCategory(context.Background(), req).Return(id, nil)

				return entity.Category{
					ID:     id,
					Name:   req.Name,
					Type:   req.Type,
					Color:  req.Color,
					Status: entity.CategoryStatusActive,
				}
			},
			assertResult: func(t *testing.T, want, got entity.Category) { require.Equal(t, want, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a storage error is propagated",
			mock: func(m *mocks.MockStorage) entity.Category {
				m.EXPECT().CreateCategory(context.Background(), req).Return(uint64(0), errStub)

				return entity.Category{}
			},
			assertResult: func(t *testing.T, want, got entity.Category) {},
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

// TestRepository_Update covers the rewrite and the found -> NotFoundError translation.
func TestRepository_Update(t *testing.T) {
	t.Parallel()

	req := port.CategoryUpdateRequest{
		ID:       fakeID(),
		Name:     fakeName(),
		Color:    fakeColor(),
		Archived: gofakeit.Bool(),
	}

	tests := []struct {
		name         string
		mock         func(m *mocks.MockStorage) entity.Category
		assertResult func(t *testing.T, want, got entity.Category)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "updates the category",
			mock: func(m *mocks.MockStorage) entity.Category {
				row := fakeCategoryModel()
				row.ID = req.ID
				m.EXPECT().UpdateCategory(context.Background(), req).Return(row, true, nil)

				return row.ToEntity()
			},
			assertResult: func(t *testing.T, want, got entity.Category) { require.Equal(t, want, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "an unknown id becomes a message-less NotFoundError",
			mock: func(m *mocks.MockStorage) entity.Category {
				m.EXPECT().UpdateCategory(context.Background(), req).Return(model.Category{}, false, nil)

				return entity.Category{}
			},
			assertResult: func(t *testing.T, want, got entity.Category) {},
			assertErr: func(t *testing.T, err error) {
				var notFound *domainerror.NotFoundError
				require.ErrorAs(t, err, &notFound)
				require.Empty(t, notFound.Message)
			},
		},
		{
			name: "a storage error is propagated",
			mock: func(m *mocks.MockStorage) entity.Category {
				m.EXPECT().UpdateCategory(context.Background(), req).Return(model.Category{}, false, errStub)

				return entity.Category{}
			},
			assertResult: func(t *testing.T, want, got entity.Category) {},
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

// TestRepository_Delete covers the removal and both error translations: FK violation -> ConflictError,
// found=false -> NotFoundError.
func TestRepository_Delete(t *testing.T) {
	t.Parallel()

	id := fakeID()

	tests := []struct {
		name      string
		mock      func(m *mocks.MockStorage)
		assertErr func(t *testing.T, err error)
	}{
		{
			name: "deletes the category",
			mock: func(m *mocks.MockStorage) {
				m.EXPECT().DeleteCategory(context.Background(), id).Return(true, nil)
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a foreign key violation becomes a message-less ConflictError",
			mock: func(m *mocks.MockStorage) {
				m.EXPECT().DeleteCategory(context.Background(), id).
					Return(false, storageError.ForeignKeyViolationError)
			},
			assertErr: func(t *testing.T, err error) {
				var conflict *domainerror.ConflictError
				require.ErrorAs(t, err, &conflict)
				require.Empty(t, conflict.Message)
			},
		},
		{
			name: "an unknown id becomes a message-less NotFoundError",
			mock: func(m *mocks.MockStorage) {
				m.EXPECT().DeleteCategory(context.Background(), id).Return(false, nil)
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
				m.EXPECT().DeleteCategory(context.Background(), id).Return(false, errStub)
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
