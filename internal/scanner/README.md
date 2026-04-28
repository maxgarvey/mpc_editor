# internal/scanner

Background workspace scanner that catalogs MPC files into the SQLite database.

## Purpose

Keeps the file browser and search index up-to-date. Walks the workspace directory, discovers WAV/PGM/SEQ/MID/SNG files, and records their metadata (size, mod time, audio format, BPM, etc.) into the `files` table and associated metadata tables. Prunes catalog entries for files that no longer exist on disk.

## Recognized File Types

| Extension | Catalog type | Metadata extracted |
|-----------|-------------|-------------------|
| `.wav` | `wav` | sample_rate, channels, bits_per_sample, frame_count |
| `.pgm` | `pgm` | midi_pgm_change, pad×layer sample names, auto-tags |
| `.seq` | `seq` | bpm, bars, version string, track names, MIDI channels |
| `.mid` | `mid` | (cataloged, no parsed metadata) |
| `.sng` | `sng` | (cataloged, no parsed metadata) |
| `.all` | `all` | (cataloged, no parsed metadata) |

## Workflow

```
1. Scanner.ScanWorkspace(workspace) walks the directory tree.
2. For each recognized file:
   a. Compute relative path.
   b. Upsert into files table (create or update if size/mod_time changed).
   c. If new or changed, parse metadata and upsert into the type-specific meta table.
   d. Regenerate auto-tags (e.g. tag_key="bpm", tag_value="120").
3. Prune stale catalog entries for paths no longer on disk.
4. Return ScanResult with counts (found, scanned, removed, errors).
```

## Invocation

- **Startup**: `Server.New()` launches `scanner.ScanWorkspace` in a goroutine immediately.
- **On demand**: `POST /workspace/scan` triggers a re-scan (e.g. after importing files or changing the workspace).
- **Post-edit**: handlers that create or modify files (new sequence, slice export, sample import) call `scanner.ScanWorkspace` in a goroutine after the write to keep the catalog fresh.

## Performance Notes

Files are only re-parsed if their `size` or `mod_time` changed since the last scan. The scan skips hidden directories (names starting with `.`). Parsing errors are collected into `ScanResult.Errors` but do not stop the scan.

## Auto-Tags

After parsing, the scanner writes machine-generated tags (tagged with `auto=1`) to `file_tags`. These can be searched in the file browser. Examples:
- `bpm=120` on a `.seq` file
- `bars=4` on a `.seq` file

Auto-tags are removed and regenerated on each re-parse to stay current.
