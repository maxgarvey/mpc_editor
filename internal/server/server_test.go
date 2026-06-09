package server

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/maxgarvey/mpc_editor/internal/db"
	"github.com/maxgarvey/mpc_editor/internal/pgm"
	"github.com/maxgarvey/mpc_editor/web"
	_ "modernc.org/sqlite"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	if err := db.ExecSchema(sqlDB); err != nil {
		t.Fatal(err)
	}

	// Set workspace to a temp directory for tests.
	workspace := t.TempDir()
	_, err = sqlDB.Exec(`UPDATE preferences SET workspace_path = ? WHERE id = 1`, workspace)
	if err != nil {
		t.Fatal(err)
	}

	queries := db.New(sqlDB)
	templateFS, staticFS := web.FS()
	srv := New(templateFS, staticFS, sqlDB, queries)
	// Wait for the startup scan to complete so tests can safely seed catalog entries
	// without the background scan racing and pruning them.
	<-srv.startupScanDone
	return srv
}

func testdataPath(name string) string {
	// Return absolute path so resolvePath doesn't join with workspace.
	abs, err := filepath.Abs(filepath.Join("..", "..", "testdata", name))
	if err != nil {
		return filepath.Join("..", "..", "testdata", name)
	}
	return abs
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
	if !strings.Contains(body, "Workspace") {
		t.Error("missing Workspace panel")
	}
	if !strings.Contains(body, "detail-panel") {
		t.Error("missing detail panel")
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

func TestAudioPad_WithCatalogPGM(t *testing.T) {
	srv := testServer(t)

	// Copy test.pgm to workspace and seed it in catalog
	pgmDest := filepath.Join(srv.session.WorkspacePath, "test.pgm")
	pgmData, err := os.ReadFile(testdataPath("test.pgm"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pgmDest, pgmData, 0o644); err != nil {
		t.Fatal(err)
	}
	pgmID := seedFile(t, srv, "test.pgm", "pgm")

	// Seed a WAV sample in catalog and link it to the pgm
	wavID := seedFile(t, srv, "kick.wav", "wav")
	ctx := context.Background()
	srv.queries.InsertPgmSample(ctx, db.InsertPgmSampleParams{ //nolint:errcheck // test setup
		PgmFileID:    pgmID,
		Pad:          0,
		Layer:        0,
		SampleName:   "kick",
		SampleFileID: sql.NullInt64{Int64: wavID, Valid: true},
	})

	// Copy a real WAV to serve as kick.wav
	wavData, _ := os.ReadFile(testdataPath("chh.wav"))
	os.WriteFile(filepath.Join(srv.session.WorkspacePath, "kick.wav"), wavData, 0o644) //nolint:errcheck // test setup

	req := httptest.NewRequest("GET", "/audio/pad/0/0?pgm=test.pgm", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
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

func TestNewSession_WithLastPGM(t *testing.T) {
	// Create a DB with last_pgm_path pointing to a real PGM file
	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	for _, ddl := range []string{
		`CREATE TABLE IF NOT EXISTS preferences (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			profile TEXT NOT NULL DEFAULT 'MPC1000',
			last_pgm_path TEXT NOT NULL DEFAULT '',
			last_wav_path TEXT NOT NULL DEFAULT '',
			audition_mode TEXT NOT NULL DEFAULT 'layer0',
			workspace_path TEXT NOT NULL DEFAULT '',
			last_detail_path TEXT NOT NULL DEFAULT ''
		)`,
		`INSERT OR IGNORE INTO preferences (id) VALUES (1)`,
		`CREATE TABLE IF NOT EXISTS files (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			path TEXT NOT NULL UNIQUE,
			file_type TEXT NOT NULL,
			size INTEGER NOT NULL DEFAULT 0,
			mod_time INTEGER NOT NULL DEFAULT 0,
			scanned INTEGER NOT NULL DEFAULT 0
		)`,
	} {
		if _, err := sqlDB.Exec(ddl); err != nil {
			t.Fatal(err)
		}
	}

	pgmPath := testdataPath("test.pgm")
	workspace := t.TempDir()
	_, err = sqlDB.Exec(
		`UPDATE preferences SET last_pgm_path = ?, workspace_path = ?, profile = 'MPC500' WHERE id = 1`,
		pgmPath, workspace,
	)
	if err != nil {
		t.Fatal(err)
	}

	queries := db.New(sqlDB)
	sess := NewSession(queries)

	if sess.Program == nil {
		t.Error("expected program to be set after restoring last pgm")
	}
	if sess.FilePath != pgmPath {
		t.Errorf("FilePath = %q, want %q", sess.FilePath, pgmPath)
	}
	if sess.Profile.Name != "MPC500" {
		t.Errorf("profile = %q, want MPC500", sess.Profile.Name)
	}
}

func TestColocateSamples(t *testing.T) {
	srv := testServer(t)

	srcDir := t.TempDir()
	targetDir := t.TempDir()

	// Write a valid WAV into srcDir
	wavData, err := os.ReadFile(testdataPath("chh.wav"))
	if err != nil {
		t.Fatal(err)
	}
	srcWav := filepath.Join(srcDir, "chh.wav")
	if err := os.WriteFile(srcWav, wavData, 0o644); err != nil {
		t.Fatal(err)
	}

	srv.session.Matrix.Set(0, 0, &pgm.SampleRef{
		Name:     "chh",
		FilePath: srcWav,
		Status:   pgm.SampleOK,
	})

	copied := srv.colocateSamples(targetDir)
	if copied != 1 {
		t.Errorf("copied = %d, want 1", copied)
	}

	ref := srv.session.Matrix.Get(0, 0)
	if ref == nil || filepath.Dir(ref.FilePath) != targetDir {
		t.Errorf("ref path = %q, want in targetDir %q", ref.FilePath, targetDir)
	}
}

func TestColocateSamples_AlreadyColocated(t *testing.T) {
	srv := testServer(t)
	targetDir := t.TempDir()

	// WAV already in the target dir
	wavData, err := os.ReadFile(testdataPath("chh.wav"))
	if err != nil {
		t.Fatal(err)
	}
	existingWav := filepath.Join(targetDir, "chh.wav")
	if err := os.WriteFile(existingWav, wavData, 0o644); err != nil {
		t.Fatal(err)
	}

	srv.session.Matrix.Set(0, 0, &pgm.SampleRef{
		Name:     "chh",
		FilePath: existingWav,
		Status:   pgm.SampleOK,
	})

	copied := srv.colocateSamples(targetDir)
	if copied != 0 {
		t.Errorf("copied = %d, want 0 (already in target dir)", copied)
	}
}

func TestAssignPath_PerLayer(t *testing.T) {
	srv := testServer(t)

	wavPath := testdataPath("chh.wav")
	form := url.Values{
		"pad":   {"2"},
		"mode":  {"per-layer"},
		"paths": {wavPath},
	}
	req := httptest.NewRequest("POST", "/assign/path", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestAssignPath_Replace(t *testing.T) {
	srv := testServer(t)

	wavPath := testdataPath("chh.wav")
	form := url.Values{
		"pad":   {"3"},
		"mode":  {"replace"},
		"paths": {wavPath},
	}
	req := httptest.NewRequest("POST", "/assign/path", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestAssignPath_Multisample(t *testing.T) {
	srv := testServer(t)

	wavPath := testdataPath("chh.wav")
	form := url.Values{
		"mode":  {"multisample"},
		"paths": {wavPath},
	}
	req := httptest.NewRequest("POST", "/assign/path", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestAssignPath_MethodNotAllowed(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/assign/path", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestHandleAssign_Multisample(t *testing.T) {
	srv := testServer(t)

	wavData, err := os.ReadFile(testdataPath("chh.wav"))
	if err != nil {
		t.Fatal(err)
	}

	var body strings.Builder
	boundary := "multisampleboundary"
	body.WriteString("--" + boundary + "\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"mode\"\r\n\r\n")
	body.WriteString("multisample\r\n")
	body.WriteString("--" + boundary + "\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"files\"; filename=\"chh.wav\"\r\n")
	body.WriteString("Content-Type: audio/wav\r\n\r\n")
	body.Write(wavData) //nolint:errcheck // test setup
	body.WriteString("\r\n--" + boundary + "--\r\n")

	req := httptest.NewRequest("POST", "/assign/upload", strings.NewReader(body.String()))
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
}

func TestHandleAssign_MethodNotAllowed(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/assign/upload", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestHandleAssign_NoFiles(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("POST", "/assign/upload", strings.NewReader(""))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (no files)", w.Code)
	}
}

func TestHandleAssign_WithFile(t *testing.T) {
	srv := testServer(t)

	wavData, err := os.ReadFile(testdataPath("chh.wav"))
	if err != nil {
		t.Fatal(err)
	}

	var body strings.Builder
	boundary := "testboundary123"
	body.WriteString("--" + boundary + "\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"files\"; filename=\"chh.wav\"\r\n")
	body.WriteString("Content-Type: audio/wav\r\n\r\n")
	body.Write(wavData) //nolint:errcheck // test setup
	body.WriteString("\r\n--" + boundary + "--\r\n")

	req := httptest.NewRequest("POST", "/assign/upload", strings.NewReader(body.String()))
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	// Should succeed (200 with HX-Redirect) or redirect
	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
}

func TestCopySamplesToWorkspace_NoWorkspace(t *testing.T) {
	srv := testServer(t)
	srv.session.WorkspacePath = ""

	origPath := testdataPath("chh.wav")
	ref := &pgm.SampleRef{FilePath: origPath, Status: pgm.SampleOK, Name: "chh"}
	srv.copySamplesToWorkspace([]*pgm.SampleRef{ref})

	if ref.FilePath != origPath {
		t.Error("FilePath should be unchanged with no workspace")
	}
}

func TestCopySamplesToWorkspace_ValidFile(t *testing.T) {
	srv := testServer(t)

	origPath := testdataPath("chh.wav")
	ref := &pgm.SampleRef{FilePath: origPath, Status: pgm.SampleOK, Name: "chh"}
	srv.copySamplesToWorkspace([]*pgm.SampleRef{ref})

	if ref.FilePath == origPath {
		t.Error("FilePath should be updated to workspace copy")
	}
	if !strings.HasPrefix(ref.FilePath, srv.session.WorkspacePath) {
		t.Errorf("FilePath %q should be in workspace %q", ref.FilePath, srv.session.WorkspacePath)
	}
}

// TestHandleAssign_MP3Transcode exercises the IsTranscodable branch in handleAssign.
func TestHandleAssign_MP3Transcode(t *testing.T) {
	srv := testServer(t)

	mp3Data, err := os.ReadFile(testdataPath("test_audio.mp3"))
	if err != nil {
		t.Fatal(err)
	}

	var body strings.Builder
	boundary := "mp3boundary"
	body.WriteString("--" + boundary + "\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"files\"; filename=\"test_audio.mp3\"\r\n")
	body.WriteString("Content-Type: audio/mpeg\r\n\r\n")
	body.Write(mp3Data) //nolint:errcheck // test setup
	body.WriteString("\r\n--" + boundary + "--\r\n")

	req := httptest.NewRequest("POST", "/assign/upload", strings.NewReader(body.String()))
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	// Either succeeds (200) or fails gracefully — just verify no panic
	if w.Code == http.StatusInternalServerError {
		t.Errorf("unexpected 500: %s", w.Body.String())
	}
}

// TestAssignPath_MP3Transcode exercises the IsTranscodable branch in handleAssignPath.
func TestAssignPath_MP3Transcode(t *testing.T) {
	srv := testServer(t)

	form := url.Values{
		"pad":   {"0"},
		"mode":  {"per-pad"},
		"paths": {testdataPath("test_audio.mp3")},
	}
	req := httptest.NewRequest("POST", "/assign/path", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code == http.StatusInternalServerError {
		t.Errorf("unexpected 500: %s", w.Body.String())
	}
}

// TestHandleAssign_WithProgramOpen exercises the s.session.FilePath != "" branch in handleAssign,
// which copies samples into the program's directory.
func TestHandleAssign_WithProgramOpen(t *testing.T) {
	srv := testServer(t)

	// Open a program so s.session.FilePath is set
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

	if srv.session.FilePath != pgmDest {
		t.Fatalf("program not opened: FilePath = %q", srv.session.FilePath)
	}

	wavData, err := os.ReadFile(testdataPath("chh.wav"))
	if err != nil {
		t.Fatal(err)
	}

	var body strings.Builder
	boundary := "assignboundary"
	body.WriteString("--" + boundary + "\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"files\"; filename=\"chh.wav\"\r\n")
	body.WriteString("Content-Type: audio/wav\r\n\r\n")
	body.Write(wavData) //nolint:errcheck // test setup
	body.WriteString("\r\n--" + boundary + "--\r\n")

	req := httptest.NewRequest("POST", "/assign/upload", strings.NewReader(body.String()))
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
}

// TestAssignPath_FromLibrary exercises the library drag-drop scenario: a WAV from
// sample_library/ is dragged to an empty pad in an open program.
func TestAssignPath_FromLibrary(t *testing.T) {
	srv := testServer(t)
	workspace := srv.session.WorkspacePath

	// Create sample_library/ directory and a WAV inside it (the drag source).
	libDir := filepath.Join(workspace, "sample_library")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatal(err)
	}
	libWav := filepath.Join(libDir, "kick.wav")
	wavData, err := os.ReadFile(testdataPath("chh.wav"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(libWav, wavData, 0o644); err != nil {
		t.Fatal(err)
	}

	// Create programs/ directory and a PGM file there, then open it.
	pgmDir := filepath.Join(workspace, "programs", "beat")
	if err := os.MkdirAll(pgmDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pgmDest := filepath.Join(pgmDir, "beat.pgm")
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

	// Simulate drag: POST /assign/path with the absolute library path and pad=13
	// (pads 0–12 are all occupied in test.pgm; 13 is the first empty slot).
	form := url.Values{
		"pad":  {"13"},
		"mode": {"per-pad"},
		"path": {libWav},
	}
	req := httptest.NewRequest("POST", "/assign/path", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	// Pad 13 should now have the sample name set.
	name := srv.session.Program.Pad(13).Layer(0).GetSampleName()
	if name != "kick" && name != "chh" {
		t.Errorf("pad 13 layer 0 = %q, want 'kick' or 'chh'", name)
	}

	// Matrix should be updated too.
	ref := srv.session.Matrix.Get(13, 0)
	if ref == nil {
		t.Error("matrix[13][0] is nil after library drag assign")
	}

	// The sample should be linked in the DB (copy path is relative to workspace).
	relCopyPath, _ := filepath.Rel(workspace, filepath.Join(pgmDir, "kick.wav"))
	link, err := srv.queries.GetSampleLinkByCopyPath(context.Background(), relCopyPath)
	if err != nil {
		t.Errorf("no library link recorded: %v", err)
	} else if link.LibraryPath != filepath.Join("sample_library", "kick.wav") {
		t.Errorf("library_path = %q, want sample_library/kick.wav", link.LibraryPath)
	}
}

// TestAssignPath_WithProgramOpen exercises the s.session.FilePath != "" branch in handleAssignPath.
func TestAssignPath_WithProgramOpen(t *testing.T) {
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

	form := url.Values{
		"pad":   {"0"},
		"mode":  {"per-pad"},
		"paths": {testdataPath("chh.wav")},
	}
	req := httptest.NewRequest("POST", "/assign/path", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}
