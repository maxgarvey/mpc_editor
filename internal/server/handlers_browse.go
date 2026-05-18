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
	"github.com/maxgarvey/mpc_editor/internal/pgm"
)

// BrowseData holds template data for the file browser.
type BrowseData struct {
	Context        string
	CurrentDir     string
	RelDir         string
	Breadcrumbs    []BreadcrumbItem
	Entries        []BrowseEntry
	Workspace      string
	SelectedPath   string // absolute path of the currently selected file (for highlighting)
	SearchQuery    string // non-empty when showing search results
	SortMode       string // "name" or "label"; empty defaults to "name"
	HasLabeledWAVs bool   // true when the current dir contains at least one labeled WAV (show Organize button)
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
	RelPath        string // relative path from workspace (set in search results)
	RelDirPath     string // directory portion of RelPath (set in search results, for display)
	IsDir          bool
	IsProject      bool   // true if directory contains a .pgm file (self-contained beat)
	Divider        bool   // true for synthetic label-group divider rows (no file)
	DividerLabel   string // label text for divider rows
	Ext            string
	Size           int64
	FileID         int64  // catalog file ID (0 if not cataloged)
	MissingSamples int64  // for .pgm: number of unresolved sample refs
	WavInfo        string // for .wav: e.g. "44100Hz 16bit stereo"
	Color          string // for .wav: CSS hex color from preset (e.g. "#e05555"), empty if unset
	Category       string // for .wav: label category (e.g. "drum")
	Subcategory    string // for .wav: label subcategory (e.g. "kick")
	Favorite       bool   // for .wav: true if starred by user
}

// resolveAbsDir converts a (possibly relative) dir string to a validated absolute path.
func (s *Server) resolveAbsDir(workspace, dir string) (string, error) {
	var absDir string
	switch {
	case dir == "":
		absDir = workspace
	case filepath.IsAbs(dir):
		absDir = dir
	default:
		absDir = filepath.Join(workspace, dir)
	}
	absDir = filepath.Clean(absDir)
	return absDir, s.validateWithinWorkspace(absDir)
}

// readDirEntries reads a directory and returns BrowseEntry values for non-hidden entries
// whose extension passes filterAllows for the given context.
func readDirEntries(absDir, filterCtx string) ([]BrowseEntry, error) {
	raw, err := os.ReadDir(absDir)
	if err != nil {
		return nil, err
	}
	var out []BrowseEntry
	for _, e := range raw {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if e.IsDir() {
			out = append(out, BrowseEntry{Name: name, Path: filepath.Join(absDir, name), IsDir: true})
			continue
		}
		ext := strings.ToLower(filepath.Ext(name))
		if !filterAllows(filterCtx, ext) {
			continue
		}
		info, _ := e.Info()
		var size int64
		if info != nil {
			size = info.Size()
		}
		out = append(out, BrowseEntry{Name: name, Path: filepath.Join(absDir, name), Ext: ext, Size: size})
	}
	return out, nil
}

