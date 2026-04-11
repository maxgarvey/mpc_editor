package server

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/maxgarvey/mpc_editor/internal/pgm"
)

func (s *Server) handleSettingsGet(w http.ResponseWriter, r *http.Request) {
	s.renderTemplate(w, "settings.html", map[string]any{
		"Session": s.session,
	})
}

func (s *Server) handleSettingsPost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	workspace := r.FormValue("workspace")
	if workspace != "" {
		absPath, err := filepath.Abs(workspace)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := os.MkdirAll(absPath, 0o755); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		s.session.WorkspacePath = absPath
		s.session.SampleDir = absPath
		s.session.Prefs.WorkspacePath = absPath
		if err := s.queries.UpdateWorkspacePath(r.Context(), absPath); err != nil {
			log.Printf("save workspace path: %v", err)
		}

		// Clear detail path — it refers to the old workspace.
		s.session.SelectedDetailPath = ""
		s.session.Prefs.LastDetailPath = ""
		_ = s.queries.UpdateLastDetailPath(r.Context(), "")

		// Re-scan in background.
		go func() {
			if result, err := s.scanner.ScanWorkspace(absPath); err != nil {
				log.Printf("settings workspace scan: %v", err)
			} else {
				log.Printf("settings workspace scan: found=%d scanned=%d removed=%d",
					result.FilesFound, result.FilesScanned, result.FilesRemoved)
			}
		}()
	}

	profile := r.FormValue("profile")
	if profile == "MPC500" || profile == "MPC1000" {
		if profile == "MPC500" {
			s.session.Profile = pgm.ProfileMPC500
		} else {
			s.session.Profile = pgm.ProfileMPC1000
		}
		s.session.Prefs.Profile = profile
		if err := s.queries.UpdateProfile(r.Context(), profile); err != nil {
			log.Printf("save profile: %v", err)
		}
	}

	w.Header().Set("HX-Redirect", "/")
	w.WriteHeader(http.StatusOK)
}
