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
| `ByteDiff` / `DiffBytes` / `DescribeOffset` | Name the field at any byte offset and list differing bytes between two buffers, for analysing hardware-saved files |

## File Layout

```
0x0000   File header (size, version string "MPC1000 PGM 1.00")
0x0018   64 pads × 0xA4 bytes each
0x2918   64 bytes: pad → MIDI note (one byte per pad)
0x2958   128 bytes: MIDI note → pad (one byte per note; 64 = no pad)
0x29D8   MIDI program change byte
0x29D9   2 sliders × 13 bytes
0x2A04   End (= ProgramFileSize)
```

Each pad occupies `0xA4` bytes at `0x18 + padIndex * 0xA4`. Layers are sub-ranges within that, then envelope, filters, and mixer.

## Workflow

```
Open:   OpenProgram(path) → reads ProgramFileSize bytes → Buffer → Program
Edit:   program.Pad(i).Layer(j).SetSampleName("kick.wav")
Save:   program.Save(path) → writes Buffer bytes verbatim to disk
New:    NewProgram() → byte-identical to a new program saved by the MPC 1000 (testdata/chromatic.pgm): layer level 70, decay 5, velocity→level 100, filter freq 100, FX send 33, slider ranges, chromatic MIDI notes from 35
Raw:    OpenProgramRaw(path) → no RepairNoteMap; SetNoteMapEntry / Pad.SetUnknownConstByte / Slider.SetUnknownConstByte
        write single bytes with no syncing (for hardware probes and forensics only, never editor code)
```

## Design Notes

- All offsets are absolute within the buffer. `Pad.base` + `Parameter.Offset` = field location.
- Pads 0–63 span all banks: bank A = 0–15, B = 16–31, C = 32–47, D = 48–63.
- The two MIDI tables must agree. `Pad.SetMIDINote` updates both, and `OpenProgram` calls `RepairNoteMap` to fix stale note→pad entries left by editors that only write pad→note (consistent entries, including which pad wins when two share a note, are kept). `PadForNote` reads the note→pad table. Both tables are consistent, with no shared notes, in every Akai factory program. Which table the hardware actually trusts is unverified. [`cmd/genpgm`](../../cmd/genpgm/README.md) generates probe programs for finding out on real hardware. The MPC500 firmware also keeps both tables in memory (pad→note at program + 0x1ED8, note→pad at + 0x1F08), so the hardware maintains both.
- Shared notes: the only public hardware-saved program with two pads on one note is seaweedfactory/MPC1000 `KEYMAP3.PGM` (saved under JJOS). Its chromatic layout stops at note 98, so pad indices 62 and 63 both play 98, and its note→pad entry for 98 is 62, the lower pad. `RepairNoteMap`'s lowest-pad rule picks the same pad. That shows what JJOS writes on save, not which pad incoming MIDI triggers, which is still unverified.
- "No pad" may not always be 64. antonhornquist/pgm-rb `test/original.pgm` uses **48** for every unmapped note, and the MPC500 has 48 pads, so the value may be the saving model's pad count (unverified; the file's origin is unknown). `RepairNoteMap` rewrites those entries to 64.
- Pad byte 0x65 and slider byte 0x01 are undocumented but always 1 in hardware files, including all 13 Akai factory programs; `NewProgram` writes 1 to match.
- A layer is empty when its name's first byte is 0. Deleting a sample on the hardware zeroes only that byte and leaves the rest of the old name behind (Akai's factory programs contain names like `\0J_KICK`), so never treat trailing name bytes as meaningful.
- Velocity→attack (`+0x69`), velocity→start (`+0x6A`) and velocity→tune (`+0x70`) are named from the MPC500 OS 1.31 firmware (the Open500 projects' reading of its Program-menu getter, 0–100 each), not from MPC1000 files. That code reads the same pad layout and every other offset it reads matches this package's, so the names are probably right for the MPC1000 too. They are zero in every public program examined, and the package does not model them yet.
- Pad `+0x60` (2 bytes, signed LE) is not modelled. It is non-zero on about 100 pads of Akai's factory programs, roughly follows layer 1 tuning ÷ 10 without matching it, and is set on some untuned pads. Since the buffer is saved verbatim, the editor preserves it.
- Factory defaults are not settled. Akai's factory programs have blank pads with layer levels 100 (all 4 layers) and FX send level 100, but `NewProgram` follows `testdata/chromatic.pgm` (70 and 33). Evidence points both ways: the MPC500 firmware's pad reset uses layer level 70, while stigi/mpc1k-node `Program01.PGM` (the MPC's default program name, origin OS unknown) is the `chromatic.pgm` blank with the drum note layout and layer levels 100. It also has non-zero bytes in the 17-byte tail at 0x29F3 that every other file leaves zero. Factory drum kits use a drum note layout (A1=37, A2=36, A3=42, A4=82, …); factory instrument programs use chromatic from 35.
- `SampleMatrix` lives outside the binary; it maps the 16-char name stored in the `.pgm` to a full filesystem path resolved at load time.
- `MultisampleBuilder` handles chromatic note assignment with tuning spread across pads — used for multi-velocity/pitched sample programs.

## Related Modules

| Module | Relationship |
|--------|-------------|
| [`internal/seq`](../seq/README.md) | Reads `.pgm` to resolve MIDI note → pad mappings for sequence display |
| [`internal/command`](../command/README.md) | `SimpleAssign` and `MultisampleAssign` operate on `Program` and `SampleMatrix` |
| [`internal/server`](../server/README.md) | Owns the active `*Program` in `Session`; handlers call `Pad`, `Layer`, `Envelope`, etc. |
| [`cmd/genpgm`](../../cmd/genpgm/README.md) | Builds hardware probe programs with the raw-access functions and diffs saved files with `DiffBytes` |
| [`internal/scanner`](../scanner/README.md) | Extracts `midi_pgm_change` and pad-sample assignments from `.pgm` files for the catalog |

## References

- Stephen Norum's PGM spec: https://www.mybunnyhug.org/fileformats/pgm/
- Java reference implementation: https://github.com/cyriux/mpcmaid
- Python implementation: https://github.com/stephenn/pympc1000
- MPC500 firmware notes (Open500): https://github.com/LA-2500/Open500-LA, https://github.com/M45K/open500
