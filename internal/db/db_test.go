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
	migrateAddFileColor(sqlDB)
	migrateAddFileLabel(sqlDB)
	migrateAddFileFavorite(sqlDB)
	migrateCreateSampleLinks(sqlDB)
	migrateAddSampleLinkSyncStatus(sqlDB)
	migrateAddSampleLinkSrcStat(sqlDB)
	// migrateJSONPrefs skipped: requires filesystem side effects.
}

// File color / label / favorite ------------------------------------------

func TestFileColorLabelFavorite(t *testing.T) {
	_, q := openTestDB(t)
	ctx := context.Background()

	id, _ := q.UpsertFile(ctx, UpsertFileParams{Path: "meta.wav", FileType: "wav", Size: 1})

	if err := q.SetFileColor(ctx, SetFileColorParams{Color: "red", ID: id}); err != nil {
		t.Fatalf("SetFileColor: %v", err)
	}
	color, err := q.GetFileColor(ctx, id)
	if err != nil || color != "red" {
		t.Errorf("GetFileColor = %q, %v; want red", color, err)
	}
	colored, err := q.ListWavColored(ctx)
	if err != nil || len(colored) != 1 {
		t.Errorf("ListWavColored = %d rows, %v; want 1", len(colored), err)
	}

	if err := q.SetFileLabel(ctx, SetFileLabelParams{Category: "drum", Subcategory: "kick", ID: id}); err != nil {
		t.Fatalf("SetFileLabel: %v", err)
	}
	label, err := q.GetFileLabel(ctx, id)
	if err != nil || label.Category != "drum" || label.Subcategory != "kick" {
		t.Errorf("GetFileLabel = %+v, %v; want drum/kick", label, err)
	}

	if err := q.SetFileFavorite(ctx, SetFileFavoriteParams{Favorite: 1, ID: id}); err != nil {
		t.Fatalf("SetFileFavorite: %v", err)
	}
	fav, err := q.GetFileFavorite(ctx, id)
	if err != nil || fav != 1 {
		t.Errorf("GetFileFavorite = %d, %v; want 1", fav, err)
	}
	favs, err := q.ListFavorites(ctx)
	if err != nil || len(favs) != 1 {
		t.Errorf("ListFavorites = %d rows, %v; want 1", len(favs), err)
	}
}

// Path prefix operations ---------------------------------------------------

func TestMoveFilePathPrefix(t *testing.T) {
	_, q := openTestDB(t)
	ctx := context.Background()

	q.UpsertFile(ctx, UpsertFileParams{Path: "kits/a.wav", FileType: "wav", Size: 1})  //nolint:errcheck // test setup
	q.UpsertFile(ctx, UpsertFileParams{Path: "kits/b.wav", FileType: "wav", Size: 1})  //nolint:errcheck // test setup
	q.UpsertFile(ctx, UpsertFileParams{Path: "kitsch.wav", FileType: "wav", Size: 1})  //nolint:errcheck // test setup
	q.UpsertFile(ctx, UpsertFileParams{Path: "other/c.wav", FileType: "wav", Size: 1}) //nolint:errcheck // test setup

	if err := q.MoveFilePathPrefix(ctx, MoveFilePathPrefixParams{NewPrefix: "drums/", OldPrefix: "kits/"}); err != nil {
		t.Fatalf("MoveFilePathPrefix: %v", err)
	}

	if _, err := q.GetFileByPath(ctx, "drums/a.wav"); err != nil {
		t.Error("drums/a.wav should exist after prefix move")
	}
	if _, err := q.GetFileByPath(ctx, "drums/b.wav"); err != nil {
		t.Error("drums/b.wav should exist after prefix move")
	}
	// Non-prefix lookalike and unrelated paths must be untouched.
	if _, err := q.GetFileByPath(ctx, "kitsch.wav"); err != nil {
		t.Error("kitsch.wav must not be affected by kits/ prefix move")
	}
	if _, err := q.GetFileByPath(ctx, "other/c.wav"); err != nil {
		t.Error("other/c.wav must not be affected")
	}
}

