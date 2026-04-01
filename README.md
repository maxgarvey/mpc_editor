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

## Build & Run

```
go run ./cmd/mpc_editor
```

On macOS, the browser opens automatically. Set `PORT` to change the default port:

```
PORT=9090 go run ./cmd/mpc_editor
```

## Architecture

```
internal/
  pgm/       Binary .pgm format: read/write, pads, layers, parameters
  audio/     WAV file I/O, beat detection slicer, marker management
  midi/      Standard MIDI File Type 0 writer/reader
  command/   Import, export, sample assignment, batch creation
  server/    HTTP handlers, session state, preferences
web/
  templates/ Go html/template files (layout + HTMX partials)
  static/    CSS, JS (HTMX 2.0, Web Audio API, Canvas waveform)
cmd/
  mpc_editor/   Entry point
```

- **Backend**: Go `net/http` server rendering HTML partials
- **Frontend**: HTMX for dynamic updates, no JS framework
- **Audio**: Web Audio API for browser-side playback; server sends WAV data
- **Waveform**: Canvas 2D rendering with server-side peak downsampling

## Development

```
go test ./...
go vet ./...
```

Test fixtures are in `testdata/` (`.pgm` and `.wav` files).
