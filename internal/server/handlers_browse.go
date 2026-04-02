package server

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// BrowseData holds template data for the file browser.
type BrowseData struct {
	Context     string
	CurrentDir  string
	RelDir      string
	Breadcrumbs []BreadcrumbItem
	Entries     []BrowseEntry
	Workspace   string
}

// BreadcrumbItem represents a segment in the breadcrumb path.
type BreadcrumbItem struct {
	Name string
	Path string // relative to workspace
}

// BrowseEntry represents a file or directory in the browser listing.
type BrowseEntry struct {
	Name  string
	Path  string // absolute path
	IsDir bool
	Ext   string
	Size  int64
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
	r.Form.Set("dir", parent)
	r.Form.Set("context", ctx)
	s.handleBrowse(w, r)
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
		return ext == ".pgm" || ext == ".wav" || ext == ".mid"
	}
}
