CREATE TABLE IF NOT EXISTS preferences (
    id               INTEGER PRIMARY KEY CHECK (id = 1),
    profile          TEXT NOT NULL DEFAULT 'MPC1000',
    last_pgm_path    TEXT NOT NULL DEFAULT '',
    last_wav_path    TEXT NOT NULL DEFAULT '',
    audition_mode    TEXT NOT NULL DEFAULT 'layer0',
    workspace_path   TEXT NOT NULL DEFAULT '',
    last_detail_path TEXT NOT NULL DEFAULT ''
);

INSERT OR IGNORE INTO preferences (id) VALUES (1);

-- Catalog of all files discovered in the workspace.
CREATE TABLE IF NOT EXISTS files (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    path       TEXT NOT NULL UNIQUE,
    file_type  TEXT NOT NULL,
    size       INTEGER NOT NULL DEFAULT 0,
    mod_time   INTEGER NOT NULL DEFAULT 0,
    scanned_at INTEGER NOT NULL DEFAULT 0,  -- 0 = not yet scanned; otherwise Unix timestamp of last scan
    color       TEXT NOT NULL DEFAULT '',    -- user-assigned display color preset name, e.g. "red"
    category    TEXT NOT NULL DEFAULT '',    -- label category, e.g. "drum"
    subcategory TEXT NOT NULL DEFAULT '',    -- label subcategory, e.g. "hihat"
    favorite    INTEGER NOT NULL DEFAULT 0  -- 1 if starred by user
);
CREATE INDEX IF NOT EXISTS idx_files_favorite ON files(favorite);
CREATE INDEX IF NOT EXISTS idx_files_file_type ON files(file_type);

-- Metadata extracted from .pgm files.
CREATE TABLE IF NOT EXISTS pgm_meta (
    file_id         INTEGER PRIMARY KEY REFERENCES files(id) ON DELETE CASCADE,
    midi_pgm_change INTEGER NOT NULL DEFAULT 0
);

-- Metadata extracted from .wav files.
CREATE TABLE IF NOT EXISTS wav_meta (
    file_id         INTEGER PRIMARY KEY REFERENCES files(id) ON DELETE CASCADE,
    sample_rate     INTEGER NOT NULL DEFAULT 0,
    channels        INTEGER NOT NULL DEFAULT 0,
    bits_per_sample INTEGER NOT NULL DEFAULT 0,
    frame_count     INTEGER NOT NULL DEFAULT 0,
    source          TEXT NOT NULL DEFAULT ''
);

-- Metadata extracted from .seq files.
CREATE TABLE IF NOT EXISTS seq_meta (
    file_id INTEGER PRIMARY KEY REFERENCES files(id) ON DELETE CASCADE,
    bpm     REAL NOT NULL DEFAULT 0,
    bars    INTEGER NOT NULL DEFAULT 0,
    version TEXT NOT NULL DEFAULT ''
);

-- Sample references: which programs use which samples.
CREATE TABLE IF NOT EXISTS pgm_samples (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    pgm_file_id    INTEGER NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    pad            INTEGER NOT NULL,
    layer          INTEGER NOT NULL,
    sample_name    TEXT NOT NULL,
    sample_file_id INTEGER REFERENCES files(id) ON DELETE SET NULL,
    UNIQUE(pgm_file_id, pad, layer)
);
CREATE INDEX IF NOT EXISTS idx_pgm_samples_sample_file_id ON pgm_samples(sample_file_id);

-- Track-to-program assignments in .seq files.
CREATE TABLE IF NOT EXISTS seq_tracks (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    seq_file_id  INTEGER NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    track        INTEGER NOT NULL,
    track_name   TEXT NOT NULL DEFAULT '',
    midi_channel INTEGER NOT NULL DEFAULT 0,
    pgm_file_id  INTEGER REFERENCES files(id) ON DELETE SET NULL,
    UNIQUE(seq_file_id, track)
);
CREATE INDEX IF NOT EXISTS idx_seq_tracks_pgm_file_id ON seq_tracks(pgm_file_id);

-- Song step entries.
CREATE TABLE IF NOT EXISTS song_steps (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    song_file_id INTEGER NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    step         INTEGER NOT NULL,
    seq_index    INTEGER NOT NULL,  -- sequence template index from .sng binary format (not derivable from step order)
    seq_file_id  INTEGER REFERENCES files(id) ON DELETE SET NULL,
    repeats      INTEGER NOT NULL DEFAULT 1,
    tempo        REAL NOT NULL DEFAULT 0,
    UNIQUE(song_file_id, step)
);
CREATE INDEX IF NOT EXISTS idx_song_steps_seq_file_id ON song_steps(seq_file_id);

-- Library provenance: tracks workspace copies back to their sample_library source.
CREATE TABLE IF NOT EXISTS sample_links (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    copy_path     TEXT NOT NULL UNIQUE,   -- relative path of the workspace copy
    library_path  TEXT NOT NULL,          -- relative path of the library source
    checksum      TEXT NOT NULL DEFAULT '',  -- SHA-256 of the library source at copy time
    copied_at     INTEGER NOT NULL DEFAULT 0, -- Unix timestamp of the copy
    sync_status   TEXT NOT NULL DEFAULT '',  -- '', 'ok', 'outdated', 'source_missing'
    src_size      INTEGER NOT NULL DEFAULT 0, -- source size at last sync check (0 = unknown)
    src_mod_time  INTEGER NOT NULL DEFAULT 0  -- source mod time at last sync check (0 = unknown)
);
CREATE INDEX IF NOT EXISTS idx_sample_links_library_path ON sample_links(library_path);

-- Tags attached to files (free-form and key:value).
CREATE TABLE IF NOT EXISTS file_tags (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    file_id   INTEGER NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    tag_key   TEXT NOT NULL DEFAULT '',
    tag_value TEXT NOT NULL,
    auto      INTEGER NOT NULL DEFAULT 0,
    UNIQUE(file_id, tag_key, tag_value)
);
