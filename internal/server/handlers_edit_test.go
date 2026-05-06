package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestHandleRemoveAllSamples(t *testing.T) {
	srv := testServer(t)
	openTestProgram(t, srv)

	req := httptest.NewRequest("POST", "/edit/remove-all-samples", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestHandleRemoveAllSamples_MethodNotAllowed(t *testing.T) {
	srv := testServer(t)
	openTestProgram(t, srv)

	req := httptest.NewRequest("GET", "/edit/remove-all-samples", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestHandleChromaticLayout(t *testing.T) {
	srv := testServer(t)
	openTestProgram(t, srv)

	req := httptest.NewRequest("POST", "/edit/chromatic-layout", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	// Verify MIDI note was set
	if srv.session.Program.Pad(0).GetMIDINote() != 35 {
		t.Errorf("pad 0 MIDI note = %d, want 35", srv.session.Program.Pad(0).GetMIDINote())
	}
}

func TestHandleChromaticLayout_MethodNotAllowed(t *testing.T) {
	srv := testServer(t)
	openTestProgram(t, srv)

	req := httptest.NewRequest("GET", "/edit/chromatic-layout", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestHandleCopySettingsToAll(t *testing.T) {
	srv := testServer(t)
	openTestProgram(t, srv)

	req := httptest.NewRequest("POST", "/edit/copy-settings-to-all", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestHandleCopySettingsToAll_MethodNotAllowed(t *testing.T) {
	srv := testServer(t)
	openTestProgram(t, srv)

	req := httptest.NewRequest("GET", "/edit/copy-settings-to-all", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestHandleProfileSwitch_MPC500(t *testing.T) {
	srv := testServer(t)

	form := url.Values{"profile": {"MPC500"}}
	req := httptest.NewRequest("POST", "/edit/profile", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestHandleProfileSwitch_MethodNotAllowed(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/edit/profile", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}
