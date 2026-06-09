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

### Sequence format (`internal/seq/`)

`.seq` files: parse, edit, and write back binary MPC sequences. `BuildGrid` produces the `StepGrid` (pad×step table) rendered by the sequence editor. Byte-level spec lives in `docs/seq-format.md`.

### Other domain packages

- `internal/midi/` — Standard MIDI File (Type 0) writer/reader; the slicer export uses it to emit a `.mid` alongside slices
- `internal/command/` — application-layer operations (import/validate samples, assign to pads, multisample layout) called by `handlers_assign.go`; no HTTP dependencies

### Database (`internal/db/`)

SQLite at `~/.mpc_editor/mpc_editor.db`. Schema in `schema.sql`, queries in `queries.sql`, generated code in `queries.sql.go` (via sqlc). Connection pool is capped at 1 to prevent `SQLITE_BUSY` between background scanner and UI writes. Migrations run inline at startup in `migrate.go` (additive `ALTER TABLE` and `CREATE TABLE IF NOT EXISTS`).

After editing `queries.sql` or `schema.sql`, run `make generate` to regenerate `queries.sql.go`.

### HTTP handlers (`internal/server/`)

Handlers are split by domain: `handlers_pad.go`, `handlers_audio.go`, `handlers_slicer.go`, `handlers_program.go`, `handlers_browse.go`, etc. All routes are registered in `server.go:registerRoutes()`. Templates are parsed at startup from embedded FS and rendered via `renderTemplate()`.

### Templates (`web/templates/`)

Go `html/template` files. `layout.html` is the shell; everything else is a partial rendered by HTMX swaps. Template functions are registered in `server.go` (e.g. `padBankLabel`, `velocityColor`, `seq`).

### JavaScript (`web/static/js/`)

Vanilla JS globals — no bundler. Files are loaded in order via `<script>` tags; all functions are global. `app.js` loads first and defines shared utilities (`escapeHtml`, `escapeAttr`, `formatBytes`) that the other modules depend on.

| File | Responsibility |
|------|---------------|
| `app.js` | Core init, HTMX event handlers, param/bank/pad tab highlighting, `initDetailContent`, Save/Settings/Mkdir modals, WAV browser preview, `SearchChips` (search + filter chips), `BrowseGroups`/`BrowseSort` (file-nav grouping and sort), `WorkspacePanel`, shared utilities |
| `drag_drop.js` | Drag-and-drop onto pad buttons and slicer canvas, XHR file upload with progress bar |
| `file_browser.js` | Context menu, inline rename, Delete modal, Move modal |
| `new_modal.js` | New Program / New Sequence / Import Files modal |
| `pad_assignment.js` | WAV-to-pad assignment, Drag-to-Pad modal, pad grid/params refresh, Sample Picker, Pad Picker |
| `device_transfer.js` | MPC USB file transfer modal |
| `audio.js` | `AudioPlayer` — Web Audio API playback, cache, pad invalidation |
| `tabs.js` | `TabManager` — tab open/close/highlight, fetch-based tab activation |
| `sequencer.js` | `SequencePlayer` / `SequenceEditor` — step grid playback and editing |
| `wav_detail_player.js` | WAV detail panel transport controls |
| `wav_waveform.js` | WAV detail panel waveform canvas |
| `waveform.js` | Slicer waveform canvas |

`layout.html` loads all modules except `waveform.js`; `slicer_page.html` loads `audio.js`, `waveform.js`, `app.js`, and `drag_drop.js`; `sequence_page.html` loads `audio.js` and `sequencer.js`.

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

## Rules

### Tests
- When adding or changing a function, add or update tests in the corresponding `*_test.go` file in the same package.
- When adding a new HTTP handler, add tests for at least: the happy path, method-not-allowed (if the handler restricts methods), and missing/invalid required params.
- Keep package coverage at or above 80%. Run `make check-coverage` before opening a PR to verify. Run `make test-cover` to see the full per-function breakdown.
- Do not use `seedFile` when the test needs a valid (parseable) file on disk — `seedFile` overwrites with a placeholder. Instead, copy the real file with `os.WriteFile` and seed the catalog separately with `srv.queries.UpsertFile(...)`.

### Documentation
- When adding a new package, create a `README.md` in that package directory following the style of existing package READMEs (one-paragraph summary, key types/functions, cross-links).
- When changing a public API (exported type, function signature, or HTTP route), update the relevant `README.md` and the architecture section of this file if the change affects the overall design.
- Do not create or update documentation for internal implementation details — only document the public interface and the "why" that isn't obvious from the code.
