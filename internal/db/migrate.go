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

	queries := New(sqlDB)
	migrateJSONPrefs(dir, queries)

	return sqlDB, queries, nil
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