// sortBrowseEntries sorts entries in place according to mode.
// "label": directories first, then WAVs grouped by subcategory with divider rows, unlabeled last.
// "name" (default): directories first, then files, both alphabetical.
// Returns the (possibly expanded) slice; caller must use the returned value when mode=="label".
func sortBrowseEntries(entries []BrowseEntry, mode string) []BrowseEntry {
	if mode == "label" {
		return sortByLabel(entries)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	return entries
}

// sortByLabel groups non-directory entries by subcategory, inserts divider rows between groups,
// and places unlabeled files at the end. Directories stay at the top, alphabetical.
func sortByLabel(entries []BrowseEntry) []BrowseEntry {
	var dirs, labeled, unlabeled []BrowseEntry
	groups := map[string][]BrowseEntry{}
	var groupOrder []string
	seen := map[string]bool{}

	for _, e := range entries {
		if e.IsDir {
			dirs = append(dirs, e)
			continue
		}
		if e.Subcategory == "" {
			unlabeled = append(unlabeled, e)
			continue
		}
		if !seen[e.Subcategory] {
			seen[e.Subcategory] = true
			groupOrder = append(groupOrder, e.Subcategory)
		}
		groups[e.Subcategory] = append(groups[e.Subcategory], e)
	}

	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name)
	})
	sort.Strings(groupOrder)
	sort.Slice(unlabeled, func(i, j int) bool {
		return strings.ToLower(unlabeled[i].Name) < strings.ToLower(unlabeled[j].Name)
	})

	out := append([]BrowseEntry{}, dirs...)
	for _, sub := range groupOrder {
		g := groups[sub]
		sort.Slice(g, func(i, j int) bool {
			return strings.ToLower(g[i].Name) < strings.ToLower(g[j].Name)
		})
		out = append(out, BrowseEntry{Divider: true, DividerLabel: sub})
		out = append(out, g...)
		labeled = append(labeled, g...)
	}
	if len(unlabeled) > 0 {
		if len(labeled) > 0 {
			out = append(out, BrowseEntry{Divider: true, DividerLabel: "other"})
		}
		out = append(out, unlabeled...)
	}
	return out
}

