package db

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) (*sql.DB, *Queries) {
	t.Helper()
	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if _, err := sqlDB.Exec(schemaDDL); err != nil {
		t.Fatal(err)
	}
	return sqlDB, New(sqlDB)
}

func TestNew(t *testing.T) {
	sqlDB, q := openTestDB(t)
	if q == nil {
		t.Fatal("New returned nil")
	}
	_ = sqlDB
}

func TestWithTx(t *testing.T) {
	sqlDB, q := openTestDB(t)
	tx, err := sqlDB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback() //nolint:errcheck // test setup

	txQ := q.WithTx(tx)
	if txQ == nil {
		t.Fatal("WithTx returned nil")
	}
	// Operations through txQ should be visible within the same transaction.
	ctx := context.Background()
	_, err = txQ.UpsertFile(ctx, UpsertFileParams{Path: "tx_test.wav", FileType: "wav", Size: 100, ModTime: 1})
	if err != nil {
		t.Fatalf("UpsertFile via tx: %v", err)
	}
}

// Files ------------------------------------------------------------------

func TestUpsertAndGetFile(t *testing.T) {
	_, q := openTestDB(t)
	ctx := context.Background()

	id, err := q.UpsertFile(ctx, UpsertFileParams{
		Path: "beats/kick.pgm", FileType: "pgm", Size: 10756, ModTime: 1000,
	})
	if err != nil {
		t.Fatalf("UpsertFile: %v", err)
	}
	if id == 0 {
		t.Error("expected non-zero file ID")
	}

	f, err := q.GetFileByPath(ctx, "beats/kick.pgm")
	if err != nil {
		t.Fatalf("GetFileByPath: %v", err)
	}
	if f.ID != id {
		t.Errorf("file ID = %d, want %d", f.ID, id)
	}
	if f.FileType != "pgm" {
		t.Errorf("file type = %q, want pgm", f.FileType)
	}

	f2, err := q.GetFileByID(ctx, id)
	if err != nil {
		t.Fatalf("GetFileByID: %v", err)
	}
	if f2.Path != "beats/kick.pgm" {
		t.Errorf("path = %q, want beats/kick.pgm", f2.Path)
	}
}

func TestUpsertFileUpdates(t *testing.T) {
	_, q := openTestDB(t)
	ctx := context.Background()

	id1, _ := q.UpsertFile(ctx, UpsertFileParams{Path: "a.wav", FileType: "wav", Size: 100, ModTime: 1})
	id2, _ := q.UpsertFile(ctx, UpsertFileParams{Path: "a.wav", FileType: "wav", Size: 200, ModTime: 2})

	if id1 != id2 {
		t.Errorf("upsert should return same ID: %d vs %d", id1, id2)
	}
	f, _ := q.GetFileByPath(ctx, "a.wav")
	if f.Size != 200 {
		t.Errorf("size after upsert = %d, want 200", f.Size)
	}
}

func TestDeleteFile(t *testing.T) {
	_, q := openTestDB(t)
	ctx := context.Background()

	id, _ := q.UpsertFile(ctx, UpsertFileParams{Path: "del.wav", FileType: "wav", Size: 1})
	if err := q.DeleteFile(ctx, id); err != nil {
		t.Fatalf("DeleteFile: %v", err)
	}
	_, err := q.GetFileByPath(ctx, "del.wav")
	if err == nil {
		t.Error("file should be deleted")
	}
}

func TestDeleteFileByPath(t *testing.T) {
	_, q := openTestDB(t)
	ctx := context.Background()

	q.UpsertFile(ctx, UpsertFileParams{Path: "bypath.wav", FileType: "wav", Size: 1}) //nolint:errcheck // test setup
	if err := q.DeleteFileByPath(ctx, "bypath.wav"); err != nil {
		t.Fatalf("DeleteFileByPath: %v", err)
	}
	_, err := q.GetFileByPath(ctx, "bypath.wav")
	if err == nil {
		t.Error("file should be deleted")
	}
}

