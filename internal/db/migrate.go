package db

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaDDL string

// Open opens (or creates) the SQLite database at ~/.mpc_editor/mpc_editor.db,
// runs the schema DDL, and returns the raw DB and a Queries handle.
func Open() (*sql.DB, *Queries, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, nil, err
	}
	dir := filepath.Join(home, ".mpc_editor")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, nil, err
	}
	dbPath := filepath.Join(dir, "mpc_editor.db")

	sqlDB, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, nil, err
	}
	// SQLite only supports one concurrent writer. Limiting to a single
	// connection lets Go's pool serialize all operations, preventing
	// SQLITE_BUSY errors between background scans and UI writes.
	sqlDB.SetMaxOpenConns(1)

	if _, err := sqlDB.Exec(schemaDDL); err != nil {
		_ = sqlDB.Close()
		return nil, nil, err
	}

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

	queries := New(sqlDB)
	migrateJSONPrefs(dir, queries)

	return sqlDB, queries, nil
}

// ExecSchema executes the canonical schema DDL against db. Useful in tests.
func ExecSchema(sqlDB *sql.DB) error {
	_, err := sqlDB.Exec(schemaDDL)
	return err
}

// migrateAddWorkspacePath adds the workspace_path column to existing databases.
func migrateAddWorkspacePath(sqlDB *sql.DB) {
	_, err := sqlDB.Exec(`ALTER TABLE preferences ADD COLUMN workspace_path TEXT NOT NULL DEFAULT ''`)
	if err != nil {
		// Ignore "duplicate column" — already migrated.
		return
	}
}

