package http

import (
	"net/http"

	"raccounting/internal/domain/usecase/category/create"
	"raccounting/internal/domain/usecase/category/delete"
	"raccounting/internal/domain/usecase/category/update"
)

type categoryRequest struct {
	Name     string `json:"name"`
	Type     uint8  `json:"type"`
	Color    string `json:"color"`
	Archived bool   `json:"archived"`
}

// handleListCategories handles GET /api/categories.
func (s *Server) handleListCategories(w http.ResponseWriter, r *http.Request) {
	current := userFromContext(r)
	if current == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	out, err := s.Categories.List.Execute(r.Context())
	if err != nil {
		writeUseCaseError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, out.Categories)
}

// handleCreateCategory handles POST /api/categories.
func (s *Server) handleCreateCategory(w http.ResponseWriter, r *http.Request) {
	current := userFromContext(r)
	if current == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var input categoryRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	out, err := s.Categories.Create.Execute(
		r.Context(),
		create.Input{
			Name:  input.Name,
			Type:  input.Type,
			Color: input.Color,
		},
	)
	if err != nil {
		writeUseCaseError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, out.Category)
}

// handleUpdateCategory handles PUT /api/categories/{id}.
func (s *Server) handleUpdateCategory(w http.ResponseWriter, r *http.Request) {
	current := userFromContext(r)
	if current == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	id, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}

	var input categoryRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	out, err := s.Categories.Update.Execute(
		r.Context(),
		update.Input{
			ID:       id,
			Name:     input.Name,
			Color:    input.Color,
			Archived: input.Archived,
		},
	)
	if err != nil {
		writeUseCaseError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, out.Category)
}

// handleDeleteCategory handles DELETE /api/categories/{id}.
func (s *Server) handleDeleteCategory(w http.ResponseWriter, r *http.Request) {
	current := userFromContext(r)
	if current == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	id, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}

	if err := s.Categories.Delete.Execute(
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
