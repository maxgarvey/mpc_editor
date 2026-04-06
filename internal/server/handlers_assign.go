package server

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/maxgarvey/mpc_editor/internal/audio"
	"github.com/maxgarvey/mpc_editor/internal/command"
)

func (s *Server) handleAssign(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form (max 50MB)
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		http.Error(w, fmt.Sprintf("parse form: %v", err), http.StatusBadRequest)
		return
	}

	padIdx := parseIntParam(r, "pad", s.session.SelectedPad)
	if padIdx < 0 || padIdx >= 64 {
		http.Error(w, "invalid pad index", http.StatusBadRequest)
		return
	}

	mode := r.FormValue("mode")
	if mode == "" {
		mode = "per-pad"
	}

	// Save uploaded files to a temp directory
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		http.Error(w, "no files uploaded", http.StatusBadRequest)
		return
	}

	tmpDir, err := os.MkdirTemp("", "mpceditor-upload-*")
	if err != nil {
		http.Error(w, fmt.Sprintf("create temp dir: %v", err), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tmpDir) //nolint:errcheck // best-effort temp cleanup

	var savedPaths []string
	for _, fh := range files {
		src, err := fh.Open()
		if err != nil {
			continue
		}

		destPath := filepath.Join(tmpDir, fh.Filename)
		dst, err := os.Create(destPath)
		if err != nil {
			_ = src.Close()
			continue
		}
		_, cpErr := io.Copy(dst, src)
		_ = src.Close()
		_ = dst.Close()
		if cpErr != nil {
			continue
		}

		// Transcode non-WAV audio to WAV.
		ext := strings.ToLower(filepath.Ext(fh.Filename))
		if audio.IsTranscodable(ext) {
			origName := strings.TrimSuffix(fh.Filename, filepath.Ext(fh.Filename))
			wavPath, err := audio.TranscodeToWAV(destPath, tmpDir, origName)
			if err != nil {
				log.Printf("assign transcode %s: %v", fh.Filename, err)
				continue
			}
			destPath = wavPath
		}

		savedPaths = append(savedPaths, destPath)
	}

	if len(savedPaths) == 0 {
		http.Error(w, "no valid files saved", http.StatusBadRequest)
		return
	}

	// Import the files
	samples, result := command.ImportSamples(savedPaths)

	// Copy samples into workspace
	s.copySamplesToWorkspace(samples)

	// Determine assign mode
	assignMode := command.AssignPerPad
	if mode == "per-layer" {
		assignMode = command.AssignPerLayer
	}

	if mode == "multisample" {
		modified, warnings := command.MultisampleAssign(s.session.Program, &s.session.Matrix, samples)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"modified":%d,"warnings":%d,"report":%q}`,
			len(modified), len(warnings), result.Report())
	} else {
		modified := command.SimpleAssign(s.session.Program, &s.session.Matrix, samples, padIdx, assignMode)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"modified":%d,"report":%q}`, len(modified), result.Report())
	}
}

// handleAssignPath assigns samples from file paths (for local app usage without upload).
func (s *Server) handleAssignPath(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, fmt.Sprintf("parse form: %v", err), http.StatusBadRequest)
		return
	}

	padIdx := parseIntParam(r, "pad", s.session.SelectedPad)
	mode := r.FormValue("mode")
	if mode == "" {
		mode = "per-pad"
	}

	// Accept comma-separated paths or multiple "path" params
	var paths []string
	if pathList := r.FormValue("paths"); pathList != "" {
		for _, p := range strings.Split(pathList, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				paths = append(paths, p)
			}
		}
	}
	for _, p := range r.Form["path"] {
		p = strings.TrimSpace(p)
		if p != "" {
			paths = append(paths, p)
		}
	}

	if len(paths) == 0 {
		http.Error(w, "no paths provided", http.StatusBadRequest)
		return
	}

	// Resolve relative paths and transcode non-WAV audio.
	var resolvedPaths []string
	var tmpDirs []string
	for _, p := range paths {
		p = s.resolvePath(p)
		ext := strings.ToLower(filepath.Ext(p))
		if audio.IsTranscodable(ext) {
			tmpDir, err := os.MkdirTemp("", "mpceditor-transcode-*")
			if err != nil {
				log.Printf("assignpath transcode temp: %v", err)
				continue
			}
			tmpDirs = append(tmpDirs, tmpDir)
			wavPath, err := audio.TranscodeToWAV(p, tmpDir)
			if err != nil {
				log.Printf("assignpath transcode %s: %v", p, err)
				continue
			}
			p = wavPath
		}
		resolvedPaths = append(resolvedPaths, p)
	}
	defer func() {
		for _, d := range tmpDirs {
			_ = os.RemoveAll(d)
		}
	}()

	samples, _ := command.ImportSamples(resolvedPaths)

	// Copy samples into workspace
	s.copySamplesToWorkspace(samples)

	assignMode := command.AssignPerPad
	if mode == "per-layer" {
		assignMode = command.AssignPerLayer
	}

	if mode == "multisample" {
		command.MultisampleAssign(s.session.Program, &s.session.Matrix, samples)
	} else {
		command.SimpleAssign(s.session.Program, &s.session.Matrix, samples, padIdx, assignMode)
	}

	// Redirect to refresh the full page
	bank := padIdx / 16
	w.Header().Set("HX-Redirect", "/?bank="+strconv.Itoa(bank))
	w.WriteHeader(http.StatusOK)
}
