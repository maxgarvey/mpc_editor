package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/maxgarvey/mpc_editor/internal/pgm"
	"github.com/maxgarvey/mpc_editor/web"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	templateFS, staticFS := web.FS()
	return New(templateFS, staticFS)
}

func testdataPath(name string) string {
	return filepath.Join("..", "..", "testdata", name)
}

func TestIndex(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "MPC Editor") {
		t.Error("missing title in response")
	}
	if !strings.Contains(body, "Bank A") {
		t.Error("missing Bank A tab")
	}
}

func TestProgramNew(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("POST", "/program/new", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if srv.session.FilePath != "" {
		t.Error("file path should be empty after new")
	}
}

func TestProgramOpenAndPadSelect(t *testing.T) {
	srv := testServer(t)

	// Open test.pgm
	form := url.Values{"path": {testdataPath("test.pgm")}}
	req := httptest.NewRequest("POST", "/program/open", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("open status = %d, want 200", w.Code)
	}
	if srv.session.FilePath == "" {
		t.Error("file path should be set after open")
	}

	// Verify pad 0 has a sample name
	name := srv.session.PadName(0)
	if name == "" {
		t.Error("pad 0 should have a sample name from test.pgm")
	}
	t.Logf("pad 0: %q", name)

	// Select pad 1
	req = httptest.NewRequest("GET", "/pad/1", http.NoBody)
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("pad select status = %d", w.Code)
	}
	if srv.session.SelectedPad != 1 {
		t.Errorf("selected pad = %d, want 1", srv.session.SelectedPad)
	}
}

func TestPadParams(t *testing.T) {
	srv := testServer(t)

	// Set mute group on pad 0
	form := url.Values{"mute_group": {"5"}}
	req := httptest.NewRequest("POST", "/pad/params", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}

	// Verify it was applied
	mg := srv.session.Program.Pad(0).GetMuteGroup()
	if mg != 5 {
		t.Errorf("mute group = %d, want 5", mg)
	}
}

func TestLayerUpdate(t *testing.T) {
	srv := testServer(t)

	form := url.Values{
		"sample_name": {"kick"},
		"level":       {"80"},
	}
	req := httptest.NewRequest("POST", "/pad/layer/0", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}

	name := srv.session.Program.Pad(0).Layer(0).GetSampleName()
	if name != "kick" {
		t.Errorf("sample name = %q, want %q", name, "kick")
	}
	level := srv.session.Program.Pad(0).Layer(0).GetLevel()
	if level != 80 {
		t.Errorf("level = %d, want 80", level)
	}
}

func TestProgramSave(t *testing.T) {
	srv := testServer(t)

	// Open test.pgm
	form := url.Values{"path": {testdataPath("test.pgm")}}
	req := httptest.NewRequest("POST", "/program/open", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	// Save to temp file
	tmp := filepath.Join(t.TempDir(), "saved.pgm")
	form = url.Values{"path": {tmp}}
	req = httptest.NewRequest("POST", "/program/save", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("save status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Saved to") {
		t.Error("expected save confirmation")
	}
}

func TestPadGrid(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/partials/pad-grid?bank=1", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "pad-btn") {
		t.Error("missing pad buttons in grid partial")
	}
}

func TestStaticFiles(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/static/css/style.css", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("CSS status = %d", w.Code)
	}

	req = httptest.NewRequest("GET", "/static/js/app.js", http.NoBody)
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("JS status = %d", w.Code)
	}
}

func TestAudioPad_NoSample(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/audio/pad/0/0", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 (no sample loaded)", w.Code)
	}
}

func TestAudioPad_WithSample(t *testing.T) {
	srv := testServer(t)

	// Open test.pgm which has samples, and the sample matrix gets populated
	// with FindSample results. The test.pgm samples may not exist on disk,
	// but chh.wav does in testdata.
	// Manually set up a sample reference for pad 0, layer 0.
	srv.session.Matrix.Set(0, 0, &pgm.SampleRef{
		Name:     "chh",
		FilePath: testdataPath("chh.wav"),
		Status:   pgm.SampleOK,
	})

	req := httptest.NewRequest("GET", "/audio/pad/0/0", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "audio/wav") && !strings.Contains(ct, "audio/x-wav") {
		t.Errorf("Content-Type = %q, want audio/wav", ct)
	}
	if w.Body.Len() == 0 {
		t.Error("empty response body")
	}
}

func TestAudioPad_InvalidIndex(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/audio/pad/999/0", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestAudioSlice_NoSlicer(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/audio/slice/0", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 (no slicer)", w.Code)
	}
}