// buildBreadcrumbs returns the workspace-relative dir string and breadcrumb items for absDir.
func buildBreadcrumbs(workspace, absDir string) (relDir string, crumbs []BreadcrumbItem) {
	relDir, _ = filepath.Rel(workspace, absDir)
	if relDir == "." {
		relDir = ""
	}
	crumbs = []BreadcrumbItem{{Name: filepath.Base(workspace), Path: ""}}
	if relDir != "" {
		parts := strings.Split(relDir, string(filepath.Separator))
		for i, part := range parts {
			crumbs = append(crumbs, BreadcrumbItem{Name: part, Path: filepath.Join(parts[:i+1]...)})
		}
	}
	return relDir, crumbs
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
	data, err := s.buildBrowseData(r.FormValue("dir"), ctx, "", "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.renderTemplate(w, "file_browser.html", data)
}

// buildBrowseData builds BrowseData for the given directory and filter context.
// sortMode is "label" or "" / "name" (default alphabetical).
func (s *Server) buildBrowseData(dir, filterCtx, selectedPath, sortMode string) (BrowseData, error) {
	workspace := s.session.WorkspacePath
	absDir, err := s.resolveAbsDir(workspace, dir)
	if err != nil {
		return BrowseData{}, err
	}
	entries, err := readDirEntries(absDir, filterCtx)
	if err != nil {
		return BrowseData{}, err
	}
	// Enrich before sort so label-sort can use Subcategory populated by enrich.
	relDir, breadcrumbs := buildBreadcrumbs(workspace, absDir)
	s.enrichBrowseEntries(entries, workspace)
	entries = sortBrowseEntries(entries, sortMode)
	var hasLabeled bool
	for _, e := range entries {
		if !e.IsDir && e.Subcategory != "" {
			hasLabeled = true
			break
		}
	}
	return BrowseData{
		Context:        filterCtx,
		CurrentDir:     absDir,
		RelDir:         relDir,
		Breadcrumbs:    breadcrumbs,
		Entries:        entries,
		Workspace:      workspace,
		SelectedPath:   selectedPath,
		SortMode:       sortMode,
		HasLabeledWAVs: hasLabeled,
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
	sortMode := r.FormValue("sort")
	data, err := s.buildBrowseData(dir, "browse", s.session.SelectedDetailPath, sortMode)
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
	// Clear the last detail path — it refers to the old workspace.
	s.session.SelectedDetailPath = ""
	s.session.Prefs.LastDetailPath = ""
	if err := s.queries.UpdateAllPreferences(r.Context(), s.session.Prefs.ToDBParams()); err != nil {
		log.Printf("save preferences: %v", err)
	}

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
			e.Color = colorToCSS(f.Color)
			e.Category = f.Category
			e.Subcategory = f.Subcategory
			e.Favorite = f.Favorite != 0
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

	// Update session if the renamed file or a containing directory was the active program.
	if s.session.FilePath == oldPath {
		s.session.FilePath = newPath
		s.session.SampleDir = filepath.Dir(newPath)
	} else if strings.HasPrefix(s.session.FilePath, oldPath+string(filepath.Separator)) {
		s.session.FilePath = newPath + s.session.FilePath[len(oldPath):]
		s.session.SampleDir = filepath.Dir(s.session.FilePath)
	}

	// Patch in-memory sample matrix for WAV renames/directory renames.
	changed := s.patchMatrixForPath(oldPath, newPath)
	if changed {
		if s.session.FilePath != "" {
			_ = s.session.Program.Save(s.session.FilePath)
		}
		w.Header().Set("HX-Redirect", "/")
		w.WriteHeader(http.StatusOK)
		return
	}

	w.Header().Set("HX-Trigger", "invalidateSampleCache")
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

	// Update session if the moved file or a containing directory was the active program.
	if s.session.FilePath == srcPath {
		s.session.FilePath = newPath
		s.session.SampleDir = filepath.Dir(newPath)
	} else if strings.HasPrefix(s.session.FilePath, srcPath+string(filepath.Separator)) {
		s.session.FilePath = newPath + s.session.FilePath[len(srcPath):]
		s.session.SampleDir = filepath.Dir(s.session.FilePath)
	}

	// Patch in-memory sample matrix for WAV/directory moves.
	changed := s.patchMatrixForPath(srcPath, newPath)
	if changed {
		if s.session.FilePath != "" {
			_ = s.session.Program.Save(s.session.FilePath)
		}
		w.Header().Set("HX-Redirect", "/")
		w.WriteHeader(http.StatusOK)
		return
	}

	w.Header().Set("HX-Trigger", "invalidateSampleCache")
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

	absDir, err := s.resolveAbsDir(workspace, r.FormValue("dir"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	rawEntries, err := os.ReadDir(absDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	type dirEntry struct {
		Name string
		Path string
	}
	var dirs []dirEntry
	for _, e := range rawEntries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		dirs = append(dirs, dirEntry{Name: e.Name(), Path: filepath.Join(absDir, e.Name())})
	}
	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name)
	})

	relDir, breadcrumbs := buildBreadcrumbs(workspace, absDir)
	s.renderTemplate(w, "move_dirs.html", map[string]any{
		"Breadcrumbs": breadcrumbs,
		"Dirs":        dirs,
		"CurrentDir":  absDir,
		"RelDir":      relDir,
	})
}

// patchMatrixForPath updates the in-memory sample matrix when a file or directory is
// renamed, moved, or deleted. Pass newAbs="" to clear affected pads.
// Returns true if any pad was modified (so the caller can decide to save the program).
func (s *Server) patchMatrixForPath(oldAbs, newAbs string) bool {
	if s.session.Program == nil {
		return false
	}
	dirPrefix := oldAbs + string(filepath.Separator)
	changed := false
	for i := range 64 {
		for j := range 4 {
			ref := s.session.Matrix.Get(i, j)
			if ref == nil {
				continue
			}
			exactMatch := ref.FilePath == oldAbs
			prefixMatch := strings.HasPrefix(ref.FilePath, dirPrefix)
			if !exactMatch && !prefixMatch {
				continue
			}
			changed = true
			if newAbs == "" {
				s.session.Matrix.Set(i, j, nil)
				_ = s.session.Program.Pad(i).Layer(j).SetSampleName("")
				continue
			}
			var newFilePath string
			if exactMatch {
				newFilePath = newAbs
			} else {
				rel, _ := strings.CutPrefix(ref.FilePath, dirPrefix)
				newFilePath = filepath.Join(newAbs, rel)
			}
			newName := ref.Name
			if exactMatch {
				stem := strings.TrimSuffix(filepath.Base(newFilePath), filepath.Ext(newFilePath))
				if len(stem) > 16 {
					stem = stem[:16]
				}
				if stem != ref.Name {
					newName = stem
					_ = s.session.Program.Pad(i).Layer(j).SetSampleName(stem)
				}
			}
			s.session.Matrix.Set(i, j, &pgm.SampleRef{
				Name: newName, FilePath: newFilePath, Status: ref.Status,
			})
		}
	}
	return changed
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
		NewPath: newRel,
		OldPath: oldRel,
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
		if suffix, ok := strings.CutPrefix(f.Path, oldPrefix); ok {
			updated := newPrefix + suffix
			if err := s.queries.UpdateFilePath(ctx, db.UpdateFilePathParams{
				NewPath: updated,
				OldPath: f.Path,
			}); err != nil {
				log.Printf("update catalog path %q: %v", f.Path, err)
			}
		}
	}
}

