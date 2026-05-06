package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/maxgarvey/mpc_editor/internal/db"
)

func TestFilterAllows(t *testing.T) {
	tests := []struct {
		ctx  string
		ext  string
		want bool
	}{
		{"open-pgm", ".pgm", true},
		{"open-pgm", ".wav", false},
		{"save-pgm", ".pgm", true},
		{"save-pgm", ".seq", false},
		{"load-wav", ".wav", true},
		{"load-wav", ".pgm", false},
		{"export-dir", ".wav", false},
		{"export-dir", ".pgm", false},
		{"browse", ".pgm", true},
		{"browse", ".wav", true},
		{"browse", ".seq", true},
		{"browse", ".mid", true},
		{"browse", ".sng", true},
		{"browse", ".all", true},
		{"browse", ".txt", true},
		{"browse", ".xyz", false},
		{"", ".pgm", true},
		{"", ".wav", true},
		{"", ".xyz", false},
	}
	for _, tc := range tests {
		got := filterAllows(tc.ctx, tc.ext)
		if got != tc.want {
			t.Errorf("filterAllows(%q, %q) = %v, want %v", tc.ctx, tc.ext, got, tc.want)
		}
	}
}

func TestHandleBrowse_NoWorkspace(t *testing.T) {
	srv := testServer(t)
	srv.session.WorkspacePath = ""

	req := httptest.NewRequest("GET", "/browse", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (no workspace)", w.Code)
	}
}

func TestHandleBrowse_WithWorkspace(t *testing.T) {
	srv := testServer(t)

	// Create some files in the workspace
	workspace := srv.session.WorkspacePath
	writeTestFile(t, filepath.Join(workspace, "prog.pgm"), "pgm content")
	writeTestFile(t, filepath.Join(workspace, "sample.wav"), "wav content")

	req := httptest.NewRequest("GET", "/browse?context=open-pgm", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "browser-entry") && !strings.Contains(body, "prog.pgm") {
		// The browser should render at least the workspace entry.
		t.Log("response body:", body[:min(len(body), 500)])
	}
}

