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

## Desktop App

Both wrappers follow the same pattern: find a free port, spawn the Go binary with `PORT=<port> NO_BROWSER=1`, wait for it to accept connections, open a window at that URL, and kill the binary on quit. The packaged app bundles the Go binary so no Go toolchain is needed to run it.

### Electron

Uses a bundled Chromium renderer. Requires [Node.js](https://nodejs.org).

```
make electron       # build Go binary + launch Electron app (requires npm)
make electron-dist  # build Go binary + package .dmg (macOS)
```

`make electron` handles `npm install` automatically. See [`electron/README.md`](electron/README.md) for Windows/Linux packaging and more detail.

### Tauri

Uses the platform's native WebView (WKWebView on macOS). Lighter binary, lower memory, no GPU/EGL noise. Requires [Rust](https://rustup.rs).

```
make tauri          # build Go binary + launch Tauri app (auto-installs tauri-cli)
make tauri-dist     # build Go binary + package distributable
```

See [`tauri/README.md`](tauri/README.md) for cross-platform packaging targets and more detail.

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

## Other Directories

| Directory | Description |
|-----------|-------------|
| [`cmd/`](cmd/README.md) | Entry-point binaries: `mpc_editor` (server) and `genseq` (test fixture generator) |
| [`electron/`](electron/README.md) | Electron desktop wrapper — packages the Go binary into a native app |
| [`tauri/`](tauri/README.md) | Tauri desktop wrapper — uses the platform's native WebView instead of Chromium |
| [`e2e/`](e2e/README.md) | Playwright end-to-end tests |
| [`testdata/`](testdata/README.md) | Static test fixtures (`.pgm`, `.seq`, `.wav`) used by unit and e2e tests |
| [`internal/`](internal/README.md) | All application logic (see package dependency graph) |
| [`.github/workflows/`](.github/workflows/README.md) | GitHub Actions CI configuration |

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
