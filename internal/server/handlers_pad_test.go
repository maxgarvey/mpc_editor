package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func openTestProgram(t *testing.T, srv *Server) {
	t.Helper()
	form := url.Values{"path": {testdataPath("test.pgm")}}
	req := httptest.NewRequest("POST", "/program/open", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	srv.Handler().ServeHTTP(httptest.NewRecorder(), req)
}

func TestHandlePadParams_Post(t *testing.T) {
	srv := testServer(t)
	openTestProgram(t, srv)

	form := url.Values{
		"attack":       {"10"},
		"decay":        {"50"},
		"decay_mode":   {"1"},
		"mixer_level":  {"100"},
		"mixer_pan":    {"64"},
		"voice_overlap": {"1"},
		"mute_group":   {"0"},
		"midi_note":    {"60"},
	}
	req := httptest.NewRequest("POST", "/pad/params", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestHandlePadParams_MethodNotAllowed(t *testing.T) {
	srv := testServer(t)
	openTestProgram(t, srv)

	req := httptest.NewRequest("GET", "/pad/params", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestHandlePadParamsPartial(t *testing.T) {
	srv := testServer(t)
	openTestProgram(t, srv)

	req := httptest.NewRequest("GET", "/partials/pad-params", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestHandlePadGrid(t *testing.T) {
	srv := testServer(t)
	openTestProgram(t, srv)

	req := httptest.NewRequest("GET", "/partials/pad-grid?bank=0", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestHandlePadSelect(t *testing.T) {
	srv := testServer(t)
	openTestProgram(t, srv)

	req := httptest.NewRequest("GET", "/pad/5", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	if srv.session.SelectedPad != 5 {
		t.Errorf("SelectedPad = %d, want 5", srv.session.SelectedPad)
	}
}

func TestHandlePadSelect_InvalidIndex(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/pad/999", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandlePadSelect_NegativeIndex(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/pad/-1", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandleLayerUpdate_AllFields(t *testing.T) {
	srv := testServer(t)

	form := url.Values{
		"level":     {"80"},
		"tuning":    {"0.5"},
		"play_mode": {"1"},
	}
	req := httptest.NewRequest("POST", "/pad/layer/1", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	layer := srv.session.Program.Pad(0).Layer(1)
	if layer.GetLevel() != 80 {
		t.Errorf("level = %d, want 80", layer.GetLevel())
	}
	if layer.GetPlayMode() != 1 {
		t.Errorf("play_mode = %d, want 1", layer.GetPlayMode())
	}
}

func TestHandleLayerUpdate_InvalidLayer(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("POST", "/pad/layer/99", http.NoBody)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandleLayerUpdate_MethodNotAllowed(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/pad/layer/0", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}
