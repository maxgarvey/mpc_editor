package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// BrowseData holds template data for the file browser.
type BrowseData struct {
	Context      string
	CurrentDir   string
	RelDir       string
	Breadcrumbs  []BreadcrumbItem
	Entries      []BrowseEntry
	Workspace    string
	SelectedPath string // absolute path of the currently selected file (for highlighting)
}

// BreadcrumbItem represents a segment in the breadcrumb path.
type BreadcrumbItem struct {
	Name string
	Path string // relative to workspace
}

// BrowseEntry represents a file or directory in the browser listing.
type BrowseEntry struct {
	Name           string
	Path           string // absolute path
	IsDir          bool
	IsProject      bool // true if directory contains a .pgm file (self-contained beat)
	Ext            string
	Size           int64
	FileID         int64  // catalog file ID (0 if not cataloged)
	MissingSamples int64  // for .pgm: number of unresolved sample refs
	WavInfo        string // for .wav: e.g. "44100Hz 16bit stereo"
}

func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	workspace := s.session.WorkspacePath
	if workspace == "" {
		http.Error(w, "no workspace configured", http.StatusBadRequest)
		return
	}

	ctx := r.FormValue("context")
	if ctx == "" {
		ctx = "open-pgm"
	}

	dir := r.FormValue("dir")
	var absDir string
	if dir == "" {
		absDir = workspace
	} else if filepath.IsAbs(dir) {
		absDir = dir
	} else {
		absDir = filepath.Join(workspace, dir)
	}
	absDir = filepath.Clean(absDir)

	if err := s.validateWithinWorkspace(absDir); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	entries, err := os.ReadDir(absDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Build filtered entry list.
	var browseEntries []BrowseEntry
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}

		if e.IsDir() {
			browseEntries = append(browseEntries, BrowseEntry{
				Name:  name,
				Path:  filepath.Join(absDir, name),
				IsDir: true,
			})
			continue
		}

		ext := strings.ToLower(filepath.Ext(name))
		if !filterAllows(ctx, ext) {
			continue
		}

		info, _ := e.Info()
		var size int64
		if info != nil {
			size = info.Size()
		}
		browseEntries = append(browseEntries, BrowseEntry{
			Name: name,
			Path: filepath.Join(absDir, name),
			Ext:  ext,
			Size: size,
		})
	}

	// Sort: directories first, then files, both alphabetical.
	sort.Slice(browseEntries, func(i, j int) bool {
		if browseEntries[i].IsDir != browseEntries[j].IsDir {
			return browseEntries[i].IsDir
		}
		return strings.ToLower(browseEntries[i].Name) < strings.ToLower(browseEntries[j].Name)
	})

	// Build relative path and breadcrumbs.
	relDir, _ := filepath.Rel(workspace, absDir)
	if relDir == "." {
		relDir = ""
	}

	breadcrumbs := []BreadcrumbItem{{Name: filepath.Base(workspace), Path: ""}}
	if relDir != "" {
		parts := strings.Split(relDir, string(filepath.Separator))
		for i, part := range parts {
			breadcrumbs = append(breadcrumbs, BreadcrumbItem{
				Name: part,
				Path: filepath.Join(parts[:i+1]...),
			})
		}
	}

	// Enrich entries with catalog data.
	s.enrichBrowseEntries(browseEntries, workspace)

	data := BrowseData{
		Context:     ctx,
		CurrentDir:  absDir,
		RelDir:      relDir,
		Breadcrumbs: breadcrumbs,
		Entries:     browseEntries,
		Workspace:   workspace,
	}

	s.renderTemplate(w, "file_browser.html", data)
}

// buildBrowseData builds an unfiltered BrowseData for the persistent browser nav panel.
func (s *Server) buildBrowseData(dir, selectedPath string) (BrowseData, error) {
	workspace := s.session.WorkspacePath

	var absDir string
	if dir == "" {
		absDir = workspace
	} else if filepath.IsAbs(dir) {
		absDir = dir
	} else {
		absDir = filepath.Join(workspace, dir)
	}
	absDir = filepath.Clean(absDir)

	if err := s.validateWithinWorkspace(absDir); err != nil {
		return BrowseData{}, err
	}

	entries, err := os.ReadDir(absDir)
	if err != nil {
		return BrowseData{}, err
	}

	var browseEntries []BrowseEntry
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}

		if e.IsDir() {
			browseEntries = append(browseEntries, BrowseEntry{
				Name:  name,
				Path:  filepath.Join(absDir, name),
				IsDir: true,
			})
			continue
		}

		ext := strings.ToLower(filepath.Ext(name))
		// No filtering — show all known MPC file types
		if !filterAllows("browse", ext) {
			continue
		}

		info, _ := e.Info()
		var size int64
		if info != nil {
			size = info.Size()
		}
		browseEntries = append(browseEntries, BrowseEntry{
			Name: name,
			Path: filepath.Join(absDir, name),
			Ext:  ext,
			Size: size,
		})
	}

	sort.Slice(browseEntries, func(i, j int) bool {
		if browseEntries[i].IsDir != browseEntries[j].IsDir {
			return browseEntries[i].IsDir
		}
		return strings.ToLower(browseEntries[i].Name) < strings.ToLower(browseEntries[j].Name)
	})

	relDir, _ := filepath.Rel(workspace, absDir)
	if relDir == "." {
		relDir = ""
	}

	breadcrumbs := []BreadcrumbItem{{Name: filepath.Base(workspace), Path: ""}}
	if relDir != "" {
		parts := strings.Split(relDir, string(filepath.Separator))
		for i, part := range parts {
			breadcrumbs = append(breadcrumbs, BreadcrumbItem{
				Name: part,
				Path: filepath.Join(parts[:i+1]...),
			})
		}
	}

	s.enrichBrowseEntries(browseEntries, workspace)

	return BrowseData{
		Context:      "browse",
		CurrentDir:   absDir,
		RelDir:       relDir,
		Breadcrumbs:  breadcrumbs,
		Entries:      browseEntries,
		Workspace:    workspace,
		SelectedPath: selectedPath,
	}, nil
}

