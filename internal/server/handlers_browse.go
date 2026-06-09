package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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
	SearchCapped   bool   // true when search results hit the limit (more matches exist)
	SortMode       string // "name" or "label"; empty defaults to "name"
	HasLabeledWAVs bool   // true when the current dir contains at least one labeled WAV (show Organize button)
}

// searchResultLimit caps catalog search results (see searchCatalog).
const searchResultLimit = 200

// BreadcrumbItem represents a segment in the breadcrumb path.
type BreadcrumbItem struct {
	Name string
	Path string // relative to workspace
}

// BrowseEntry represents a file or directory in the browser listing.
type BrowseEntry struct {
	Name              string
	Path              string // absolute path
	RelPath           string // relative path from workspace (set in search results)
	RelDirPath        string // directory portion of RelPath (set in search results, for display)
	IsDir             bool
	IsProject         bool   // true if directory contains a .pgm file (self-contained beat)
	Divider           bool   // true for synthetic label-group divider rows (no file)
	DividerLabel      string // label text for divider rows
	Collapsible       bool   // true if divider is a clickable collapse toggle
	DividerGroup      string // group key for collapsible dividers (e.g. "samples")
	Group             string // group key for file entries (matches DividerGroup of their header)
	Ext               string
	Size              int64
	FileID            int64  // catalog file ID (0 if not cataloged)
	MissingSamples    int64  // for .pgm: number of unresolved sample refs
	WavInfo           string // for .wav: e.g. "44100Hz 16bit stereo"
	Color             string // for .wav: CSS hex color from preset (e.g. "#e05555"), empty if unset
	Category          string // for .wav: label category (e.g. "drum")
	Subcategory       string // for .wav: label subcategory (e.g. "kick")
	Favorite          bool   // for .wav: true if starred by user
	IsLibraryRoot     bool   // true when this entry IS the sample_library/ directory
	IsLibrary         bool   // true when this entry lives inside sample_library/
	LibraryCopyOf     string // relative library path this copy was made from (non-empty if linked)
	LibrarySyncStatus string // 'ok', 'outdated', 'source_missing', or '' if unchecked
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

// groupBrowseEntries inserts collapsible type-group dividers into a pre-sorted entry list.
// Directories (already first) are left ungrouped. Files are bucketed as:
// Samples (.wav), Programs (.pgm/.all/.txt), Sequences (.seq), Songs (.sng), Other (everything else).
// Groups with no files are omitted. Input must already be sorted (dirs first, then files alpha).
func groupBrowseEntries(entries []BrowseEntry) []BrowseEntry {
	type groupDef struct {
		key   string
		label string
	}
	order := []groupDef{
		{"samples", "Samples"},
		{"programs", "Programs"},
		{"sequences", "Sequences"},
		{"songs", "Songs"},
		{"other", "Other"},
	}
	extGroup := func(ext string) string {
		switch ext {
		case ".wav":
			return "samples"
		case ".pgm", ".all", ".txt":
			return "programs"
		case ".seq":
			return "sequences"
		case ".sng":
			return "songs"
		default:
			return "other"
		}
	}

	buckets := map[string][]BrowseEntry{}
	var out []BrowseEntry
	for _, e := range entries {
		if e.IsDir {
			out = append(out, e)
			continue
		}
		g := extGroup(e.Ext)
		e.Group = g
		buckets[g] = append(buckets[g], e)
	}
	for _, gd := range order {
		files := buckets[gd.key]
		if len(files) == 0 {
			continue
		}
		out = append(out, BrowseEntry{
			Divider:      true,
			DividerLabel: gd.label,
			Collapsible:  true,
			DividerGroup: gd.key,
		})
		out = append(out, files...)
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
	s.enrichBrowseEntries(entries, workspace, absDir)
	entries = sortBrowseEntries(entries, sortMode)
	if filterCtx == "browse" && sortMode != "label" {
		entries = groupBrowseEntries(entries)
	}
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
// badge fields (MissingSamples for .pgm, WavInfo for .wav, library flags).
// All entries live in absDir; catalog data is fetched with three batched
// queries instead of several per file.
func (s *Server) enrichBrowseEntries(entries []BrowseEntry, workspace, absDir string) {
	ctx := context.Background()
	libDir := s.libDir()

	prefix := ""
	if rel, err := filepath.Rel(workspace, absDir); err == nil && rel != "." {
		prefix = rel + "/"
	}

	fileByPath := map[string]db.ListFilesWithWavMetaForPrefixRow{}
	if rows, err := s.queries.ListFilesWithWavMetaForPrefix(ctx, prefix); err == nil {
		for _, r := range rows {
			fileByPath[r.Path] = r
		}
	}
	missingByID := map[int64]int64{}
	if rows, err := s.queries.CountMissingSamplesForPrefix(ctx, prefix); err == nil {
		for _, r := range rows {
			missingByID[r.PgmFileID] = r.Missing
		}
	}
	linkByPath := map[string]db.SampleLink{}
	if links, err := s.queries.ListSampleLinksForDir(ctx, prefix+"%"); err == nil {
		for _, l := range links {
			linkByPath[l.CopyPath] = l
		}
	}

	for i := range entries {
		e := &entries[i]
		if e.IsDir {
			e.IsProject = dirContainsPGM(e.Path)
			e.IsLibraryRoot = filepath.Clean(e.Path) == filepath.Clean(libDir)
			e.IsLibrary = s.isUnderLibrary(e.Path)
			continue
		}

		e.IsLibrary = s.isUnderLibrary(e.Path)

		relPath, err := filepath.Rel(workspace, e.Path)
		if err != nil {
			continue
		}

		// Check if this copy has a library link.
		if !e.IsLibrary {
			if link, ok := linkByPath[relPath]; ok {
				e.LibraryCopyOf = link.LibraryPath
				e.LibrarySyncStatus = link.SyncStatus
			}
		}

		f, ok := fileByPath[relPath]
		if !ok {
			continue
		}
		e.FileID = f.ID

		switch e.Ext {
		case ".pgm":
			e.MissingSamples = missingByID[f.ID]
		case ".wav":
			if f.SampleRate > 0 {
				ch := "mono"
				if f.Channels == 2 {
					ch = "stereo"
				}
				e.WavInfo = fmt.Sprintf("%dHz %dbit %s", f.SampleRate, f.BitsPerSample, ch)
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
			if err := s.session.Program.Save(s.session.FilePath); err != nil {
				log.Printf("save program after path change: %v", err)
			}
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
			if err := s.session.Program.Save(s.session.FilePath); err != nil {
				log.Printf("save program after path change: %v", err)
			}
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
	_ = s.queries.RenameSampleLink(ctx, db.RenameSampleLinkParams{NewPath: newRel, OldPath: oldRel})

	// For directories, update all files under the old path prefix in one statement.
	oldPrefix := oldRel + string(filepath.Separator)
	newPrefix := newRel + string(filepath.Separator)
	if err := s.queries.MoveFilePathPrefix(ctx, db.MoveFilePathPrefixParams{
		NewPrefix: newPrefix,
		OldPrefix: oldPrefix,
	}); err != nil {
		log.Printf("update catalog path prefix %q: %v", oldPrefix, err)
	}
	_ = s.queries.MoveSampleLinkPrefix(ctx, db.MoveSampleLinkPrefixParams{
		NewPrefix: newPrefix,
		OldPrefix: oldPrefix,
	})
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
		if err := s.session.Program.Save(s.session.FilePath); err != nil {
			log.Printf("save program after organize: %v", err)
		}
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
	_ = s.queries.DeleteSampleLinkByCopyPath(ctx, relPath)
	dirPrefix := relPath + string(filepath.Separator)
	_ = s.queries.DeleteFilesByPathPrefix(ctx, dirPrefix)
	_ = s.queries.DeleteSampleLinksByPathPrefix(ctx, dirPrefix)

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
			if err := s.session.Program.Save(s.session.FilePath); err != nil {
				log.Printf("save program after delete: %v", err)
			}
		}
	}

	w.Header().Set("HX-Trigger", `{"refreshBrowser":true,"invalidateSampleCache":true}`)
	w.WriteHeader(http.StatusOK)
}

// searchCatalog queries the catalog for WAV files matching q and/or chips (subcategory filters).
// Rank order (when q is non-empty): exact subcategory → exact category → partial subcategory →
// partial category → tag match → filename match. Favorites float above within each tier.
// chips narrows results to specific subcategories. favoritesOnly adds an AND f.favorite=1 filter.
func (s *Server) searchCatalog(ctx context.Context, q string, chips []string, favoritesOnly bool) ([]db.File, error) {
	if q == "" && len(chips) == 0 && favoritesOnly {
		return s.queries.ListFavorites(ctx)
	}

	qLike := "%" + strings.ToLower(q) + "%"

	var whereParts []string
	var whereArgs []interface{}

	if q != "" {
		whereParts = append(whereParts, `(LOWER(f.path) LIKE ? OR LOWER(f.category) LIKE ? OR LOWER(f.subcategory) LIKE ? OR LOWER(ft.tag_value) LIKE ?)`)
		whereArgs = append(whereArgs, qLike, qLike, qLike, qLike)
	}
	if len(chips) > 0 {
		ph := make([]string, len(chips))
		for i, c := range chips {
			ph[i] = "?"
			whereArgs = append(whereArgs, strings.ToLower(c))
		}
		whereParts = append(whereParts, `LOWER(f.subcategory) IN (`+strings.Join(ph, ",")+`)`)
	}
	if favoritesOnly {
		whereParts = append(whereParts, "f.favorite = 1")
	}

	orderBy := "f.favorite DESC"
	var orderArgs []interface{}
	if q != "" {
		orderBy += `, CASE
		    WHEN LOWER(f.subcategory) = LOWER(?) THEN 0
		    WHEN LOWER(f.category)    = LOWER(?) THEN 1
		    WHEN LOWER(f.subcategory) LIKE ?      THEN 2
		    WHEN LOWER(f.category)    LIKE ?      THEN 3
		    WHEN MIN(LOWER(ft.tag_value)) LIKE ?  THEN 4
		    ELSE 5
		END`
		orderArgs = append(orderArgs, q, q, qLike, qLike, qLike)
	}
	orderBy += ", f.path"

	sqlStr := `SELECT DISTINCT f.id, f.path, f.file_type, f.size, f.mod_time, f.scanned_at,
		f.color, f.category, f.subcategory, f.favorite
	FROM files f
	LEFT JOIN file_tags ft ON ft.file_id = f.id
	WHERE ` + strings.Join(whereParts, " AND ") + `
	GROUP BY f.id
	ORDER BY ` + orderBy + `
	LIMIT ` + strconv.Itoa(searchResultLimit)

	allArgs := make([]any, 0, len(whereArgs)+len(orderArgs))
	allArgs = append(allArgs, whereArgs...)
	allArgs = append(allArgs, orderArgs...)
	rows, err := s.sqlDB.QueryContext(ctx, sqlStr, allArgs...)
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

// searchDirs walks the workspace and returns directories whose names contain q (case-insensitive).
// Capped at 20 results to keep response time bounded.
func (s *Server) searchDirs(workspace, q string) []BrowseEntry {
	qLower := strings.ToLower(q)
	var results []BrowseEntry
	_ = filepath.WalkDir(workspace, func(path string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() || path == workspace {
			return nil
		}
		name := d.Name()
		if strings.HasPrefix(name, ".") {
			return filepath.SkipDir
		}
		if strings.Contains(strings.ToLower(name), qLower) {
			relPath, _ := filepath.Rel(workspace, path)
			dir := filepath.Dir(relPath)
			if dir == "." {
				dir = ""
			}
			results = append(results, BrowseEntry{
				Name:       name,
				Path:       path,
				RelPath:    relPath,
				RelDirPath: dir,
				IsDir:      true,
				IsProject:  dirContainsPGM(path),
			})
			if len(results) >= 20 {
				return fmt.Errorf("limit")
			}
		}
		return nil
	})
	return results
}

// handleBrowseSearch searches the catalog for WAV files matching q, chips, and/or favorites.
// GET /browse/search?q=...&chips=kick&chips=snare&favorites=1
func (s *Server) handleBrowseSearch(w http.ResponseWriter, r *http.Request) {
	workspace := s.session.WorkspacePath
	if workspace == "" {
		http.Error(w, "no workspace configured", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	q := strings.TrimSpace(r.FormValue("q"))
	chips := r.Form["chips"]
	favoritesOnly := r.FormValue("favorites") == "1"

	// Nothing active — fall back to directory nav.
	if q == "" && len(chips) == 0 && !favoritesOnly {
		data, err := s.buildBrowseData("", "browse", s.session.SelectedDetailPath, "")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.renderTemplate(w, "file_browser_nav.html", data)
		return
	}

	ctx := r.Context()
	files, err := s.searchCatalog(ctx, q, chips, favoritesOnly)
	if err != nil {
		http.Error(w, "search failed", http.StatusInternalServerError)
		return
	}

	var entries []BrowseEntry
	if q != "" && len(chips) == 0 && !favoritesOnly {
		entries = append(entries, s.searchDirs(workspace, q)...)
	}
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

	var labelParts []string
	if q != "" {
		labelParts = append(labelParts, q)
	}
	labelParts = append(labelParts, chips...)
	if favoritesOnly {
		labelParts = append(labelParts, "★")
	}
	data := BrowseData{
		SearchQuery:  strings.Join(labelParts, " + "),
		SearchCapped: len(files) >= searchResultLimit,
		Entries:      entries,
		Workspace:    workspace,
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
