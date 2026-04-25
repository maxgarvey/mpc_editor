# MPC 1000/500 Sequence File Format (.SEQ)

The `.SEQ` format stores a single sequence — a pattern of MIDI events across
multiple tracks with tempo and bar-count information.

All multi-byte values are **little-endian**. Sequencer resolution is fixed at
**96 PPQN** (pulses per quarter note), giving 384 ticks per 4/4 bar.

> This spec is derived from direct hex analysis of real MPC 1000 files
> (`Sequence01.SEQ`, `BasicDrumsSeq.SEQ`) captured from the hardware.
> Fields marked **[?]** are observed in the binary but not yet understood.
> Update this file whenever new evidence changes or clarifies the format.

---

## File Layout

```
Offset      Size     Section
------      ----     -------
0x0000      48 B     File header
0x0030      ~4048 B  Internal timing/clock map (fixed structure, variable fill)
0x1000      3072 B   Track headers (64 × 48 bytes)
0x1C00      16 B     Event section separator (ff ff ff ff ff 00 ff ...)
0x1C10      N × 16 B Events (variable count)
EOF - 16    16 B     End terminator (ff ff ff 7f ff ff ff ff ...)
```

The total file size is `0x1C10 + (N × 16) + 16` bytes (events + terminator).
Bytes 0–0x1C0F are fixed size; the file grows or shrinks only by adding/removing
16-byte events.

---

## Header (0x0000–0x002F, 48 bytes)

| Offset | Size | Field       | Notes |
|--------|------|-------------|-------|
| 0x00   | 4    | `file_size` | Total file size as uint32 LE — equals `len(data)` |
| 0x04   | 16   | `version`   | ASCII string, e.g. `MPC1000 SEQ 4.40`, null-padded |
| 0x14   | 8    | [?]         | Unknown; often `00 01 01 00 01 00` + variable bytes |
| 0x1A   | 2    | [?]         | Both files have `0xe8 0x03` = 1000 (purpose unclear) |
| 0x1C   | 2    | `bars`      | Loop length in bars (uint16 LE); confirmed: 1 → `01 00`, 2 → `02 00` |
| 0x1E   | 2    | [?]         | Zero in observed files |
| 0x20   | 4    | `bpm`       | BPM × 10 as uint32 LE; 120.0 BPM → `b0 04 00 00` = 1200 |
| 0x24   | 12   | [?]         | All zeros in observed files |

---

## Internal Timing/Clock Map (0x0030–0x0FFF)

A sequence of 4-byte entries that count upward in tick positions. The pattern
alternates between two interleaved sequences and ends with zero-padding before
0x1000. The purpose is not fully understood — likely an internal timing
reference used by the MPC hardware.

**Observed structure:** each entry is `[flag, counter_lo, counter_hi, 0x60]`
where `flag` alternates between `0x00` and `0x80`, and the counter increments
in steps related to bar length (384 ticks). This region has a fixed size and
is not affected by the number of events.

---

## Track Headers (0x1000–0x1BFF, 64 × 48 bytes)

Each of the 64 tracks occupies one 48-byte chunk at `0x1000 + (trackIndex × 48)`.
Tracks are **0-indexed** internally; in the file the first track is `Track01`.

| Chunk offset | Size | Field         | Notes |
|-------------|------|---------------|-------|
| 0           | 16   | `name`        | Track name, null-padded (e.g. `Track01`) |
| 16          | 16   | `pgm_name`    | Associated PGM filename; first byte is often `0x00`; name starts at byte 17 |
| 32          | 2    | [?]           | Zero in observed files |
| 34          | 1    | `midi_channel`| MIDI channel (1–16 for active track, 0 for empty/unused) |
| 35          | 1    | [?]           | `0x01` in active tracks, `0x00` in empty tracks |
| 36          | 4    | [?]           | `64 00 00 00` = 100 in both files; possibly volume |
| 40          | 2    | [?]           | `1e 00` = 30; purpose unknown |
| 42          | 4    | [?]           | `6e 00 00 00` = 110; purpose unknown |
| 46          | 1    | [?]           | Zero |
| 47          | 1    | [?]           | `32` = 50; possibly pan (0–100, 50 = center) |