// handleBrowseNav handles HTMX requests to navigate the persistent browser panel.
func (s *Server) handleBrowseNav(w http.ResponseWriter, r *http.Request) {
	workspace := s.session.WorkspacePath
	if workspace == "" {
		http.Error(w, "no workspace configured", http.StatusBadRequest)
		return
	}

	dir := r.FormValue("dir")
	data, err := s.buildBrowseData(dir, s.session.SelectedDetailPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.renderTemplate(w, "file_browser_nav.html", data)
}

func (s *Server) handleWorkspaceSet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := r.FormValue("path")
	if path == "" {
		http.Error(w, "path is required", http.StatusBadRequest)
		return
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := os.MkdirAll(absPath, 0o755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.session.WorkspacePath = absPath
	s.session.Prefs.WorkspacePath = absPath
	if err := s.queries.UpdateWorkspacePath(r.Context(), absPath); err != nil {
		log.Printf("save workspace path: %v", err)
	}

	// Clear the last detail path — it refers to the old workspace.
	s.session.SelectedDetailPath = ""
	s.session.Prefs.LastDetailPath = ""
	_ = s.queries.UpdateLastDetailPath(r.Context(), "")

	// Re-scan the new workspace in the background.
	go func() {
		if result, err := s.scanner.ScanWorkspace(absPath); err != nil {
			log.Printf("workspace scan after set: %v", err)
		} else {
			log.Printf("workspace scan: found=%d scanned=%d removed=%d",
				result.FilesFound, result.FilesScanned, result.FilesRemoved)
		}
	}()

	w.Header().Set("HX-Redirect", "/")
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleWorkspaceMkdir(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	parent := r.FormValue("parent")
	name := r.FormValue("name")
	ctx := r.FormValue("context")

	if name == "" {
		http.Error(w, "folder name is required", http.StatusBadRequest)
		return
	}

	// Reject path separators and traversal in name.
	if strings.ContainsAny(name, `/\`) || name == ".." || name == "." {
		http.Error(w, "invalid folder name", http.StatusBadRequest)
		return
	}

	dir := filepath.Join(s.session.WorkspacePath, parent, name)
	if err := s.validateWithinWorkspace(dir); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Re-render browser at the parent directory.
	if ctx == "browse" {
		r.Form.Set("dir", parent)
		s.handleBrowseNav(w, r)
		return
	}
	r.Form.Set("dir", parent)
	r.Form.Set("context", ctx)
	s.handleBrowse(w, r)
}

// enrichBrowseEntries looks up catalog data for each entry and populates
// badge fields (MissingSamples for .pgm, WavInfo for .wav).
func (s *Server) enrichBrowseEntries(entries []BrowseEntry, workspace string) {
	ctx := context.Background()
	for i := range entries {
		e := &entries[i]
		if e.IsDir {
			e.IsProject = dirContainsPGM(e.Path)
			continue
		}

		relPath, err := filepath.Rel(workspace, e.Path)
		if err != nil {
			continue
		}

		f, err := s.queries.GetFileByPath(ctx, relPath)
		if err != nil {
			continue
		}
		e.FileID = f.ID

		switch e.Ext {
		case ".pgm":
			missing, err := s.queries.CountMissingSamples(ctx, f.ID)
			if err == nil {
				e.MissingSamples = missing
			}
		case ".wav":
			meta, err := s.queries.GetWavMeta(ctx, f.ID)
			if err == nil {
				ch := "mono"
				if meta.Channels == 2 {
					ch = "stereo"
				}
				e.WavInfo = fmt.Sprintf("%dHz %dbit %s", meta.SampleRate, meta.BitsPerSample, ch)
			}
		}
	}
}

// dirContainsPGM checks if a directory contains at least one .pgm file (shallow).
func dirContainsPGM(dirPath string) bool {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.ToLower(filepath.Ext(e.Name())) == ".pgm" {
			return true
		}
	}
	return false
}

// filterAllows returns true if the file extension is allowed for the given browse context.
func filterAllows(ctx, ext string) bool {
	switch ctx {
	case "open-pgm", "save-pgm":
		return ext == ".pgm"
	case "load-wav":
		return ext == ".wav"
	case "export-dir", "batch-dir":
		return false // directories only
	default:
		return ext == ".pgm" || ext == ".wav" || ext == ".mid" ||
			ext == ".seq" || ext == ".sng" || ext == ".all" || ext == ".txt"
	}
}
