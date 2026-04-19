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

	"github.com/maxgarvey/mpc_editor/internal/db"
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
	ensureWorkspaceDirs(absPath)

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

func (s *Server) handleWorkspaceRename(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	oldPath := r.FormValue("path")
	newName := r.FormValue("name")

	if oldPath == "" || newName == "" {
		http.Error(w, "path and name are required", http.StatusBadRequest)
		return
	}

	if strings.ContainsAny(newName, `/\`) || newName == ".." || newName == "." {
		http.Error(w, "invalid file name", http.StatusBadRequest)
		return
	}

	if err := s.validateWithinWorkspace(oldPath); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	newPath := filepath.Join(filepath.Dir(oldPath), newName)
	if err := s.validateWithinWorkspace(newPath); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	if _, err := os.Stat(newPath); err == nil {
		http.Error(w, "a file with that name already exists", http.StatusConflict)
		return
	}

	if err := os.Rename(oldPath, newPath); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Update catalog database path.
	s.updateCatalogPath(r.Context(), oldPath, newPath)

	// Update session if the renamed file was the active program.
	if s.session.FilePath == oldPath {
		s.session.FilePath = newPath
		s.session.SampleDir = filepath.Dir(newPath)
	}

	parentDir := filepath.Dir(oldPath)
	relDir, _ := filepath.Rel(s.session.WorkspacePath, parentDir)
	if relDir == "." {
		relDir = ""
	}
	r.Form.Set("dir", relDir)
	s.handleBrowseNav(w, r)
}

func (s *Server) handleWorkspaceMove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	srcPath := r.FormValue("path")
	destDir := r.FormValue("dest")

	if srcPath == "" || destDir == "" {
		http.Error(w, "path and dest are required", http.StatusBadRequest)
		return
	}

	if err := s.validateWithinWorkspace(srcPath); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if err := s.validateWithinWorkspace(destDir); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	destInfo, err := os.Stat(destDir)
	if err != nil || !destInfo.IsDir() {
		http.Error(w, "destination must be an existing directory", http.StatusBadRequest)
		return
	}

	// Prevent moving a directory into itself.
	absSrc, _ := filepath.Abs(srcPath)
	absDest, _ := filepath.Abs(destDir)
	if strings.HasPrefix(absDest, absSrc+string(filepath.Separator)) {
		http.Error(w, "cannot move a directory into itself", http.StatusBadRequest)
		return
	}

	newPath := filepath.Join(destDir, filepath.Base(srcPath))
	if _, err := os.Stat(newPath); err == nil {
		http.Error(w, "a file with that name already exists in the destination", http.StatusConflict)
		return
	}

	if err := os.Rename(srcPath, newPath); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Update catalog database path.
	s.updateCatalogPath(r.Context(), srcPath, newPath)

	// Update session if the moved file was the active program.
	if s.session.FilePath == srcPath {
		s.session.FilePath = newPath
		s.session.SampleDir = filepath.Dir(newPath)
	}

	// Re-render the nav at the parent of the source (where the file disappeared from).
	parentDir := filepath.Dir(srcPath)
	relDir, _ := filepath.Rel(s.session.WorkspacePath, parentDir)
	if relDir == "." {
		relDir = ""
	}
	r.Form.Set("dir", relDir)
	s.handleBrowseNav(w, r)
}

func (s *Server) handleWorkspaceDirs(w http.ResponseWriter, r *http.Request) {
	workspace := s.session.WorkspacePath
	if workspace == "" {
		http.Error(w, "no workspace configured", http.StatusBadRequest)
		return
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

	type dirEntry struct {
		Name string
		Path string
	}

	var dirs []dirEntry
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		dirs = append(dirs, dirEntry{
			Name: e.Name(),
			Path: filepath.Join(absDir, e.Name()),
		})
	}

	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name)
	})

	// Build breadcrumbs for the directory picker.
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

	s.renderTemplate(w, "move_dirs.html", map[string]any{
		"Breadcrumbs": breadcrumbs,
		"Dirs":        dirs,
		"CurrentDir":  absDir,
	})
}

// updateCatalogPath updates the catalog database when a file or directory is renamed/moved.
func (s *Server) updateCatalogPath(ctx context.Context, oldAbs, newAbs string) {
	workspace := s.session.WorkspacePath
	oldRel, err := filepath.Rel(workspace, oldAbs)
	if err != nil {
		return
	}
	newRel, err := filepath.Rel(workspace, newAbs)
	if err != nil {
		return
	}

	// For a single file, update its path directly.
	if err := s.queries.UpdateFilePath(ctx, db.UpdateFilePathParams{
		Path:   newRel,
		Path_2: oldRel,
	}); err != nil {
		log.Printf("update catalog path: %v", err)
	}

	// For directories, update all files under the old path prefix.
	oldPrefix := oldRel + string(filepath.Separator)
	newPrefix := newRel + string(filepath.Separator)
	files, err := s.queries.ListAllFiles(ctx)
	if err != nil {
		return
	}
	for _, f := range files {
		if strings.HasPrefix(f.Path, oldPrefix) {
			updated := newPrefix + strings.TrimPrefix(f.Path, oldPrefix)
			if err := s.queries.UpdateFilePath(ctx, db.UpdateFilePathParams{
				Path:   updated,
				Path_2: f.Path,
			}); err != nil {
				log.Printf("update catalog path %q: %v", f.Path, err)
			}
		}
	}
}

// handleWorkspaceDelete deletes a file or directory from disk and/or the catalog.
// POST /workspace/delete?path=<relPath>&mode=disk|catalog
func (s *Server) handleWorkspaceDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	relPath := r.FormValue("path")
	if relPath == "" {
		http.Error(w, "path is required", http.StatusBadRequest)
		return
	}

	mode := r.FormValue("mode")
	if mode != "disk" && mode != "catalog" {
		http.Error(w, "mode must be 'disk' or 'catalog'", http.StatusBadRequest)
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

	ctx := r.Context()

	// Remove from catalog: delete the file entry and any files under a directory prefix.
	_ = s.queries.DeleteFileByPath(ctx, relPath)
	dirPrefix := relPath + string(filepath.Separator)
	if files, err := s.queries.ListAllFiles(ctx); err == nil {
		for _, f := range files {
			if strings.HasPrefix(f.Path, dirPrefix) {
				_ = s.queries.DeleteFileByPath(ctx, f.Path)
			}
		}
	}

	// For disk mode, also remove the file/directory from the filesystem.
	if mode == "disk" {
		if err := os.RemoveAll(absPath); err != nil {
			http.Error(w, fmt.Sprintf("delete: %v", err), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("HX-Trigger", "refreshBrowser")
	w.WriteHeader(http.StatusOK)
}

// filterAllows returns true if the file extension is allowed for the given browse context.
func filterAllows(ctx, ext string) bool {
	switch ctx {
	case "open-pgm", "save-pgm":
		return ext == ".pgm"
	case "load-wav":
		return ext == ".wav"
	case "export-dir":
		return false // directories only
	default:
		return ext == ".pgm" || ext == ".wav" || ext == ".mid" ||
			ext == ".seq" || ext == ".sng" || ext == ".all" || ext == ".txt"
	}
}
