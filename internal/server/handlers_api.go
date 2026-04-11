package server

import (
	"encoding/json"
	"net/http"
)

// handleAPISamples returns a JSON list of WAV file paths in the workspace.
func (s *Server) handleAPISamples(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	files, err := s.queries.ListFilesByType(ctx, "wav")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type sampleEntry struct {
		Path string `json:"path"`
		Name string `json:"name"`
	}

	samples := make([]sampleEntry, 0, len(files))
	for _, f := range files {
		samples = append(samples, sampleEntry{
			Path: f.Path,
			Name: f.Path,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(samples)
}
