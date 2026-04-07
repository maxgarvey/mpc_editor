package server

import (
	"log"
	"net/http"
)

func (s *Server) handleDeviceStatus(w http.ResponseWriter, r *http.Request) {
	dev := s.detector.Current()
	s.renderTemplate(w, "device_status.html", map[string]any{
		"Device": dev,
	})
}

func (s *Server) handleDeviceDetect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	dev := s.detector.Scan()
	s.renderTemplate(w, "device_status.html", map[string]any{
		"Device": dev,
	})
}

func (s *Server) handleDeviceUseAsWorkspace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	dev := s.detector.Current()
	if dev == nil {
		http.Error(w, "no MPC device detected", http.StatusBadRequest)
		return
	}

	s.session.WorkspacePath = dev.MountPath
	s.session.Prefs.WorkspacePath = dev.MountPath
	if err := s.queries.UpdateWorkspacePath(r.Context(), dev.MountPath); err != nil {
		log.Printf("save workspace path: %v", err)
	}

	// Clear detail path — it refers to the old workspace.
	s.session.SelectedDetailPath = ""
	s.session.Prefs.LastDetailPath = ""
	_ = s.queries.UpdateLastDetailPath(r.Context(), "")

	// Re-scan in background.
	go func() {
		if result, err := s.scanner.ScanWorkspace(dev.MountPath); err != nil {
			log.Printf("device workspace scan: %v", err)
		} else {
			log.Printf("device workspace scan: found=%d scanned=%d removed=%d",
				result.FilesFound, result.FilesScanned, result.FilesRemoved)
		}
	}()

	w.Header().Set("HX-Redirect", "/")
	w.WriteHeader(http.StatusOK)
}
