package http

import (
	"net/http"

	"raccounting/internal/domain/usecase/currency/create"
	"raccounting/internal/domain/usecase/currency/delete"
	"raccounting/internal/domain/usecase/currency/update"
)

type currencyRequest struct {
	Code     string  `json:"code"`
	Symbol   string  `json:"symbol"`
	Name     string  `json:"name"`
	Rate     float64 `json:"rate"`
	Default  bool    `json:"default"`
	Archived bool    `json:"archived"`
}

// handleListCurrencies handles GET /api/currencies.
func (s *Server) handleListCurrencies(w http.ResponseWriter, r *http.Request) {
	current := userFromContext(r)
	if current == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	out, err := s.Currencies.List.Execute(r.Context())
	if err != nil {
		writeUseCaseError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, out.Currencies)
}

// handleCreateCurrency handles POST /api/currencies.
func (s *Server) handleCreateCurrency(w http.ResponseWriter, r *http.Request) {
	current := userFromContext(r)
	if current == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var input currencyRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	out, err := s.Currencies.Create.Execute(
		r.Context(),
		create.Input{
			Code:    input.Code,
			Symbol:  input.Symbol,
			Name:    input.Name,
			Rate:    input.Rate,
			Default: input.Default,
		},
	)
	if err != nil {
		writeUseCaseError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, out.Currency)
}

// handleUpdateCurrency handles PUT /api/currencies/{code}.
func (s *Server) handleUpdateCurrency(w http.ResponseWriter, r *http.Request) {
	current := userFromContext(r)
	if current == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	code := r.PathValue("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "Invalid currency code")
		return
	}

	var input currencyRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	out, err := s.Currencies.Update.Execute(
		r.Context(),
		update.Input{
			Code:     code,
			Symbol:   input.Symbol,
			Name:     input.Name,
			Rate:     input.Rate,
			Default:  input.Default,
			Archived: input.Archived,
		},
	)
	if err != nil {
		writeUseCaseError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, out.Currency)
}

// handleDeleteCurrency handles DELETE /api/currencies/{code}.
func (s *Server) handleDeleteCurrency(w http.ResponseWriter, r *http.Request) {
	current := userFromContext(r)
	if current == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	code := r.PathValue("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "Invalid currency code")
		return
	}

	if err := s.Currencies.Delete.Execute(
		r.Context(),
		delete.Input{
			Code: code,
		},
	); err != nil {
		writeUseCaseError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
