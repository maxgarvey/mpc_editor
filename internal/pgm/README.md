# internal/pgm

Reads, writes, and edits Akai MPC 1000/500 program files (`.pgm`).

## Purpose

A `.pgm` file is a fixed-size binary buffer (10,756 bytes for MPC1000) that defines one drum program: 64 pads, each with up to 4 sample layers, per-pad envelope, filter, mixer, and global MIDI settings. This package wraps that buffer with typed accessors — no serialization step exists; the `Buffer` **is** the file.

## Key Types

| Type | Role |
|------|------|
| `Buffer` | Thin wrapper around `[]byte` with little-endian read/write helpers (`GetByte`, `SetShort`, `GetString`, etc.) |
| `Program` | Owns a `Buffer`; entry point for all pad/layer/slider access |
| `Pad` | View into the buffer at a pad's base offset; exposes `Layer`, `Envelope`, `Filter`, `Mixer`, `MIDINote` |
| `Layer` | Per-layer parameters: sample name, level, tuning, play mode, velocity range |
| `Envelope` / `Filter` / `Mixer` / `Slider` | Typed sub-views into the same buffer at calculated offsets |
| `Parameter` | Describes a single field: offset, type (`TypeInt`, `TypeEnum`, `TypeTuning`, `TypeRange`), and valid range |
| `Profile` | Hardware variant: `ProfileMPC1000` (4×4, 2 sliders, 2 filters) or `ProfileMPC500` (4×3, 1 slider, 1 filter) |
| `SampleMatrix` | Session-level map of (pad, layer) → resolved filesystem `SampleRef` |

## File Layout

```
0x0000   File header (size, version string "MPC1000 PGM 1.00")
0x0018   64 pads × 0xA4 bytes each
0x2918   64-byte MIDI note→pad map (one byte per pad)
0x29D8   MIDI program change byte
0x2A04   End (= ProgramFileSize)
```

Each pad occupies `0xA4` bytes at `0x18 + padIndex * 0xA4`. Layers are sub-ranges within that, then envelope, filters, and mixer.

## Workflow

```
Open:   OpenProgram(path) → reads ProgramFileSize bytes → Buffer → Program
Edit:   program.Pad(i).Layer(j).SetSampleName("kick.wav")
Save:   program.Save(path) → writes Buffer bytes verbatim to disk
New:    NewProgram() → zero-filled buffer + sensible defaults (level=100, chromatic MIDI notes)
```

## Design Notes

- All offsets are absolute within the buffer. `Pad.base` + `Parameter.Offset` = field location.
- Pads 0–63 span all banks: bank A = 0–15, B = 16–31, C = 32–47, D = 48–63.
- `SampleMatrix` lives outside the binary; it maps the 16-char name stored in the `.pgm` to a full filesystem path resolved at load time.
- `MultisampleBuilder` handles chromatic note assignment with tuning spread across pads — used for multi-velocity/pitched sample programs.

## References

- Stephen Norum's PGM spec: https://www.mybunnyhug.org/fileformats/pgm/
- Java reference implementation: https://github.com/cyriux/mpcmaid
- Python implementation: https://github.com/stephenn/pympc1000
