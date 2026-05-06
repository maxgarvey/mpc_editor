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

func TestHandleProjectNew(t *testing.T) {
	srv := testServer(t)

	form := url.Values{"name": {"MyProject"}}
	req := httptest.NewRequest("POST", "/project/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("JSON parse: %v", err)
	}
	if _, ok := resp["pgm_abs"]; !ok {
		t.Error("response missing pgm_abs")
	}
}

func TestHandleProjectNew_MethodNotAllowed(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/project/new", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestHandleProjectNew_EmptyName(t *testing.T) {
	srv := testServer(t)

	form := url.Values{"name": {""}}
	req := httptest.NewRequest("POST", "/project/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandleProjectNew_InvalidName(t *testing.T) {
	srv := testServer(t)

	tests := []string{"../escape", "bad/slash", "bad\\slash", strings.Repeat("a", 17)}
	for _, name := range tests {
		form := url.Values{"name": {name}}
		req := httptest.NewRequest("POST", "/project/new", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, req)
		if w.Code == 200 {
			t.Errorf("name=%q: expected error status, got 200", name)
		}
	}
}

func TestHandleProjectNew_WithParent(t *testing.T) {
	srv := testServer(t)

	// Use the workspace itself as parent
	parent := srv.session.WorkspacePath
	form := url.Values{"name": {"SubProj"}, "parent": {parent}}
	req := httptest.NewRequest("POST", "/project/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
}

func TestHandleSampleReport_NoProgram(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("POST", "/program/sample-report", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (no program)", w.Code)
	}
}

func TestHandleSampleReport_MethodNotAllowed(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/program/sample-report", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestHandleSampleReport_WithProgram(t *testing.T) {
	srv := testServer(t)

	// Open test.pgm and copy it to workspace so sample-report can write the output
	pgmSrc := testdataPath("test.pgm")
	pgmDest := filepath.Join(srv.session.WorkspacePath, "test.pgm")
	if err := copyFileForTest(pgmSrc, pgmDest); err != nil {
		t.Fatal(err)
	}

	// Open the program first
	form := url.Values{"path": {pgmDest}}
	openReq := httptest.NewRequest("POST", "/program/open", strings.NewReader(form.Encode()))
	openReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	srv.Handler().ServeHTTP(httptest.NewRecorder(), openReq)

	req := httptest.NewRequest("POST", "/program/sample-report", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
}

func TestHandleSampleReport_WithCatalogWAV(t *testing.T) {
	srv := testServer(t)

	// Copy test.pgm to workspace and open it
	pgmDest := filepath.Join(srv.session.WorkspacePath, "test.pgm")
	if err := copyFileForTest(testdataPath("test.pgm"), pgmDest); err != nil {
		t.Fatal(err)
	}
	form := url.Values{"path": {pgmDest}}
	openReq := httptest.NewRequest("POST", "/program/open", strings.NewReader(form.Encode()))
	openReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	srv.Handler().ServeHTTP(httptest.NewRecorder(), openReq)

	// Get the sample name from pad 0
	sampleName := srv.session.Program.Pad(0).Layer(0).GetSampleName()
	if sampleName == "" {
		t.Skip("test.pgm has no sample on pad 0")
	}

	// Seed the sample WAV in the catalog with metadata and tags
	ctx := context.Background()
	wavID := seedFile(t, srv, sampleName+".wav", "wav")
	srv.queries.UpsertWavMeta(ctx, db.UpsertWavMetaParams{ //nolint:errcheck
		FileID: wavID, SampleRate: 44100, Channels: 1, BitsPerSample: 16, FrameCount: 44100,
		Source: "from sample pack",
	})
	srv.queries.AddFileTag(ctx, db.AddFileTagParams{ //nolint:errcheck
		FileID: wavID, TagKey: "", TagValue: "kick", Auto: 0,
	})

	req := httptest.NewRequest("POST", "/program/sample-report", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Report saved to") {
		t.Errorf("response = %q, want 'Report saved to'", w.Body.String())
	}
}

func TestHandleProgramSave_NoProgram(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("POST", "/program/save", http.NoBody)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	// No program loaded → should return error
	if w.Code == 200 {
		t.Error("expected error status when no program is loaded")
	}
}

func TestHandleProgramNew_MethodNotAllowed(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/program/new", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func copyFileForTest(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
