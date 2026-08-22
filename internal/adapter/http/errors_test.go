package http

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	domainerror "raccounting/internal/domain/error"
)

func TestWriteUseCaseError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{
			name:       "validation error maps to 400",
			err:        &domainerror.ValidationError{Message: "bad input"},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "not found error maps to 404",
			err:        &domainerror.NotFoundError{Message: "missing"},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "conflict error maps to 409",
			err:        &domainerror.ConflictError{Message: "conflict"},
			wantStatus: http.StatusConflict,
		},
		{
			name:       "unauthorized error maps to 401",
			err:        &domainerror.UnauthorizedError{Message: "nope"},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "unknown error maps to 500",
			err:        errors.New("something broke"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()

			writeUseCaseError(rec, tt.err)

			require.Equal(t, tt.wantStatus, rec.Code)
			require.JSONEq(t, `{"error":"`+tt.err.Error()+`"}`, rec.Body.String())
		})
	}
}
