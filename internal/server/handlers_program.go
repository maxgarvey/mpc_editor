package server

import (
	"log"
	"net/http"
	"path/filepath"

	"github.com/maxgarvey/mpc_editor/internal/pgm"
)

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	bank := parseIntParam(r, "bank", s.session.SelectedPad/16)

	data := map[string]any{
		"Session":   s.session,
		"PadGrid":   s.padGridData(bank),
		"PadParams": s.padParamsData(),
		"Banks":     []int{0, 1, 2, 3},
		"Bank":      bank,
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

	if err := s.session.Program.Save(path); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.session.FilePath = path

	s.session.Prefs.LastPGMPath = path
	if err := s.queries.UpdateLastPGMPath(r.Context(), path); err != nil {
		log.Printf("save last pgm path: %v", err)
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("Saved to " + path))
}