func TestAudioInfo(t *testing.T) {
	srv := testServer(t)

	// Set up a sample
	srv.session.Matrix.Set(2, 0, &pgm.SampleRef{
		Name:     "kick",
		FilePath: "/fake/kick.wav",
		Status:   pgm.SampleOK,
	})

	req := httptest.NewRequest("GET", "/audio/info", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"pad":2`) {
		t.Errorf("expected pad 2 in audio info, got: %s", body)
	}
}

func TestSlicerPage_Empty(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/slicer", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Load a WAV") {
		t.Error("expected empty slicer prompt")
	}
}

func TestSlicerLoad(t *testing.T) {
	srv := testServer(t)

	form := url.Values{"path": {testdataPath("myLoop.wav")}}
	req := httptest.NewRequest("POST", "/slicer/load", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	if srv.session.Slicer == nil {
		t.Fatal("slicer should be active after load")
	}
	if srv.session.Slicer.Markers.Size() != 9 {
		t.Errorf("markers = %d, want 9", srv.session.Slicer.Markers.Size())
	}
	body := w.Body.String()
	if !strings.Contains(body, "waveform-canvas") {
		t.Error("expected waveform canvas in response")
	}
}

func TestSlicerWaveform(t *testing.T) {
	srv := testServer(t)
	loadTestSlicer(t, srv)

	req := httptest.NewRequest("GET", "/slicer/waveform?width=500", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	// Parse JSON to verify structure
	body := w.Body.String()
	if !strings.Contains(body, `"markers"`) {
		t.Error("missing markers in waveform JSON")
	}
	if !strings.Contains(body, `"channels"`) {
		t.Error("missing channels in waveform JSON")
	}
}

func TestSlicerSensitivity(t *testing.T) {
	srv := testServer(t)
	loadTestSlicer(t, srv)

	originalCount := srv.session.Slicer.Markers.Size()

	form := url.Values{"sensitivity": {"200"}}
	req := httptest.NewRequest("POST", "/slicer/sensitivity", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	if srv.session.Slicer.GetSensitivity() != 200 {
		t.Errorf("sensitivity = %d, want 200", srv.session.Slicer.GetSensitivity())
	}
	t.Logf("markers: %d -> %d (sensitivity 130 -> 200)", originalCount, srv.session.Slicer.Markers.Size())
}

func TestSlicerMarkerOps(t *testing.T) {
	srv := testServer(t)
	loadTestSlicer(t, srv)

	// Next marker
	req := httptest.NewRequest("GET", "/slicer/marker/next", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("next: status = %d", w.Code)
	}
	if srv.session.Slicer.Markers.SelectedIndex() != 1 {
		t.Errorf("selected = %d, want 1", srv.session.Slicer.Markers.SelectedIndex())
	}

	// Delete
	req = httptest.NewRequest("POST", "/slicer/marker/delete", http.NoBody)
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("delete: status = %d", w.Code)
	}
	if srv.session.Slicer.Markers.Size() != 8 {
		t.Errorf("after delete: size = %d, want 8", srv.session.Slicer.Markers.Size())
	}

	// Insert
	req = httptest.NewRequest("POST", "/slicer/marker/insert", http.NoBody)
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("insert: status = %d", w.Code)
	}
	if srv.session.Slicer.Markers.Size() != 9 {
		t.Errorf("after insert: size = %d, want 9", srv.session.Slicer.Markers.Size())
	}
}

func TestSlicerExport(t *testing.T) {
	srv := testServer(t)
	loadTestSlicer(t, srv)

	dir := t.TempDir()
	form := url.Values{"dir": {dir}, "prefix": {"test_"}}
	req := httptest.NewRequest("POST", "/slicer/export", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	if !strings.Contains(body, `"exported"`) {
		t.Error("missing exported count in response")
	}
	// Should export 9 slices + 1 MIDI = 10 files
	if !strings.Contains(body, `"exported":10`) {
		t.Logf("export response: %s", body)
	}
}

func TestAssignPath_PerPad(t *testing.T) {
	srv := testServer(t)

	wavPath := testdataPath("chh.wav")
	form := url.Values{
		"pad":   {"0"},
		"mode":  {"per-pad"},
		"paths": {wavPath},
	}
	req := httptest.NewRequest("POST", "/assign/path", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}

	// Verify pad 0 layer 0 has the sample name
	name := srv.session.Program.Pad(0).Layer(0).GetSampleName()
	if name != "chh" {
		t.Errorf("pad 0 layer 0 = %q, want %q", name, "chh")
	}

	// Verify matrix was updated
	ref := srv.session.Matrix.Get(0, 0)
	if ref == nil {
		t.Error("matrix[0][0] is nil")
	}
}

func TestAssignPath_NoPaths(t *testing.T) {
	srv := testServer(t)

	form := url.Values{"pad": {"0"}}
	req := httptest.NewRequest("POST", "/assign/path", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestBatchPage(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/batch", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Batch Program Creation") {
		t.Error("missing batch page content")
	}
}

func TestBatchRun(t *testing.T) {
	srv := testServer(t)

	// Create temp dir with WAV files
	root := t.TempDir()
	subDir := filepath.Join(root, "drums")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write minimal WAV files
	wavHeader := []byte{
		'R', 'I', 'F', 'F', 38, 0, 0, 0, 'W', 'A', 'V', 'E',
		'f', 'm', 't', ' ', 16, 0, 0, 0, 1, 0, 1, 0,
		0x44, 0xAC, 0, 0, 0x88, 0x58, 0x01, 0, 2, 0, 16, 0,
		'd', 'a', 't', 'a', 2, 0, 0, 0, 0, 0,
	}
	if err := os.WriteFile(filepath.Join(subDir, "kick.wav"), wavHeader, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "snare.wav"), wavHeader, 0o644); err != nil {
		t.Fatal(err)
	}

	form := url.Values{"dir": {root}}
	req := httptest.NewRequest("POST", "/batch/run", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Created 1 programs") {
		t.Logf("body: %s", body)
	}

	// Verify .pgm was created
	if _, err := os.Stat(filepath.Join(subDir, "drums.pgm")); err != nil {
		t.Error("drums.pgm not created")
	}
}

func TestBatchRun_NoDir(t *testing.T) {
	srv := testServer(t)

	form := url.Values{}
	req := httptest.NewRequest("POST", "/batch/run", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestRemoveAllSamples(t *testing.T) {
	srv := testServer(t)

	// Set some sample names
	_ = srv.session.Program.Pad(0).Layer(0).SetSampleName("kick")
	_ = srv.session.Program.Pad(1).Layer(0).SetSampleName("snare")

	req := httptest.NewRequest("POST", "/edit/remove-all-samples", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}

	// Verify all samples cleared
	if name := srv.session.Program.Pad(0).Layer(0).GetSampleName(); name != "" {
		t.Errorf("pad 0 layer 0 = %q, want empty", name)
	}
	if name := srv.session.Program.Pad(1).Layer(0).GetSampleName(); name != "" {
		t.Errorf("pad 1 layer 0 = %q, want empty", name)
	}
}

func TestChromaticLayout(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("POST", "/edit/chromatic-layout", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}

	// Pad 0 should be MIDI note 35 (B0)
	if note := srv.session.Program.Pad(0).GetMIDINote(); note != 35 {
		t.Errorf("pad 0 note = %d, want 35", note)
	}
	// Pad 25 should be MIDI note 60 (C3)
	if note := srv.session.Program.Pad(25).GetMIDINote(); note != 60 {
		t.Errorf("pad 25 note = %d, want 60", note)
	}
}

func TestCopySettingsToAll(t *testing.T) {
	srv := testServer(t)

	// Set up pad 0 with specific settings
	pad0 := srv.session.Program.Pad(0)
	pad0.SetVoiceOverlap(1) // Mono
	pad0.SetMuteGroup(5)
	pad0.Envelope().SetAttack(50)
	pad0.Mixer().SetLevel(80)
	_ = pad0.Layer(0).SetSampleName("kick")
	pad0.Layer(0).SetLevel(90)

	srv.session.SelectedPad = 0

	req := httptest.NewRequest("POST", "/edit/copy-settings-to-all", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}

	// Verify pad 5 got the settings
	pad5 := srv.session.Program.Pad(5)
	if pad5.GetVoiceOverlap() != 1 {
		t.Errorf("pad 5 voice overlap = %d, want 1", pad5.GetVoiceOverlap())
	}
	if pad5.GetMuteGroup() != 5 {
		t.Errorf("pad 5 mute group = %d, want 5", pad5.GetMuteGroup())
	}
	if pad5.Envelope().GetAttack() != 50 {
		t.Errorf("pad 5 attack = %d, want 50", pad5.Envelope().GetAttack())
	}
	if pad5.Layer(0).GetLevel() != 90 {
		t.Errorf("pad 5 layer 0 level = %d, want 90", pad5.Layer(0).GetLevel())
	}

	// But sample name should NOT be copied
	if name := pad5.Layer(0).GetSampleName(); name == "kick" {
		t.Error("sample name should not be copied")
	}
}

func TestProfileSwitch(t *testing.T) {
	srv := testServer(t)

	form := url.Values{"profile": {"MPC500"}}
	req := httptest.NewRequest("POST", "/edit/profile", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	if srv.session.Profile.Name != "MPC500" {
		t.Errorf("profile = %q, want MPC500", srv.session.Profile.Name)
	}
}

func TestPreferences(t *testing.T) {
	p := DefaultPreferences()
	if p.Profile != "MPC1000" {
		t.Errorf("default profile = %q, want MPC1000", p.Profile)
	}
	if p.AuditionMode != "layer0" {
		t.Errorf("default audition = %q, want layer0", p.AuditionMode)
	}
}

func loadTestSlicer(t *testing.T, srv *Server) {
	t.Helper()
	form := url.Values{"path": {testdataPath("myLoop.wav")}}
	req := httptest.NewRequest("POST", "/slicer/load", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("load slicer: status = %d", w.Code)
	}
}

func Test404(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/nonexistent", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}
