package http

import (
	"net/http"

	"raccounting/internal/domain/usecase/settings/update"
)

// handleGetSettings handles GET /api/settings.
func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	current := userFromContext(r)
	if current == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	settings, err := s.Settings.Get.Execute(r.Context(), current.ID)
	if err != nil {
		writeUseCaseError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, settings)
}

type updateSettingsRequest struct {
	Language string `json:"language"`
}

// handleUpdateSettings handles PATCH /api/settings.
func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	current := userFromContext(r)
	if current == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var input updateSettingsRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	out, err := s.Settings.Update.Execute(
		r.Context(),
		update.Input{
			UserID:   current.ID,
			Language: input.Language,
		},
	)
	if err != nil {
		writeUseCaseError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, out.Settings)
}
