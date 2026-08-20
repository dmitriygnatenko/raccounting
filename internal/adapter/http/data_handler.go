package http

import (
	"encoding/json"
	"fmt"
	"net/http"

	"raccounting/internal/domain/entity"
	"raccounting/internal/domain/usecase/data/restore"
)

// maxBackupBodyBytes caps a POST /api/data/import request body — much larger than
// maxJSONBodyBytes since a backup carries every transaction the user has ever recorded, not one
// form submission.
const maxBackupBodyBytes = 64 * 1024 * 1024

// handleExportData handles GET /api/data/export — it streams the signed-in user's full
// entity.Backup as a downloadable JSON file.
func (s *Server) handleExportData(w http.ResponseWriter, r *http.Request) {
	current := userFromContext(r)
	if current == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	backup, err := s.Data.Export.Execute(r.Context(), current.ID)
	if err != nil {
		writeUseCaseError(w, err)
		return
	}

	filename := fmt.Sprintf("raccounting-backup-%s.json", backup.ExportedAt.Format("2006-01-02"))

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.WriteHeader(http.StatusOK)

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(backup)
}

// handleImportData handles POST /api/data/import — it replaces every table raccounting stores with
// the uploaded entity.Backup. See the restore use case for why this can't be undone.
func (s *Server) handleImportData(w http.ResponseWriter, r *http.Request) {
	current := userFromContext(r)
	if current == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBackupBodyBytes)
	defer r.Body.Close()

	var backup entity.Backup
	if err := json.NewDecoder(r.Body).Decode(&backup); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid backup file")
		return
	}

	if err := s.Data.Restore.Execute(r.Context(), restore.Input{
		UserID: current.ID,
		Backup: backup,
	}); err != nil {
		writeUseCaseError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