// handleWorkspaceOrganize moves labeled WAV files in a flat directory into
// per-subcategory subdirectories. POST /workspace/organize?dir=<absDir>.
func (s *Server) handleWorkspaceOrganize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	absDir := r.FormValue("dir")
	if absDir == "" {
		absDir = s.session.WorkspacePath
	}

	if err := s.validateWithinWorkspace(absDir); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	dirInfo, err := os.Stat(absDir)
	if err != nil || !dirInfo.IsDir() {
		http.Error(w, "dir must be an existing directory", http.StatusBadRequest)
		return
	}

	rawEntries, err := os.ReadDir(absDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	workspace := s.session.WorkspacePath
	ctx := r.Context()

	var moved int
	for _, de := range rawEntries {
		if de.IsDir() || strings.HasPrefix(de.Name(), ".") {
			continue
		}
		if !strings.EqualFold(filepath.Ext(de.Name()), ".wav") {
			continue
		}
		srcAbs := filepath.Join(absDir, de.Name())
		srcRel, err := filepath.Rel(workspace, srcAbs)
		if err != nil {
			continue
		}

		dbFile, err := s.queries.GetFileByPath(ctx, srcRel)
		if err != nil || dbFile.Subcategory == "" {
			continue
		}

		destSubdir := filepath.Join(absDir, dbFile.Subcategory)
		if err := os.MkdirAll(destSubdir, 0o755); err != nil {
			log.Printf("organize mkdir %q: %v", destSubdir, err)
			continue
		}

		destAbs := filepath.Join(destSubdir, de.Name())
		if _, err := os.Stat(destAbs); err == nil {
			continue
		}

		if err := os.Rename(srcAbs, destAbs); err != nil {
			log.Printf("organize rename %q: %v", srcAbs, err)
			continue
		}

		s.updateCatalogPath(ctx, srcAbs, destAbs)
		s.patchMatrixForPath(srcAbs, destAbs)
		moved++
	}

	if moved > 0 && s.session.FilePath != "" {
		_ = s.session.Program.Save(s.session.FilePath)
	}

	relDir, _ := filepath.Rel(workspace, absDir)
	if relDir == "." {
		relDir = ""
	}
	r.Form.Set("dir", relDir)
	s.handleBrowseNav(w, r)
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

		// If the active program was deleted, reset session.
		if s.session.FilePath == absPath ||
			strings.HasPrefix(s.session.FilePath, absPath+string(filepath.Separator)) {
			s.session.Program = pgm.NewProgram()
			s.session.FilePath = ""
			s.session.SampleDir = ""
			s.session.Matrix.Clear()
			w.Header().Set("HX-Redirect", "/")
			w.WriteHeader(http.StatusOK)
			return
		}

		// Clear any matrix pads that referenced the deleted path.
		changed := s.patchMatrixForPath(absPath, "")
		if changed && s.session.FilePath != "" {
			_ = s.session.Program.Save(s.session.FilePath)
		}
	}

	w.Header().Set("HX-Trigger", `{"refreshBrowser":true,"invalidateSampleCache":true}`)
	w.WriteHeader(http.StatusOK)
}