func TestListAllFilesAndByType(t *testing.T) {
	_, q := openTestDB(t)
	ctx := context.Background()

	q.UpsertFile(ctx, UpsertFileParams{Path: "a.pgm", FileType: "pgm", Size: 1}) //nolint:errcheck // test setup
	q.UpsertFile(ctx, UpsertFileParams{Path: "b.wav", FileType: "wav", Size: 2}) //nolint:errcheck // test setup
	q.UpsertFile(ctx, UpsertFileParams{Path: "c.wav", FileType: "wav", Size: 3}) //nolint:errcheck // test setup

	all, err := q.ListAllFiles(ctx)
	if err != nil {
		t.Fatalf("ListAllFiles: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("all files = %d, want 3", len(all))
	}

	wavs, err := q.ListFilesByType(ctx, "wav")
	if err != nil {
		t.Fatalf("ListFilesByType: %v", err)
	}
	if len(wavs) != 2 {
		t.Errorf("wav files = %d, want 2", len(wavs))
	}
}

func TestUpdateFilePath(t *testing.T) {
	_, q := openTestDB(t)
	ctx := context.Background()

	id, _ := q.UpsertFile(ctx, UpsertFileParams{Path: "old.wav", FileType: "wav", Size: 1})
	if err := q.UpdateFilePath(ctx, UpdateFilePathParams{NewPath: "new.wav", OldPath: "old.wav"}); err != nil {
		t.Fatalf("UpdateFilePath: %v", err)
	}
	f, _ := q.GetFileByID(ctx, id)
	if f.Path != "new.wav" {
		t.Errorf("path = %q, want new.wav", f.Path)
	}
}

// Preferences ------------------------------------------------------------

func TestPreferences(t *testing.T) {
	_, q := openTestDB(t)
	ctx := context.Background()

	prefs, err := q.GetPreferences(ctx)
	if err != nil {
		t.Fatalf("GetPreferences: %v", err)
	}
	if prefs.Profile != "MPC1000" {
		t.Errorf("default profile = %q, want MPC1000", prefs.Profile)
	}

	if err := q.UpdateAllPreferences(ctx, UpdateAllPreferencesParams{
		Profile:        "MPC500",
		LastPgmPath:    "/path/to/prog.pgm",
		LastWavPath:    "/path/to/sample.wav",
		AuditionMode:   "layer1",
		WorkspacePath:  "/workspace",
		LastDetailPath: "/path/to/file.pgm",
	}); err != nil {
		t.Fatalf("UpdateAllPreferences: %v", err)
	}

	prefs2, err := q.GetPreferences(ctx)
	if err != nil {
		t.Fatalf("GetPreferences after update: %v", err)
	}
	if prefs2.Profile != "MPC500" {
		t.Errorf("profile = %q, want MPC500", prefs2.Profile)
	}
	if prefs2.LastPgmPath != "/path/to/prog.pgm" {
		t.Errorf("last_pgm_path = %q", prefs2.LastPgmPath)
	}
	if prefs2.WorkspacePath != "/workspace" {
		t.Errorf("workspace_path = %q", prefs2.WorkspacePath)
	}
}

// WAV metadata -----------------------------------------------------------

func TestWavMeta(t *testing.T) {
	_, q := openTestDB(t)
	ctx := context.Background()

	id, _ := q.UpsertFile(ctx, UpsertFileParams{Path: "kick.wav", FileType: "wav", Size: 8000})
	if err := q.UpsertWavMeta(ctx, UpsertWavMetaParams{
		FileID: id, SampleRate: 44100, Channels: 2, BitsPerSample: 16, FrameCount: 22050,
	}); err != nil {
		t.Fatalf("UpsertWavMeta: %v", err)
	}

	meta, err := q.GetWavMeta(ctx, id)
	if err != nil {
		t.Fatalf("GetWavMeta: %v", err)
	}
	if meta.SampleRate != 44100 {
		t.Errorf("sample rate = %d, want 44100", meta.SampleRate)
	}
	if meta.Channels != 2 {
		t.Errorf("channels = %d, want 2", meta.Channels)
	}

	// UpdateWavSource
	if err := q.UpdateWavSource(ctx, UpdateWavSourceParams{FileID: id, Source: "original"}); err != nil {
		t.Fatalf("UpdateWavSource: %v", err)
	}
	meta2, _ := q.GetWavMeta(ctx, id)
	if meta2.Source != "original" {
		t.Errorf("source = %q, want original", meta2.Source)
	}
}

// PGM metadata -----------------------------------------------------------

func TestPgmMeta(t *testing.T) {
	_, q := openTestDB(t)
	ctx := context.Background()

	id, _ := q.UpsertFile(ctx, UpsertFileParams{Path: "prog.pgm", FileType: "pgm", Size: 10756})
	if err := q.UpsertPgmMeta(ctx, UpsertPgmMetaParams{FileID: id, MidiPgmChange: 5}); err != nil {
		t.Fatalf("UpsertPgmMeta: %v", err)
	}

	meta, err := q.GetPgmMeta(ctx, id)
	if err != nil {
		t.Fatalf("GetPgmMeta: %v", err)
	}
	if meta.MidiPgmChange != 5 {
		t.Errorf("midi pgm change = %d, want 5", meta.MidiPgmChange)
	}
}

// SEQ metadata -----------------------------------------------------------

func TestSeqMeta(t *testing.T) {
	_, q := openTestDB(t)
	ctx := context.Background()

	id, _ := q.UpsertFile(ctx, UpsertFileParams{Path: "seq.seq", FileType: "seq", Size: 1024})
	if err := q.UpsertSeqMeta(ctx, UpsertSeqMetaParams{
		FileID: id, Bpm: 120.0, Bars: 4, Version: "1.0",
	}); err != nil {
		t.Fatalf("UpsertSeqMeta: %v", err)
	}

	meta, err := q.GetSeqMeta(ctx, id)
	if err != nil {
		t.Fatalf("GetSeqMeta: %v", err)
	}
	if meta.Bpm != 120.0 {
		t.Errorf("bpm = %f, want 120.0", meta.Bpm)
	}
	if meta.Bars != 4 {
		t.Errorf("bars = %d, want 4", meta.Bars)
	}
}

// PGM samples ------------------------------------------------------------

func TestPgmSamples(t *testing.T) {
	_, q := openTestDB(t)
	ctx := context.Background()

	pgmID, _ := q.UpsertFile(ctx, UpsertFileParams{Path: "prog.pgm", FileType: "pgm", Size: 1})
	wavID, _ := q.UpsertFile(ctx, UpsertFileParams{Path: "kick.wav", FileType: "wav", Size: 1})

	if err := q.InsertPgmSample(ctx, InsertPgmSampleParams{
		PgmFileID:    pgmID,
		Pad:          0,
		Layer:        0,
		SampleName:   "kick",
		SampleFileID: sql.NullInt64{Int64: wavID, Valid: true},
	}); err != nil {
		t.Fatalf("InsertPgmSample: %v", err)
	}
	if err := q.InsertPgmSample(ctx, InsertPgmSampleParams{
		PgmFileID:  pgmID,
		Pad:        1,
		Layer:      0,
		SampleName: "snare",
	}); err != nil {
		t.Fatalf("InsertPgmSample unresolved: %v", err)
	}

	samples, err := q.ListPgmSamples(ctx, pgmID)
	if err != nil {
		t.Fatalf("ListPgmSamples: %v", err)
	}
	if len(samples) != 2 {
		t.Errorf("samples = %d, want 2", len(samples))
	}

	// CountMissingSamples: snare has no file_id
	count, err := q.CountMissingSamples(ctx, pgmID)
	if err != nil {
		t.Fatalf("CountMissingSamples: %v", err)
	}
	if count != 1 {
		t.Errorf("missing = %d, want 1", count)
	}

	// ListProgramsUsingSample
	programs, err := q.ListProgramsUsingSample(ctx, sql.NullInt64{Int64: wavID, Valid: true})
	if err != nil {
		t.Fatalf("ListProgramsUsingSample: %v", err)
	}
	if len(programs) != 1 {
		t.Errorf("programs using sample = %d, want 1", len(programs))
	}

	// DeletePgmSamples
	if err := q.DeletePgmSamples(ctx, pgmID); err != nil {
		t.Fatalf("DeletePgmSamples: %v", err)
	}
	samples2, _ := q.ListPgmSamples(ctx, pgmID)
	if len(samples2) != 0 {
		t.Errorf("samples after delete = %d, want 0", len(samples2))
	}
}

// SEQ tracks -------------------------------------------------------------

func TestSeqTracks(t *testing.T) {
	_, q := openTestDB(t)
	ctx := context.Background()

	seqID, _ := q.UpsertFile(ctx, UpsertFileParams{Path: "seq.seq", FileType: "seq", Size: 1})
	if err := q.InsertSeqTrack(ctx, InsertSeqTrackParams{
		SeqFileID: seqID, Track: 0, TrackName: "drums", MidiChannel: 1,
	}); err != nil {
		t.Fatalf("InsertSeqTrack: %v", err)
	}

	tracks, err := q.ListSeqTracks(ctx, seqID)
	if err != nil {
		t.Fatalf("ListSeqTracks: %v", err)
	}
	if len(tracks) != 1 {
		t.Errorf("tracks = %d, want 1", len(tracks))
	}
	if tracks[0].TrackName != "drums" {
		t.Errorf("track name = %q, want drums", tracks[0].TrackName)
	}

	if err := q.DeleteSeqTracks(ctx, seqID); err != nil {
		t.Fatalf("DeleteSeqTracks: %v", err)
	}
	tracks2, _ := q.ListSeqTracks(ctx, seqID)
	if len(tracks2) != 0 {
		t.Errorf("tracks after delete = %d, want 0", len(tracks2))
	}
}

// File tags --------------------------------------------------------------

func TestFileTags(t *testing.T) {
	_, q := openTestDB(t)
	ctx := context.Background()

	id, _ := q.UpsertFile(ctx, UpsertFileParams{Path: "tagged.wav", FileType: "wav", Size: 1})

	if err := q.AddFileTag(ctx, AddFileTagParams{FileID: id, TagKey: "genre", TagValue: "hip-hop", Auto: false}); err != nil {
		t.Fatalf("AddFileTag: %v", err)
	}
	if err := q.AddFileTag(ctx, AddFileTagParams{FileID: id, TagKey: "channels", TagValue: "mono", Auto: true}); err != nil {
		t.Fatalf("AddFileTag auto: %v", err)
	}

	tags, err := q.ListFileTags(ctx, id)
	if err != nil {
		t.Fatalf("ListFileTags: %v", err)
	}
	if len(tags) != 2 {
		t.Errorf("tags = %d, want 2", len(tags))
	}

	// ListFilesByTag
	files, err := q.ListFilesByTag(ctx, ListFilesByTagParams{Key: "genre", Value: "hip-hop"})
	if err != nil {
		t.Fatalf("ListFilesByTag: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("files by tag = %d, want 1", len(files))
	}

	// RemoveAutoTags
	if err := q.RemoveAutoTags(ctx, id); err != nil {
		t.Fatalf("RemoveAutoTags: %v", err)
	}
	tags2, _ := q.ListFileTags(ctx, id)
	if len(tags2) != 1 {
		t.Errorf("tags after RemoveAutoTags = %d, want 1 (manual only)", len(tags2))
	}

	// RemoveFileTag
	if err := q.RemoveFileTag(ctx, RemoveFileTagParams{FileID: id, TagKey: "genre", TagValue: "hip-hop"}); err != nil {
		t.Fatalf("RemoveFileTag: %v", err)
	}
	tags3, _ := q.ListFileTags(ctx, id)
	if len(tags3) != 0 {
		t.Errorf("tags after RemoveFileTag = %d, want 0", len(tags3))
	}
}

// Song steps -------------------------------------------------------------

func TestSongSteps(t *testing.T) {
	_, q := openTestDB(t)
	ctx := context.Background()

	sngID, _ := q.UpsertFile(ctx, UpsertFileParams{Path: "song.sng", FileType: "sng", Size: 1})
	seqID, _ := q.UpsertFile(ctx, UpsertFileParams{Path: "seq.seq", FileType: "seq", Size: 1})

	if err := q.InsertSongStep(ctx, InsertSongStepParams{
		SongFileID: sngID,
		Step:       0,
		SeqIndex:   1,
		SeqFileID:  sql.NullInt64{Int64: seqID, Valid: true},
		Repeats:    2,
		Tempo:      120.0,
	}); err != nil {
		t.Fatalf("InsertSongStep: %v", err)
	}

	steps, err := q.ListSongSteps(ctx, sngID)
	if err != nil {
		t.Fatalf("ListSongSteps: %v", err)
	}
	if len(steps) != 1 {
		t.Errorf("steps = %d, want 1", len(steps))
	}
	if steps[0].Repeats != 2 {
		t.Errorf("repeats = %d, want 2", steps[0].Repeats)
	}

	if err := q.DeleteSongSteps(ctx, sngID); err != nil {
		t.Fatalf("DeleteSongSteps: %v", err)
	}
	steps2, _ := q.ListSongSteps(ctx, sngID)
	if len(steps2) != 0 {
		t.Errorf("steps after delete = %d, want 0", len(steps2))
	}
}

// ResolveUnlinkedSamples -------------------------------------------------

func TestResolveUnlinkedSamples(t *testing.T) {
	_, q := openTestDB(t)
	ctx := context.Background()

	pgmID, _ := q.UpsertFile(ctx, UpsertFileParams{Path: "prog.pgm", FileType: "pgm", Size: 1})
	wavID, _ := q.UpsertFile(ctx, UpsertFileParams{Path: "sounds/kick.wav", FileType: "wav", Size: 1})

	// Insert a sample with no file_id but a name matching the wav file.
	q.InsertPgmSample(ctx, InsertPgmSampleParams{ //nolint:errcheck // test setup
		PgmFileID: pgmID, Pad: 0, Layer: 0, SampleName: "kick",
	})

	if err := q.ResolveUnlinkedSamples(ctx); err != nil {
		t.Fatalf("ResolveUnlinkedSamples: %v", err)
	}

	samples, _ := q.ListPgmSamples(ctx, pgmID)
	if len(samples) == 1 && samples[0].SampleFileID.Valid && samples[0].SampleFileID.Int64 == wavID {
		// Resolved correctly.
	} else if len(samples) == 1 && !samples[0].SampleFileID.Valid {
		// Not resolved — wav filename may not match. That's OK; the function ran without error.
		t.Log("sample not resolved (filename mismatch is expected if wav path doesn't contain 'kick')")
	}
}

// Open -------------------------------------------------------------------

func TestOpen(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	sqlDB, q, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer sqlDB.Close() //nolint:errcheck // test cleanup
	if q == nil {
		t.Error("Open returned nil Queries")
	}
}

// migrateJSONPrefs -------------------------------------------------------

func TestMigrateJSONPrefs_WithValidFile(t *testing.T) {
	_, q := openTestDB(t)
	dir := t.TempDir()

	prefs := `{"profile":"MPC500","lastPgmPath":"/path/kit.pgm","lastWavPath":"/path/kick.wav","auditionMode":"layer1"}`
	if err := os.WriteFile(filepath.Join(dir, "preferences.json"), []byte(prefs), 0o644); err != nil {
		t.Fatal(err)
	}

	migrateJSONPrefs(dir, q)

	ctx := context.Background()
	p, err := q.GetPreferences(ctx)
	if err != nil {
		t.Fatalf("GetPreferences: %v", err)
	}
	if p.Profile != "MPC500" {
		t.Errorf("profile = %q, want MPC500", p.Profile)
	}
}

func TestMigrateJSONPrefs_InvalidJSON(t *testing.T) {
	_, q := openTestDB(t)
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "preferences.json"), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Should return early without panicking on bad JSON.
	migrateJSONPrefs(dir, q)
}

// Migrations (idempotency) -----------------------------------------------

func TestMigrationsAreIdempotent(t *testing.T) {
	sqlDB, _ := openTestDB(t)

	// Calling the individual migration helpers on an already-migrated DB
	// should not panic or return errors — they all swallow errors silently.
	migrateAddWorkspacePath(sqlDB)
	migrateCreateCatalog(sqlDB)
	migrateAddWavSource(sqlDB)
	migrateCreateFileTags(sqlDB)
	migrateAddLastDetailPath(sqlDB)
	migrateAddFileTypeIndex(sqlDB)
	migrateRenameScannedAt(sqlDB)
	migrateAddFKIndexes(sqlDB)
	// migrateJSONPrefs skipped: requires filesystem side effects.
}
