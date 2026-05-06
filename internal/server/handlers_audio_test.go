package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleAudioFile(t *testing.T) {
	srv := testServer(t)
	wavPath := testdataPath("chh.wav")

	req := httptest.NewRequest("GET", "/audio/file?path="+url.QueryEscape(wavPath), http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "audio/wav") {
		t.Errorf("Content-Type = %q, want audio/wav", ct)
	}
}

func TestHandleAudioFile_MissingPath(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/audio/file", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandleAudioFile_NotWAV(t *testing.T) {
	srv := testServer(t)
	pgmPath := testdataPath("test.pgm")

	req := httptest.NewRequest("GET", "/audio/file?path="+url.QueryEscape(pgmPath), http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandleAudioWaveform(t *testing.T) {
	srv := testServer(t)
	wavPath := testdataPath("chh.wav")

	req := httptest.NewRequest("GET", "/audio/waveform?path="+url.QueryEscape(wavPath), http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func TestHandleAudioWaveform_MissingPath(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/audio/waveform", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandleAudioWaveform_SmallWidth(t *testing.T) {
	srv := testServer(t)
	wavPath := testdataPath("chh.wav")

	req := httptest.NewRequest("GET", "/audio/waveform?path="+url.QueryEscape(wavPath)+"&width=10", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d (small width should be clamped to 100)", w.Code)
	}
}

func TestHandleAudioWaveform_LargeWidth(t *testing.T) {
	srv := testServer(t)
	wavPath := testdataPath("chh.wav")

	req := httptest.NewRequest("GET", "/audio/waveform?path="+url.QueryEscape(wavPath)+"&width=9999", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d (large width should be clamped to 4000)", w.Code)
	}
}

func TestHandleAudioWaveform_InvalidWAV(t *testing.T) {
	srv := testServer(t)

	f, err := os.CreateTemp(t.TempDir(), "*.wav")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("not a wav file") //nolint:errcheck // test setup
	f.Close()                       //nolint:errcheck // test setup

	req := httptest.NewRequest("GET", "/audio/waveform?path="+url.QueryEscape(f.Name()), http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for invalid WAV", w.Code)
	}
}

func TestHandleAudioCrop_MethodNotAllowed(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/audio/crop", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestHandleAudioCrop_MissingPath(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("POST", "/audio/crop", http.NoBody)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func workspaceWAV(t *testing.T, srv *Server) string {
	t.Helper()
	src := testdataPath("chh.wav")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(srv.session.WorkspacePath, "chh.wav")
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return dst
}

func TestHandleAudioCrop_InvalidFrameRange(t *testing.T) {
	srv := testServer(t)
	wavPath := workspaceWAV(t, srv)

	form := url.Values{
		"path": {wavPath},
		"from": {"100"},
		"to":   {"50"},
		"mode": {"replace"},
	}
	req := httptest.NewRequest("POST", "/audio/crop", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (to <= from)", w.Code)
	}
}

func TestHandleAudioCrop_InvalidMode(t *testing.T) {
	srv := testServer(t)
	wavPath := workspaceWAV(t, srv)

	form := url.Values{
		"path": {wavPath},
		"from": {"0"},
		"to":   {"100"},
		"mode": {"bad"},
	}
	req := httptest.NewRequest("POST", "/audio/crop", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (invalid mode)", w.Code)
	}
}

func TestHandleAudioCrop_Replace(t *testing.T) {
	srv := testServer(t)
	wavPath := workspaceWAV(t, srv)

	form := url.Values{
		"path": {wavPath},
		"from": {"0"},
		"to":   {"100"},
		"mode": {"replace"},
	}
	req := httptest.NewRequest("POST", "/audio/crop", strings.NewReader(form.Encode()))
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
	if resp["path"] == nil {
		t.Error("response missing path")
	}
}

func TestHandleAudioCrop_Copy(t *testing.T) {
	srv := testServer(t)
	wavPath := workspaceWAV(t, srv)

	form := url.Values{
		"path": {wavPath},
		"from": {"0"},
		"to":   {"100"},
		"mode": {"copy"},
	}
	req := httptest.NewRequest("POST", "/audio/crop", strings.NewReader(form.Encode()))
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
	if resp["path"] == nil {
		t.Error("response missing path")
	}
}

func TestHandleAudioCrop_OutsideWorkspace(t *testing.T) {
	srv := testServer(t)

	// Use a path outside the workspace
	form := url.Values{
		"path": {testdataPath("chh.wav")},
		"from": {"0"},
		"to":   {"100"},
		"mode": {"replace"},
	}
	req := httptest.NewRequest("POST", "/audio/crop", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 (outside workspace)", w.Code)
	}
}

func TestHandleAudioSlice_Valid(t *testing.T) {
	srv := testServer(t)
	loadTestSlicer(t, srv) // myLoop.wav has 9 markers

	req := httptest.NewRequest("GET", "/audio/slice/0", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "audio/wav") {
		t.Errorf("Content-Type = %q, want audio/wav", ct)
	}
}

func TestHandleAudioSlice_OutOfRange(t *testing.T) {
	srv := testServer(t)
	loadTestSlicer(t, srv)

	req := httptest.NewRequest("GET", "/audio/slice/9999", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandleAudioSlice_InvalidIndex(t *testing.T) {
	srv := testServer(t)
	loadTestSlicer(t, srv)

	req := httptest.NewRequest("GET", "/audio/slice/bad", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}