// searchCatalog queries the catalog for WAV files matching q, ranked by label precision.
// Rank order: exact subcategory → exact category → partial subcategory → partial category →
// tag match → filename match. Favorites float above non-favorites within each rank tier.
// When favoritesOnly is true, only starred files are returned (q is ignored).
func (s *Server) searchCatalog(ctx context.Context, q string, favoritesOnly bool) ([]db.File, error) {
	if favoritesOnly {
		return s.queries.ListFavorites(ctx)
	}
	qLike := "%" + strings.ToLower(q) + "%"
	rows, err := s.sqlDB.QueryContext(ctx, `
		SELECT DISTINCT f.id, f.path, f.file_type, f.size, f.mod_time, f.scanned_at,
		                f.color, f.category, f.subcategory, f.favorite
		FROM files f
		LEFT JOIN file_tags ft ON ft.file_id = f.id
		WHERE f.file_type = 'wav'
		  AND (
		        LOWER(f.path)        LIKE ?
		     OR LOWER(f.category)    LIKE ?
		     OR LOWER(f.subcategory) LIKE ?
		     OR LOWER(ft.tag_value)  LIKE ?
		  )
		GROUP BY f.id
		ORDER BY
		    f.favorite DESC,
		    CASE
		        WHEN LOWER(f.subcategory) = LOWER(?) THEN 0
		        WHEN LOWER(f.category)    = LOWER(?) THEN 1
		        WHEN LOWER(f.subcategory) LIKE ?      THEN 2
		        WHEN LOWER(f.category)    LIKE ?      THEN 3
		        WHEN MIN(LOWER(ft.tag_value)) LIKE ?  THEN 4
		        ELSE 5
		    END,
		    f.path
		LIMIT 200`,
		qLike, qLike, qLike, qLike,
		q, q, qLike, qLike, qLike,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var files []db.File
	for rows.Next() {
		var f db.File
		if err := rows.Scan(&f.ID, &f.Path, &f.FileType, &f.Size, &f.ModTime, &f.ScannedAt,
			&f.Color, &f.Category, &f.Subcategory, &f.Favorite); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

// handleBrowseSearch searches the catalog for WAV files matching the query across
// filename, label (category/subcategory), and tags. Results are ranked by label precision.
// GET /browse/search?q=...&favorites=1
func (s *Server) handleBrowseSearch(w http.ResponseWriter, r *http.Request) {
	workspace := s.session.WorkspacePath
	if workspace == "" {
		http.Error(w, "no workspace configured", http.StatusBadRequest)
		return
	}

	q := strings.TrimSpace(r.FormValue("q"))
	favoritesOnly := r.FormValue("favorites") == "1"

	if q == "" && !favoritesOnly {
		data, err := s.buildBrowseData("", "browse", s.session.SelectedDetailPath, "")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.renderTemplate(w, "file_browser_nav.html", data)
		return
	}

	ctx := r.Context()
	files, err := s.searchCatalog(ctx, q, favoritesOnly)
	if err != nil {
		http.Error(w, "search failed", http.StatusInternalServerError)
		return
	}

	var entries []BrowseEntry
	for _, f := range files {
		relPath := f.Path
		absPath := filepath.Join(workspace, relPath)
		ext := strings.ToLower(filepath.Ext(f.Path))
		dir := filepath.Dir(relPath)
		if dir == "." {
			dir = ""
		}
		entries = append(entries, BrowseEntry{
			Name:        filepath.Base(f.Path),
			Path:        absPath,
			RelPath:     relPath,
			RelDirPath:  dir,
			Ext:         ext,
			FileID:      f.ID,
			Color:       colorToCSS(f.Color),
			Category:    f.Category,
			Subcategory: f.Subcategory,
			Favorite:    f.Favorite != 0,
		})
	}

	searchLabel := q
	if favoritesOnly {
		searchLabel = "★ Favorites"
	}
	data := BrowseData{
		SearchQuery: searchLabel,
		Entries:     entries,
		Workspace:   workspace,
	}
	s.renderTemplate(w, "file_browser_nav.html", data)
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
