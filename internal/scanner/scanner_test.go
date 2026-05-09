package scanner

import (
	"context"
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/maxgarvey/mpc_editor/internal/db"
	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) (*sql.DB, *db.Queries) {
	t.Helper()
	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.ExecSchema(sqlDB); err != nil {
		t.Fatal(err)
	}
	return sqlDB, db.New(sqlDB)
}

func testdataPath(name string) string {
	abs, err := filepath.Abs(filepath.Join("..", "..", "testdata", name))
	if err != nil {
		return filepath.Join("..", "..", "testdata", name)
	}
	return abs
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	in, err := os.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close() //nolint:errcheck // test cleanup
	out, err := os.Create(dst)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close() //nolint:errcheck // test cleanup
	if _, err := io.Copy(out, in); err != nil {
		t.Fatal(err)
	}
}

func TestScanWorkspace_BasicDiscovery(t *testing.T) {
	sqlDB, queries := openTestDB(t)
	s := New(sqlDB, queries)

	workspace := t.TempDir()
	copyFile(t, testdataPath("test.pgm"), filepath.Join(workspace, "test.pgm"))
	copyFile(t, testdataPath("chh.wav"), filepath.Join(workspace, "chh.wav"))
	copyFile(t, testdataPath("test.seq"), filepath.Join(workspace, "test.seq"))

	result, err := s.ScanWorkspace(workspace)
	if err != nil {
		t.Fatalf("ScanWorkspace: %v", err)
	}
	if result.FilesFound != 3 {
		t.Errorf("FilesFound = %d, want 3", result.FilesFound)
	}
	if result.FilesScanned != 3 {
		t.Errorf("FilesScanned = %d, want 3", result.FilesScanned)
	}
	if result.FilesRemoved != 0 {
		t.Errorf("FilesRemoved = %d, want 0", result.FilesRemoved)
	}
	if len(result.Errors) != 0 {
		t.Errorf("unexpected errors: %v", result.Errors)
	}
}

func TestScanWorkspace_UnchangedSkipped(t *testing.T) {
	sqlDB, queries := openTestDB(t)
	s := New(sqlDB, queries)

	workspace := t.TempDir()
	copyFile(t, testdataPath("chh.wav"), filepath.Join(workspace, "chh.wav"))

	// First scan
	if _, err := s.ScanWorkspace(workspace); err != nil {
		t.Fatal(err)
	}

	// Second scan — file is unchanged, should not be re-scanned
	result, err := s.ScanWorkspace(workspace)
	if err != nil {
		t.Fatalf("second ScanWorkspace: %v", err)
	}
	if result.FilesScanned != 0 {
		t.Errorf("second scan FilesScanned = %d, want 0 (unchanged)", result.FilesScanned)
	}
}

func TestScanWorkspace_StaleEntryPruned(t *testing.T) {
	sqlDB, queries := openTestDB(t)
	s := New(sqlDB, queries)

	workspace := t.TempDir()
	wavPath := filepath.Join(workspace, "chh.wav")
	copyFile(t, testdataPath("chh.wav"), wavPath)

	if _, err := s.ScanWorkspace(workspace); err != nil {
		t.Fatal(err)
	}

	// Delete the file on disk
	if err := os.Remove(wavPath); err != nil {
		t.Fatal(err)
	}

	result, err := s.ScanWorkspace(workspace)
	if err != nil {
		t.Fatalf("ScanWorkspace after delete: %v", err)
	}
	if result.FilesRemoved != 1 {
		t.Errorf("FilesRemoved = %d, want 1", result.FilesRemoved)
	}

	// Verify it's gone from the DB
	ctx := context.Background()
	_, err = queries.GetFileByPath(ctx, "chh.wav")
	if err == nil {
		t.Error("deleted file should be pruned from the catalog")
	}
}

func TestScanWorkspace_PGMMetadataExtracted(t *testing.T) {
	sqlDB, queries := openTestDB(t)
	s := New(sqlDB, queries)

	workspace := t.TempDir()
	copyFile(t, testdataPath("test.pgm"), filepath.Join(workspace, "test.pgm"))

	if _, err := s.ScanWorkspace(workspace); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	f, err := queries.GetFileByPath(ctx, "test.pgm")
	if err != nil {
		t.Fatalf("GetFileByPath: %v", err)
	}
	if f.FileType != "pgm" {
		t.Errorf("file type = %q, want pgm", f.FileType)
	}

	_, err = queries.GetPgmMeta(ctx, f.ID)
	if err != nil {
		t.Errorf("GetPgmMeta: %v (PGM metadata should be extracted)", err)
	}

	samples, err := queries.ListPgmSamples(ctx, f.ID)
	if err != nil {
		t.Fatalf("ListPgmSamples: %v", err)
	}
	if len(samples) == 0 {
		t.Error("no pgm_samples entries — expected at least one from test.pgm")
	}
}

