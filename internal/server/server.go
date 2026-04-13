package server

import (
	"context"
	"database/sql"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/maxgarvey/mpc_editor/internal/db"
	"github.com/maxgarvey/mpc_editor/internal/device"
	"github.com/maxgarvey/mpc_editor/internal/scanner"
)

// Server is the HTTP server for the MPC Editor web application.
type Server struct {
	session   *Session
	queries   *db.Queries
	scanner   *scanner.Scanner
	detector  *device.Detector
	templates *template.Template
	mux       *http.ServeMux
	staticFS  fs.FS
}

// New creates a new Server with the given embedded filesystem for templates and static assets.
func New(templateFS, staticFS fs.FS, sqlDB *sql.DB, queries *db.Queries) *Server {
	s := &Server{
		session:  NewSession(queries),
		queries:  queries,
		scanner:  scanner.New(sqlDB, queries),
		detector: device.New(),
		mux:      http.NewServeMux(),
		staticFS: staticFS,
	}

	funcMap := template.FuncMap{
		"upper": strings.ToUpper,
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
		"mod": func(a, b int) int {
			return a % b
		},
		"velocityOpacity": func(vel byte) float64 {
			// Map velocity 0-127 to opacity 0.5-1.0
			return 0.5 + float64(vel)/127.0*0.5
		},
		"velocityColor": func(vel byte) string {
			switch {
			case vel < 43:
				return "#4488cc"
			case vel < 86:
				return "#44aa44"
			default:
				return "#cc4444"
			}
		},
	}

	tmpl, err := template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/*.html", "templates/partials/*.html")
	if err != nil {
		log.Fatalf("parse templates: %v", err)
	}
	s.templates = tmpl

	s.registerRoutes()

	// Auto-scan workspace on startup (background, non-blocking).
	go func() {
		if s.session.WorkspacePath == "" {
			return
		}
		result, err := s.scanner.ScanWorkspace(s.session.WorkspacePath)
		if err != nil {
			log.Printf("startup scan: %v", err)
			return
		}
		log.Printf("startup scan: found=%d scanned=%d removed=%d",
			result.FilesFound, result.FilesScanned, result.FilesRemoved)
	}()

	// Start MPC device detection in background.
	go s.detector.Start(context.Background())

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
	s.mux.HandleFunc("/project/new", s.handleProjectNew)
	s.mux.HandleFunc("/program/open", s.handleProgramOpen)
	s.mux.HandleFunc("/program/save", s.handleProgramSave)
	s.mux.HandleFunc("/program/sample-report", s.handleSampleReport)

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
	s.mux.HandleFunc("/audio/file", s.handleAudioFile)
	s.mux.HandleFunc("/audio/waveform", s.handleAudioWaveform)
	s.mux.HandleFunc("/audio/info", s.handleAudioInfo)

	// Slicer
	s.mux.HandleFunc("/slicer", s.handleSlicerPage)
	s.mux.HandleFunc("/slicer/load", s.handleSlicerLoad)
	s.mux.HandleFunc("/slicer/waveform", s.handleSlicerWaveform)
	s.mux.HandleFunc("/slicer/sensitivity", s.handleSlicerSensitivity)
	s.mux.HandleFunc("/slicer/marker/", s.handleSlicerMarker)
	s.mux.HandleFunc("/slicer/export", s.handleSlicerExport)

	// Sequence viewer
	s.mux.HandleFunc("/sequence", s.handleSequencePage)
	s.mux.HandleFunc("/sequence/events", s.handleSequenceEvents)

	// Edit operations
	s.mux.HandleFunc("/edit/remove-all-samples", s.handleRemoveAllSamples)
	s.mux.HandleFunc("/edit/chromatic-layout", s.handleChromaticLayout)
	s.mux.HandleFunc("/edit/copy-settings-to-all", s.handleCopySettingsToAll)
	s.mux.HandleFunc("/edit/profile", s.handleProfileSwitch)

	// Batch
	s.mux.HandleFunc("/batch", s.handleBatchPage)
	s.mux.HandleFunc("/batch/run", s.handleBatchRun)

	// Detail panel (type-dispatched)
	s.mux.HandleFunc("/detail/select", s.handleDetailSelect)
	s.mux.HandleFunc("/detail", s.handleDetail)

	// File browser and workspace
	s.mux.HandleFunc("/browse/nav", s.handleBrowseNav)
	s.mux.HandleFunc("/browse", s.handleBrowse)
	s.mux.HandleFunc("/workspace/set", s.handleWorkspaceSet)
	s.mux.HandleFunc("/workspace/mkdir", s.handleWorkspaceMkdir)
	s.mux.HandleFunc("/workspace/rename", s.handleWorkspaceRename)
	s.mux.HandleFunc("/workspace/move", s.handleWorkspaceMove)
	s.mux.HandleFunc("/workspace/dirs", s.handleWorkspaceDirs)
	s.mux.HandleFunc("/workspace/scan", s.handleWorkspaceScan)
	s.mux.HandleFunc("/workspace/import", s.handleWorkspaceImport)
	s.mux.HandleFunc("/file/tags/add", s.handleTagAdd)
	s.mux.HandleFunc("/file/tags/remove", s.handleTagRemove)
	s.mux.HandleFunc("/file/source", s.handleSetWavSource)
	s.mux.HandleFunc("/file/", s.handleFileDetail)

	// Settings
	s.mux.HandleFunc("/settings", s.handleSettingsGet)
	s.mux.HandleFunc("/settings/save", s.handleSettingsPost)

	// Device detection
	s.mux.HandleFunc("/device/status", s.handleDeviceStatus)
	s.mux.HandleFunc("/device/detect", s.handleDeviceDetect)
	s.mux.HandleFunc("/device/use-as-workspace", s.handleDeviceUseAsWorkspace)

	// API
	s.mux.HandleFunc("/api/samples", s.handleAPISamples)
	s.mux.HandleFunc("/api/programs", s.handleAPIPrograms)
	s.mux.HandleFunc("/api/program-pads", s.handleAPIProgramPads)
	s.mux.HandleFunc("/api/assign-to-program", s.handleAPIAssignToProgram)
	s.mux.HandleFunc("/api/pad-params/", s.handleAPIPadParams)

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
