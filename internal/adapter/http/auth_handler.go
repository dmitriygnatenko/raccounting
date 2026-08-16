package http

import (
	"net/http"

	"raccounting/internal/domain/usecase/auth/login"
	"raccounting/internal/domain/usecase/auth/logout"
	"raccounting/internal/domain/usecase/auth/updatecredentials"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Language string `json:"language"`
}

// handleLogin handles POST /api/auth/login. Raccounting is single-user: the first successful call
// against an empty database auto-provisions the one account.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var input loginRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	out, err := s.Auth.Login.Execute(
		r.Context(),
		login.Input{
			Username: input.Username,
			Password: input.Password,
			Language: input.Language,
		},
	)
	if err != nil {
		writeUseCaseError(w, err)
		return
	}

	s.setSessionCookie(w, out.Session)

	writeJSON(w, http.StatusOK, out.User)
}

// handleMe handles GET /api/auth/me — reports the current session's user. requireAuth has already
// verified the session and attached the user to the request context, so this just returns it.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	current := userFromContext(r)
	if current == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	writeJSON(w, http.StatusOK, current)
}

// handleLogout handles POST /api/auth/logout.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.Auth.Logout.Execute(
		r.Context(),
		logout.Input{
			Token: sessionToken(r),
		},
	)

	s.clearSessionCookie(w)

	w.WriteHeader(http.StatusNoContent)
}

type updateCredentialsRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewUsername     string `json:"newUsername"`
	NewPassword     string `json:"newPassword"`
}

// handleUpdateCredentials handles PATCH /api/auth/credentials.
func (s *Server) handleUpdateCredentials(w http.ResponseWriter, r *http.Request) {
	current := userFromContext(r)
	if current == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var input updateCredentialsRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	out, err := s.Auth.UpdateCredentials.Execute(
		r.Context(),
		updatecredentials.Input{
			User:            *current,
			CurrentPassword: input.CurrentPassword,
			NewUsername:     input.NewUsername,
			NewPassword:     input.NewPassword,
		},
	)
	if err != nil {
		writeUseCaseError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, out.User)
}
