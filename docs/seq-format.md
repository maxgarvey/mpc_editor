# MPC 1000/500 Sequence File Format (.SEQ)

The .SEQ format stores a single sequence — a pattern of MIDI events across
multiple tracks with tempo and time signature information.

All multi-byte values are **little-endian**. Sequencer resolution is fixed at
**96 PPQN** (pulses per quarter note).

> Format details derived from reverse-engineering by the
> [mpc1k-seq](https://github.com/JOJ0/mpc1k-seq) project (JJOS firmware).
> Compatibility with stock Akai OS is assumed but not fully verified.

---

## Header (0x00-0x37, 56 bytes)

| Offset | Size | Type | Field | Description |
|--------|------|------|-------|-------------|
| 0x00 | 2 | uint16 LE | unknown_01 | Unknown purpose |
| 0x02 | 2 | uint16 LE | unknown_02 | Typically zero |
| 0x04 | 16 | char[16] | version | Version string, e.g. `MPC1000 SEQ 4.40` |
| 0x14 | 8 | uint16 LE x4 | unknown_03 | Four unknown shorts |
| 0x1C | 2 | uint16 LE | bars | Loop length in bars |
| 0x1E | 2 | uint16 LE | unknown_07 | Typically zero |
| 0x20 | 2 | uint16 LE | bpm | BPM x 10 (e.g. 1200 = 120.0 BPM) |
| 0x22 | 14 | uint16 LE x7 | unknown_08 | Seven unknown shorts |
| 0x30 | 4 | uint16 LE x2 | tempo_map_01 | Tempo map entry 1 |
| 0x34 | 4 | uint16 LE x2 | tempo_map_02 | Tempo map entry 2 |

## Track Data (offset 0x0FD0)

64 track chunks, each **48 bytes**. Each chunk contains:

- Track name (up to 16 characters, null-padded)
- MIDI channel assignment (1-16)
- Program number (instrument/PGM reference)
- Track status (active/muted)

MPC 1000 supports all 64 tracks. MPC 500 supports fewer tracks due to
hardware limitations.

## Event Data (offset 0x1C10)

Variable-length section containing MIDI events. Each event:

| Field | Size | Description |
|-------|------|-------------|
| Tick | 4 bytes | Position in ticks (96 PPQN) |
| Track | 1 byte | Track number (0-63) |
| Type | 1 byte | Event type (e.g. 0x90 = note-on) |
| Note | 1 byte | MIDI note number (0-127) |
| Velocity | 1 byte | Velocity (1-127) |
| Duration | variable | Duration in ticks |
| Padding | 7 bytes | Zero padding |

### Tick Resolution

| Musical value | Ticks |
|---------------|-------|
| Whole note | 384 |
| Half note | 192 |
| Quarter note | 96 |
| Eighth note | 48 |
| Sixteenth note | 24 |
| Thirty-second | 12 |

## WAV Filename Storage

WAV filenames referenced by the sequence are stored **non-contiguously** in
two 8-byte chunks at different locations within the file. This unusual encoding
appears to be a space optimization in the original format design. The maximum
searchable string length per segment is 8 characters.

## Unknown Fields

Several header fields remain unidentified. These likely contain:
- Time signature (numerator/denominator)
- Loop mode settings
- Additional tempo map entries
- Quantization settings

Further reverse-engineering with hex dumps of known sequences would be needed
to identify these fields.
