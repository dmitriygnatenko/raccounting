package http

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriteJSON(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()

	writeJSON(rec, http.StatusCreated, map[string]int{"id": 7})

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))
	require.JSONEq(t, `{"id":7}`, rec.Body.String())
}

func TestWriteError(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()

	writeError(rec, http.StatusBadRequest, "boom")

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.JSONEq(t, `{"error":"boom"}`, rec.Body.String())
}

func TestDecodeJSON(t *testing.T) {
	t.Parallel()

	t.Run("valid body decodes into dst", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Cash"}`))
		rec := httptest.NewRecorder()

		var dst struct {
			Name string `json:"name"`
		}

		err := decodeJSON(rec, req, &dst)
		require.NoError(t, err)
		require.Equal(t, "Cash", dst.Name)
	})

	t.Run("malformed body returns an error", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{not json`))
		rec := httptest.NewRecorder()

		var dst map[string]any

		err := decodeJSON(rec, req, &dst)
		require.Error(t, err)
	})

	t.Run("body over maxJSONBodyBytes is rejected", func(t *testing.T) {
		t.Parallel()

		oversized := bytes.Repeat([]byte("a"), maxJSONBodyBytes+1)
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(oversized))
		rec := httptest.NewRecorder()

		var dst []byte

		err := decodeJSON(rec, req, &dst)
		require.Error(t, err)
	})
}

func TestParseIDParam(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		value  string
		wantID uint64
		wantOK bool
	}{
		{name: "valid positive id", value: "42", wantID: 42, wantOK: true},
		{name: "zero is rejected", value: "0", wantID: 0, wantOK: false},
		{name: "non-numeric is rejected", value: "abc", wantID: 0, wantOK: false},
		{name: "negative is rejected", value: "-1", wantID: 0, wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.SetPathValue("id", tt.value)
			rec := httptest.NewRecorder()

			id, ok := parseIDParam(rec, req, "id")

			require.Equal(t, tt.wantOK, ok)
			require.Equal(t, tt.wantID, id)

			if !tt.wantOK {
				require.Equal(t, http.StatusBadRequest, rec.Code)
				require.JSONEq(t, `{"error":"Invalid id"}`, rec.Body.String())
			}
		})
	}
}