**Empty track detection:** if `midi_channel == 0` and `name` is blank, the
track has no content.

---

## Event Section Separator (0x1C00–0x1C0F)

A fixed 16-byte block that precedes all events:

```
ff ff ff ff  ff 00 ff ff  ff ff ff ff  ff ff ff ff
```

Byte 5 is `0x00` (distinguishing it from actual `0xFF`-filled sentinel bytes).
The parser skips this by starting at `0x1C10`.

---

## Events (0x1C10 onward, 16 bytes each)

**Each event is exactly 16 bytes** (not 8 — the 8-byte bit-packed format
described in older references does not match real MPC 1000 files).

| Byte  | Size | Field      | Notes |
|-------|------|------------|-------|
| 0–3   | 4    | `tick`     | Absolute tick position (uint32 LE, 96 PPQN) |
| 4     | 1    | `track`    | Track number, **1-indexed** (0x01 = Track01); convert to 0-indexed internally |
| 5     | 1    | `status`   | MIDI status byte (`0x90` = NoteOn channel 0) |
| 6     | 1    | `note`     | MIDI note number (0–127) |
| 7     | 1    | `velocity` | Velocity (0–127) |
| 8–11  | 4    | `duration` | Note duration in ticks (uint32 LE) |
| 12    | 1    | [?]        | Always `0x00` in observed files |
| 13    | 1    | `pad_idx`  | Pad index (appears to be `note - 36` for factory-mapped pads; redundant with note) |
| 14–15 | 2    | [?]        | Always `0x00 0x00` in observed files |

### Confirmed examples from Sequence01.SEQ (1 bar, 120 BPM)

| Description  | Tick | Status | Note | Vel  | Duration | Pad[13] |
|-------------|------|--------|------|------|----------|---------|
| Pad A1, beat 1 | 0  | 0x90 | 36 | 0x7f | 12 | 0x00 |
| Pad A5, beat 3 | 192 | 0x90 | 40 | 0x7f | 13 | 0x04 |

Tick 192 = beat 3 at 96 PPQN (2 quarter notes × 96 ticks). ✓

---

## End Terminator

The event list ends with a 16-byte terminator:

```
ff ff ff 7f  ff ff ff ff  ff ff ff ff  ff ff ff ff
```

Byte 3 is `0x7F` (not `0xFF`), which distinguishes it from events and the
separator. Parsing stops when this pattern is found.

---

## Note-to-Pad Mapping

MPC pads are mapped to MIDI notes in the sequence data. The MPC factory default
for a fresh PGM is:

| Bank | Pads   | MIDI notes |
|------|--------|-----------|
| A    | A1–A16 | 36–51     |
| B    | B1–B16 | 52–67     |
| C    | C1–C16 | 68–83     |
| D    | D1–D16 | 84–99     |

Formula: `note = 36 + pad_index` where pad_index is 0-based across all 64 pads.
Bank A is pads 0–15, Bank B is 16–31, etc.

**Caveat:** programs can remap pads to any MIDI note. The note stored in byte[6]
reflects the PGM's mapping at the time of recording, not the pad layout. The
`pad_idx` field in byte[13] appears to store the pad's physical index regardless
of the MIDI note remapping, but this is not fully verified.

**Code note:** the codebase currently uses `note - 35` as a chromatic fallback
(pad 0 = note 35). The correct factory default from observed files is `note - 36`
(pad 0 = note 36). This discrepancy should be resolved when the fallback is used.

---

## Tick Resolution Reference

| Musical value  | Ticks |
|----------------|-------|
| 1 bar (4/4)    | 384   |
| Half note      | 192   |
| Quarter note   | 96    |
| Eighth note    | 48    |
| Sixteenth note | 24    |
| 32nd note      | 12    |

---

## Verification Snippet

To inspect events in a real file:
```bash
xxd -s 0x1C10 path/to/file.SEQ | head -20
```
Each 16-byte row is one event. Check: byte[5]=0x90 (NoteOn), bytes[0-3] are
the tick (LE), bytes[8-11] are duration (LE).
