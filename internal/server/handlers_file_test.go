package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/maxgarvey/mpc_editor/internal/db"
)

// seedFile inserts a catalog entry and creates an empty placeholder file so the
// background startup scan doesn't prune it as a stale entry.
func seedFile(t *testing.T, srv *Server, path, fileType string) int64 {
	t.Helper()
	ctx := context.Background()
	// Create a placeholder file on disk so the scanner doesn't prune it.
	diskPath := filepath.Join(srv.session.WorkspacePath, path)
	if err := os.MkdirAll(filepath.Dir(diskPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(diskPath, []byte("placeholder"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.queries.UpsertFile(ctx, db.UpsertFileParams{
		Path: path, FileType: fileType, Size: 100, ModTime: 1,
	}); err != nil {
		t.Fatal(err)
	}
	f, err := srv.queries.GetFileByPath(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	return f.ID
}

func TestHandleFileDetail_ByID(t *testing.T) {
	srv := testServer(t)
	id := seedFile(t, srv, "test.pgm", "pgm")

	req := httptest.NewRequest("GET", "/file/?id="+itoa(id), http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "file-detail") {
		t.Error("response missing file-detail")
	}
}

func TestHandleFileDetail_ByPath(t *testing.T) {
	srv := testServer(t)
	id := seedFile(t, srv, "kick.wav", "wav")

	req := httptest.NewRequest("GET", "/file/"+itoa(id), http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestHandleFileDetail_InvalidID(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/file/notanumber", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandleFileDetail_NotFound(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/file/9999", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestHandleFileDetail_WAV(t *testing.T) {
	srv := testServer(t)
	id := seedFile(t, srv, "samples/snare.wav", "wav")

	ctx := context.Background()
	srv.queries.UpsertWavMeta(ctx, db.UpsertWavMetaParams{ //nolint:errcheck // test setup
		FileID: id, SampleRate: 44100, Channels: 1, BitsPerSample: 16, FrameCount: 88200,
	})

	req := httptest.NewRequest("GET", "/file/"+itoa(id), http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "44100") {
		t.Error("response missing sample rate")
	}
}

func TestHandleFileDetail_SEQ(t *testing.T) {
	srv := testServer(t)
	id := seedFile(t, srv, "beats/groove.seq", "seq")

	ctx := context.Background()
	srv.queries.UpsertSeqMeta(ctx, db.UpsertSeqMetaParams{ //nolint:errcheck // test setup
		FileID: id, Bpm: 120.0, Bars: 4, Version: "V1.00",
	})

	req := httptest.NewRequest("GET", "/file/"+itoa(id), http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "V1.00") {
		t.Error("response missing seq version")
	}
}

func TestHandleTagAdd(t *testing.T) {
	srv := testServer(t)
	id := seedFile(t, srv, "drum.wav", "wav")

	form := url.Values{"id": {itoa(id)}, "tag": {"kick"}}
	req := httptest.NewRequest("POST", "/file/tags/add", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestHandleTagAdd_KeyValue(t *testing.T) {
	srv := testServer(t)
	id := seedFile(t, srv, "drum2.wav", "wav")

	form := url.Values{"id": {itoa(id)}, "tag": {"genre:house"}}
	req := httptest.NewRequest("POST", "/file/tags/add", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestHandleTagAdd_MethodNotAllowed(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/file/tags/add", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestHandleTagAdd_InvalidID(t *testing.T) {
	srv := testServer(t)

	form := url.Values{"id": {"abc"}, "tag": {"kick"}}
	req := httptest.NewRequest("POST", "/file/tags/add", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandleTagAdd_EmptyTag(t *testing.T) {
	srv := testServer(t)
	id := seedFile(t, srv, "drum3.wav", "wav")

	form := url.Values{"id": {itoa(id)}, "tag": {""}}
	req := httptest.NewRequest("POST", "/file/tags/add", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandleTagRemove(t *testing.T) {
	srv := testServer(t)
	id := seedFile(t, srv, "bass.wav", "wav")

	ctx := context.Background()
	srv.queries.AddFileTag(ctx, db.AddFileTagParams{ //nolint:errcheck // test setup
		FileID: id, TagKey: "", TagValue: "bass", Auto: false,
	})

	form := url.Values{"id": {itoa(id)}, "key": {""}, "value": {"bass"}}
	req := httptest.NewRequest("POST", "/file/tags/remove", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestHandleTagRemove_MethodNotAllowed(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/file/tags/remove", http.NoBody)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestHandleTagRemove_InvalidID(t *testing.T) {
	srv := testServer(t)

	form := url.Values{"id": {"nope"}, "key": {""}, "value": {"kick"}}
	req := httptest.NewRequest("POST", "/file/tags/remove", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// TestHandleTagRemove_AutoTagProtected verifies that auto=1 tags cannot be removed
// via the HTTP endpoint — the SQL guard (AND auto = 0) keeps them in the DB.
func TestHandleTagRemove_AutoTagProtected(t *testing.T) {
	srv := testServer(t)
	id := seedFile(t, srv, "kick.wav", "wav")

	ctx := context.Background()
	srv.queries.AddFileTag(ctx, db.AddFileTagParams{ //nolint:errcheck // test setup
		FileID: id, TagKey: "bpm", TagValue: "120", Auto: true,
	})

	form := url.Values{"id": {itoa(id)}, "key": {"bpm"}, "value": {"120"}}
	req := httptest.NewRequest("POST", "/file/tags/remove", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	// Auto tag must still be present in DB
	tags, err := srv.queries.ListFileTags(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, tag := range tags {
		if tag.TagKey == "bpm" && tag.TagValue == "120" {
			found = true
			break
		}
	}
	if !found {
		t.Error("auto-tag was deleted but should have been protected by AND auto = 0 guard")
	}
}

func itoa(n int64) string {
	return fmt.Sprintf("%d", n)
}
