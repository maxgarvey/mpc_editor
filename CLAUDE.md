# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
make build        # compile binary
make run          # build and run server (opens browser on macOS)
make test         # run all tests
make test-race    # run tests with race detector
make test-cover   # run tests with coverage report
make lint         # run golangci-lint (auto-installs golangci-lint v2.11.4)
make check        # vet + lint + tests
make fmt          # format code (fails if gofmt changes anything)
make generate     # regenerate sqlc DB code from SQL definitions
make dev          # live reload with watchexec
make test-e2e     # run Playwright end-to-end tests (headless, from e2e/)
```

Run a single test package: `go test ./internal/pgm/...`  
Run a single test: `go test -run TestName ./internal/pgm/...`

Set `PORT` env var to change the default port (8080).

## Architecture

Single-binary local web app. Go `net/http` backend serves HTML partials; frontend uses HTMX 2.0 for dynamic updates with no JS framework. Audio playback uses the Web Audio API; waveform rendering uses Canvas 2D with server-side peak downsampling.

### State model

`internal/server/Session` (`session.go`) is the single in-memory session — one user, one global session. It holds:
- The active `*pgm.Program` (in-memory binary buffer)
- `SampleMatrix` mapping pad×layer → resolved filesystem path
- `WorkspacePath`, `SelectedPad`, active `*audio.Slicer`, preferences
- Restored from SQLite on startup via `Preferences`

### Binary format (`internal/pgm/`)

`.pgm` files are fixed-size binary buffers (`Buffer` wraps `[]byte`). Every field is a typed parameter object (`OffIntParam`, `OffStringParam`, etc.) that encodes/decodes at a fixed byte offset. `Program` owns a single `Buffer`; pads/layers/envelope/filter/mixer each hold a pointer into it with their own offsets. No serialization step — the buffer IS the file.

- `ProfileMPC1000` (4×4, 2 sliders, 2 filters) vs `ProfileMPC500` (4×3, 1 slider, 1 filter)
- 64 pads total, 4 layers per pad; banks A–D are just offset windows into the 64-pad array

### Database (`internal/db/`)

SQLite at `~/.mpc_editor/mpc_editor.db`. Schema in `schema.sql`, queries in `queries.sql`, generated code in `queries.sql.go` (via sqlc). Connection pool is capped at 1 to prevent `SQLITE_BUSY` between background scanner and UI writes. Migrations run inline at startup in `migrate.go` (additive `ALTER TABLE` and `CREATE TABLE IF NOT EXISTS`).

After editing `queries.sql` or `schema.sql`, run `make generate` to regenerate `queries.sql.go`.

### HTTP handlers (`internal/server/`)

Handlers are split by domain: `handlers_pad.go`, `handlers_audio.go`, `handlers_slicer.go`, `handlers_program.go`, `handlers_browse.go`, etc. All routes are registered in `server.go:registerRoutes()`. Templates are parsed at startup from embedded FS and rendered via `renderTemplate()`.

### Templates (`web/templates/`)

Go `html/template` files. `layout.html` is the shell; everything else is a partial rendered by HTMX swaps. Template functions are registered in `server.go` (e.g. `padBankLabel`, `velocityColor`, `seq`).

### Audio pipeline (`internal/audio/`)

- `wav.go`: WAV I/O
- `slicer.go` + `beatdetect.go`: energy-based transient detection, produces `Markers`
- `marker.go`: marker list with select/nudge/insert/delete
- `waveform.go`: server-side peak downsampling for canvas rendering
- `transcode.go`: crop/export utilities

### Workspace scanner (`internal/scanner/`)

Background goroutine that indexes WAV/PGM/SEQ files from the workspace into the SQLite catalog. Runs on startup and on-demand via `/workspace/scan`.

### MPC device detection (`internal/device/`)

Polls for USB mass storage devices matching MPC vendor/product IDs. Runs as a background goroutine started in `server.New()`.
