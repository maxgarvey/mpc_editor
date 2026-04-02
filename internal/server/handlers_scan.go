package server

import (
	"encoding/json"
	"log"
	"net/http"
)

func (s *Server) handleWorkspaceScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	workspace := s.session.WorkspacePath
	if workspace == "" {
		http.Error(w, "no workspace configured", http.StatusBadRequest)
		return
	}

	result, err := s.scanner.ScanWorkspace(workspace)
	if err != nil {
		log.Printf("workspace scan: %v", err)
		http.Error(w, "scan failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("scan complete: found=%d scanned=%d removed=%d errors=%d",
		result.FilesFound, result.FilesScanned, result.FilesRemoved, len(result.Errors))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result) //nolint:errcheck // best-effort JSON response
}
