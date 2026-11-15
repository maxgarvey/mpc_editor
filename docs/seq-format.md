# MPC 1000/500 Sequence File Format (.SEQ)

The `.SEQ` format stores a single sequence — a pattern of MIDI events across
multiple tracks with tempo and bar-count information.

All multi-byte values are **little-endian**. Sequencer resolution is fixed at
**96 PPQN** (pulses per quarter note), giving 384 ticks per 4/4 bar.

> This spec is derived from direct hex analysis of real MPC 1000 files
> (`Sequence01.SEQ`, `BasicDrumsSeq.SEQ`) captured from the hardware, plus
> the event records in Akai's official MPC 1000 factory `ALL_SEQS&SONGS.ALL`
> (akaipro.com legacy downloads), which uses the same 16-byte event layout.
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
| 12    | 1    | [?]        | `0x00`, except in the `ff ff ff ff` case below |
| 13    | 1    | `pad_idx`  | Pad index in recorded files; **not used for playback** (see below) |
| 14–15 | 2    | [?]        | `0x00 0x00`, except in the `ff ff ff ff` case below |

Bytes 12–15 are `00 00 00 00` in almost every event. In the factory `.ALL`, about
170 of 13,000 note events have `ff ff ff ff` there instead, with ordinary tick,
note, velocity and duration values. Their meaning is unknown, so parsers must not
require these bytes to be zero.

**Byte 13 is not needed for playback.** It is `0x00` in every one of the roughly
13,000 note events in Akai's factory sequences, even though their notes span
35–98 and hit pads in all four banks of the factory kits. If the sequencer played
by byte 13, every factory beat would play pad A1. The hardware therefore plays
events by note, through the program's note tables. Recorded files such as
`Sequence01.SEQ` do fill it in, but whether it then holds the physical pad or
`note - 36` is unverified: in those files both readings give the same number.

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

Sequence events store MIDI notes, not pads. The program's pad→note table
(see [`internal/pgm`](../internal/pgm/README.md)) decides which pad a note plays.
Two layouts appear in Akai's own files:

| Layout | Used by | Bank A notes (A1–A16) |
|--------|---------|-----------------------|
| Drum | Factory drum kits; MPC Maid `default.pgm` | 37, 36, 42, 82, 40, 38, 46, 44, 48, 47, 45, 43, 49, 55, 51, 53 |
| Chromatic from 35 | Factory instrument programs; `testdata/chromatic.pgm` | 35–50 (`note = 35 + pad_index` across all 64 pads) |

All 9 factory drum kits share an identical 64-entry drum table, so a factory
sequence's notes only make sense against that layout. A plain `note - 36`
mapping matches neither layout.

**Caveat:** programs can remap pads to any MIDI note. The note stored in byte[6]
reflects the PGM's mapping at the time of recording, not the pad layout. Map
events to pads by note (byte[6]) and ignore byte[13], which can be zero (see
Events above).

**Code note:** when no program is available, `internal/seq/grid.go` falls back to
`note - 35`, which matches the chromatic layout. Factory drum-kit sequences need
the program's table to land on the right pads.

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
