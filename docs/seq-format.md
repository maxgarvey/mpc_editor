# MPC 1000/500 Sequence File Format (.SEQ)

The .SEQ format stores a single sequence -- a pattern of MIDI events across
multiple tracks with tempo and time signature information.

All multi-byte values are **little-endian**. Sequencer resolution is fixed at
**96 PPQN** (pulses per quarter note).

> Format details derived from reverse-engineering by the
> [mpc1k-seq](https://github.com/JOJ0/mpc1k-seq) project (header/tracks) and
> [izzyreal/mpc](https://github.com/izzyreal/mpc) (8-byte bit-packed event
> encoding, confirmed compatible with MPC 1000).

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

- Bytes 0-15: Track name (16 characters, null-padded)
- Byte 16: MIDI channel assignment (1-16)
- Byte 17: Program number (instrument/PGM reference)
- Byte 18: Track status (active/muted)
- Bytes 19-47: Reserved/unknown

MPC 1000 supports all 64 tracks. MPC 500 supports fewer tracks due to
hardware limitations.

## Event Data (offset 0x1C10)

Each event is exactly **8 bytes**. The event section is terminated by the
sentinel `ff ff ff 7f ff ff ff ff` (byte 3 = 0x7F, not 0xFF — confirmed from
a real MPC1000 file). Parsed naively this yields byte 4 = 0xFF which falls
outside the NoteOn range 0x00–0x7F, signalling end-of-data.

**NoteOff markers**: The MPC inserts note=0 / velocity=0 events at small tick
values (typically 11–23 ticks) between musical events. These are internal
NoteOff markers closing out notes from the previous loop iteration; they are
not musical content.

### Event type determination (byte 4)

- `0x00-0x7F` = **NoteOn** (byte 4 IS the note number)
- `0xA0` = Poly Pressure
- `0xB0` = Control Change
- `0xC0` = Program Change
- `0xD0` = Channel Pressure
- `0xE0` = Pitch Bend

### NoteOn event bit layout (8 bytes)

| Byte | Bits | Field |
|------|------|-------|
| 0 | 0-7 | Tick low byte |
| 1 | 0-7 | Tick high byte (uint16 LE portion) |
| 2 | 0-3 | Tick overflow (bits 16-19, x 65536) |
| 2 | 4-7 | Duration high 4 bits |
| 3 | 0-5 | Track index (0-63) |
| 3 | 6-7 | Duration middle 2 bits |
| 4 | 0-7 | Note number (0-127) |
| 5 | 0-7 | Duration low 8 bits |
| 6 | 0-6 | Velocity (0-127) |
| 6 | 7 | Variation type bit 1 |
| 7 | 0-6 | Variation value |
| 7 | 7 | Variation type bit 2 |

### Field reassembly

**Tick (20-bit):**
```
tick = uint16LE(byte0, byte1) + (byte2 & 0x0F) * 65536
```

**Duration (14-bit):**
```
duration = ((byte2 & 0xF0) << 6) + ((byte3 & 0xC0) << 2) + byte5 - track * 4
```
The `- track * 4` compensates for track bits leaking into the duration calculation.

### Other event types

- **CC (byte 4 = 0xB0):** byte 5 = CC number, byte 6 = CC value
- **Program Change (byte 4 = 0xC0):** byte 5 = program index
- **Pitch Bend (byte 4 = 0xE0):** bytes 5-6 = bend amount

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

WAV filenames referenced by the sequence are stored non-contiguously in
two 8-byte chunks at different locations within the file. This unusual encoding
appears to be a space optimization in the original format design.

## Unknown Fields

Several header fields remain unidentified. These likely contain:
- Time signature (numerator/denominator)
- Loop mode settings
- Additional tempo map entries
- Quantization settings

## Validation

To verify the event format against a real .SEQ file:
```bash
xxd -s 0x1C10 -l 64 path/to/file.SEQ
```
Check: 8-byte alignment, note values in byte 4 (0-127 range), `0xFF x 8`
terminator at end of event data.
