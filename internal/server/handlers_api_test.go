package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/maxgarvey/mpc_editor/internal/db"
)

func TestHandleAPISamples_Empty(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/api/samples", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var samples []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &samples); err != nil {
		t.Fatalf("JSON parse: %v", err)
	}
}

func TestHandleAPISamples_WithEntries(t *testing.T) {
	srv := testServer(t)

	// Seed a WAV file into the catalog
	ctx := context.Background()
	srv.queries.UpsertFile(ctx, db.UpsertFileParams{ //nolint:errcheck // test setup
		Path: "samples/kick.wav", FileType: "wav", Size: 8000, ModTime: 1,
	})

	req := httptest.NewRequest("GET", "/api/samples", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	var samples []map[string]any
	json.Unmarshal(w.Body.Bytes(), &samples) //nolint:errcheck // test setup
	if len(samples) != 1 {
		t.Errorf("samples = %d, want 1", len(samples))
	}
}

func TestHandleAPIPrograms_Empty(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/api/programs", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func TestHandleAPIPrograms_WithEntries(t *testing.T) {
	srv := testServer(t)

	ctx := context.Background()
	srv.queries.UpsertFile(ctx, db.UpsertFileParams{ //nolint:errcheck // test setup
		Path: "beats/drum_kit.pgm", FileType: "pgm", Size: 10756, ModTime: 1,
	})

	req := httptest.NewRequest("GET", "/api/programs", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	var programs []map[string]any
	json.Unmarshal(w.Body.Bytes(), &programs) //nolint:errcheck // test setup
	if len(programs) != 1 {
		t.Errorf("programs = %d, want 1", len(programs))
	}
}

func TestHandleAPIProgramPads(t *testing.T) {
	srv := testServer(t)
	pgmPath := testdataPath("test.pgm")

	req := httptest.NewRequest("GET", "/api/program-pads?path="+url.QueryEscape(pgmPath), http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q", ct)
	}
}

func TestHandleAPIPadParams(t *testing.T) {
	srv := testServer(t)
	pgmPath := testdataPath("test.pgm")

	req := httptest.NewRequest("GET", "/api/pad-params/0?pgm="+url.QueryEscape(pgmPath), http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestIsAllowedImportExt(t *testing.T) {
	allowed := []string{".wav", ".pgm", ".seq", ".mid", ".sng", ".all"}
	for _, ext := range allowed {
		if !isAllowedImportExt(ext) {
			t.Errorf("isAllowedImportExt(%q) = false, want true", ext)
		}
	}
	notAllowed := []string{".txt", ".xyz", ".doc", ""}
	for _, ext := range notAllowed {
		if isAllowedImportExt(ext) {
			t.Errorf("isAllowedImportExt(%q) = true, want false", ext)
		}
	}
}

func TestHandleImportFormats(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/workspace/import/formats", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("JSON parse: %v", err)
	}
	if _, ok := resp["audio"]; !ok {
		t.Error("response missing 'audio'")
	}
	if _, ok := resp["project"]; !ok {
		t.Error("response missing 'project'")
	}
}

func TestHandleImportDirScan(t *testing.T) {
	srv := testServer(t)

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "kick.wav"), []byte("RIFF"), 0o644) //nolint:errcheck // test setup
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hi"), 0o644)  //nolint:errcheck // test setup

	req := httptest.NewRequest("GET", "/workspace/import/scan?dir="+url.QueryEscape(dir), http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("JSON: %v", err)
	}
	if count, _ := resp["count"].(float64); count < 1 {
		t.Errorf("count = %v, want >= 1 (kick.wav)", count)
	}
}

func TestHandleImportDirScan_MissingDir(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/workspace/import/scan", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandleImportDirScan_InvalidDir(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/workspace/import/scan?dir=/nonexistent/xyz", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandleWorkspaceScan(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("POST", "/workspace/scan", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestHandleWorkspaceScan_NoWorkspace(t *testing.T) {
	srv := testServer(t)
	srv.session.WorkspacePath = ""

	req := httptest.NewRequest("POST", "/workspace/scan", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (no workspace)", w.Code)
	}
}

func TestHandleWorkspaceScan_MethodNotAllowed(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/workspace/scan", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestHandleAPIAssignToProgram(t *testing.T) {
	srv := testServer(t)

	// Copy test.pgm to workspace so we can write to it
	pgmDst := filepath.Join(srv.session.WorkspacePath, "test.pgm")
	pgmData, err := os.ReadFile(testdataPath("test.pgm"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pgmDst, pgmData, 0o644); err != nil {
		t.Fatal(err)
	}

	form := url.Values{
		"pgm_path": {pgmDst},
		"wav_path": {testdataPath("chh.wav")},
		"pad":      {"0"},
	}
	req := httptest.NewRequest("POST", "/api/assign-to-program", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q", ct)
	}
}

func TestHandleAPIAssignToProgram_MethodNotAllowed(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/api/assign-to-program", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestHandleAPIAssignToProgram_MissingParams(t *testing.T) {
	srv := testServer(t)

	form := url.Values{"pgm_path": {"test.pgm"}}
	req := httptest.NewRequest("POST", "/api/assign-to-program", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// TestHandleAPIAssignToProgram_LongName exercises the sampleName > 16 truncation branch.
func TestHandleAPIAssignToProgram_LongName(t *testing.T) {
	srv := testServer(t)

	pgmDst := filepath.Join(srv.session.WorkspacePath, "test.pgm")
	pgmData, err := os.ReadFile(testdataPath("test.pgm"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pgmDst, pgmData, 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a WAV with a long name (>16 chars before extension)
	wavData, err := os.ReadFile(testdataPath("chh.wav"))
	if err != nil {
		t.Fatal(err)
	}
	longName := "very_long_sample_name_over_limit.wav" // well over 16 chars
	longWavPath := filepath.Join(t.TempDir(), longName)
	if err := os.WriteFile(longWavPath, wavData, 0o644); err != nil {
		t.Fatal(err)
	}

	form := url.Values{
		"pgm_path": {pgmDst},
		"wav_path": {longWavPath},
		"pad":      {"1"},
	}
	req := httptest.NewRequest("POST", "/api/assign-to-program", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
}

// TestHandleAPIAssignToProgram_SessionPGM exercises the isSessionPgm == true branch.
func TestHandleAPIAssignToProgram_SessionPGM(t *testing.T) {
	srv := testServer(t)

	pgmDst := filepath.Join(srv.session.WorkspacePath, "test.pgm")
	pgmData, err := os.ReadFile(testdataPath("test.pgm"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pgmDst, pgmData, 0o644); err != nil {
		t.Fatal(err)
	}

	openForm := url.Values{"path": {pgmDst}}
	openReq := httptest.NewRequest("POST", "/program/open", strings.NewReader(openForm.Encode()))
	openReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	srv.Handler().ServeHTTP(httptest.NewRecorder(), openReq)

	form := url.Values{
		"pgm_path": {pgmDst},
		"wav_path": {testdataPath("chh.wav")},
		"pad":      {"0"},
	}
	req := httptest.NewRequest("POST", "/api/assign-to-program", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
}

// TestHandleAPIPrograms_WithCurrentSession exercises the currentRel path in handleAPIPrograms.
func TestHandleAPIPrograms_WithCurrentSession(t *testing.T) {
	srv := testServer(t)

	pgmDst := filepath.Join(srv.session.WorkspacePath, "test.pgm")
	pgmData, err := os.ReadFile(testdataPath("test.pgm"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pgmDst, pgmData, 0o644); err != nil {
		t.Fatal(err)
	}

	// Seed catalog without overwriting the real PGM file
	ctx := context.Background()
	if _, err := srv.queries.UpsertFile(ctx, db.UpsertFileParams{
		Path: "test.pgm", FileType: "pgm", Size: int64(len(pgmData)), ModTime: 1,
	}); err != nil {
		t.Fatal(err)
	}

	openForm := url.Values{"path": {pgmDst}}
	openReq := httptest.NewRequest("POST", "/program/open", strings.NewReader(openForm.Encode()))
	openReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	srv.Handler().ServeHTTP(httptest.NewRecorder(), openReq)

	req := httptest.NewRequest("GET", "/api/programs", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	var programs []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &programs); err != nil {
		t.Fatalf("JSON: %v", err)
	}
	if len(programs) != 1 {
		t.Fatalf("programs = %d, want 1", len(programs))
	}
	if current, _ := programs[0]["current"].(bool); !current {
		t.Error("expected current=true for active session program")
	}
}
