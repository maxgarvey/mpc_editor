# internal

All application logic lives here. Nothing in `internal/` is importable by external Go modules — it is private to this repository by Go convention.

## Packages

| Package | Description |
|---------|-------------|
| [`pgm/`](pgm/README.md) | Binary `.pgm` program format: read, write, pad/layer/envelope/filter/mixer parameters |
| [`seq/`](seq/README.md) | Binary `.seq` sequence format: parse events, build step grid, write back |
| [`audio/`](audio/README.md) | WAV I/O, energy-based beat detection, slice marker management, waveform downsampling |
| [`midi/`](midi/README.md) | Standard MIDI File (Type 0) writer and reader |
| [`command/`](command/README.md) | Application-layer operations: import samples, assign to pads, export programs |
| [`server/`](server/README.md) | HTTP handlers, session state, template rendering, route registration |
| [`db/`](db/README.md) | SQLite schema, migrations, and sqlc-generated query layer |
| [`scanner/`](scanner/README.md) | Background workspace scanner: catalogs WAV/PGM/SEQ files, extracts metadata, writes auto-tags |
| [`device/`](device/README.md) | MPC USB device detection via filesystem polling |

## Dependency Graph

```
server ──► pgm, seq, audio, midi, command, db, scanner, device
scanner──► pgm, seq, audio, db
command──► pgm, audio
seq    ──► pgm (note→pad mapping at display time)
midi   ──► (no internal deps)
audio  ──► (no internal deps)
pgm    ──► (no internal deps)
db     ──► (no internal deps)
device ──► (no internal deps)
```

`server` is the only package with broad dependencies. All others are self-contained or have a single dependency, making them independently testable.
