package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func loadSlicerWAV(t *testing.T, srv *Server) {
	t.Helper()
	wavPath := testdataPath("chh.wav")
	form := url.Values{"path": {wavPath}}
	req := httptest.NewRequest("POST", "/slicer/load", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("slicer load: status = %d", w.Code)
	}
}

func TestHandleSlicerLoad(t *testing.T) {
	srv := testServer(t)
	wavPath := testdataPath("chh.wav")

	form := url.Values{"path": {wavPath}}
	req := httptest.NewRequest("POST", "/slicer/load", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	if srv.session.Slicer == nil {
		t.Error("slicer should be set after load")
	}
}

func TestHandleSlicerLoad_MethodNotAllowed(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/slicer/load", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestHandleSlicerLoad_MissingPath(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("POST", "/slicer/load", http.NoBody)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandleSlicerWaveform(t *testing.T) {
	srv := testServer(t)
	loadSlicerWAV(t, srv)

	req := httptest.NewRequest("GET", "/slicer/waveform?width=500", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestHandleSlicerSensitivity(t *testing.T) {
	srv := testServer(t)
	loadSlicerWAV(t, srv)

	form := url.Values{"sensitivity": {"150"}}
	req := httptest.NewRequest("POST", "/slicer/sensitivity", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestHandleSlicerSensitivity_NoSlicer(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("POST", "/slicer/sensitivity", http.NoBody)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestHandleSlicerMarker_Select(t *testing.T) {
	srv := testServer(t)
	loadSlicerWAV(t, srv)

	req := httptest.NewRequest("GET", "/slicer/marker/select?index=0", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestHandleSlicerMarker_Next(t *testing.T) {
	srv := testServer(t)
	loadSlicerWAV(t, srv)

	req := httptest.NewRequest("GET", "/slicer/marker/next", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestHandleSlicerMarker_Insert(t *testing.T) {
	srv := testServer(t)
	loadSlicerWAV(t, srv)

	req := httptest.NewRequest("POST", "/slicer/marker/insert", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestHandleSlicerMarker_Delete(t *testing.T) {
	srv := testServer(t)
	loadSlicerWAV(t, srv)

	req := httptest.NewRequest("POST", "/slicer/marker/delete", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestHandleSlicerMarker_Nudge(t *testing.T) {
	srv := testServer(t)
	loadSlicerWAV(t, srv)

	form := url.Values{"ticks": {"50"}}
	req := httptest.NewRequest("POST", "/slicer/marker/nudge", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestHandleSlicerMarker_Prev(t *testing.T) {
	srv := testServer(t)
	loadSlicerWAV(t, srv)

	req := httptest.NewRequest("GET", "/slicer/marker/prev", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestHandleSlicerMarker_DeleteMethodNotAllowed(t *testing.T) {
	srv := testServer(t)
	loadSlicerWAV(t, srv)

	req := httptest.NewRequest("GET", "/slicer/marker/delete", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestHandleSlicerMarker_InsertMethodNotAllowed(t *testing.T) {
	srv := testServer(t)
	loadSlicerWAV(t, srv)

	req := httptest.NewRequest("GET", "/slicer/marker/insert", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestHandleSlicerMarker_NudgeMethodNotAllowed(t *testing.T) {
	srv := testServer(t)
	loadSlicerWAV(t, srv)

	req := httptest.NewRequest("GET", "/slicer/marker/nudge", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestHandleSlicerMarker_NoSlicer(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/slicer/marker/select", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestHandleSlicerMarker_Unknown(t *testing.T) {
	srv := testServer(t)
	loadSlicerWAV(t, srv)

	req := httptest.NewRequest("GET", "/slicer/marker/unknown-action", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandleSlicerExport(t *testing.T) {
	srv := testServer(t)
	loadSlicerWAV(t, srv)

	form := url.Values{"dir": {srv.session.WorkspacePath}}
	req := httptest.NewRequest("POST", "/slicer/export", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
}

func TestHandleSlicerExport_MethodNotAllowed(t *testing.T) {
	srv := testServer(t)
	loadSlicerWAV(t, srv)

	req := httptest.NewRequest("GET", "/slicer/export", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestHandleSlicerExport_NoSlicer(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("POST", "/slicer/export", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}
