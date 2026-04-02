CREATE TABLE IF NOT EXISTS preferences (
    id             INTEGER PRIMARY KEY CHECK (id = 1),
    profile        TEXT NOT NULL DEFAULT 'MPC1000',
    last_pgm_path  TEXT NOT NULL DEFAULT '',
    last_wav_path  TEXT NOT NULL DEFAULT '',
    audition_mode  TEXT NOT NULL DEFAULT 'layer0',
    workspace_path TEXT NOT NULL DEFAULT ''
);

INSERT OR IGNORE INTO preferences (id) VALUES (1);
