package server

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/maxgarvey/mpc_editor/internal/pgm"
)

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Build browse data for the left panel.
	browseData, err := s.buildBrowseData("", s.session.SelectedDetailPath)
	if err != nil {
		log.Printf("build browse data: %v", err)
		browseData = BrowseData{}
	}

	// Compute the relative path for the last viewed detail file so JS can auto-open it.
	var lastDetailRelPath string
	if s.session.SelectedDetailPath != "" && s.session.WorkspacePath != "" {
		if rel, err := filepath.Rel(s.session.WorkspacePath, s.session.SelectedDetailPath); err == nil {
			lastDetailRelPath = rel
		}
	}

	data := map[string]any{
		"Session":           s.session,
		"BrowseData":        browseData,
		"DetailHTML":        nil,
		"LastDetailRelPath": lastDetailRelPath,
		"Device":            s.detector.Current(),
	}
	s.renderTemplate(w, "layout.html", data)
}

func (s *Server) handleProgramNew(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.session.Program = pgm.NewProgram()
	s.session.FilePath = ""
	s.session.Matrix.Clear()
	s.session.SelectedPad = 0

	// Return full page via redirect for HTMX
	w.Header().Set("HX-Redirect", "/")
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleProjectNew(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "project name is required", http.StatusBadRequest)
		return
	}

	// Sanitize: reject path separators and special names.
	if strings.ContainsAny(name, `/\`) || name == "." || name == ".." {
		http.Error(w, "invalid project name", http.StatusBadRequest)
		return
	}

	// Enforce MPC 1000 16-char filename limit.
	if len(name) > 16 {
		http.Error(w, "name too long (max 16 characters for MPC compatibility)", http.StatusBadRequest)
		return
	}

	// Create project folder inside the current browse directory (or workspace root).
	parentDir := s.session.WorkspacePath
	if browseDir := r.FormValue("parent"); browseDir != "" {
		parentDir = s.resolvePath(browseDir)
	}
	projectDir := filepath.Join(parentDir, name)

	if err := s.validateWithinWorkspace(projectDir); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		http.Error(w, fmt.Sprintf("create project dir: %v", err), http.StatusInternalServerError)
		return
	}

	// Create and save a blank program inside the folder.
	prog := pgm.NewProgram()
	pgmPath := filepath.Join(projectDir, name+".pgm")
	if err := prog.Save(pgmPath); err != nil {
		http.Error(w, fmt.Sprintf("save program: %v", err), http.StatusInternalServerError)
		return
	}

	// Open the new program.
	s.session.Program = prog
	s.session.FilePath = pgmPath
	s.session.SampleDir = projectDir
	s.session.SelectedPad = 0
	s.session.Matrix.Clear()

	s.session.Prefs.LastPGMPath = pgmPath
	_ = s.queries.UpdateLastPGMPath(r.Context(), pgmPath)

	// Trigger a background scan to index the new project.
	go func() {
		if _, err := s.scanner.ScanWorkspace(s.session.WorkspacePath); err != nil {
			log.Printf("post-project-new scan: %v", err)
		}
	}()

	w.Header().Set("HX-Redirect", "/")
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleProgramOpen(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := s.resolvePath(r.FormValue("path"))
	if path == "" {
		http.Error(w, "path is required", http.StatusBadRequest)
		return
	}

	prog, err := pgm.OpenProgram(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.session.Program = prog
	s.session.FilePath = path
	s.session.SampleDir = filepath.Dir(path)
	s.session.SelectedPad = 0
	s.session.Matrix.Clear()

	s.session.Prefs.LastPGMPath = path
	if err := s.queries.UpdateLastPGMPath(r.Context(), path); err != nil {
		log.Printf("save last pgm path: %v", err)
	}

	// Populate sample matrix from program
	for i := 0; i < 64; i++ {
		pad := prog.Pad(i)
		for j := 0; j < 4; j++ {
			name := pad.Layer(j).GetSampleName()
			if name != "" {
				ref := pgm.FindSampleInDirs(name, s.session.SampleDir, s.session.WorkspacePath)
				s.session.Matrix.Set(i, j, &ref)
			}
		}
	}

	w.Header().Set("HX-Redirect", "/")
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleProgramSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := r.FormValue("path")
	if path != "" {
		path = s.resolvePath(path)
	} else {
		path = s.session.FilePath
	}
	if path == "" {
		http.Error(w, "no file path specified", http.StatusBadRequest)
		return
	}

	// Co-locate samples: copy any referenced samples into the same
	// directory as the .pgm so the MPC 1000 can find them.
	pgmDir := filepath.Dir(path)
	copied := s.colocateSamples(pgmDir)

	if err := s.session.Program.Save(path); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.session.FilePath = path
	s.session.SampleDir = pgmDir

	s.session.Prefs.LastPGMPath = path
	if err := s.queries.UpdateLastPGMPath(r.Context(), path); err != nil {
		log.Printf("save last pgm path: %v", err)
	}

	msg := "Saved to " + path
	if copied > 0 {
		msg += fmt.Sprintf(" (%d samples copied to project folder)", copied)
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(msg))
}

func (s *Server) handleSampleReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	prog := s.session.Program
	if prog == nil {
		http.Error(w, "no program loaded", http.StatusBadRequest)
		return
	}

	pgmPath := s.session.FilePath
	if pgmPath == "" {
		http.Error(w, "program has not been saved yet", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	workspace := s.session.WorkspacePath

	// Collect unique samples across all pads/layers.
	type sampleEntry struct {
		name  string
		pads  []string // e.g. "A01 layer 0", "A01 layer 1"
		found bool
	}
	seen := make(map[string]*sampleEntry)
	var order []string

	for padIdx := range 64 {
		pad := prog.Pad(padIdx)
		for layerIdx := range 4 {
			name := pad.Layer(layerIdx).GetSampleName()
			if name == "" {
				continue
			}
			bank := string(rune('A' + padIdx/16))
			padNum := fmt.Sprintf("%s%02d", bank, (padIdx%16)+1)
			label := fmt.Sprintf("%s layer %d", padNum, layerIdx)

			if entry, ok := seen[name]; ok {
				entry.pads = append(entry.pads, label)
			} else {
				ref := pgm.FindSampleInDirs(name, s.session.SampleDir, workspace)
				seen[name] = &sampleEntry{
					name:  name,
					pads:  []string{label},
					found: ref.FilePath != "",
				}
				order = append(order, name)
			}
		}
	}

	// Build the report.
	var b strings.Builder
	pgmName := filepath.Base(pgmPath)
	b.WriteString("Sample Report for " + pgmName + "\n")
	b.WriteString(strings.Repeat("=", 40+len(pgmName)) + "\n\n")

	if len(order) == 0 {
		b.WriteString("No samples used in this program.\n")
	}

	for i, name := range order {
		entry := seen[name]
		fmt.Fprintf(&b, "%d. %s\n", i+1, name)
		b.WriteString("   Pads: " + strings.Join(entry.pads, ", ") + "\n")

		if !entry.found {
			b.WriteString("   Status: NOT FOUND in workspace\n")
		} else {
			b.WriteString("   Status: found\n")
		}

		// Look up WAV metadata, source, and tags from the DB.
		wavFiles, err := s.queries.ListFilesByType(ctx, "wav")
		if err == nil {
			for _, f := range wavFiles {
				base := filepath.Base(f.Path)
				baseNoExt := strings.TrimSuffix(base, filepath.Ext(base))
				if !strings.EqualFold(baseNoExt, name) {
					continue
				}

				// WAV metadata
				if meta, err := s.queries.GetWavMeta(ctx, f.ID); err == nil {
					ch := "mono"
					if meta.Channels == 2 {
						ch = "stereo"
					}
					fmt.Fprintf(&b, "   Audio: %dHz %dbit %s",
						meta.SampleRate, meta.BitsPerSample, ch)
					if meta.SampleRate > 0 {
						dur := float64(meta.FrameCount) / float64(meta.SampleRate)
						fmt.Fprintf(&b, " (%.2fs)", dur)
					}
					b.WriteString("\n")
					if meta.Source != "" {
						b.WriteString("   Source: " + meta.Source + "\n")
					}
				}

				// Tags
				tags, _ := s.queries.ListFileTags(ctx, f.ID)
				if len(tags) > 0 {
					var tagStrs []string
					for _, t := range tags {
						if t.TagKey != "" {
							tagStrs = append(tagStrs, t.TagKey+":"+t.TagValue)
						} else {
							tagStrs = append(tagStrs, t.TagValue)
						}
					}
					b.WriteString("   Tags: " + strings.Join(tagStrs, ", ") + "\n")
				}

				// Programs using this sample
				programs, _ := s.queries.ListProgramsUsingSample(ctx, sql.NullInt64{Int64: f.ID, Valid: true})
				if len(programs) > 1 {
					var pgmNames []string
					for _, p := range programs {
						pgmNames = append(pgmNames, filepath.Base(p.Path))
					}
					b.WriteString("   Also used in: " + strings.Join(pgmNames, ", ") + "\n")
				}

				break
			}
		}

		b.WriteString("\n")
	}

	// Write the file next to the .pgm.
	txtPath := strings.TrimSuffix(pgmPath, filepath.Ext(pgmPath)) + "_samples.txt"
	if err := os.WriteFile(txtPath, []byte(b.String()), 0o644); err != nil {
		http.Error(w, "failed to write report: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("Report saved to " + filepath.Base(txtPath)))
}