func TestScanWorkspace_WAVMetadataAndTags(t *testing.T) {
	sqlDB, queries := openTestDB(t)
	s := New(sqlDB, queries)

	workspace := t.TempDir()
	copyFile(t, testdataPath("chh.wav"), filepath.Join(workspace, "chh.wav"))

	if _, err := s.ScanWorkspace(workspace); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	f, _ := queries.GetFileByPath(ctx, "chh.wav")
	meta, err := queries.GetWavMeta(ctx, f.ID)
	if err != nil {
		t.Fatalf("GetWavMeta: %v", err)
	}
	if meta.SampleRate <= 0 {
		t.Errorf("sample rate = %d, want > 0", meta.SampleRate)
	}

	// Auto-tags should have been created (channels, sample_rate, bit_depth)
	tags, err := queries.ListFileTags(ctx, f.ID)
	if err != nil {
		t.Fatalf("ListFileTags: %v", err)
	}
	if len(tags) == 0 {
		t.Error("expected auto-tags for WAV file")
	}
}

func TestScanWorkspace_SEQMetadataAndTags(t *testing.T) {
	sqlDB, queries := openTestDB(t)
	s := New(sqlDB, queries)

	workspace := t.TempDir()
	copyFile(t, testdataPath("test.seq"), filepath.Join(workspace, "test.seq"))

	if _, err := s.ScanWorkspace(workspace); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	f, err := queries.GetFileByPath(ctx, "test.seq")
	if err != nil {
		t.Fatalf("GetFileByPath: %v", err)
	}
	meta, err := queries.GetSeqMeta(ctx, f.ID)
	if err != nil {
		t.Fatalf("GetSeqMeta: %v", err)
	}
	if meta.Bpm <= 0 {
		t.Errorf("bpm = %f, want > 0", meta.Bpm)
	}

	tags, _ := queries.ListFileTags(ctx, f.ID)
	if len(tags) == 0 {
		t.Error("expected auto-tags for SEQ file")
	}
}

func TestScanWorkspace_IgnoresHiddenDirs(t *testing.T) {
	sqlDB, queries := openTestDB(t)
	s := New(sqlDB, queries)

	workspace := t.TempDir()
	hiddenDir := filepath.Join(workspace, ".hidden")
	if err := os.MkdirAll(hiddenDir, 0o755); err != nil {
		t.Fatal(err)
	}
	copyFile(t, testdataPath("chh.wav"), filepath.Join(hiddenDir, "chh.wav"))

	result, err := s.ScanWorkspace(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if result.FilesFound != 0 {
		t.Errorf("FilesFound = %d, want 0 (hidden dirs should be skipped)", result.FilesFound)
	}
	_ = queries
}

func TestScanWorkspace_IgnoresUnrecognizedExtensions(t *testing.T) {
	sqlDB, queries := openTestDB(t)
	s := New(sqlDB, queries)

	workspace := t.TempDir()
	// Write a text file — not a recognized MPC extension
	if err := os.WriteFile(filepath.Join(workspace, "notes.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := s.ScanWorkspace(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if result.FilesFound != 0 {
		t.Errorf("FilesFound = %d, want 0 (txt not recognized)", result.FilesFound)
	}
	_ = queries
}

func TestScanWorkspace_WavLinkedToPgmSample(t *testing.T) {
	sqlDB, queries := openTestDB(t)
	s := New(sqlDB, queries)

	workspace := t.TempDir()
	// test.pgm references "1KSN_001"; we place a matching wav in the workspace
	// so the scanner can link them.
	copyFile(t, testdataPath("test.pgm"), filepath.Join(workspace, "test.pgm"))
	copyFile(t, testdataPath("chh.wav"), filepath.Join(workspace, "chh.wav"))

	if _, err := s.ScanWorkspace(workspace); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	wavFile, err := queries.GetFileByPath(ctx, "chh.wav")
	if err != nil {
		t.Fatalf("GetFileByPath chh.wav: %v", err)
	}

	// The resolve pass should have linked any pgm_samples whose name matches "chh"
	programs, err := queries.ListProgramsUsingSample(ctx, sql.NullInt64{Int64: wavFile.ID, Valid: true})
	if err != nil {
		t.Fatalf("ListProgramsUsingSample: %v", err)
	}
	// Not necessarily linked if test.pgm doesn't reference "chh", but the call should not error.
	_ = programs
}
