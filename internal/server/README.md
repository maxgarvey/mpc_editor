# internal/server

HTTP server, request handlers, session state, and template rendering for the MPC Editor web UI.

## Purpose

Ties everything together: serves the single-page HTMX application, handles all user actions (edit pad parameters, slice audio, manage sequences, browse files), and owns the one global in-memory session.

## Architecture

```
server.go          Server struct, route registration, template func map
session.go         Session: in-memory state (active program, slicer, workspace path)
preferences.go     Persistent user prefs (profile, last paths) — load/save via DB
template_data.go   Shared structs passed to Go html/template files
workspace.go       Workspace path validation and resolution helpers

handlers_pad.go        Pad parameter read/write, batch copy
handlers_edit.go       Program-level edits (MIDI note, profile switch)
handlers_audio.go      WAV audition (stream PCM to browser), waveform peak data
handlers_slicer.go     Beat detection UI: load WAV, adjust sensitivity, export slices
handlers_sequence.go   Sequence step grid: view, insert, edit, delete, multi-select, playback events
handlers_browse.go     Workspace file browser, search, context-menu actions
handlers_file.go       File operations: rename, move, delete, new-folder
handlers_program.go    Open/save/new .pgm file, session program management
handlers_assign.go     Drag-and-drop sample assignment to pads
handlers_import.go     Import samples from external directories
handlers_device.go     MPC USB device status + use-as-workspace action
handlers_scan.go       Trigger workspace rescan
handlers_settings.go   Settings modal (profile, workspace path, preferences)
handlers_api.go        JSON API endpoints consumed by JS
handlers_detail.go     Detail panel tab management
```

## Session (`session.go`)

A single `Session` is created at startup and holds:

| Field | Type | Purpose |
|-------|------|---------|
| `Program` | `*pgm.Program` | The active program binary (always non-nil) |
| `Matrix` | `pgm.SampleMatrix` | Pad×layer → filesystem path map |
| `FilePath` | `string` | Path to the current `.pgm` file |
| `WorkspacePath` | `string` | Root of the MPC workspace directory tree |
| `Slicer` | `*audio.Slicer` | Active slicer instance (nil when no WAV is loaded) |
| `Profile` | `pgm.Profile` | MPC1000 or MPC500 hardware variant |
| `Prefs` | `Preferences` | Persisted settings loaded from SQLite on startup |

On startup, the session restores the last-opened program and workspace from `Preferences`. If the file still exists, it is re-opened and the sample matrix is rebuilt.

## Route Map (summary)

| Path | Handler |
|------|---------|
| `GET /` | Main layout + detail panel |
| `GET /sequence?path=...` | Sequence step grid (HTMX partial or full page) |
| `POST /sequence/event/edit` | Toggle / move / delete / multi-select operations on events |
| `POST /sequence/toggle-loop` | Toggle the loop flag; returns JSON `{"loop":bool}` |
| `GET /sequence/events` | JSON event list for JS playback engine |
| `POST /sequence/update` | Update BPM/bars; re-renders grid partial |
| `GET /browse` | Workspace file tree (HTMX partial) |
| `GET /browse/search` | Filtered file list |
| `POST /edit/pad` | Write a pad parameter |
| `POST /edit/profile` | Switch MPC profile |
| `GET /audio/stream` | Stream WAV PCM to Web Audio API |
| `GET /audio/waveform` | Downsampled peak pairs for canvas |
| `POST /slicer/load` | Load WAV into slicer |
| `POST /slicer/export` | Export slices + generate MIDI |

## Template Rendering

Templates are parsed from the embedded `web/templates/` FS at startup with a `template.FuncMap` that adds helpers: `add`, `mul`, `mod`, `seq`, `padBankLabel`, `velocityColor`, `velocityOpacity`. All handler responses call `s.renderTemplate(w, name, data)`.

HTMX requests (`HX-Request: true`) receive partial HTML; full-page requests receive the complete layout.

## Security Notes

All user-supplied paths are validated via `s.resolvePath()` and `s.validateWithinWorkspace()` before any file I/O to prevent directory traversal. File rename/move/delete operations are also workspace-scoped.

## Related Modules

| Module | Relationship |
|--------|-------------|
| [`internal/pgm`](../pgm/README.md) | `Session.Program` is a `*pgm.Program`; pad/layer handlers read and write it directly |
| [`internal/seq`](../seq/README.md) | Sequence handlers open `.seq` files, call `BuildGrid`, `WriteEvents`, `PatchLoop`, `PatchFile` |
| [`internal/audio`](../audio/README.md) | `Session.Slicer` is an `*audio.Slicer`; audio stream and waveform handlers use `OpenWAV` and `DownsamplePeaks` |
| [`internal/midi`](../midi/README.md) | Slicer export handler calls `midi.WriteSMF` to produce a `.mid` file |
| [`internal/command`](../command/README.md) | Import, assign, and export handlers delegate to `command.ImportSamples`, `SimpleAssign`, `ExportProgram` |
| [`internal/db`](../db/README.md) | `Server.queries` is a `*db.Queries`; all catalog reads/writes (preferences, file tags, seq meta) go through it |
| [`internal/scanner`](../scanner/README.md) | `Server.scanner` is called after file writes to keep the catalog fresh |
| [`internal/device`](../device/README.md) | `Server.detector` polls for USB MPC devices; `handlers_device.go` exposes the result to the UI |
| [`web`](../../web/README.md) | Templates and static assets are embedded via `web.TemplateFS` / `web.StaticFS` and served by the server |
