package server

import (
	"encoding/json"
	"net/http"

	"github.com/maxgarvey/mpc_editor/internal/command"
)

func (s *Server) handleBatchPage(w http.ResponseWriter, r *http.Request) {
	s.renderTemplate(w, "batch_page.html", map[string]any{
		"Result": nil,
	})
}

func (s *Server) handleBatchRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	dir := r.FormValue("dir")
	if dir == "" {
		http.Error(w, "directory path is required", http.StatusBadRequest)
		return
	}

	result := command.BatchCreate(dir)

	// If request wants JSON (e.g., from fetch), return JSON
	if r.Header.Get("Accept") == "application/json" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"created":  result.Created,
			"skipped":  result.Skipped,
			"errors":   result.Errors,
			"programs": result.Programs,
			"report":   result.Report(),
		})
		return
	}

	// Otherwise return HTML partial
	s.renderTemplate(w, "batch_page.html", map[string]any{
		"Result": &result,
		"Dir":    dir,
	})
}
