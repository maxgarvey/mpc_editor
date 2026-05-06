package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/maxgarvey/mpc_editor/internal/seq"
)

func TestSessionHasProgram(t *testing.T) {
	srv := testServer(t)
	if !srv.session.HasProgram() {
		t.Error("new session should have a program (blank)")
	}
}

func TestIsTempDir(t *testing.T) {
	if !isTempDir(t.TempDir()) {
		t.Error("TempDir should be detected as temp")
	}
	if isTempDir("/usr/local/bin") {
		t.Error("/usr/local/bin should not be a temp dir")
	}
}

func TestDefaultWorkspacePath(t *testing.T) {
	p := defaultWorkspacePath()
	if p == "" {
		t.Error("defaultWorkspacePath should return a non-empty path")
	}
}

// makeTestSeq creates a seq file large enough for WriteEvents.
func makeTestSeq(t *testing.T) string {
	t.Helper()
	data := seq.Create(120.0, 2, "TestSeq", "", false, nil)
	p := filepath.Join(t.TempDir(), "test.SEQ")
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestHandleSetWavSource(t *testing.T) {
	srv := testServer(t)
	id := seedFile(t, srv, "mysample.wav", "wav")

	form := url.Values{"id": {itoa(id)}, "source": {"from vinyl"}}
	req := httptest.NewRequest("POST", "/file/source", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
}

func TestHandleSetWavSource_InvalidID(t *testing.T) {
	srv := testServer(t)

	form := url.Values{"id": {"notanumber"}, "source": {"test"}}
	req := httptest.NewRequest("POST", "/file/source", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandleSequenceEventEdit_Toggle(t *testing.T) {
	srv := testServer(t)
	tmp := makeTestSeq(t)

	form := url.Values{
		"path":   {tmp},
		"action": {"toggle"},
		"bar":    {"1"},
		"pad":    {"0"},
		"step":   {"0"},
	}
	req := httptest.NewRequest("POST", "/sequence/event/edit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
}

func TestHandleSequenceEventEdit_Move(t *testing.T) {
	srv := testServer(t)
	tmp := makeTestSeq(t)

	form := url.Values{
		"path":      {tmp},
		"action":    {"move"},
		"from_bar":  {"1"},
		"from_pad":  {"0"},
		"from_step": {"0"},
		"to_bar":    {"1"},
		"to_pad":    {"1"},
		"to_step":   {"2"},
	}
	req := httptest.NewRequest("POST", "/sequence/event/edit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
}

// TestHandleSequenceEventEdit_Toggle_WithEvents toggles at a tick that already has an event,
// covering the loop-body match (existing=true+continue) and non-match (append) branches.
// It also uses bar=0 (triggers bar<1 clamp) and tick=0 param (triggers rawTick override).
func TestHandleSequenceEventEdit_Toggle_WithEvents(t *testing.T) {
	srv := testServer(t)

	events := []seq.Event{
		{Tick: 0, Type: seq.EventNoteOn, Note: 35, Velocity: 100, Duration: 23},
		{Tick: 24, Type: seq.EventNoteOn, Note: 36, Velocity: 100, Duration: 23},
	}
	data := seq.Create(120.0, 2, "Test", "", false, events)
	tmp := filepath.Join(t.TempDir(), "with_events.SEQ")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		t.Fatal(err)
	}

	form := url.Values{
		"path":   {tmp},
		"action": {"toggle"},
		"bar":    {"0"},
		"pad":    {"0"},
		"step":   {"0"},
		"tick":   {"0"},
	}
	req := httptest.NewRequest("POST", "/sequence/event/edit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
}

// TestHandleSequenceEventEdit_Toggle_PadOutOfRange uses padIndex >= PadCount() (64)
// to cover the padToNote fallback return path (padIndex+35).
func TestHandleSequenceEventEdit_Toggle_PadOutOfRange(t *testing.T) {
	srv := testServer(t)
	tmp := makeTestSeq(t)

	form := url.Values{
		"path":   {tmp},
		"action": {"toggle"},
		"bar":    {"1"},
		"pad":    {"64"},
		"step":   {"0"},
	}
	req := httptest.NewRequest("POST", "/sequence/event/edit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
}

func TestHandleSequenceEventEdit_MethodNotAllowed(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/sequence/event/edit", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestHandleSequenceEventEdit_MissingPath(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("POST", "/sequence/event/edit", http.NoBody)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}
