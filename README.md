# MPC Editor

A web-based editor for Akai MPC 500/1000 program (.pgm) files, built with Go and HTMX.

Runs locally as a single-binary web app on `http://127.0.0.1:8080`.

## Features

- Open, edit, and save MPC1000/MPC500 .pgm program files (binary-compatible with hardware)
- Visual 4x4 pad grid with bank switching (A/B/C/D, 64 pads total)
- Per-pad parameter editing: 4 sample layers, tuning, level, play mode, velocity range
- Envelope, filter (2x), and mixer controls per pad
- Voice overlap, mute group, and MIDI note assignment
- Audio sample slicer with energy-based transient detection
- Waveform display with interactive marker editing (select, nudge, insert, delete)
- WAV slice export + Standard MIDI File generation from markers
- Batch program creation from directories of WAV files
- Drag-and-drop sample assignment to pads
- In-browser sample audition via Web Audio API
- Chromatic layout and copy-settings-to-all-pads utilities
- MPC1000 and MPC500 profile support
- Persistent user preferences
- Workspace file browser with right-click context menu (rename, move, delete)
- Workspace scanner indexes WAV/PGM files into a local SQLite catalog
- MPC device auto-detection (USB mass storage) with use-as-workspace shortcut
- Sequence viewer for .seq files
- File tagging and source-URL tracking
- Import samples from external directories into workspace

## Build & Run

```
make build    # compile binary
make run      # build and start server
make install  # install to $GOPATH/bin
```

On macOS, the browser opens automatically. Set `PORT` to change the default port:

```
PORT=9090 make run
```

## Architecture

| Package | Description |
|---------|-------------|
| [`internal/pgm`](internal/pgm/README.md) | Binary `.pgm` format: read/write, pads, layers, parameters |
| [`internal/seq`](internal/seq/README.md) | Binary `.seq` sequence format: parse, events, step grid |
| [`internal/audio`](internal/audio/README.md) | WAV I/O, energy-based beat detection, marker management, waveform downsampling |
| [`internal/midi`](internal/midi/README.md) | Standard MIDI File Type 0 writer/reader |
| [`internal/command`](internal/command/README.md) | Import, export, sample assignment, batch creation |
| [`internal/server`](internal/server/README.md) | HTTP handlers, session state, template rendering |
| [`internal/db`](internal/db/README.md) | SQLite schema, migrations, sqlc-generated queries |
| [`internal/scanner`](internal/scanner/README.md) | Background workspace scanner, file catalog, auto-tags |
| [`internal/device`](internal/device/README.md) | MPC USB device auto-detection (macOS/Linux) |
| [`web`](web/README.md) | Go templates, CSS, JS (HTMX 2.0, Web Audio API, Canvas waveform) |

- **Backend**: Go `net/http` server rendering HTML partials
- **Frontend**: HTMX for dynamic updates, no JS framework
- **Audio**: Web Audio API for browser-side playback; server sends WAV data
- **Waveform**: Canvas 2D rendering with server-side peak downsampling

## File Format Docs

| Document | Description |
|----------|-------------|
| [`docs/seq-format.md`](docs/seq-format.md) | Byte-level `.SEQ` format spec (derived from hex analysis of real files) |
| [`docs/references.md`](docs/references.md) | External format references, open-source MPC projects, community resources |

## Development

```
make test         # run tests
make test-race    # run tests with race detector
make test-cover   # run tests with coverage report
make lint         # run golangci-lint (auto-installs)
make check        # vet + lint + tests
make fmt          # format code
make generate     # regenerate sqlc DB code from SQL definitions
make dev          # live reload (requires watchexec)
make test-e2e     # run Playwright end-to-end tests (headless)
make help         # show all targets
```

Test fixtures are in `testdata/` (`.pgm` and `.wav` files). End-to-end tests are in `e2e/` (Playwright).
