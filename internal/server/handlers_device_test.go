package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleDeviceStatus(t *testing.T) {
	srv := testServer(t)

	// First poll should always render (sentinel initial state).
	req := httptest.NewRequest("GET", "/device/status", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("first poll status = %d, want 200", w.Code)
	}

	// Second poll with unchanged state should return 204.
	req2 := httptest.NewRequest("GET", "/device/status", http.NoBody)
	w2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w2, req2)

	if w2.Code != http.StatusNoContent {
		t.Errorf("second poll status = %d, want 204 (unchanged)", w2.Code)
	}
}

func TestHandleDeviceDetect(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("POST", "/device/detect", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestHandleDeviceDetect_MethodNotAllowed(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/device/detect", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestHandleDeviceLs_Workspace(t *testing.T) {
	srv := testServer(t)
	workspace := srv.session.WorkspacePath
	writeTestFile(t, filepath.Join(workspace, "test.pgm"), "")

	req := httptest.NewRequest("GET", "/device/ls?root=workspace", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestHandleDeviceLs_NoMPC(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/device/ls?root=mpc", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (no MPC device)", w.Code)
	}
}

func TestHandleDeviceLs_UnknownRoot(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/device/ls?root=unknown", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandleDeviceMkdir_Workspace(t *testing.T) {
	srv := testServer(t)

	form := url.Values{"root": {"workspace"}, "dir": {"newsubdir"}}
	req := httptest.NewRequest("POST", "/device/mkdir", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(srv.session.WorkspacePath, "newsubdir")); err != nil {
		t.Errorf("dir should exist: %v", err)
	}
}

func TestHandleDeviceMkdir_MethodNotAllowed(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/device/mkdir", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestIsWithin(t *testing.T) {
	if !isWithin("/a/b/c", "/a/b") {
		t.Error("/a/b/c should be within /a/b")
	}
	if !isWithin("/a/b", "/a/b") {
		t.Error("/a/b should be within itself")
	}
	if isWithin("/a/bc", "/a/b") {
		t.Error("/a/bc should not be within /a/b")
	}
	if isWithin("/x/y", "/a/b") {
		t.Error("/x/y should not be within /a/b")
	}
}

func TestCopyPath_File(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	srcFile := filepath.Join(src, "test.wav")
	writeTestFile(t, srcFile, "content")

	n, errs := copyPath(srcFile, filepath.Join(dst, "test.wav"))
	if n != 1 {
		t.Errorf("n = %d, want 1", n)
	}
	if len(errs) != 0 {
		t.Errorf("errs = %v", errs)
	}
}

func TestCopyPath_Dir(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "out")
	writeTestFile(t, filepath.Join(src, "a.wav"), "")
	writeTestFile(t, filepath.Join(src, "b.pgm"), "")

	n, errs := copyPath(src, dst)
	if n != 2 {
		t.Errorf("n = %d, want 2", n)
	}
	if len(errs) != 0 {
		t.Errorf("errs = %v", errs)
	}
}
