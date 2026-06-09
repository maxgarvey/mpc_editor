-- name: GetPreferences :one
SELECT id, profile, last_pgm_path, last_wav_path, audition_mode, workspace_path, last_detail_path
FROM preferences WHERE id = 1;

-- name: UpdateAllPreferences :exec
UPDATE preferences SET profile = ?, last_pgm_path = ?, last_wav_path = ?, audition_mode = ?, workspace_path = ?, last_detail_path = ?
WHERE id = 1;

-- File catalog

-- name: UpsertFile :one
INSERT INTO files (path, file_type, size, mod_time, scanned_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(path) DO UPDATE SET
    file_type  = excluded.file_type,
    size       = excluded.size,
    mod_time   = excluded.mod_time,
    scanned_at = excluded.scanned_at
RETURNING id;

-- name: GetFileByPath :one
SELECT id, path, file_type, size, mod_time, scanned_at, color, category, subcategory, favorite FROM files WHERE path = ?;

-- name: GetFileByID :one
SELECT id, path, file_type, size, mod_time, scanned_at, color, category, subcategory, favorite FROM files WHERE id = ?;

-- name: ListFilesByType :many
SELECT id, path, file_type, size, mod_time, scanned_at, color, category, subcategory, favorite FROM files WHERE file_type = ? ORDER BY path;

-- name: ListAllFiles :many
SELECT id, path, file_type, size, mod_time, scanned_at, color, category, subcategory, favorite FROM files ORDER BY path;

-- name: DeleteFile :exec
DELETE FROM files WHERE id = ?;

-- name: UpdateFilePath :exec
UPDATE files SET path = sqlc.arg(new_path) WHERE path = sqlc.arg(old_path);

-- name: DeleteFileByPath :exec
DELETE FROM files WHERE path = ?;

-- Prefix variants for directory rename/move/delete. substr() comparison is used
-- instead of LIKE so that % and _ in file names cannot over-match.

-- name: MoveFilePathPrefix :exec
UPDATE files SET path = sqlc.arg(new_prefix) || substr(path, length(sqlc.arg(old_prefix)) + 1)
WHERE substr(path, 1, length(sqlc.arg(old_prefix))) = sqlc.arg(old_prefix);

-- name: DeleteFilesByPathPrefix :exec
DELETE FROM files WHERE substr(path, 1, length(sqlc.arg(prefix))) = sqlc.arg(prefix);

-- name: ListFilesWithWavMetaForPrefix :many
SELECT f.id, f.path, f.file_type, f.size, f.mod_time, f.scanned_at,
       f.color, f.category, f.subcategory, f.favorite,
       COALESCE(w.sample_rate, 0)     AS sample_rate,
       COALESCE(w.channels, 0)        AS channels,
       COALESCE(w.bits_per_sample, 0) AS bits_per_sample
FROM files f
LEFT JOIN wav_meta w ON w.file_id = f.id
WHERE substr(f.path, 1, length(sqlc.arg(prefix))) = sqlc.arg(prefix);

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

-- name: CountMissingSamplesForPrefix :many
SELECT ps.pgm_file_id, COUNT(*) AS missing
FROM pgm_samples ps
JOIN files f ON f.id = ps.pgm_file_id
WHERE ps.sample_file_id IS NULL AND ps.sample_name != ''
  AND substr(f.path, 1, length(sqlc.arg(prefix))) = sqlc.arg(prefix)
GROUP BY ps.pgm_file_id;

-- name: ResolveUnlinkedSamples :exec
UPDATE pgm_samples SET sample_file_id = (
    SELECT f.id FROM files f
    WHERE f.file_type = 'wav'
    AND (
        LOWER(f.path) LIKE '%/' || LOWER(pgm_samples.sample_name) || '.wav'
        OR LOWER(f.path) = LOWER(pgm_samples.sample_name) || '.wav'
    )
    LIMIT 1
)
WHERE sample_file_id IS NULL AND sample_name != '';

-- SEQ track references

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

-- Song step references

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
DELETE FROM file_tags WHERE file_id = ? AND tag_key = ? AND tag_value = ? AND auto = 0;

-- name: RemoveAutoTags :exec
DELETE FROM file_tags WHERE file_id = ? AND auto = 1;

-- name: SetFileColor :exec
UPDATE files SET color = ? WHERE id = ?;

-- name: GetFileColor :one
SELECT color FROM files WHERE id = ?;

-- name: ListWavColored :many
SELECT id, path, color FROM files WHERE file_type = 'wav' AND color != '' ORDER BY path;

-- name: SetFileLabel :exec
UPDATE files SET category = ?, subcategory = ? WHERE id = ?;

-- name: GetFileLabel :one
SELECT category, subcategory FROM files WHERE id = ?;

-- name: SetFileFavorite :exec
UPDATE files SET favorite = ? WHERE id = ?;

-- name: GetFileFavorite :one
SELECT favorite FROM files WHERE id = ?;

-- name: ListFavorites :many
SELECT id, path, file_type, size, mod_time, scanned_at, color, category, subcategory, favorite FROM files WHERE favorite = 1 ORDER BY path;

-- Sample library links

-- name: UpsertSampleLink :exec
INSERT INTO sample_links (copy_path, library_path, checksum, copied_at, sync_status, src_size, src_mod_time)
VALUES (?, ?, ?, ?, 'ok', ?, ?)
ON CONFLICT(copy_path) DO UPDATE SET
    library_path = excluded.library_path,
    checksum     = excluded.checksum,
    copied_at    = excluded.copied_at,
    sync_status  = 'ok',
    src_size     = excluded.src_size,
    src_mod_time = excluded.src_mod_time;

-- name: GetSampleLinkByCopyPath :one
SELECT id, copy_path, library_path, checksum, copied_at, sync_status, src_size, src_mod_time FROM sample_links WHERE copy_path = ?;

-- name: DeleteSampleLinkByCopyPath :exec
DELETE FROM sample_links WHERE copy_path = ?;

-- name: DeleteSampleLinksByPathPrefix :exec
DELETE FROM sample_links WHERE substr(copy_path, 1, length(sqlc.arg(prefix))) = sqlc.arg(prefix);

-- name: RenameSampleLink :exec
UPDATE sample_links SET copy_path = sqlc.arg(new_path) WHERE copy_path = sqlc.arg(old_path);

-- name: MoveSampleLinkPrefix :exec
UPDATE sample_links SET copy_path = sqlc.arg(new_prefix) || substr(copy_path, length(sqlc.arg(old_prefix)) + 1)
WHERE substr(copy_path, 1, length(sqlc.arg(old_prefix))) = sqlc.arg(old_prefix);

-- name: ListSampleLinksForDir :many
SELECT id, copy_path, library_path, checksum, copied_at, sync_status, src_size, src_mod_time FROM sample_links WHERE copy_path LIKE ?;

-- name: ListAllSampleLinks :many
SELECT id, copy_path, library_path, checksum, copied_at, sync_status, src_size, src_mod_time FROM sample_links ORDER BY copy_path;

-- name: UpdateSampleLinkSync :exec
UPDATE sample_links SET sync_status = sqlc.arg(sync_status), checksum = sqlc.arg(checksum),
    src_size = sqlc.arg(src_size), src_mod_time = sqlc.arg(src_mod_time)
WHERE copy_path = sqlc.arg(copy_path);

-- name: ListFilesByTag :many
SELECT DISTINCT f.id, f.path, f.file_type, f.size
FROM files f
JOIN file_tags ft ON ft.file_id = f.id
WHERE ft.tag_value = sqlc.arg(value) OR (ft.tag_key = sqlc.arg(key) AND ft.tag_value = sqlc.arg(value))
ORDER BY f.path;
