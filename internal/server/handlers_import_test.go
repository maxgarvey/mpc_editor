package server

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleImportDirExecute(t *testing.T) {
	srv := testServer(t)

	// Create source dir with a real WAV file (copy from testdata)
	srcDir := t.TempDir()
	wavSrc := testdataPath("chh.wav")
	data, err := os.ReadFile(wavSrc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "chh.wav"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	form := url.Values{
		"src_dir": {srcDir},
		"dest":    {filepath.Join(srv.session.WorkspacePath, "sample_library")},
		"flatten": {"1"},
	}
	req := httptest.NewRequest("POST", "/workspace/import/dir", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
}

func TestHandleImportDirExecute_MethodNotAllowed(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/workspace/import/dir", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestHandleImportDirExecute_MissingDir(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("POST", "/workspace/import/dir", http.NoBody)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestUniqueName(t *testing.T) {
	dir := t.TempDir()
	// No conflict
	if got := uniqueBaseName(dir, "kick", ".wav"); got != "kick" {
		t.Errorf("no conflict: got %q, want kick", got)
	}
	// Create the file to force a conflict
	os.WriteFile(filepath.Join(dir, "kick.wav"), []byte{}, 0o644) //nolint:errcheck // test setup
	if got := uniqueBaseName(dir, "kick", ".wav"); got != "kick_2" {
		t.Errorf("with conflict: got %q, want kick_2", got)
	}
}

func TestCopyFileImport(t *testing.T) {
	src := filepath.Join(t.TempDir(), "src.wav")
	dst := filepath.Join(t.TempDir(), "dst.wav")
	if err := os.WriteFile(src, []byte("WAV data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := copyFileImport(src, dst); err != nil {
		t.Fatalf("copyFileImport: %v", err)
	}
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "WAV data" {
		t.Errorf("content = %q, want WAV data", string(data))
	}
}

func TestHandleWorkspaceImport_MethodNotAllowed(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/workspace/import", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestHandleWorkspaceImport_NoFiles(t *testing.T) {
	srv := testServer(t)

	// POST with empty multipart form → "no files uploaded"
	req := httptest.NewRequest("POST", "/workspace/import", strings.NewReader(""))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	// Should fail at parse or no-files stage
	if w.Code == 200 {
		t.Error("expected non-200 with no files")
	}
}

func TestHandleImportDirExecute_NoFlatten(t *testing.T) {
	srv := testServer(t)

	// Create source dir with a subdirectory containing a WAV
	srcDir := t.TempDir()
	subDir := filepath.Join(srcDir, "kicks")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	wavData, err := os.ReadFile(testdataPath("chh.wav"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "chh.wav"), wavData, 0o644); err != nil {
		t.Fatal(err)
	}

	form := url.Values{
		"src_dir": {srcDir},
		"dest":    {filepath.Join(srv.session.WorkspacePath, "imported")},
		"flatten": {"0"},
	}
	req := httptest.NewRequest("POST", "/workspace/import/dir", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
}

func TestHandleImportDirExecute_WithSource(t *testing.T) {
	srv := testServer(t)

	srcDir := t.TempDir()
	wavData, err := os.ReadFile(testdataPath("chh.wav"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "chh.wav"), wavData, 0o644); err != nil {
		t.Fatal(err)
	}

	form := url.Values{
		"src_dir": {srcDir},
		"dest":    {filepath.Join(srv.session.WorkspacePath, "sourced")},
		"flatten": {"1"},
		"source":  {"vinyl"},
	}
	req := httptest.NewRequest("POST", "/workspace/import/dir", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
}

func TestHandleWorkspaceImport_OutsideWorkspace(t *testing.T) {
	srv := testServer(t)

	wavData, _ := os.ReadFile(testdataPath("chh.wav"))

	var body strings.Builder
	boundary := "boundary123"
	body.WriteString("--" + boundary + "\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"files\"; filename=\"chh.wav\"\r\n")
	body.WriteString("Content-Type: audio/wav\r\n\r\n")
	body.Write(wavData) //nolint:errcheck // test setup
	body.WriteString("\r\n")
	body.WriteString("--" + boundary + "\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"dest\"\r\n\r\n")
	body.WriteString("/tmp\r\n")
	body.WriteString("--" + boundary + "--\r\n")

	req := httptest.NewRequest("POST", "/workspace/import", strings.NewReader(body.String()))
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", w.Code)
	}
}

func TestHandleWorkspaceImport_WithWAV(t *testing.T) {
	srv := testServer(t)

	wavData, err := os.ReadFile(testdataPath("chh.wav"))
	if err != nil {
		t.Fatal(err)
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("files", "chh.wav")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(wavData); err != nil {
		t.Fatal(err)
	}
	if err := mw.WriteField("dest", srv.session.WorkspacePath); err != nil {
		t.Fatal(err)
	}
	mw.Close() //nolint:errcheck // test setup

	req := httptest.NewRequest("POST", "/workspace/import", &body)
	req.Header.Set("Content-Type", fmt.Sprintf("multipart/form-data; boundary=%s", mw.Boundary()))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
}

func TestHandleWorkspaceImport_WithSource(t *testing.T) {
	srv := testServer(t)

	wavData, err := os.ReadFile(testdataPath("chh.wav"))
	if err != nil {
		t.Fatal(err)
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, _ := mw.CreateFormFile("files", "snare.wav")
	part.Write(wavData)                              //nolint:errcheck // test setup
	mw.WriteField("dest", srv.session.WorkspacePath) //nolint:errcheck // test setup
	mw.WriteField("source", "from vinyl")            //nolint:errcheck // test setup
	mw.Close()                                       //nolint:errcheck // test setup

	req := httptest.NewRequest("POST", "/workspace/import", &body)
	req.Header.Set("Content-Type", fmt.Sprintf("multipart/form-data; boundary=%s", mw.Boundary()))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
}
