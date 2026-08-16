package delete

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"raccounting/internal/domain/entity"
	domainerror "raccounting/internal/domain/error"
	"raccounting/internal/port"
)

// fakeAccountRepository is a hand-rolled test double for port.AccountRepository — only Delete is
// exercised here.
type fakeAccountRepository struct {
	deleteFn func(ctx context.Context, id uint64) error
}

func (f *fakeAccountRepository) List(context.Context) ([]entity.Account, error) {
	panic("not stubbed")
}

func (f *fakeAccountRepository) FindByID(context.Context, uint64) (entity.Account, error) {
	panic("not stubbed")
}

func (f *fakeAccountRepository) Create(context.Context, port.AccountCreateRequest) (entity.Account, error) {
	panic("not stubbed")
}

func (f *fakeAccountRepository) Update(context.Context, port.AccountUpdateRequest) (entity.Account, error) {
	panic("not stubbed")
}

func (f *fakeAccountRepository) Delete(ctx context.Context, id uint64) error {
	return f.deleteFn(ctx, id)
}

var errStub = errors.New("stub failure")

func TestUseCase_Execute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		mock      func(accounts *fakeAccountRepository) Input
		assertErr func(t *testing.T, err error)
	}{
		{
			name: "not found is reported as-is",
			mock: func(accounts *fakeAccountRepository) Input {
				accounts.deleteFn = func(context.Context, uint64) error {
					return &domainerror.NotFoundError{Message: "Account not found"}
				}

				return Input{ID: 99}
			},
			assertErr: func(t *testing.T, err error) {
				require.True(t, domainerror.IsNotFoundError(err))
			},
		},
		{
			name: "still in use is reported as a conflict",
			mock: func(accounts *fakeAccountRepository) Input {
				accounts.deleteFn = func(context.Context, uint64) error {
					return &domainerror.ConflictError{Message: "This account is in use — remove its transactions first"}
				}

				return Input{ID: 1}
			},
			assertErr: func(t *testing.T, err error) {
				require.True(t, domainerror.IsConflictError(err))
				require.EqualError(t, err, "This account is in use — remove its transactions first")
			},
		},
		{
			name: "unexpected storage error is wrapped in a generic message",
			mock: func(accounts *fakeAccountRepository) Input {
				accounts.deleteFn = func(context.Context, uint64) error { return errStub }

				return Input{ID: 1}
			},
			assertErr: func(t *testing.T, err error) {
				require.EqualError(t, err, "Failed to delete account")
			},
		},
		{
			name: "successful delete returns no error",
			mock: func(accounts *fakeAccountRepository) Input {
				accounts.deleteFn = func(_ context.Context, id uint64) error {
					require.Equal(t, uint64(2), id)
					return nil
				}

				return Input{ID: 2}
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			accounts := &fakeAccountRepository{}
			uc := New(accounts)
			input := tt.mock(accounts)

			err := uc.Execute(context.Background(), input)
			tt.assertErr(t, err)
		})
	}
}