// migrateCreateCatalog creates the file catalog tables for existing databases.
// New databases get them from schema.sql; this handles upgrades.
func migrateCreateCatalog(sqlDB *sql.DB) {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS files (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			path       TEXT NOT NULL UNIQUE,
			file_type  TEXT NOT NULL,
			size       INTEGER NOT NULL DEFAULT 0,
			mod_time   INTEGER NOT NULL DEFAULT 0,
			scanned_at INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS pgm_meta (
			file_id         INTEGER PRIMARY KEY REFERENCES files(id) ON DELETE CASCADE,
			midi_pgm_change INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS wav_meta (
			file_id         INTEGER PRIMARY KEY REFERENCES files(id) ON DELETE CASCADE,
			sample_rate     INTEGER NOT NULL DEFAULT 0,
			channels        INTEGER NOT NULL DEFAULT 0,
			bits_per_sample INTEGER NOT NULL DEFAULT 0,
			frame_count     INTEGER NOT NULL DEFAULT 0,
			source          TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS seq_meta (
			file_id INTEGER PRIMARY KEY REFERENCES files(id) ON DELETE CASCADE,
			bpm     REAL NOT NULL DEFAULT 0,
			bars    INTEGER NOT NULL DEFAULT 0,
			version TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS pgm_samples (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			pgm_file_id    INTEGER NOT NULL REFERENCES files(id) ON DELETE CASCADE,
			pad            INTEGER NOT NULL,
			layer          INTEGER NOT NULL,
			sample_name    TEXT NOT NULL,
			sample_file_id INTEGER REFERENCES files(id) ON DELETE SET NULL,
			UNIQUE(pgm_file_id, pad, layer)
		)`,
		`CREATE TABLE IF NOT EXISTS seq_tracks (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			seq_file_id  INTEGER NOT NULL REFERENCES files(id) ON DELETE CASCADE,
			track        INTEGER NOT NULL,
			track_name   TEXT NOT NULL DEFAULT '',
			midi_channel INTEGER NOT NULL DEFAULT 0,
			pgm_file_id  INTEGER REFERENCES files(id) ON DELETE SET NULL,
			UNIQUE(seq_file_id, track)
		)`,
		`CREATE TABLE IF NOT EXISTS song_steps (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			song_file_id INTEGER NOT NULL REFERENCES files(id) ON DELETE CASCADE,
			step         INTEGER NOT NULL,
			seq_index    INTEGER NOT NULL,
			seq_file_id  INTEGER REFERENCES files(id) ON DELETE SET NULL,
			repeats      INTEGER NOT NULL DEFAULT 1,
			tempo        REAL NOT NULL DEFAULT 0,
			UNIQUE(song_file_id, step)
		)`,
	}
	for _, ddl := range tables {
		_, _ = sqlDB.Exec(ddl)
	}
}

// migrateAddWavSource adds the source column to existing wav_meta tables.
func migrateAddWavSource(sqlDB *sql.DB) {
	_, err := sqlDB.Exec(`ALTER TABLE wav_meta ADD COLUMN source TEXT NOT NULL DEFAULT ''`)
	if err != nil {
		// Ignore "duplicate column" — already migrated.
		return
	}
}

// migrateCreateFileTags creates the file_tags table for existing databases.
func migrateCreateFileTags(sqlDB *sql.DB) {
	_, _ = sqlDB.Exec(`CREATE TABLE IF NOT EXISTS file_tags (
		id        INTEGER PRIMARY KEY AUTOINCREMENT,
		file_id   INTEGER NOT NULL REFERENCES files(id) ON DELETE CASCADE,
		tag_key   TEXT NOT NULL DEFAULT '',
		tag_value TEXT NOT NULL,
		auto      INTEGER NOT NULL DEFAULT 0,
		UNIQUE(file_id, tag_key, tag_value)
	)`)
}

// migrateAddLastDetailPath adds the last_detail_path column to existing databases.
func migrateAddLastDetailPath(sqlDB *sql.DB) {
	_, _ = sqlDB.Exec(`ALTER TABLE preferences ADD COLUMN last_detail_path TEXT NOT NULL DEFAULT ''`)
}

// migrateAddFileTypeIndex adds an index on files.file_type for faster type-filtered queries.
func migrateAddFileTypeIndex(sqlDB *sql.DB) {
	_, _ = sqlDB.Exec(`CREATE INDEX IF NOT EXISTS idx_files_file_type ON files(file_type)`)
}

// migrateRenameScannedAt renames the files.scanned column to scanned_at to clarify
// that it stores a Unix timestamp (0 = never scanned), not a boolean.
func migrateRenameScannedAt(sqlDB *sql.DB) {
	_, _ = sqlDB.Exec(`ALTER TABLE files RENAME COLUMN scanned TO scanned_at`)
}

// migrateAddFKIndexes adds indexes on FK columns used in JOINs that SQLite
// does not create automatically.
func migrateAddFKIndexes(sqlDB *sql.DB) {
	_, _ = sqlDB.Exec(`CREATE INDEX IF NOT EXISTS idx_pgm_samples_sample_file_id ON pgm_samples(sample_file_id)`)
	_, _ = sqlDB.Exec(`CREATE INDEX IF NOT EXISTS idx_seq_tracks_pgm_file_id ON seq_tracks(pgm_file_id)`)
	_, _ = sqlDB.Exec(`CREATE INDEX IF NOT EXISTS idx_song_steps_seq_file_id ON song_steps(seq_file_id)`)
}

// migrateAddFileColor adds the color column to existing files tables.
func migrateAddFileColor(sqlDB *sql.DB) {
	_, _ = sqlDB.Exec(`ALTER TABLE files ADD COLUMN color TEXT NOT NULL DEFAULT ''`)
}

// migrateAddFileLabel adds category and subcategory columns to existing files tables.
func migrateAddFileLabel(sqlDB *sql.DB) {
	_, _ = sqlDB.Exec(`ALTER TABLE files ADD COLUMN category TEXT NOT NULL DEFAULT ''`)
	_, _ = sqlDB.Exec(`ALTER TABLE files ADD COLUMN subcategory TEXT NOT NULL DEFAULT ''`)
}

// migrateAddFileFavorite adds the favorite column to the files table.
func migrateAddFileFavorite(sqlDB *sql.DB) {
	_, _ = sqlDB.Exec(`ALTER TABLE files ADD COLUMN favorite INTEGER NOT NULL DEFAULT 0`)
	_, _ = sqlDB.Exec(`CREATE INDEX IF NOT EXISTS idx_files_favorite ON files(favorite)`)
}

// migrateCreateSampleLinks creates the sample_links table for tracking library provenance.
func migrateCreateSampleLinks(sqlDB *sql.DB) {
	_, _ = sqlDB.Exec(`CREATE TABLE IF NOT EXISTS sample_links (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		copy_path    TEXT NOT NULL UNIQUE,
		library_path TEXT NOT NULL,
		checksum     TEXT NOT NULL DEFAULT '',
		copied_at    INTEGER NOT NULL DEFAULT 0
	)`)
	_, _ = sqlDB.Exec(`CREATE INDEX IF NOT EXISTS idx_sample_links_library_path ON sample_links(library_path)`)
}

// migrateAddSampleLinkSyncStatus adds the sync_status column to existing sample_links tables.
func migrateAddSampleLinkSyncStatus(sqlDB *sql.DB) {
	_, _ = sqlDB.Exec(`ALTER TABLE sample_links ADD COLUMN sync_status TEXT NOT NULL DEFAULT ''`)
}

// migrateAddSampleLinkSrcStat adds the source size/mod-time columns used to skip
// re-checksumming unchanged library sources during sync checks.
func migrateAddSampleLinkSrcStat(sqlDB *sql.DB) {
	_, _ = sqlDB.Exec(`ALTER TABLE sample_links ADD COLUMN src_size INTEGER NOT NULL DEFAULT 0`)
	_, _ = sqlDB.Exec(`ALTER TABLE sample_links ADD COLUMN src_mod_time INTEGER NOT NULL DEFAULT 0`)
}

// migrateJSONPrefs migrates preferences from the old JSON file to the database.
// If preferences.json exists, its values are written to the DB and the file is
// renamed to preferences.json.bak.
func migrateJSONPrefs(dir string, queries *Queries) {
	jsonPath := filepath.Join(dir, "preferences.json")
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return
	}

	var p struct {
		Profile      string `json:"profile"`
		LastPGMPath  string `json:"lastPgmPath"`
		LastWAVPath  string `json:"lastWavPath"`
		AuditionMode string `json:"auditionMode"`
	}
	if json.Unmarshal(data, &p) != nil {
		return
	}

	ctx := context.Background()
	_ = queries.UpdateAllPreferences(ctx, UpdateAllPreferencesParams{
		Profile:      p.Profile,
		LastPgmPath:  p.LastPGMPath,
		LastWavPath:  p.LastWAVPath,
		AuditionMode: p.AuditionMode,
	})
	_ = os.Rename(jsonPath, jsonPath+".bak")
}
