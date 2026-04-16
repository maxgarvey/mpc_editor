package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/maxgarvey/mpc_editor/internal/audio"
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
	w.Header().Set("Cache-Control", "no-cache")
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

// handleAudioFile serves a WAV file from the workspace by relative path.
func (s *Server) handleAudioFile(w http.ResponseWriter, r *http.Request) {
	relPath := r.FormValue("path")
	if relPath == "" {
		http.Error(w, "path is required", http.StatusBadRequest)
		return
	}

	path := s.resolvePath(relPath)
	if path == "" {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	if !strings.HasSuffix(strings.ToLower(path), ".wav") {
		http.Error(w, "only WAV files supported", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFile(w, r, path)
}

// handleAudioWaveform returns peak data for waveform visualization of an arbitrary WAV file.
func (s *Server) handleAudioWaveform(w http.ResponseWriter, r *http.Request) {
	relPath := r.FormValue("path")
	if relPath == "" {
		http.Error(w, "path is required", http.StatusBadRequest)
		return
	}

	path := s.resolvePath(relPath)
	if path == "" {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	buckets := parseIntParam(r, "width", 1000)
	if buckets < 100 {
		buckets = 100
	}
	if buckets > 4000 {
		buckets = 4000
	}

	sample, err := audio.OpenWAV(path)
	if err != nil {
		http.Error(w, fmt.Sprintf("open WAV: %v", err), http.StatusBadRequest)
		return
	}

	channels := sample.AsSamples()
	var chPeaks [][]audio.Peak
	for _, ch := range channels {
		chPeaks = append(chPeaks, audio.DownsamplePeaks(ch, buckets))
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"channels":    chPeaks,
		"frameLength": sample.FrameLength,
	})
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

// handleAudioCrop crops a WAV file to the given frame range.
// POST /audio/crop?path=<relPath>&from=<frame>&to=<frame>&mode=replace|copy
func (s *Server) handleAudioCrop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	relPath := r.FormValue("path")
	if relPath == "" {
		http.Error(w, "path is required", http.StatusBadRequest)
		return
	}

	absPath := s.resolvePath(relPath)
	if absPath == "" {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	if err := s.validateWithinWorkspace(absPath); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	from := parseIntParam(r, "from", -1)
	to := parseIntParam(r, "to", -1)
	if from < 0 || to <= from {
		http.Error(w, "invalid frame range", http.StatusBadRequest)
		return
	}

	mode := r.FormValue("mode")
	if mode != "replace" && mode != "copy" {
		http.Error(w, "mode must be 'replace' or 'copy'", http.StatusBadRequest)
		return
	}

	sample, err := audio.OpenWAV(absPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("open WAV: %v", err), http.StatusBadRequest)
		return
	}

	if to > sample.FrameLength {
		to = sample.FrameLength
	}

	cropped := sample.SubRegion(from, to)

	var savePath string
	if mode == "replace" {
		savePath = absPath
	} else {
		// Generate a new filename: original_crop.wav
		ext := filepath.Ext(absPath)
		base := strings.TrimSuffix(absPath, ext)
		savePath = base + "_crop" + ext
		// Avoid overwriting existing crop files.
		for i := 2; ; i++ {
			if _, err := os.Stat(savePath); err != nil {
				break
			}
			savePath = fmt.Sprintf("%s_crop%d%s", base, i, ext)
		}
	}

	if err := cropped.SaveWAV(savePath); err != nil {
		http.Error(w, fmt.Sprintf("save cropped WAV: %v", err), http.StatusInternalServerError)
		return
	}

	// Rescan so the new/modified file is indexed before the client opens it.
	if _, err := s.scanner.ScanWorkspace(s.session.WorkspacePath); err != nil {
		log.Printf("post-crop scan: %v", err)
	}

	saveRel, _ := filepath.Rel(s.session.WorkspacePath, savePath)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("HX-Trigger", "refreshBrowser")
	json.NewEncoder(w).Encode(map[string]any{
		"path":    saveRel,
		"frames":  cropped.FrameLength,
		"mode":    mode,
		"message": fmt.Sprintf("Saved %d frames to %s", cropped.FrameLength, filepath.Base(savePath)),
	})
}
