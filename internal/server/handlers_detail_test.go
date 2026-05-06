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

func TestHandleDetailSelect(t *testing.T) {
	srv := testServer(t)

	form := url.Values{"path": {testdataPath("test.pgm")}}
	req := httptest.NewRequest("POST", "/detail/select", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}
	if srv.session.SelectedDetailPath == "" {
		t.Error("SelectedDetailPath should be set")
	}
}

func TestHandleDetailSelect_MethodNotAllowed(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/detail/select", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestHandleDetail_Dispatch(t *testing.T) {
	srv := testServer(t)

	tests := []struct {
		name     string
		path     string
		wantBody string
	}{
		{"pgm", testdataPath("test.pgm"), "detail-pgm"},
		{"wav", testdataPath("chh.wav"), "detail-wav"},
		{"seq", testdataPath("test.seq"), "step-grid"},
		{"sng", testdataPath("test.sng"), "detail-sng"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/detail?path="+url.QueryEscape(tc.path), http.NoBody)
			w := httptest.NewRecorder()
			srv.Handler().ServeHTTP(w, req)
			if w.Code != 200 {
				t.Fatalf("status = %d, want 200 (body: %s)", w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tc.wantBody) {
				t.Errorf("body missing %q\nbody: %.500s", tc.wantBody, w.Body.String())
			}
		})
	}
}

func TestHandleDetail_Empty(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/detail", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestHandleDetail_TXT(t *testing.T) {
	srv := testServer(t)

	// Write a plain text file
	f, err := os.CreateTemp(t.TempDir(), "*.txt")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("hello world") //nolint:errcheck
	f.Close()

	req := httptest.NewRequest("GET", "/detail?path="+url.QueryEscape(f.Name()), http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "hello world") {
		t.Error("expected file content in response")
	}
}