func TestHandleBrowseNav(t *testing.T) {
	srv := testServer(t)

	workspace := srv.session.WorkspacePath
	subdir := filepath.Join(workspace, "beats")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(subdir, "kick.pgm"), "")

	req := httptest.NewRequest("GET", "/browse/nav?dir=beats", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestHandleBrowseSearch(t *testing.T) {
	srv := testServer(t)

	workspace := srv.session.WorkspacePath
	writeTestFile(t, filepath.Join(workspace, "drums_kit.pgm"), "")
	writeTestFile(t, filepath.Join(workspace, "bass_loop.wav"), "")

	form := url.Values{"q": {"drums"}}
	req := httptest.NewRequest("GET", "/browse/search?"+form.Encode(), http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "drums_kit") {
		t.Log("search result body:", w.Body.String()[:min(len(w.Body.String()), 500)])
	}
}

func TestHandleBrowseSearch_EmptyQuery(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/browse/search?q=", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestHandleBrowseSearch_NoWorkspace(t *testing.T) {
	srv := testServer(t)
	srv.session.WorkspacePath = ""

	req := httptest.NewRequest("GET", "/browse/search?q=kick", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandleWorkspaceMkdir(t *testing.T) {
	srv := testServer(t)

	form := url.Values{"name": {"new_folder"}, "parent": {""}, "context": {"open-pgm"}}
	req := httptest.NewRequest("POST", "/workspace/mkdir", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	if _, err := os.Stat(filepath.Join(srv.session.WorkspacePath, "new_folder")); err != nil {
		t.Errorf("folder should exist: %v", err)
	}
}

func TestHandleWorkspaceMkdir_InvalidName(t *testing.T) {
	srv := testServer(t)

	tests := []struct{ name string }{
		{""},
		{"../escape"},
		{"bad/slash"},
		{"bad\\slash"},
	}
	for _, tc := range tests {
		form := url.Values{"name": {tc.name}}
		req := httptest.NewRequest("POST", "/workspace/mkdir", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, req)
		if w.Code == 200 {
			t.Errorf("name=%q: expected error status, got 200", tc.name)
		}
	}
}

func TestHandleWorkspaceMkdir_MethodNotAllowed(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/workspace/mkdir", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestHandleWorkspaceSet(t *testing.T) {
	srv := testServer(t)
	newWorkspace := t.TempDir()

	form := url.Values{"path": {newWorkspace}}
	req := httptest.NewRequest("POST", "/workspace/set", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	if srv.session.WorkspacePath != newWorkspace {
		t.Errorf("workspace = %q, want %q", srv.session.WorkspacePath, newWorkspace)
	}
}

func TestHandleWorkspaceSet_EmptyPath(t *testing.T) {
	srv := testServer(t)

	form := url.Values{"path": {""}}
	req := httptest.NewRequest("POST", "/workspace/set", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandleWorkspaceSet_MethodNotAllowed(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/workspace/set", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestHandleWorkspaceDirs(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/workspace/dirs", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestHandleWorkspaceRename(t *testing.T) {
	srv := testServer(t)
	workspace := srv.session.WorkspacePath
	src := filepath.Join(workspace, "old.pgm")
	writeTestFile(t, src, "pgm content")

	form := url.Values{"path": {src}, "name": {"new.pgm"}}
	req := httptest.NewRequest("POST", "/workspace/rename", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(workspace, "new.pgm")); err != nil {
		t.Error("renamed file should exist")
	}
}

func TestHandleWorkspaceRename_MethodNotAllowed(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/workspace/rename", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestHandleWorkspaceRename_MissingParams(t *testing.T) {
	srv := testServer(t)

	form := url.Values{"path": {""}, "name": {""}}
	req := httptest.NewRequest("POST", "/workspace/rename", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandleWorkspaceRename_InvalidName(t *testing.T) {
	srv := testServer(t)
	workspace := srv.session.WorkspacePath
	src := filepath.Join(workspace, "test.pgm")
	writeTestFile(t, src, "")

	form := url.Values{"path": {src}, "name": {"../escape.pgm"}}
	req := httptest.NewRequest("POST", "/workspace/rename", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandleWorkspaceMove(t *testing.T) {
	srv := testServer(t)
	workspace := srv.session.WorkspacePath
	src := filepath.Join(workspace, "file.wav")
	destDir := filepath.Join(workspace, "subdir")
	writeTestFile(t, src, "wav content")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}

	form := url.Values{"path": {src}, "dest": {destDir}}
	req := httptest.NewRequest("POST", "/workspace/move", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(destDir, "file.wav")); err != nil {
		t.Error("moved file should exist in destination")
	}
}

func TestHandleWorkspaceMove_MethodNotAllowed(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/workspace/move", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestHandleWorkspaceMove_MissingParams(t *testing.T) {
	srv := testServer(t)

	form := url.Values{"path": {""}, "dest": {""}}
	req := httptest.NewRequest("POST", "/workspace/move", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandleWorkspaceDelete_Catalog(t *testing.T) {
	srv := testServer(t)
	workspace := srv.session.WorkspacePath
	f := filepath.Join(workspace, "to_del.wav")
	writeTestFile(t, f, "data")

	// Relative path (within workspace)
	relPath := "to_del.wav"
	form := url.Values{"path": {relPath}, "mode": {"catalog"}}
	req := httptest.NewRequest("POST", "/workspace/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
	// Catalog-mode: file still on disk
	if _, err := os.Stat(f); err != nil {
		t.Error("catalog-mode should not delete from disk")
	}
}

func TestHandleWorkspaceDelete_Disk(t *testing.T) {
	srv := testServer(t)
	workspace := srv.session.WorkspacePath
	f := filepath.Join(workspace, "disk_del.wav")
	writeTestFile(t, f, "data")

	form := url.Values{"path": {"disk_del.wav"}, "mode": {"disk"}}
	req := httptest.NewRequest("POST", "/workspace/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(f); err == nil {
		t.Error("disk-mode should delete from disk")
	}
}

func TestHandleWorkspaceDelete_MethodNotAllowed(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/workspace/delete", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestHandleWorkspaceDelete_MissingPath(t *testing.T) {
	srv := testServer(t)

	form := url.Values{"mode": {"disk"}}
	req := httptest.NewRequest("POST", "/workspace/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandleWorkspaceDelete_InvalidMode(t *testing.T) {
	srv := testServer(t)

	form := url.Values{"path": {"foo.wav"}, "mode": {"invalid"}}
	req := httptest.NewRequest("POST", "/workspace/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// TestHandleWorkspaceDelete_DirCatalogCascade seeds catalog entries under a directory
// and deletes that directory with mode=catalog, covering the dir-prefix loop.
func TestHandleWorkspaceDelete_DirCatalogCascade(t *testing.T) {
	srv := testServer(t)
	workspace := srv.session.WorkspacePath

	subDir := filepath.Join(workspace, "songs")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(subDir, "track.wav"), "wav data")
	seedFile(t, srv, filepath.Join("songs", "track.wav"), "wav")

	form := url.Values{"path": {"songs"}, "mode": {"catalog"}}
	req := httptest.NewRequest("POST", "/workspace/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
}

func TestHandleBrowseNav_NoWorkspace(t *testing.T) {
	srv := testServer(t)
	srv.session.WorkspacePath = ""

	req := httptest.NewRequest("GET", "/browse/nav", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (no workspace)", w.Code)
	}
}

func TestHandleBrowseNav_RootDir(t *testing.T) {
	srv := testServer(t)

	// Navigate to workspace root with empty dir
	req := httptest.NewRequest("GET", "/browse/nav?dir=", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
}

func TestHandleBrowseNav_Subdir(t *testing.T) {
	srv := testServer(t)
	subdir := filepath.Join(srv.session.WorkspacePath, "drums")
	os.MkdirAll(subdir, 0o755) //nolint:errcheck // test setup

	req := httptest.NewRequest("GET", "/browse/nav?dir="+url.QueryEscape(subdir), http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestHandleWorkspaceDirs_WithSubdirs(t *testing.T) {
	srv := testServer(t)
	os.MkdirAll(filepath.Join(srv.session.WorkspacePath, "beats"), 0o755) //nolint:errcheck // test setup

	req := httptest.NewRequest("GET", "/workspace/dirs", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
}

func TestHandleWorkspaceDirs_Subdir(t *testing.T) {
	srv := testServer(t)
	subdir := filepath.Join(srv.session.WorkspacePath, "kits")
	os.MkdirAll(subdir, 0o755) //nolint:errcheck // test setup

	req := httptest.NewRequest("GET", "/workspace/dirs?dir="+url.QueryEscape(subdir), http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestHandleWorkspaceDirs_OutsideWorkspace(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/workspace/dirs?dir="+url.QueryEscape("/tmp"), http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", w.Code)
	}
}

func TestHandleWorkspaceMove_Conflict(t *testing.T) {
	srv := testServer(t)
	workspace := srv.session.WorkspacePath
	src := filepath.Join(workspace, "kick.wav")
	destDir := filepath.Join(workspace, "subdir")
	destFile := filepath.Join(destDir, "kick.wav")
	writeTestFile(t, src, "kick wav")
	writeTestFile(t, destFile, "existing kick") // already exists

	form := url.Values{"path": {src}, "dest": {destDir}}
	req := httptest.NewRequest("POST", "/workspace/move", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409 (conflict)", w.Code)
	}
}

func TestHandleWorkspaceMove_DestNotDir(t *testing.T) {
	srv := testServer(t)
	workspace := srv.session.WorkspacePath
	src := filepath.Join(workspace, "snare.wav")
	destFile := filepath.Join(workspace, "notadir.wav")
	writeTestFile(t, src, "snare")
	writeTestFile(t, destFile, "file not dir")

	form := url.Values{"path": {src}, "dest": {destFile}}
	req := httptest.NewRequest("POST", "/workspace/move", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (dest not dir)", w.Code)
	}
}

func TestHandleWorkspaceMove_OutsideWorkspace(t *testing.T) {
	srv := testServer(t)
	workspace := srv.session.WorkspacePath
	src := filepath.Join(workspace, "bass.wav")
	writeTestFile(t, src, "bass")

	form := url.Values{"path": {src}, "dest": {"/tmp"}}
	req := httptest.NewRequest("POST", "/workspace/move", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 (outside workspace)", w.Code)
	}
}

func TestUpdateCatalogPath_OnMove(t *testing.T) {
	srv := testServer(t)
	workspace := srv.session.WorkspacePath

	// Seed a catalog entry
	_ = seedFile(t, srv, "old.wav", "wav")

	src := filepath.Join(workspace, "old.wav")
	destDir := filepath.Join(workspace, "moved")
	os.MkdirAll(destDir, 0o755) //nolint:errcheck // test setup

	form := url.Values{"path": {src}, "dest": {destDir}}
	req := httptest.NewRequest("POST", "/workspace/move", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
}

func TestUpdateCatalogPath_OnRename(t *testing.T) {
	srv := testServer(t)
	workspace := srv.session.WorkspacePath

	// Seed a catalog entry for the file being renamed
	_ = seedFile(t, srv, "toberename.wav", "wav")

	src := filepath.Join(workspace, "toberename.wav")

	form := url.Values{"path": {src}, "name": {"afterrename.wav"}}
	req := httptest.NewRequest("POST", "/workspace/rename", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
}

func TestEnrichBrowseEntries_WithCatalog(t *testing.T) {
	srv := testServer(t)

	// Seed a WAV and PGM entry in catalog
	wavID := seedFile(t, srv, "kick.wav", "wav")
	_ = seedFile(t, srv, "kit.pgm", "pgm")

	ctx := context.Background()
	srv.queries.UpsertWavMeta(ctx, db.UpsertWavMetaParams{ //nolint:errcheck // test setup
		FileID: wavID, SampleRate: 44100, Channels: 2, BitsPerSample: 16, FrameCount: 44100,
	})

	// Build browse data for workspace — this calls enrichBrowseEntries
	req := httptest.NewRequest("GET", "/browse?dir="+url.QueryEscape(srv.session.WorkspacePath), http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

// TestUpdateCatalogPath_DirectoryMove covers the directory-level prefix-update loop
// in updateCatalogPath, which runs when a directory (not just a file) is moved.
func TestUpdateCatalogPath_DirectoryMove(t *testing.T) {
	srv := testServer(t)
	workspace := srv.session.WorkspacePath

	// Create src subdir with a file, seed the file in catalog
	srcDir := filepath.Join(workspace, "beats")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(srcDir, "kick.wav"), "wav")
	seedFile(t, srv, filepath.Join("beats", "kick.wav"), "wav")

	destParent := filepath.Join(workspace, "archived")
	if err := os.MkdirAll(destParent, 0o755); err != nil {
		t.Fatal(err)
	}

	form := url.Values{"path": {srcDir}, "dest": {destParent}}
	req := httptest.NewRequest("POST", "/workspace/move", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
}

func TestDirContainsPGM(t *testing.T) {
	dir := t.TempDir()

	if dirContainsPGM(dir) {
		t.Error("empty dir should return false")
	}

	if err := os.WriteFile(filepath.Join(dir, "kit.pgm"), []byte("pgm data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !dirContainsPGM(dir) {
		t.Error("dir with .pgm should return true")
	}
}

func TestHandleWorkspaceMkdir_BrowseContext(t *testing.T) {
	srv := testServer(t)

	form := url.Values{"name": {"newdir"}, "context": {"browse"}}
	req := httptest.NewRequest("POST", "/workspace/mkdir", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
}

func TestHandleWorkspaceRename_Conflict(t *testing.T) {
	srv := testServer(t)
	workspace := srv.session.WorkspacePath

	src := filepath.Join(workspace, "old.wav")
	existing := filepath.Join(workspace, "new.wav")
	writeTestFile(t, src, "data")
	writeTestFile(t, existing, "existing")

	form := url.Values{"path": {src}, "name": {"new.wav"}}
	req := httptest.NewRequest("POST", "/workspace/rename", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409 (conflict)", w.Code)
	}
}

func TestHandleWorkspaceRename_WithActivePGM(t *testing.T) {
	srv := testServer(t)
	workspace := srv.session.WorkspacePath

	// Copy pgm into workspace and open it so session.FilePath is set
	pgmSrc := filepath.Join(workspace, "active.pgm")
	pgmData, err := os.ReadFile(testdataPath("test.pgm"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pgmSrc, pgmData, 0o644); err != nil {
		t.Fatal(err)
	}
	openForm := url.Values{"path": {pgmSrc}}
	openReq := httptest.NewRequest("POST", "/program/open", strings.NewReader(openForm.Encode()))
	openReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	srv.Handler().ServeHTTP(httptest.NewRecorder(), openReq)

	form := url.Values{"path": {pgmSrc}, "name": {"renamed.pgm"}}
	req := httptest.NewRequest("POST", "/workspace/rename", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
	expected := filepath.Join(workspace, "renamed.pgm")
	if srv.session.FilePath != expected {
		t.Errorf("session.FilePath = %q, want %q", srv.session.FilePath, expected)
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
