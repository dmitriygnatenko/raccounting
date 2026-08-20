package http

import (
	"net/http"

	"raccounting/internal/domain/usecase/account/create"
	"raccounting/internal/domain/usecase/account/delete"
	"raccounting/internal/domain/usecase/account/update"
)

type accountRequest struct {
	Name         string `json:"name"`
	Type         uint8  `json:"type"`
	CurrencyCode string `json:"currency"`
	Balance      int64  `json:"balance"`
	Archived     bool   `json:"archived"`
}

// handleListAccounts handles GET /api/accounts.
func (s *Server) handleListAccounts(w http.ResponseWriter, r *http.Request) {
	current := userFromContext(r)
	if current == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	out, err := s.Accounts.List.Execute(r.Context())
	if err != nil {
		writeUseCaseError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, out.Accounts)
}

// handleCreateAccount handles POST /api/accounts.
func (s *Server) handleCreateAccount(w http.ResponseWriter, r *http.Request) {
	current := userFromContext(r)
	if current == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var input accountRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	out, err := s.Accounts.Create.Execute(
		r.Context(),
		create.Input{
			Name:         input.Name,
			Type:         input.Type,
			CurrencyCode: input.CurrencyCode,
			Balance:      input.Balance,
		},
	)
	if err != nil {
		writeUseCaseError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, out.Account)
}

// handleUpdateAccount handles PUT /api/accounts/{id}.
func (s *Server) handleUpdateAccount(w http.ResponseWriter, r *http.Request) {
	current := userFromContext(r)
	if current == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	id, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}

	var input accountRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	out, err := s.Accounts.Update.Execute(
		r.Context(),
		update.Input{
			ID:           id,
			Name:         input.Name,
			Type:         input.Type,
			CurrencyCode: input.CurrencyCode,
			Archived:     input.Archived,
		},
	)
	if err != nil {
		writeUseCaseError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, out.Account)
}

// handleDeleteAccount handles DELETE /api/accounts/{id}.
func (s *Server) handleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	current := userFromContext(r)
	if current == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	id, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}

	if err := s.Accounts.Delete.Execute(
		r.Context(),
		delete.Input{
			ID: id,
		},
	); err != nil {
		writeUseCaseError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
