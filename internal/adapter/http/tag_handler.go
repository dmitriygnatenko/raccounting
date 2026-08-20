package http

import (
	"net/http"

	"raccounting/internal/domain/usecase/tag/create"
	"raccounting/internal/domain/usecase/tag/delete"
	"raccounting/internal/domain/usecase/tag/update"
)

type tagRequest struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

// handleListTags handles GET /api/tags.
func (s *Server) handleListTags(w http.ResponseWriter, r *http.Request) {
	current := userFromContext(r)
	if current == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	out, err := s.Tags.List.Execute(r.Context())
	if err != nil {
		writeUseCaseError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, out.Tags)
}

// handleCreateTag handles POST /api/tags.
func (s *Server) handleCreateTag(w http.ResponseWriter, r *http.Request) {
	current := userFromContext(r)
	if current == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var input tagRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	out, err := s.Tags.Create.Execute(
		r.Context(),
		create.Input{
			Name:  input.Name,
			Color: input.Color,
		},
	)
	if err != nil {
		writeUseCaseError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, out.Tag)
}

// handleUpdateTag handles PUT /api/tags/{id}.
func (s *Server) handleUpdateTag(w http.ResponseWriter, r *http.Request) {
	current := userFromContext(r)
	if current == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	id, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}

	var input tagRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	out, err := s.Tags.Update.Execute(
		r.Context(),
		update.Input{
			ID:    id,
			Name:  input.Name,
			Color: input.Color,
		},
	)
	if err != nil {
		writeUseCaseError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, out.Tag)
}

// handleDeleteTag handles DELETE /api/tags/{id}.
func (s *Server) handleDeleteTag(w http.ResponseWriter, r *http.Request) {
	current := userFromContext(r)
	if current == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	id, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}

	if err := s.Tags.Delete.Execute(
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
