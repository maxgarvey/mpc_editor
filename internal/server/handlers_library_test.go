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

func TestHandleLibraryCheck_MethodNotAllowed(t *testing.T) {
	srv := testServer(t)
	req := httptest.NewRequest(http.MethodGet, "/library/check", http.NoBody)
	w := httptest.NewRecorder()
	srv.handleLibraryCheck(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("got %d, want 405", w.Code)
	}
}

func TestHandleLibraryCheck_MissingPath(t *testing.T) {
	srv := testServer(t)
	req := httptest.NewRequest(http.MethodPost, "/library/check", http.NoBody)
	w := httptest.NewRecorder()
	srv.handleLibraryCheck(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

func TestHandleLibraryCheck_UpdatesStatus(t *testing.T) {
	srv := testServer(t)
	workspace := srv.session.WorkspacePath
	ctx := context.Background()

	// Create source in library.
	libDir := filepath.Join(workspace, "sample_library")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatal(err)
	}
	srcPath := filepath.Join(libDir, "kick.wav")
	if err := os.WriteFile(srcPath, makeMinimalWAV(), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create copy in programs dir.
	pgmDir := filepath.Join(workspace, "programs", "beat")
	if err := os.MkdirAll(pgmDir, 0o755); err != nil {
		t.Fatal(err)
	}
	copyPath := filepath.Join(pgmDir, "kick.wav")
	if err := os.WriteFile(copyPath, makeMinimalWAV(), 0o644); err != nil {
		t.Fatal(err)
	}

	// Seed a link with no sync_status and matching checksum.
	checksum, _ := checksumFile(srcPath)
	if err := srv.queries.UpsertSampleLink(ctx, db.UpsertSampleLinkParams{
		CopyPath:    filepath.Join("programs", "beat", "kick.wav"),
		LibraryPath: filepath.Join("sample_library", "kick.wav"),
		Checksum:    checksum,
		CopiedAt:    1000,
	}); err != nil {
		t.Fatal(err)
	}

	form := url.Values{"path": {copyPath}}
	req := httptest.NewRequest(http.MethodPost, "/library/check",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.handleLibraryCheck(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", w.Code)
	}

	// The link should now have sync_status = 'ok'.
	link, err := srv.queries.GetSampleLinkByCopyPath(ctx, filepath.Join("programs", "beat", "kick.wav"))
	if err != nil {
		t.Fatal(err)
	}
	if link.SyncStatus != "ok" {
		t.Errorf("SyncStatus = %q, want 'ok'", link.SyncStatus)
	}
}

func TestCheckSyncStatus_SourceMissing(t *testing.T) {
	srv := testServer(t)
	workspace := srv.session.WorkspacePath
	ctx := context.Background()

	// Seed a link pointing to a non-existent source.
	copyRel := filepath.Join("programs", "beat", "snare.wav")
	if err := srv.queries.UpsertSampleLink(ctx, db.UpsertSampleLinkParams{
		CopyPath:    copyRel,
		LibraryPath: filepath.Join("sample_library", "snare.wav"),
		Checksum:    "abc123",
		CopiedAt:    1000,
	}); err != nil {
		t.Fatal(err)
	}
	_ = workspace

	status := srv.checkSyncStatus(ctx, copyRel)
	if status != "source_missing" {
		t.Errorf("got %q, want 'source_missing'", status)
	}
}

func TestHandleLibraryUpdate_MethodNotAllowed(t *testing.T) {
	srv := testServer(t)
	req := httptest.NewRequest(http.MethodGet, "/library/update", http.NoBody)
	w := httptest.NewRecorder()
	srv.handleLibraryUpdate(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("got %d, want 405", w.Code)
	}
}

func TestHandleLibraryUpdate_NoLink(t *testing.T) {
	srv := testServer(t)
	workspace := srv.session.WorkspacePath

	copyPath := filepath.Join(workspace, "programs", "beat", "nonexistent.wav")
	form := url.Values{"path": {copyPath}}
	req := httptest.NewRequest(http.MethodPost, "/library/update",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.handleLibraryUpdate(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

// makeMinimalWAV returns a tiny valid-looking byte slice for test WAV files.
func makeMinimalWAV() []byte {
	// 44-byte PCM WAV header for a silent 1-sample mono 44100Hz 16-bit file.
	b := make([]byte, 44)
	copy(b[0:4], "RIFF")
	b[4], b[5], b[6], b[7] = 36, 0, 0, 0 // chunk size
	copy(b[8:12], "WAVE")
	copy(b[12:16], "fmt ")
	b[16] = 16                                       // subchunk size
	b[20], b[21] = 1, 0                              // PCM
	b[22], b[23] = 1, 0                              // mono
	b[24], b[25], b[26], b[27] = 0x44, 0xAC, 0, 0    // 44100
	b[28], b[29], b[30], b[31] = 0x88, 0x58, 0x01, 0 // byte rate
	b[32], b[33] = 2, 0                              // block align
	b[34], b[35] = 16, 0                             // bits per sample
	copy(b[36:40], "data")
	b[40], b[41], b[42], b[43] = 0, 0, 0, 0 // data chunk size
	return b
}

func TestHandleLibraryUpdate_RecopiesOutdatedSource(t *testing.T) {
	srv := testServer(t)
	workspace := srv.session.WorkspacePath
	ctx := context.Background()

	libDir := filepath.Join(workspace, "sample_library")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatal(err)
	}
	srcPath := filepath.Join(libDir, "kick.wav")
	if err := os.WriteFile(srcPath, makeMinimalWAV(), 0o644); err != nil {
		t.Fatal(err)
	}

	pgmDir := filepath.Join(workspace, "programs", "beat")
	if err := os.MkdirAll(pgmDir, 0o755); err != nil {
		t.Fatal(err)
	}
	copyPath := filepath.Join(pgmDir, "kick.wav")
	if err := os.WriteFile(copyPath, []byte("stale copy"), 0o644); err != nil {
		t.Fatal(err)
	}

	relCopy := filepath.Join("programs", "beat", "kick.wav")
	if err := srv.queries.UpsertSampleLink(ctx, db.UpsertSampleLinkParams{
		CopyPath:    relCopy,
		LibraryPath: filepath.Join("sample_library", "kick.wav"),
		Checksum:    "old-checksum",
		CopiedAt:    1000,
	}); err != nil {
		t.Fatal(err)
	}

	form := url.Values{"path": {copyPath}}
	req := httptest.NewRequest(http.MethodPost, "/library/update", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200; body: %s", w.Code, w.Body.String())
	}

	// The copy must now match the library source byte-for-byte.
	srcData, _ := os.ReadFile(srcPath)
	copyData, err := os.ReadFile(copyPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(copyData) != string(srcData) {
		t.Error("copy should be overwritten with the library source")
	}

	// The link should record the fresh checksum and 'ok' status.
	link, err := srv.queries.GetSampleLinkByCopyPath(ctx, relCopy)
	if err != nil {
		t.Fatal(err)
	}
	wantChecksum, _ := checksumFile(srcPath)
	if link.Checksum != wantChecksum {
		t.Errorf("checksum = %q, want fresh source checksum", link.Checksum)
	}
	if link.SyncStatus != "ok" {
		t.Errorf("SyncStatus = %q, want ok", link.SyncStatus)
	}
}

func TestHandleLibraryUpdate_SourceMissing(t *testing.T) {
	srv := testServer(t)
	workspace := srv.session.WorkspacePath
	ctx := context.Background()

	copyPath := filepath.Join(workspace, "programs", "ghost.wav")
	if err := os.MkdirAll(filepath.Dir(copyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(copyPath, makeMinimalWAV(), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := srv.queries.UpsertSampleLink(ctx, db.UpsertSampleLinkParams{
		CopyPath:    filepath.Join("programs", "ghost.wav"),
		LibraryPath: filepath.Join("sample_library", "gone.wav"),
	}); err != nil {
		t.Fatal(err)
	}

	form := url.Values{"path": {copyPath}}
	req := httptest.NewRequest(http.MethodPost, "/library/update", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400 when library source is missing", w.Code)
	}
}
