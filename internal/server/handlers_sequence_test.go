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
	"github.com/maxgarvey/mpc_editor/internal/seq"
)

func TestHandleSequencePage_HTMX(t *testing.T) {
	srv := testServer(t)
	seqPath := testdataPath("test.seq")

	req := httptest.NewRequest("GET", "/sequence?path="+url.QueryEscape(seqPath), http.NoBody)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "step-grid") {
		t.Error("HTMX response missing step-grid")
	}
}

func TestHandleSequencePage_Full(t *testing.T) {
	srv := testServer(t)
	seqPath := testdataPath("test.seq")

	req := httptest.NewRequest("GET", "/sequence?path="+url.QueryEscape(seqPath), http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestHandleSequencePage_MissingPath(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/sequence", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	// Handler renders error in template rather than returning 400
	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "path is required") {
		t.Error("response missing 'path is required' error")
	}
}

func TestHandleSequencePage_InvalidPath(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/sequence?path=/nonexistent/file.seq", http.NoBody)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	// Error should render in the template, not as a 500
	if w.Code == http.StatusInternalServerError {
		t.Errorf("got 500; should render error gracefully")
	}
}

func TestHandleSequencePage_WithDivisionAndTSig(t *testing.T) {
	srv := testServer(t)
	seqPath := testdataPath("test.seq")

	req := httptest.NewRequest("GET",
		"/sequence?path="+url.QueryEscape(seqPath)+"&tsig=3_4&division=48",
		http.NoBody)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestHandleSequenceEvents(t *testing.T) {
	srv := testServer(t)
	seqPath := testdataPath("test.seq")

	req := httptest.NewRequest("GET",
		"/sequence/events?path="+url.QueryEscape(seqPath)+"&bar=0",
		http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("JSON parse: %v", err)
	}
	if _, ok := resp["bpm"]; !ok {
		t.Error("response missing 'bpm'")
	}
	if _, ok := resp["events"]; !ok {
		t.Error("response missing 'events'")
	}
}

func TestHandleSequenceEvents_MissingPath(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/sequence/events", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandleSequenceEvents_SpecificBar(t *testing.T) {
	srv := testServer(t)
	seqPath := testdataPath("test.seq")

	req := httptest.NewRequest("GET",
		"/sequence/events?path="+url.QueryEscape(seqPath)+"&bar=1",
		http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp) //nolint:errcheck // test setup
	if resp["currentBar"] != float64(1) {
		t.Errorf("currentBar = %v, want 1", resp["currentBar"])
	}
}

func TestHandleSequenceUpdate(t *testing.T) {
	srv := testServer(t)

	// Copy test.seq to a temp file so we can modify it without side effects
	tmp := copySeqToTemp(t, testdataPath("test.seq"))

	form := url.Values{
		"path": {tmp},
		"bpm":  {"95.5"},
		"bars": {"2"},
	}
	req := httptest.NewRequest("POST", "/sequence/update", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "step-grid") {
		t.Error("response missing step-grid")
	}
}

func TestHandleSequenceUpdate_MissingPath(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("POST", "/sequence/update", http.NoBody)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandleSequenceUpdate_NoBPMChange(t *testing.T) {
	srv := testServer(t)
	tmp := copySeqToTemp(t, testdataPath("test.seq"))

	// Post without bpm/bars — should render without patching
	form := url.Values{"path": {tmp}}
	req := httptest.NewRequest("POST", "/sequence/update", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestHandleSequenceToggleLoop(t *testing.T) {
	srv := testServer(t)
	tmp := copySeqToTemp(t, testdataPath("test.seq"))

	form := url.Values{"path": {tmp}}
	req := httptest.NewRequest("POST", "/sequence/toggle-loop", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "loop") {
		t.Error("response missing loop field")
	}
}

// TestHandleSequenceToggleLoop_LoopOn uses a SEQ with Loop=true so the toggle
// sets it to false, covering the `{"loop":false}` response branch.
func TestHandleSequenceToggleLoop_LoopOn(t *testing.T) {
	srv := testServer(t)
	tmp := copySeqToTemp(t, testdataPath(filepath.Join("seq", "verify_loop_on.SEQ")))

	form := url.Values{"path": {tmp}}
	req := httptest.NewRequest("POST", "/sequence/toggle-loop", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"loop":false`) {
		t.Errorf("expected loop:false, got: %s", w.Body.String())
	}
}

func TestHandleSequenceToggleLoop_MethodNotAllowed(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/sequence/toggle-loop", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestHandleSequenceToggleLoop_MissingPath(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("POST", "/sequence/toggle-loop", http.NoBody)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandleSequenceNew(t *testing.T) {
	srv := testServer(t)

	form := url.Values{"name": {"MySeq"}, "dir": {srv.session.WorkspacePath}}
	req := httptest.NewRequest("POST", "/sequence/new", strings.NewReader(form.Encode()))
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
	if _, ok := resp["seq_abs"]; !ok {
		t.Error("response missing seq_abs")
	}
}

func TestHandleSequenceNew_MethodNotAllowed(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/sequence/new", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestHandleSequenceNew_InvalidName(t *testing.T) {
	srv := testServer(t)

	tests := []string{"", "bad/slash", "../escape", strings.Repeat("a", 17)}
	for _, name := range tests {
		form := url.Values{"name": {name}}
		req := httptest.NewRequest("POST", "/sequence/new", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, req)
		if w.Code == 200 {
			t.Errorf("name=%q: expected error status, got 200", name)
		}
	}
}

func TestQuantizeTick(t *testing.T) {
	tests := []struct {
		tick uint32
		q    int
		want uint32
	}{
		{0, 24, 0},
		{12, 24, 24},
		{11, 24, 0},
		{24, 24, 24},
		{36, 24, 48},
		{100, 48, 96},
	}
	for _, tc := range tests {
		got := quantizeTick(tc.tick, tc.q)
		if got != tc.want {
			t.Errorf("quantizeTick(%d, %d) = %d, want %d", tc.tick, tc.q, got, tc.want)
		}
	}
}

func TestAllPadSampleNames_Empty(t *testing.T) {
	srv := testServer(t)
	names := srv.allPadSampleNames("")
	if len(names) != 64 {
		t.Fatalf("len = %d, want 64", len(names))
	}
	for i, n := range names {
		if n != "" {
			t.Errorf("names[%d] = %q, want empty", i, n)
		}
	}
}

func TestAllPadSampleNames_WithPGM(t *testing.T) {
	srv := testServer(t)
	names := srv.allPadSampleNames(testdataPath("test.pgm"))
	if len(names) != 64 {
		t.Fatalf("len = %d, want 64", len(names))
	}
	// test.pgm has at least some samples assigned
	hasName := false
	for _, n := range names {
		if n != "" {
			hasName = true
			break
		}
	}
	if !hasName {
		t.Error("expected at least one pad sample name from test.pgm")
	}
}

func TestHandleSequenceEventEdit_Delete(t *testing.T) {
	srv := testServer(t)
	tmp := makeTestSeq(t)

	form := url.Values{
		"path":   {tmp},
		"action": {"delete"},
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

func TestHandleSequenceEventEdit_Update(t *testing.T) {
	srv := testServer(t)
	tmp := makeTestSeq(t)

	form := url.Values{
		"path":     {tmp},
		"action":   {"update"},
		"bar":      {"1"},
		"pad":      {"0"},
		"step":     {"0"},
		"velocity": {"80"},
		"duration": {"12"},
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

func TestHandleSequenceEventEdit_MultiDelete(t *testing.T) {
	srv := testServer(t)
	tmp := makeTestSeq(t)

	form := url.Values{
		"path":   {tmp},
		"action": {"multi_delete"},
		"events": {`[{"pad":0,"global_step":0}]`},
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

func TestHandleSequenceEventEdit_MultiMove(t *testing.T) {
	srv := testServer(t)
	tmp := makeTestSeq(t)

	form := url.Values{
		"path":   {tmp},
		"action": {"multi_move"},
		"events": {`[{"pad":0,"global_step":0,"to_pad":1,"to_global_step":2}]`},
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

func TestPadForNoteFunc_WithPGM(t *testing.T) {
	srv := testServer(t)
	openTestProgram(t, srv)
	pgmPath := srv.session.FilePath
	rel, _ := filepath.Rel(srv.session.WorkspacePath, pgmPath)

	fn := srv.padForNoteFunc(rel)
	// Note 35 maps to pad 0 (chromatic default for uninitialized pads)
	pad := fn(35)
	if pad < 0 || pad >= 64 {
		t.Errorf("padForNoteFunc(35) = %d, out of range", pad)
	}
}

func TestHandleSequenceEventEdit_Quantize(t *testing.T) {
	srv := testServer(t)
	tmp := makeTestSeq(t)

	form := url.Values{
		"path":   {tmp},
		"action": {"quantize"},
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

func TestHandleSequenceEventEdit_MultiUpdate(t *testing.T) {
	srv := testServer(t)
	tmp := makeTestSeq(t)

	form := url.Values{
		"path":     {tmp},
		"action":   {"multi_update"},
		"events":   {`[{"pad":0,"global_step":0}]`},
		"velocity": {"75"},
		"duration": {"16"},
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

func TestHandleSequenceEventEdit_MultiQuantize(t *testing.T) {
	srv := testServer(t)
	tmp := makeTestSeq(t)

	form := url.Values{
		"path":   {tmp},
		"action": {"multi_quantize"},
		"events": {`[{"pad":0,"global_step":0}]`},
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

func TestHandleSequenceEventEdit_UnknownAction(t *testing.T) {
	srv := testServer(t)
	tmp := makeTestSeq(t)

	form := url.Values{
		"path":   {tmp},
		"action": {"bad_action"},
	}
	req := httptest.NewRequest("POST", "/sequence/event/edit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (unknown action)", w.Code)
	}
}

func TestHandleSequenceEvents_WithPGM(t *testing.T) {
	srv := testServer(t)
	seqPath := testdataPath("test.seq")
	pgmPath := testdataPath("test.pgm")

	req := httptest.NewRequest("GET",
		"/sequence/events?path="+url.QueryEscape(seqPath)+"&pgm="+url.QueryEscape(pgmPath),
		http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp) //nolint:errcheck // test setup
	if _, ok := resp["padSampleNames"]; !ok {
		t.Error("response missing padSampleNames")
	}
}

func TestHandleSequenceEvents_InvalidSeq(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/sequence/events?path=/nonexistent/bad.seq", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestSessionPgmRelPath_WithSession(t *testing.T) {
	srv := testServer(t)

	// Open test program so session has a FilePath
	openTestProgram(t, srv)
	relPath := srv.sessionPgmRelPath()
	// The test.pgm is in testdata (not workspace), so relPath may be absolute or non-empty
	if relPath == "" && srv.session.FilePath != "" {
		t.Error("expected non-empty relPath when session has a file path")
	}
}

// TestHandleSequenceNew_NoDirWithFilePath exercises the branch where dir is empty
// but session.FilePath is set, so the SEQ is created next to the open program.
// TestHandleSequenceEvents_WithEvents uses a SEQ file that contains NoteOn events
// to cover the event-processing loop body in handleSequenceEvents.
func TestHandleSequenceEvents_WithEvents(t *testing.T) {
	srv := testServer(t)
	seqPath := testdataPath(filepath.Join("seq", "general_quarter_notes.SEQ"))

	req := httptest.NewRequest("GET",
		"/sequence/events?path="+url.QueryEscape(seqPath)+"&bar=0",
		http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	var resp struct {
		Events []map[string]any `json:"events"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("JSON parse: %v", err)
	}
	if len(resp.Events) == 0 {
		t.Error("expected events in response from general_quarter_notes.SEQ")
	}
}

func TestHandleSequenceNew_NoDirWithFilePath(t *testing.T) {
	srv := testServer(t)

	pgmDest := filepath.Join(srv.session.WorkspacePath, "test.pgm")
	pgmData, err := os.ReadFile(testdataPath("test.pgm"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pgmDest, pgmData, 0o644); err != nil {
		t.Fatal(err)
	}
	openForm := url.Values{"path": {pgmDest}}
	openReq := httptest.NewRequest("POST", "/program/open", strings.NewReader(openForm.Encode()))
	openReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	srv.Handler().ServeHTTP(httptest.NewRecorder(), openReq)

	// No "dir" param → handler uses filepath.Dir(session.FilePath)
	form := url.Values{"name": {"WorkspaceSeq"}}
	req := httptest.NewRequest("POST", "/sequence/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
}

// TestHandleSequenceUpdate_WithWorkspaceSEQ exercises the catalog update block that
// runs when BPM or bars change and the SEQ lives inside the workspace.
func TestHandleSequenceUpdate_WithWorkspaceSEQ(t *testing.T) {
	srv := testServer(t)

	// Copy test.seq into workspace so the catalog relPath lookup succeeds
	seqData, err := os.ReadFile(testdataPath("test.seq"))
	if err != nil {
		t.Fatal(err)
	}
	seqDest := filepath.Join(srv.session.WorkspacePath, "ws.seq")
	if err := os.WriteFile(seqDest, seqData, 0o644); err != nil {
		t.Fatal(err)
	}

	// Seed catalog entry without overwriting the file (seedFile would clobber it)
	ctx := context.Background()
	if _, err := srv.queries.UpsertFile(ctx, db.UpsertFileParams{
		Path: "ws.seq", FileType: "seq", Size: int64(len(seqData)), ModTime: 1,
	}); err != nil {
		t.Fatal(err)
	}

	form := url.Values{
		"path": {seqDest},
		"bpm":  {"110.0"},
		"bars": {"4"},
	}
	req := httptest.NewRequest("POST", "/sequence/update", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "step-grid") {
		t.Error("response missing step-grid")
	}
}

// TestHandleSequenceUpdate_OOBTagsSection verifies that when BPM changes and the SEQ
// is in the workspace catalog, the response includes an OOB tags section element that
// reflects the updated auto-tags.
func TestHandleSequenceUpdate_OOBTagsSection(t *testing.T) {
	srv := testServer(t)

	seqData, err := os.ReadFile(testdataPath("test.seq"))
	if err != nil {
		t.Fatal(err)
	}
	seqDest := filepath.Join(srv.session.WorkspacePath, "oob.seq")
	if err := os.WriteFile(seqDest, seqData, 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if _, err := srv.queries.UpsertFile(ctx, db.UpsertFileParams{
		Path: "oob.seq", FileType: "seq", Size: int64(len(seqData)), ModTime: 1,
	}); err != nil {
		t.Fatal(err)
	}

	form := url.Values{
		"path": {seqDest},
		"bpm":  {"120.0"},
		"bars": {"2"},
	}
	req := httptest.NewRequest("POST", "/sequence/update", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, `hx-swap-oob="outerHTML:#tags-section"`) {
		t.Error("response missing OOB tags-section swap attribute")
	}
	if !strings.Contains(body, "bpm") {
		t.Error("response missing bpm auto-tag")
	}
	if !strings.Contains(body, "bars") {
		t.Error("response missing bars auto-tag")
	}
}

// TestHandleSequencePage_WithCatalogPGM seeds a PGM in the catalog so that
// pgmFilesInWorkspace returns non-empty results, covering the loop-body append path.
func TestHandleSequencePage_WithCatalogPGM(t *testing.T) {
	srv := testServer(t)

	ctx := context.Background()
	if _, err := srv.queries.UpsertFile(ctx, db.UpsertFileParams{
		Path: "kit.pgm", FileType: "pgm", Size: 1024, ModTime: 1,
	}); err != nil {
		t.Fatal(err)
	}

	seqPath := testdataPath("test.seq")
	req := httptest.NewRequest("GET", "/sequence?path="+url.QueryEscape(seqPath), http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func copySeqToTemp(t *testing.T, src string) string {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir() + "/copy.seq"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return tmp
}

// Editing a sequence through the handler must keep its program change and
// other non-note records (they were silently dropped on every save before).
func TestHandleSequenceEventEdit_KeepsNonNoteEvents(t *testing.T) {
	srv := testServer(t)
	path := filepath.Join(t.TempDir(), "pc.SEQ")
	data := seq.Create(120, 1, "", "", true, []seq.Event{
		{Tick: 0, Type: seq.EventProgramChange, Note: 0},                        // maps to pad 0 under the fallback
		{Tick: 0, Type: seq.EventNoteOn, Note: 35, Velocity: 100, Duration: 23}, // pad A1
		{Tick: 96, Type: seq.EventNoteOn, Note: 36, Velocity: 100, Duration: 23},
		{Tick: 192, Type: 0x80, Status: 0x82, Note: 3},
	})
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	form := url.Values{"path": {path}, "action": {"delete"}, "bar": {"1"}, "pad": {"0"}, "step": {"0"}}
	req := httptest.NewRequest("POST", "/sequence/event/edit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}

	s, err := seq.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	var pcs, x82, notes int
	for _, e := range s.Events {
		switch {
		case e.Status == 0xC0:
			pcs++
		case e.Status == 0x82:
			x82++
		case e.IsNote():
			notes++
			if e.Tick == 0 {
				t.Errorf("note at tick 0 should have been deleted: %+v", e)
			}
		}
	}
	if pcs != 1 || x82 != 1 || notes != 1 {
		t.Fatalf("program changes=%d 0x82=%d notes=%d, want 1/1/1", pcs, x82, notes)
	}
}

// writeMeterSeq writes a sequence whose clock map has the given bar lengths
// (zero-filled after the end tick, as in Akai's factory "Standard" sequence).
func writeMeterSeq(t *testing.T, lengths []int, events []seq.Event) string {
	t.Helper()
	data := seq.Create(95, len(lengths), "", "", false, events)
	tick := 0
	for n := 0; n < 1000; n++ {
		off := 0x30 + n*4
		if n > len(lengths) {
			copy(data[off:off+4], []byte{0, 0, 0, 0})
			continue
		}
		data[off], data[off+1], data[off+2], data[off+3] = byte(tick), byte(tick>>8), byte(tick>>16), 0x60
		if n < len(lengths) {
			tick += lengths[n]
		}
	}
	path := filepath.Join(t.TempDir(), "meter.SEQ")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestGridParamsFor(t *testing.T) {
	waltz, _ := seq.Open(writeMeterSeq(t, []int{288, 288}, nil))
	mixed, _ := seq.Open(writeMeterSeq(t, []int{384, 288, 384}, nil))
	odd, _ := seq.Open(writeMeterSeq(t, []int{100, 100}, nil))

	cases := []struct {
		name     string
		query    string
		s        *seq.Sequence
		wantTSig string
		wantTPB  int
		wantNote bool
	}{
		{"explicit wins", "tsig=5_4", waltz, "5_4", 480, false},
		{"derived 3/4", "", waltz, "3_4", 288, false},
		{"mixed uses first bar", "", mixed, "4_4", 384, true},
		{"unknown length falls back", "", odd, "4_4", 384, false},
		{"nil sequence", "", nil, "4_4", 384, false},
		{"bad tsig", "tsig=x", waltz, "4_4", 384, false},
	}
	for _, c := range cases {
		r := httptest.NewRequest("GET", "/?"+c.query, http.NoBody)
		gp, tsig, div, note := gridParamsFor(r, c.s)
		if tsig != c.wantTSig || gp.TicksPerBar != c.wantTPB || div != "24" || (note != "") != c.wantNote {
			t.Errorf("%s: tsig=%q tpb=%d div=%q note=%q", c.name, tsig, gp.TicksPerBar, div, note)
		}
	}
}

func TestMeterNoteListsChanges(t *testing.T) {
	s, _ := seq.Open(writeMeterSeq(t, []int{384, 384, 288, 288, 384}, nil))
	note := meterNote(s)
	for _, want := range []string{"bar 1 4/4", "bar 3 3/4", "bar 5 4/4"} {
		if !strings.Contains(note, want) {
			t.Errorf("note %q missing %q", note, want)
		}
	}
}

func TestHandleSequencePage_MixedMeterNote(t *testing.T) {
	srv := testServer(t)
	path := writeMeterSeq(t, []int{384, 288}, nil)
	req := httptest.NewRequest("GET", "/sequence?path="+url.QueryEscape(path), http.NoBody)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "seq-meter-note") {
		t.Fatalf("status %d; meter note missing", w.Code)
	}
}

// The playback endpoint must use the clock map's length, not Bars × 384.
func TestHandleSequenceEvents_UsesClockMapLength(t *testing.T) {
	srv := testServer(t)
	path := writeMeterSeq(t, []int{288, 288}, []seq.Event{
		{Tick: 0, Type: seq.EventNoteOn, Note: 36, Velocity: 100, Duration: 10},
		{Tick: 500, Type: seq.EventNoteOn, Note: 36, Velocity: 100, Duration: 10},
		{Tick: 600, Type: seq.EventNoteOn, Note: 36, Velocity: 100, Duration: 10}, // past the 576-tick end
	})
	req := httptest.NewRequest("GET", "/sequence/events?path="+url.QueryEscape(path), http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	var resp sequenceEventsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.TotalTicks != 576 || resp.BeatsPerBar != 3 || resp.Bars != 2 || len(resp.Events) != 2 {
		t.Fatalf("totalTicks=%d beats=%d bars=%d events=%d", resp.TotalTicks, resp.BeatsPerBar, resp.Bars, len(resp.Events))
	}
}
