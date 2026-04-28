# internal/seq

Reads, writes, and edits Akai MPC 1000/500 sequence files (`.SEQ`).

## Purpose

A `.SEQ` file stores one pattern: a tempo, a bar count, 64 track headers, and a list of MIDI events. This package parses the binary format, provides an in-memory `Sequence` struct, and re-serializes events back to disk. It also builds a `StepGrid` — the 2D pad×step table used by the sequence editor UI.

## Key Types

| Type | Role |
|------|------|
| `Sequence` | Parsed file: `BPM`, `Bars`, `Loop`, `Tracks[64]`, `Events[]` |
| `Track` | Track metadata: name, associated PGM filename, MIDI channel |
| `Event` | One MIDI event: `Tick`, `Track`, `Type`, `Note`, `Velocity`, `Duration` |
| `GridParams` | Display parameters: time signature (`BeatsPerBar`, `BeatDenom`) + step division (`TicksPerStep`) |
| `StepGrid` | Visualization model: `BankAPadRows` + `ExtraBankPadRows`, each a `[]PadRow` |
| `PadRow` | One pad's cells across all bars: `[]StepCell` |
| `StepCell` | One cell: `Active`, `Velocity`, `Duration`, `Bar`, `StepInBar`, `GlobalStep` |

## Timing Constants

| Constant | Value | Meaning |
|----------|-------|---------|
| `PPQN` | 96 | Ticks per quarter note |
| `TicksPerStep` | 24 | Default: 1/16th note |
| `StepsPerBar` | 16 | Default: 4/4 at 1/16 |
| `TicksPerBar` | 384 | = 16 × 24 |

Step division is configurable: 1/8 (48 ticks), 1/16 (24), 1/32 (12), 1/64 (6), triplets.

## File Layout

```
0x0000   48 B    File header (size, version, bars, BPM, loop flag at 0x17)
0x0030   ~4 KB   Internal timing/clock map (1000 × 4-byte entries)
0x1000   3 KB    64 track headers (64 × 48 bytes)
0x1C00   16 B    Event section separator
0x1C10   N×16 B  Events (sorted by tick, variable count)
EOF-16   16 B    End terminator (ff ff ff 7f ...)
```

See `docs/seq-format.md` for the complete byte-level specification.

## Coordinate System

`GlobalStep = (Bar - 1) × StepsPerBar + StepInBar`

GlobalStep maps directly to a tick: `tick = globalStep × TicksPerStep`. The UI uses the CSS class `step-col-{GlobalStep}` to address cells without needing bar/step coordinates.

## Workflow

```
Parse:      Open(path) → seq.Parse(data) → Sequence
Edit grid:  BuildGrid(sequence, noteToPadMap, gridParams) → StepGrid
Write:      WriteEvents(path, sequence) — preserves header, rewrites event region
Patch:      PatchFile(path, bpm, bars) — in-place header update
            PatchLoop(path, loop)      — toggles loop flag (byte 0x17)
Create:     Create(bpm, bars, name, pgm, loop, events) → []byte (full fresh file)
```

## Note-to-Pad Mapping

Events store MIDI notes, not pad indices. The mapping from note → pad is provided by the loaded `.pgm` file. The default factory mapping is `note - 36` for bank A (pad 0 = note 36). A chromatic fallback (`note - 35`) is used when no program is loaded.

## References

- Binary format spec: `docs/seq-format.md` (project-internal, derived from hex analysis of real files)
- Python reference: https://github.com/JOJ0/mpc1k-seq
