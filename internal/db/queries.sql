-- name: GetPreferences :one
SELECT profile, last_pgm_path, last_wav_path, audition_mode, workspace_path
FROM preferences WHERE id = 1;

-- name: UpdateProfile :exec
UPDATE preferences SET profile = ? WHERE id = 1;

-- name: UpdateLastPGMPath :exec
UPDATE preferences SET last_pgm_path = ? WHERE id = 1;

-- name: UpdateLastWAVPath :exec
UPDATE preferences SET last_wav_path = ? WHERE id = 1;

-- name: UpdateAuditionMode :exec
UPDATE preferences SET audition_mode = ? WHERE id = 1;

-- name: UpdateWorkspacePath :exec
UPDATE preferences SET workspace_path = ? WHERE id = 1;

-- name: UpdateAllPreferences :exec
UPDATE preferences SET profile = ?, last_pgm_path = ?, last_wav_path = ?, audition_mode = ?, workspace_path = ?
WHERE id = 1;

-- File catalog

-- name: UpsertFile :one
INSERT INTO files (path, file_type, size, mod_time, scanned)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(path) DO UPDATE SET
    size = excluded.size,
    mod_time = excluded.mod_time,
    scanned = excluded.scanned
RETURNING id;

-- name: GetFileByPath :one
SELECT id, path, file_type, size, mod_time, scanned FROM files WHERE path = ?;

-- name: GetFileByID :one
SELECT id, path, file_type, size, mod_time, scanned FROM files WHERE id = ?;

-- name: ListFilesByType :many
SELECT id, path, file_type, size, mod_time, scanned FROM files WHERE file_type = ? ORDER BY path;

-- name: ListAllFiles :many
SELECT id, path, file_type, size, mod_time, scanned FROM files ORDER BY path;

-- name: DeleteFile :exec
DELETE FROM files WHERE id = ?;

-- name: DeleteFileByPath :exec
DELETE FROM files WHERE path = ?;

-- PGM metadata

-- name: UpsertPgmMeta :exec
INSERT INTO pgm_meta (file_id, midi_pgm_change)
VALUES (?, ?)
ON CONFLICT(file_id) DO UPDATE SET midi_pgm_change = excluded.midi_pgm_change;

-- name: GetPgmMeta :one
SELECT file_id, midi_pgm_change FROM pgm_meta WHERE file_id = ?;

-- WAV metadata

-- name: UpsertWavMeta :exec
INSERT INTO wav_meta (file_id, sample_rate, channels, bits_per_sample, frame_count, source)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(file_id) DO UPDATE SET
    sample_rate = excluded.sample_rate,
    channels = excluded.channels,
    bits_per_sample = excluded.bits_per_sample,
    frame_count = excluded.frame_count,
    source = CASE WHEN excluded.source = '' THEN wav_meta.source ELSE excluded.source END;

-- name: GetWavMeta :one
SELECT file_id, sample_rate, channels, bits_per_sample, frame_count, source FROM wav_meta WHERE file_id = ?;

-- name: UpdateWavSource :exec
UPDATE wav_meta SET source = ? WHERE file_id = ?;

-- SEQ metadata

-- name: UpsertSeqMeta :exec
INSERT INTO seq_meta (file_id, bpm, bars, version)
VALUES (?, ?, ?, ?)
ON CONFLICT(file_id) DO UPDATE SET
    bpm = excluded.bpm, bars = excluded.bars, version = excluded.version;

-- name: GetSeqMeta :one
SELECT file_id, bpm, bars, version FROM seq_meta WHERE file_id = ?;

-- PGM sample references

-- name: DeletePgmSamples :exec
DELETE FROM pgm_samples WHERE pgm_file_id = ?;

-- name: InsertPgmSample :exec
INSERT INTO pgm_samples (pgm_file_id, pad, layer, sample_name, sample_file_id)
VALUES (?, ?, ?, ?, ?);

-- name: ListPgmSamples :many
SELECT ps.pad, ps.layer, ps.sample_name, ps.sample_file_id,
       f.path AS sample_path
FROM pgm_samples ps
LEFT JOIN files f ON f.id = ps.sample_file_id
WHERE ps.pgm_file_id = ?
ORDER BY ps.pad, ps.layer;

-- name: ListProgramsUsingSample :many
SELECT DISTINCT f.id, f.path
FROM pgm_samples ps
JOIN files f ON f.id = ps.pgm_file_id
WHERE ps.sample_file_id = ?
ORDER BY f.path;

-- name: CountMissingSamples :one
SELECT COUNT(*) FROM pgm_samples
WHERE pgm_file_id = ? AND sample_file_id IS NULL AND sample_name != '';

-- name: ResolveUnlinkedSamples :exec
UPDATE pgm_samples SET sample_file_id = (
    SELECT f.id FROM files f
    WHERE f.file_type = 'wav'
    AND (LOWER(REPLACE(f.path, '.wav', '')) LIKE '%' || LOWER(pgm_samples.sample_name)
         OR LOWER(REPLACE(f.path, '.WAV', '')) LIKE '%' || LOWER(pgm_samples.sample_name))
    LIMIT 1
)
WHERE sample_file_id IS NULL AND sample_name != '';

-- SEQ track references (future)

-- name: DeleteSeqTracks :exec
DELETE FROM seq_tracks WHERE seq_file_id = ?;

-- name: InsertSeqTrack :exec
INSERT INTO seq_tracks (seq_file_id, track, track_name, midi_channel, pgm_file_id)
VALUES (?, ?, ?, ?, ?);

-- name: ListSeqTracks :many
SELECT st.track, st.track_name, st.midi_channel, st.pgm_file_id,
       f.path AS pgm_path
FROM seq_tracks st
LEFT JOIN files f ON f.id = st.pgm_file_id
WHERE st.seq_file_id = ?
ORDER BY st.track;

-- Song step references (future)

-- name: DeleteSongSteps :exec
DELETE FROM song_steps WHERE song_file_id = ?;

-- name: InsertSongStep :exec
INSERT INTO song_steps (song_file_id, step, seq_index, seq_file_id, repeats, tempo)
VALUES (?, ?, ?, ?, ?, ?);

-- name: ListSongSteps :many
SELECT ss.step, ss.seq_index, ss.seq_file_id, ss.repeats, ss.tempo,
       f.path AS seq_path
FROM song_steps ss
LEFT JOIN files f ON f.id = ss.seq_file_id
WHERE ss.song_file_id = ?
ORDER BY ss.step;

-- File tags

-- name: ListFileTags :many
SELECT id, file_id, tag_key, tag_value, auto FROM file_tags
WHERE file_id = ? ORDER BY tag_key, tag_value;

-- name: AddFileTag :exec
INSERT OR IGNORE INTO file_tags (file_id, tag_key, tag_value, auto)
VALUES (?, ?, ?, ?);

-- name: RemoveFileTag :exec
DELETE FROM file_tags WHERE file_id = ? AND tag_key = ? AND tag_value = ?;

-- name: RemoveAutoTags :exec
DELETE FROM file_tags WHERE file_id = ? AND auto = 1;

-- name: ListFilesByTag :many
SELECT DISTINCT f.id, f.path, f.file_type, f.size
FROM files f
JOIN file_tags ft ON ft.file_id = f.id
WHERE ft.tag_value = ? OR (ft.tag_key = ? AND ft.tag_value = ?)
ORDER BY f.path;