func TestHandleDetail_TXTSampleReport(t *testing.T) {
	srv := testServer(t)

	reportContent := `Sample Report for test.pgm
1. KICK_001
  Pads: A1
  Status: found
  Audio: /sounds/KICK_001.wav
  Source: original
  Tags: percussion, kick
2. SNARE_001
  Pads: A2
  Status: NOT FOUND
`
	f, _ := os.CreateTemp(t.TempDir(), "*.txt")
	f.WriteString(reportContent) //nolint:errcheck
	f.Close()

	req := httptest.NewRequest("GET", "/detail?path="+url.QueryEscape(f.Name()), http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestHandleDetail_UnknownExtension(t *testing.T) {
	srv := testServer(t)

	f, _ := os.CreateTemp(t.TempDir(), "*.xyz")
	f.Close()

	req := httptest.NewRequest("GET", "/detail?path="+url.QueryEscape(f.Name()), http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestParseSampleReport(t *testing.T) {
	text := `Sample Report for prog.pgm
1. KICK_001
  Pads: A1
  Status: found
  Audio: /sounds/KICK_001.wav
  Source: original
  Tags: kick
  Also used in: prog2.pgm
2. SNARE_001
  Pads: A2
  Status: NOT FOUND
`
	entries := parseSampleReport(text)
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(entries))
	}

	e := entries[0]
	if e.Number != 1 {
		t.Errorf("entry[0].Number = %d, want 1", e.Number)
	}
	if e.Name != "KICK_001" {
		t.Errorf("entry[0].Name = %q, want KICK_001", e.Name)
	}
	if !e.Found {
		t.Error("entry[0].Found should be true")
	}
	if e.Status != "found" {
		t.Errorf("entry[0].Status = %q, want found", e.Status)
	}
	if e.Audio != "/sounds/KICK_001.wav" {
		t.Errorf("entry[0].Audio = %q", e.Audio)
	}
	if e.Source != "original" {
		t.Errorf("entry[0].Source = %q", e.Source)
	}
	if e.Tags != "kick" {
		t.Errorf("entry[0].Tags = %q", e.Tags)
	}
	if e.AlsoIn != "prog2.pgm" {
		t.Errorf("entry[0].AlsoIn = %q", e.AlsoIn)
	}
	if e.Pads != "A1" {
		t.Errorf("entry[0].Pads = %q", e.Pads)
	}

	e2 := entries[1]
	if e2.Found {
		t.Error("entry[1].Found should be false")
	}
	if e2.Status != "NOT FOUND" {
		t.Errorf("entry[1].Status = %q", e2.Status)
	}
}

func TestParseSampleReport_Empty(t *testing.T) {
	entries := parseSampleReport("")
	if len(entries) != 0 {
		t.Errorf("empty input = %d entries, want 0", len(entries))
	}
}

func TestParseSampleReport_NoEntries(t *testing.T) {
	entries := parseSampleReport("Sample Report for prog.pgm\nno entries here\n")
	if len(entries) != 0 {
		t.Errorf("= %d entries, want 0", len(entries))
	}
}

func TestRenderDetailSEQ_InvalidPath(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/detail?path="+url.QueryEscape("/nonexistent/file.seq"), http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	// Should render error state, not 500
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200 (error should be rendered in template)", w.Code)
	}
}

func TestRenderDetailWAV_WithCatalogEntry(t *testing.T) {
	srv := testServer(t)

	// Copy chh.wav into workspace so resolvePath + validateWithinWorkspace work
	wavData, err := os.ReadFile(testdataPath("chh.wav"))
	if err != nil {
		t.Fatal(err)
	}
	wavDest := filepath.Join(srv.session.WorkspacePath, "chh.wav")
	if err := os.WriteFile(wavDest, wavData, 0o644); err != nil {
		t.Fatal(err)
	}

	// Seed into catalog
	id := seedFile(t, srv, "chh.wav", "wav")

	// Add a tag so loadTags gets exercised
	ctx := context.Background()
	srv.queries.AddFileTag(ctx, db.AddFileTagParams{ //nolint:errcheck
		FileID: id, TagKey: "", TagValue: "hihat", Auto: 0,
	})

	// Seed wav meta so WavMeta branch executes
	srv.queries.UpsertWavMeta(ctx, db.UpsertWavMetaParams{ //nolint:errcheck
		FileID: id, SampleRate: 44100, Channels: 1, BitsPerSample: 16, FrameCount: 22050,
	})

	req := httptest.NewRequest("GET", "/detail?path="+url.QueryEscape(wavDest), http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "detail-wav") {
		t.Error("response missing detail-wav")
	}
}

func TestRenderDetailSNG_WithCatalogEntry(t *testing.T) {
	srv := testServer(t)

	sngData, err := os.ReadFile(testdataPath("test.sng"))
	if err != nil {
		t.Fatal(err)
	}
	sngDest := filepath.Join(srv.session.WorkspacePath, "test.sng")
	if err := os.WriteFile(sngDest, sngData, 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	srv.queries.UpsertFile(ctx, db.UpsertFileParams{ //nolint:errcheck
		Path: "test.sng", FileType: "sng", Size: int64(len(sngData)), ModTime: 1,
	})

	req := httptest.NewRequest("GET", "/detail?path="+url.QueryEscape(sngDest), http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "detail-sng") {
		t.Error("missing detail-sng in response")
	}
}

func TestRenderDetailFile_WithCatalogEntry(t *testing.T) {
	srv := testServer(t)

	content := []byte("binary content")
	fileDest := filepath.Join(srv.session.WorkspacePath, "test.xyz")
	if err := os.WriteFile(fileDest, content, 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	srv.queries.UpsertFile(ctx, db.UpsertFileParams{ //nolint:errcheck
		Path: "test.xyz", FileType: "xyz", Size: int64(len(content)), ModTime: 1,
	})

	req := httptest.NewRequest("GET", "/detail?path="+url.QueryEscape(fileDest), http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestRenderDetailPGM_WithWorkspaceFile(t *testing.T) {
	srv := testServer(t)

	// Copy test.pgm into the workspace so relative path lookup works
	pgmDest := filepath.Join(srv.session.WorkspacePath, "test.pgm")
	data, err := os.ReadFile(testdataPath("test.pgm"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pgmDest, data, 0o644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/detail?path="+url.QueryEscape(pgmDest), http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "detail-pgm") {
		t.Error("missing detail-pgm class in response")
	}
}
