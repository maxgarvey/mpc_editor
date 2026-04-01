package server

import (
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"strconv"
)

// Server is the HTTP server for the MPC Editor web application.
type Server struct {
	session   *Session
	templates *template.Template
	mux       *http.ServeMux
	staticFS  fs.FS
}

// New creates a new Server with the given embedded filesystem for templates and static assets.
func New(templateFS, staticFS fs.FS) *Server {
	s := &Server{
		session:  NewSession(),
		mux:      http.NewServeMux(),
		staticFS: staticFS,
	}

	funcMap := template.FuncMap{
		"seq": func(n int) []int {
			s := make([]int, n)
			for i := range s {
				s[i] = i
			}
			return s
		},
		"add": func(a, b int) int { return a + b },
		"mul": func(a, b int) int { return a * b },
		"padBankLabel": func(bank int) string {
			return string(rune('A' + bank))
		},
		"padDisplayIndex": func(padIndex int) int {
			return (padIndex % 16) + 1
		},
	}

	tmpl, err := template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/*.html", "templates/partials/*.html")
	if err != nil {
		log.Fatalf("parse templates: %v", err)
	}
	s.templates = tmpl

	s.registerRoutes()
	return s
}

// Handler returns the HTTP handler for the server.
func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) registerRoutes() {
	// Static files
	s.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(s.staticFS))))

	// Main page
	s.mux.HandleFunc("/", s.handleIndex)

	// Program operations
	s.mux.HandleFunc("/program/new", s.handleProgramNew)
	s.mux.HandleFunc("/program/open", s.handleProgramOpen)
	s.mux.HandleFunc("/program/save", s.handleProgramSave)

	// Pad operations
	s.mux.HandleFunc("/pad/", s.handlePadSelect)
	s.mux.HandleFunc("/pad/params", s.handlePadParams)
	s.mux.HandleFunc("/pad/layer/", s.handleLayerUpdate)

	// File assignment
	s.mux.HandleFunc("/assign/upload", s.handleAssign)
	s.mux.HandleFunc("/assign/path", s.handleAssignPath)

	// Audio playback
	s.mux.HandleFunc("/audio/pad/", s.handleAudioPad)
	s.mux.HandleFunc("/audio/slice/", s.handleAudioSlice)
	s.mux.HandleFunc("/audio/info", s.handleAudioInfo)

	// Slicer
	s.mux.HandleFunc("/slicer", s.handleSlicerPage)
	s.mux.HandleFunc("/slicer/load", s.handleSlicerLoad)
	s.mux.HandleFunc("/slicer/waveform", s.handleSlicerWaveform)
	s.mux.HandleFunc("/slicer/sensitivity", s.handleSlicerSensitivity)
	s.mux.HandleFunc("/slicer/marker/", s.handleSlicerMarker)
	s.mux.HandleFunc("/slicer/export", s.handleSlicerExport)

	// Edit operations
	s.mux.HandleFunc("/edit/remove-all-samples", s.handleRemoveAllSamples)
	s.mux.HandleFunc("/edit/chromatic-layout", s.handleChromaticLayout)
	s.mux.HandleFunc("/edit/copy-settings-to-all", s.handleCopySettingsToAll)
	s.mux.HandleFunc("/edit/profile", s.handleProfileSwitch)

	// Batch
	s.mux.HandleFunc("/batch", s.handleBatchPage)
	s.mux.HandleFunc("/batch/run", s.handleBatchRun)

	// Pad grid partial
	s.mux.HandleFunc("/partials/pad-grid", s.handlePadGrid)
	s.mux.HandleFunc("/partials/pad-params", s.handlePadParamsPartial)
}

func (s *Server) renderTemplate(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), 500)
		log.Printf("template %s: %v", name, err)
	}
}

func parseIntParam(r *http.Request, key string, defaultVal int) int {
	v := r.FormValue(key)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return n
}
