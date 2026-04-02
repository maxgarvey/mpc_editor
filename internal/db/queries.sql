-- name: GetPreferences :one
SELECT profile, last_pgm_path, last_wav_path, audition_mode
FROM preferences WHERE id = 1;

-- name: UpdateProfile :exec
UPDATE preferences SET profile = ? WHERE id = 1;

-- name: UpdateLastPGMPath :exec
UPDATE preferences SET last_pgm_path = ? WHERE id = 1;

-- name: UpdateLastWAVPath :exec
UPDATE preferences SET last_wav_path = ? WHERE id = 1;

-- name: UpdateAuditionMode :exec
UPDATE preferences SET audition_mode = ? WHERE id = 1;

-- name: UpdateAllPreferences :exec
UPDATE preferences SET profile = ?, last_pgm_path = ?, last_wav_path = ?, audition_mode = ?
WHERE id = 1;
