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

	sqlDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, nil, err
	}

	if _, err := sqlDB.Exec(schemaDDL); err != nil {
		_ = sqlDB.Close()
		return nil, nil, err
	}

	migrateAddWorkspacePath(sqlDB)
	migrateCreateCatalog(sqlDB)
	migrateAddWavSource(sqlDB)
	migrateCreateFileTags(sqlDB)

	queries := New(sqlDB)
	migrateJSONPrefs(dir, queries)

	return sqlDB, queries, nil
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
			id        INTEGER PRIMARY KEY AUTOINCREMENT,
			path      TEXT NOT NULL UNIQUE,
			file_type TEXT NOT NULL,
			size      INTEGER NOT NULL DEFAULT 0,
			mod_time  INTEGER NOT NULL DEFAULT 0,
			scanned   INTEGER NOT NULL DEFAULT 0
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
