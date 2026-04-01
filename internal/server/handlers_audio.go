package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func (s *Server) handleAudioPad(w http.ResponseWriter, r *http.Request) {
	// URL: /audio/pad/{padIndex}/{layerIndex}
	path := strings.TrimPrefix(r.URL.Path, "/audio/pad/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		http.Error(w, "usage: /audio/pad/{pad}/{layer}", http.StatusBadRequest)
		return
	}

	padIdx, err := strconv.Atoi(parts[0])
	if err != nil || padIdx < 0 || padIdx >= 64 {
		http.Error(w, "invalid pad index", http.StatusBadRequest)
		return
	}
	layerIdx, err := strconv.Atoi(parts[1])
	if err != nil || layerIdx < 0 || layerIdx >= 4 {
		http.Error(w, "invalid layer index", http.StatusBadRequest)
		return
	}

	ref := s.session.Matrix.Get(padIdx, layerIdx)
	if ref == nil || ref.FilePath == "" {
		http.Error(w, "no sample loaded", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("Cache-Control", "max-age=300")
	http.ServeFile(w, r, ref.FilePath)
}

func (s *Server) handleAudioSlice(w http.ResponseWriter, r *http.Request) {
	// URL: /audio/slice/{index}
	path := strings.TrimPrefix(r.URL.Path, "/audio/slice/")
	idx, err := strconv.Atoi(strings.TrimRight(path, "/"))
	if err != nil || idx < 0 {
		http.Error(w, "invalid slice index", http.StatusBadRequest)
		return
	}

	if s.session.Slicer == nil {
		http.Error(w, "no slicer active", http.StatusNotFound)
		return
	}

	if idx >= s.session.Slicer.Markers.Size() {
		http.Error(w, "slice index out of range", http.StatusBadRequest)
		return
	}

	slice := s.session.Slicer.GetSlice(idx)

	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"slice_%d.wav\"", idx))
	if err := slice.WriteWAV(w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleAudioInfo returns JSON with which pads have audio available.
func (s *Server) handleAudioInfo(w http.ResponseWriter, r *http.Request) {
	type padInfo struct {
		Pad   int `json:"pad"`
		Layer int `json:"layer"`
	}

	var pads []padInfo
	for i := 0; i < 64; i++ {
		for j := 0; j < 4; j++ {
			ref := s.session.Matrix.Get(i, j)
			if ref != nil && ref.FilePath != "" {
				pads = append(pads, padInfo{Pad: i, Layer: j})
				break // only report first available layer per pad
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"pads": pads})
}
