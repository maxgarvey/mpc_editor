package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/maxgarvey/mpc_editor/internal/db"
)

func TestIsUnderLibrary(t *testing.T) {
	srv := testServer(t)
	workspace := srv.session.WorkspacePath
	lib := filepath.Join(workspace, "sample_library")

	tests := []struct {
		path string
		want bool
	}{
		{filepath.Join(lib, "kick.wav"), true},
		{filepath.Join(lib, "sub", "snare.wav"), true},
		{lib, true},
		{filepath.Join(workspace, "programs", "kick.wav"), false},
		{filepath.Join(workspace, "kick.wav"), false},
	}
	for _, tc := range tests {
		got := srv.isUnderLibrary(tc.path)
		if got != tc.want {
			t.Errorf("isUnderLibrary(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestRecordLibraryLinkIfApplicable(t *testing.T) {
	srv := testServer(t)
	workspace := srv.session.WorkspacePath
	ctx := context.Background()

	// Create a library source file.
	libDir := filepath.Join(workspace, "sample_library")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatal(err)
	}
	srcPath := filepath.Join(libDir, "kick.wav")
	if err := os.WriteFile(srcPath, []byte("RIFF fake wav"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a destination (program copy).
	pgmDir := filepath.Join(workspace, "programs", "mybeat")
	if err := os.MkdirAll(pgmDir, 0o755); err != nil {
		t.Fatal(err)
	}
	dstPath := filepath.Join(pgmDir, "kick.wav")
	if err := os.WriteFile(dstPath, []byte("RIFF fake wav"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Recording a link from a library source should insert a row.
	srv.recordLibraryLinkIfApplicable(ctx, srcPath, dstPath)

	srcRel := filepath.Join("sample_library", "kick.wav")
	dstRel := filepath.Join("programs", "mybeat", "kick.wav")

	link, err := srv.queries.GetSampleLinkByCopyPath(ctx, dstRel)
	if err != nil {
		t.Fatalf("link not recorded: %v", err)
	}
	if link.LibraryPath != srcRel {
		t.Errorf("LibraryPath = %q, want %q", link.LibraryPath, srcRel)
	}
	if link.Checksum == "" {
		t.Error("Checksum should not be empty")
	}

	// Recording a link from a non-library source should not insert a row.
	nonLibSrc := filepath.Join(workspace, "programs", "other", "snare.wav")
	if err := os.MkdirAll(filepath.Dir(nonLibSrc), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nonLibSrc, []byte("RIFF fake wav"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst2 := filepath.Join(pgmDir, "snare.wav")
	if err := os.WriteFile(dst2, []byte("RIFF fake wav"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv.recordLibraryLinkIfApplicable(ctx, nonLibSrc, dst2)

	dst2Rel := filepath.Join("programs", "mybeat", "snare.wav")
	if _, err := srv.queries.GetSampleLinkByCopyPath(ctx, dst2Rel); err == nil {
		t.Error("expected no link for non-library source, but found one")
	}
}

func TestSampleLinkedMap(t *testing.T) {
	srv := testServer(t)
	ctx := context.Background()
	workspace := srv.session.WorkspacePath

	// Seed two links for the same pgm dir.
	if err := srv.queries.UpsertSampleLink(ctx, db.UpsertSampleLinkParams{
		CopyPath:    "programs/beat/kick.wav",
		LibraryPath: "sample_library/kick.wav",
		Checksum:    "abc",
		CopiedAt:    1000,
	}); err != nil {
		t.Fatal(err)
	}
	if err := srv.queries.UpsertSampleLink(ctx, db.UpsertSampleLinkParams{
		CopyPath:    "programs/beat/snare.wav",
		LibraryPath: "sample_library/snare.wav",
		Checksum:    "def",
		CopiedAt:    1001,
	}); err != nil {
		t.Fatal(err)
	}
	// And one for a different dir.
	if err := srv.queries.UpsertSampleLink(ctx, db.UpsertSampleLinkParams{
		CopyPath:    "programs/other/clap.wav",
		LibraryPath: "sample_library/clap.wav",
		Checksum:    "ghi",
		CopiedAt:    1002,
	}); err != nil {
		t.Fatal(err)
	}

	_ = workspace // used implicitly via srv
	m := srv.sampleLinkedMap(ctx, "programs/beat")
	if len(m) != 2 {
		t.Fatalf("expected 2 links, got %d", len(m))
	}
	if m["programs/beat/kick.wav"] != "sample_library/kick.wav" {
		t.Errorf("kick link wrong: %q", m["programs/beat/kick.wav"])
	}
	if m["programs/beat/snare.wav"] != "sample_library/snare.wav" {
		t.Errorf("snare link wrong: %q", m["programs/beat/snare.wav"])
	}
	if _, ok := m["programs/other/clap.wav"]; ok {
		t.Error("clap.wav from other dir should not appear in beat map")
	}
}

func TestCheckSyncStatusFastPath(t *testing.T) {
	srv := testServer(t)
	workspace := srv.session.WorkspacePath
	ctx := context.Background()

	libDir := filepath.Join(workspace, "sample_library")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatal(err)
	}
	srcPath := filepath.Join(libDir, "kick.wav")
	if err := os.WriteFile(srcPath, []byte("original data"), 0o644); err != nil {
		t.Fatal(err)
	}
	dstPath := filepath.Join(workspace, "programs", "kick.wav")
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dstPath, []byte("original data"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv.recordLibraryLinkIfApplicable(ctx, srcPath, dstPath)

	relCopy := filepath.Join("programs", "kick.wav")
	if got := srv.checkSyncStatus(ctx, relCopy); got != "ok" {
		t.Fatalf("initial status = %q, want ok", got)
	}

	// Tamper with the source but restore its recorded size and mod time:
	// the fast path must trust the stat and skip re-checksumming.
	link, err := srv.queries.GetSampleLinkByCopyPath(ctx, relCopy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(srcPath, []byte("or1g1nal data"), 0o644); err != nil { // same length as "original data"
		t.Fatal(err)
	}
	recorded := time.Unix(link.SrcModTime, 0)
	if err := os.Chtimes(srcPath, recorded, recorded); err != nil {
		t.Fatal(err)
	}
	if got := srv.checkSyncStatus(ctx, relCopy); got != "ok" {
		t.Errorf("unchanged stat: status = %q, want ok (fast path)", got)
	}

	// Bump the mod time: the slow path must re-checksum and detect the change.
	bumped := recorded.Add(2 * time.Second)
	if err := os.Chtimes(srcPath, bumped, bumped); err != nil {
		t.Fatal(err)
	}
	if got := srv.checkSyncStatus(ctx, relCopy); got != "outdated" {
		t.Errorf("changed stat: status = %q, want outdated", got)
	}

	// Deleting the source must report source_missing.
	if err := os.Remove(srcPath); err != nil {
		t.Fatal(err)
	}
	if got := srv.checkSyncStatus(ctx, relCopy); got != "source_missing" {
		t.Errorf("missing source: status = %q, want source_missing", got)
	}
}