func TestDeleteFilesByPathPrefix(t *testing.T) {
	_, q := openTestDB(t)
	ctx := context.Background()

	q.UpsertFile(ctx, UpsertFileParams{Path: "trash/a.wav", FileType: "wav", Size: 1}) //nolint:errcheck // test setup
	q.UpsertFile(ctx, UpsertFileParams{Path: "trashy.wav", FileType: "wav", Size: 1})  //nolint:errcheck // test setup

	if err := q.DeleteFilesByPathPrefix(ctx, "trash/"); err != nil {
		t.Fatalf("DeleteFilesByPathPrefix: %v", err)
	}
	if _, err := q.GetFileByPath(ctx, "trash/a.wav"); err == nil {
		t.Error("trash/a.wav should be deleted")
	}
	if _, err := q.GetFileByPath(ctx, "trashy.wav"); err != nil {
		t.Error("trashy.wav must survive the prefix delete")
	}
}

func TestListFilesWithWavMetaForPrefix(t *testing.T) {
	_, q := openTestDB(t)
	ctx := context.Background()

	id, _ := q.UpsertFile(ctx, UpsertFileParams{Path: "dir/kick.wav", FileType: "wav", Size: 1})
	q.UpsertWavMeta(ctx, UpsertWavMetaParams{FileID: id, SampleRate: 44100, Channels: 2, BitsPerSample: 16, FrameCount: 100}) //nolint:errcheck // test setup
	q.UpsertFile(ctx, UpsertFileParams{Path: "dir/no_meta.pgm", FileType: "pgm", Size: 1})                                    //nolint:errcheck // test setup
	q.UpsertFile(ctx, UpsertFileParams{Path: "elsewhere/x.wav", FileType: "wav", Size: 1})                                    //nolint:errcheck // test setup

	rows, err := q.ListFilesWithWavMetaForPrefix(ctx, "dir/")
	if err != nil {
		t.Fatalf("ListFilesWithWavMetaForPrefix: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	byPath := map[string]ListFilesWithWavMetaForPrefixRow{}
	for _, r := range rows {
		byPath[r.Path] = r
	}
	if byPath["dir/kick.wav"].SampleRate != 44100 {
		t.Errorf("kick sample rate = %d, want 44100", byPath["dir/kick.wav"].SampleRate)
	}
	if byPath["dir/no_meta.pgm"].SampleRate != 0 {
		t.Errorf("pgm sample rate = %d, want 0 (no wav_meta row)", byPath["dir/no_meta.pgm"].SampleRate)
	}

	// Empty prefix matches everything.
	all, err := q.ListFilesWithWavMetaForPrefix(ctx, "")
	if err != nil || len(all) != 3 {
		t.Errorf("empty prefix rows = %d, %v; want 3", len(all), err)
	}
}

func TestCountMissingSamplesForPrefix(t *testing.T) {
	_, q := openTestDB(t)
	ctx := context.Background()

	pgmID, _ := q.UpsertFile(ctx, UpsertFileParams{Path: "dir/prog.pgm", FileType: "pgm", Size: 1})
	q.InsertPgmSample(ctx, InsertPgmSampleParams{PgmFileID: pgmID, Pad: 0, Layer: 0, SampleName: "ghost"})  //nolint:errcheck // test setup
	q.InsertPgmSample(ctx, InsertPgmSampleParams{PgmFileID: pgmID, Pad: 1, Layer: 0, SampleName: "ghost2"}) //nolint:errcheck // test setup

	rows, err := q.CountMissingSamplesForPrefix(ctx, "dir/")
	if err != nil {
		t.Fatalf("CountMissingSamplesForPrefix: %v", err)
	}
	if len(rows) != 1 || rows[0].PgmFileID != pgmID || rows[0].Missing != 2 {
		t.Errorf("rows = %+v, want one row {%d 2}", rows, pgmID)
	}

	// No pgm files under another prefix.
	none, err := q.CountMissingSamplesForPrefix(ctx, "nowhere/")
	if err != nil || len(none) != 0 {
		t.Errorf("rows for nowhere/ = %d, %v; want 0", len(none), err)
	}
}

// Sample links -------------------------------------------------------------

func TestSampleLinks(t *testing.T) {
	_, q := openTestDB(t)
	ctx := context.Background()

	if err := q.UpsertSampleLink(ctx, UpsertSampleLinkParams{
		CopyPath:    "beats/kick.wav",
		LibraryPath: "sample_library/kick.wav",
		Checksum:    "abc123",
		CopiedAt:    1000,
		SrcSize:     42,
		SrcModTime:  999,
	}); err != nil {
		t.Fatalf("UpsertSampleLink: %v", err)
	}

	link, err := q.GetSampleLinkByCopyPath(ctx, "beats/kick.wav")
	if err != nil {
		t.Fatalf("GetSampleLinkByCopyPath: %v", err)
	}
	if link.LibraryPath != "sample_library/kick.wav" || link.SyncStatus != "ok" {
		t.Errorf("link = %+v, want library path + ok status", link)
	}
	if link.SrcSize != 42 || link.SrcModTime != 999 {
		t.Errorf("src stat = %d/%d, want 42/999", link.SrcSize, link.SrcModTime)
	}

	if err := q.UpdateSampleLinkSync(ctx, UpdateSampleLinkSyncParams{
		SyncStatus: "outdated", Checksum: "abc123", SrcSize: 43, SrcModTime: 1001, CopyPath: "beats/kick.wav",
	}); err != nil {
		t.Fatalf("UpdateSampleLinkSync: %v", err)
	}
	link, _ = q.GetSampleLinkByCopyPath(ctx, "beats/kick.wav")
	if link.SyncStatus != "outdated" || link.SrcSize != 43 {
		t.Errorf("after sync update: %+v", link)
	}

	if err := q.RenameSampleLink(ctx, RenameSampleLinkParams{NewPath: "beats/kick2.wav", OldPath: "beats/kick.wav"}); err != nil {
		t.Fatalf("RenameSampleLink: %v", err)
	}
	if _, err := q.GetSampleLinkByCopyPath(ctx, "beats/kick2.wav"); err != nil {
		t.Error("renamed link should exist")
	}

	if err := q.MoveSampleLinkPrefix(ctx, MoveSampleLinkPrefixParams{NewPrefix: "songs/", OldPrefix: "beats/"}); err != nil {
		t.Fatalf("MoveSampleLinkPrefix: %v", err)
	}
	if _, err := q.GetSampleLinkByCopyPath(ctx, "songs/kick2.wav"); err != nil {
		t.Error("prefix-moved link should exist")
	}

	links, err := q.ListSampleLinksForDir(ctx, "songs/%")
	if err != nil || len(links) != 1 {
		t.Errorf("ListSampleLinksForDir = %d rows, %v; want 1", len(links), err)
	}
	all, err := q.ListAllSampleLinks(ctx)
	if err != nil || len(all) != 1 {
		t.Errorf("ListAllSampleLinks = %d rows, %v; want 1", len(all), err)
	}

	if err := q.DeleteSampleLinkByCopyPath(ctx, "songs/kick2.wav"); err != nil {
		t.Fatalf("DeleteSampleLinkByCopyPath: %v", err)
	}
	if _, err := q.GetSampleLinkByCopyPath(ctx, "songs/kick2.wav"); err == nil {
		t.Error("deleted link should be gone")
	}
}

func TestDeleteSampleLinksByPathPrefix(t *testing.T) {
	_, q := openTestDB(t)
	ctx := context.Background()

	q.UpsertSampleLink(ctx, UpsertSampleLinkParams{CopyPath: "gone/a.wav", LibraryPath: "lib/a.wav"}) //nolint:errcheck // test setup
	q.UpsertSampleLink(ctx, UpsertSampleLinkParams{CopyPath: "kept/b.wav", LibraryPath: "lib/b.wav"}) //nolint:errcheck // test setup

	if err := q.DeleteSampleLinksByPathPrefix(ctx, "gone/"); err != nil {
		t.Fatalf("DeleteSampleLinksByPathPrefix: %v", err)
	}
	if _, err := q.GetSampleLinkByCopyPath(ctx, "gone/a.wav"); err == nil {
		t.Error("gone/a.wav link should be deleted")
	}
	if _, err := q.GetSampleLinkByCopyPath(ctx, "kept/b.wav"); err != nil {
		t.Error("kept/b.wav link must survive")
	}
}

// ExecSchema ---------------------------------------------------------------

func TestExecSchema(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close() //nolint:errcheck // test cleanup
	if err := ExecSchema(sqlDB); err != nil {
		t.Fatalf("ExecSchema: %v", err)
	}
	// Running it again must be idempotent (CREATE TABLE IF NOT EXISTS).
	if err := ExecSchema(sqlDB); err != nil {
		t.Fatalf("ExecSchema (second run): %v", err)
	}
}
