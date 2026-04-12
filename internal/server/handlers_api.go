package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/maxgarvey/mpc_editor/internal/pgm"
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

// handleAPIPrograms returns a JSON list of PGM files, most recently modified first.
func (s *Server) handleAPIPrograms(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	files, err := s.queries.ListFilesByType(ctx, "pgm")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type programEntry struct {
		Path    string `json:"path"`
		Name    string `json:"name"`
		Current bool   `json:"current"`
	}

	// Sort by mod_time descending (most recent first).
	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime > files[j].ModTime
	})

	// Determine the current session PGM relative path for marking.
	var currentRel string
	if s.session.FilePath != "" {
		if rel, err := filepath.Rel(s.session.WorkspacePath, s.session.FilePath); err == nil {
			currentRel = rel
		}
	}

	programs := make([]programEntry, 0, len(files))
	for _, f := range files {
		programs = append(programs, programEntry{
			Path:    f.Path,
			Name:    filepath.Base(f.Path),
			Current: f.Path == currentRel,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(programs)
}

// handleAPIProgramPads returns JSON pad info for a PGM file and bank.
func (s *Server) handleAPIProgramPads(w http.ResponseWriter, r *http.Request) {
	relPath := r.FormValue("path")
	if relPath == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}
	bank := parseIntParam(r, "bank", 0)
	if bank < 0 || bank > 3 {
		bank = 0
	}

	absPath := s.resolvePath(relPath)

	type padEntry struct {
		Index   int    `json:"index"`
		Display int    `json:"display"`
		Name    string `json:"name"`
		Layers  int    `json:"layers"`
	}

	// Check if this is the currently loaded program.
	var prog *pgm.Program
	if absPath == s.session.FilePath && s.session.Program != nil {
		prog = s.session.Program
	} else {
		var err error
		prog, err = pgm.OpenProgram(absPath)
		if err != nil {
			http.Error(w, fmt.Sprintf("open pgm: %v", err), http.StatusBadRequest)
			return
		}
	}

	start := bank * 16
	pads := make([]padEntry, 16)
	for i := range pads {
		idx := start + i
		pad := prog.Pad(idx)
		layerCount := 0
		name := ""
		for j := range 4 {
			sn := pad.Layer(j).GetSampleName()
			if sn != "" {
				layerCount++
				if name == "" {
					name = sn
				}
			}
		}
		pads[i] = padEntry{
			Index:   idx,
			Display: i + 1,
			Name:    name,
			Layers:  layerCount,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(pads)
}

// handleAPIAssignToProgram assigns a WAV sample to a pad in a specific PGM file.
func (s *Server) handleAPIAssignToProgram(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, fmt.Sprintf("parse form: %v", err), http.StatusBadRequest)
		return
	}

	pgmRel := r.FormValue("pgm_path")
	wavRel := r.FormValue("wav_path")
	padIdx := parseIntParam(r, "pad", 0)

	if pgmRel == "" || wavRel == "" {
		http.Error(w, "pgm_path and wav_path required", http.StatusBadRequest)
		return
	}
	if padIdx < 0 || padIdx >= 64 {
		http.Error(w, "pad must be 0-63", http.StatusBadRequest)
		return
	}

	pgmAbs := s.resolvePath(pgmRel)
	wavAbs := s.resolvePath(wavRel)
	sampleName := strings.TrimSuffix(filepath.Base(wavRel), filepath.Ext(wavRel))
	// MPC programs have a 16-character sample name limit.
	if len(sampleName) > 16 {
		sampleName = sampleName[:16]
		// Copy the WAV to the PGM's directory with the truncated name so
		// FindSampleInDirs can locate it.
		destPath := filepath.Join(filepath.Dir(pgmAbs), sampleName+".wav")
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			if err := copyFile(wavAbs, destPath); err != nil {
				log.Printf("assign-to-program: copy renamed sample: %v", err)
			}
		}
	}

	// Check if this is the session's current program.
	isSessionPgm := pgmAbs == s.session.FilePath && s.session.Program != nil

	var prog *pgm.Program
	if isSessionPgm {
		prog = s.session.Program
	} else {
		var err error
		prog, err = pgm.OpenProgram(pgmAbs)
		if err != nil {
			http.Error(w, fmt.Sprintf("open pgm: %v", err), http.StatusBadRequest)
			return
		}
	}

	// Find first empty layer, or use layer 0 if all occupied.
	pad := prog.Pad(padIdx)
	targetLayer := 0
	for j := range 4 {
		if pad.Layer(j).GetSampleName() == "" {
			targetLayer = j
			break
		}
		if j == 3 {
			// All layers full; overwrite layer 0.
			targetLayer = 0
		}
	}

	_ = pad.Layer(targetLayer).SetSampleName(sampleName)

	// Save PGM to disk.
	if err := prog.Save(pgmAbs); err != nil {
		http.Error(w, fmt.Sprintf("save pgm: %v", err), http.StatusInternalServerError)
		return
	}

	// Update session state so renderDetailPGM reuses it with the correct
	// SelectedPad and a fully populated Matrix.
	if isSessionPgm {
		ref := pgm.FindSampleInDirs(sampleName, s.session.SampleDir, s.session.WorkspacePath)
		s.session.Matrix.Set(padIdx, targetLayer, &ref)
		s.session.SelectedPad = padIdx
	} else {
		// Switch session to the target program.
		s.session.Program = prog
		s.session.FilePath = pgmAbs
		s.session.SampleDir = filepath.Dir(pgmAbs)
		s.session.SelectedPad = padIdx
		s.session.Matrix.Clear()
		for i := range 64 {
			pad := prog.Pad(i)
			for j := range 4 {
				name := pad.Layer(j).GetSampleName()
				if name != "" {
					ref := pgm.FindSampleInDirs(name, s.session.SampleDir, s.session.WorkspacePath)
					s.session.Matrix.Set(i, j, &ref)
				}
			}
		}
	}

	// Background rescan to keep DB current.
	go func() {
		if result, err := s.scanner.ScanWorkspace(s.session.WorkspacePath); err != nil {
			log.Printf("post-assign scan: %v", err)
		} else {
			log.Printf("post-assign scan: found=%d scanned=%d",
				result.FilesFound, result.FilesScanned)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"pad":%d,"layer":%d,"sample":%q}`, padIdx, targetLayer, sampleName)
}

// copyFile copies src to dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close() //nolint:errcheck // best-effort close on read-only file

	out, err := os.Create(dst)
	if err != nil {
		return err
	}

	_, err = io.Copy(out, in)
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	return err
}
