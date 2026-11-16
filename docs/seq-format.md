# MPC 1000/500 Sequence File Format (.SEQ)

The `.SEQ` format stores a single sequence — a pattern of MIDI events across
multiple tracks with tempo and bar-count information.

All multi-byte values are **little-endian**. Sequencer resolution is fixed at
**96 PPQN** (pulses per quarter note), giving 384 ticks per 4/4 bar.

> This spec is derived from direct hex analysis of real MPC 1000 files
> (`Sequence01.SEQ`, `BasicDrumsSeq.SEQ`) captured from the hardware, plus
> the 9 sequences in Akai's official MPC 1000 factory `ALL_SEQS&SONGS.ALL`
> (akaipro.com legacy downloads). Each sequence in a `.ALL` file is stored in
> exactly this layout; see [`all-format.md`](all-format.md).
> Fields marked **[?]** are observed in the binary but not yet understood.
> Update this file whenever new evidence changes or clarifies the format.

---

## File Layout

```
Offset      Size     Section
------      ----     -------
0x0000      48 B     File header
0x0030      4000 B   Clock map: start tick of each bar (1000 × 4 bytes)
0x0FD0      48 B     Track-shaped record; zero in .SEQ files (see below)
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
| 0x04   | 16   | `version`   | ASCII string, e.g. `MPC1000 SEQ 4.40`, null-padded. In a `.ALL` block this holds the sequence name instead |
| 0x14   | 1    | [?]         | `0x00` in every observed file |
| 0x15   | 1    | `seq_number`| Sequence number: 1–9 in directory order in the factory `.ALL`, 1 in single `.SEQ` files (likely the slot the sequence was saved from) |
| 0x16   | 1    | [?]         | `0x01` everywhere except the factory "Standard" sequence (47 bars, mixed meter) and the `.ALL` blank template, which have `0x00` |
| 0x17   | 1    | `loop` [?]  | Treated as the loop flag (`0x01` = loop "1-End"). Factory: `0x01` only on the four Rock_Taste grooves, `0x00` on the long demo arrangements; hardware `Sequence01` `0x00`, `BasicDrumsSeq` `0x01`. Unconfirmed on hardware |
| 0x18   | 1    | [?]         | `0x01` in every observed file |
| 0x19   | 1    | [?]         | `0x00` in every observed file |
| 0x1A   | 2    | [?]         | `0xe8 0x03` = 1000 in every file; equals the number of clock map entries, so probably that count |
| 0x1C   | 2    | `bars`      | Loop length in bars (uint16 LE); factory values up to 129 |
| 0x1E   | 2    | [?]         | Zero in observed files |
| 0x20   | 2    | `bpm`       | BPM × 10 as uint16 LE; 120.0 → `b0 04`; factory values include 149.2 and 67.9. Bytes 0x22–0x23 are always zero |
| 0x22   | 14   | [?]         | All zeros in observed files |

---

## Clock Map (0x0030–0x0FCF)

A table of bar start ticks, 1000 entries × 4 bytes. Entry *n* is the tick at
which bar *n* (0-indexed) starts:

| Byte | Meaning |
|------|---------|
| 0–2  | start tick, 24-bit LE |
| 3    | `0x60` (96) in every observed entry — probably ticks per beat (a quarter note) |

Entry `bars` holds the end tick (the sequence length). **This is the only place
time signatures are stored.** Akai's factory "Standard" sequence has bars 1–9 in
4/4 (384 ticks), bars 10–25 in 3/4 (288 ticks) and bars 26–47 in 4/4 again.
Entry *n* minus entry *n*−1 is the length of bar *n*:

```
entry  9: 80 0d 00 60   3456   (+384)
entry 10: a0 0e 00 60   3744   (+288: bar 10 is 3/4)
...
entry 26: 00 21 00 60   8448   (+384: bar 26 is 4/4 again)
entry 47: 80 40 00 60  16512   end tick
entry 48: 00 00 00 00          zero-filled from here
```

Two conventions exist for the entries after the end tick: hardware files in
pure 4/4 keep counting +384 through all 1000 entries, while "Standard" zero-fills
them. Which one the MPC writes after a time signature change, and whether it
reads past the end tick, is unverified. Some third-party files (e.g.
`testdata/test.seq`) leave the whole map zeroed.

**Editor behaviour:** `seq.Parse` exposes the map as `Sequence.BarStarts` and
falls back to 384 ticks per bar when it is zeroed or not increasing.
`PatchFile` rebuilds it when the bar count changes, keeping existing bar lengths
and the file's convention for entries past the end. `Create` writes a uniform 4/4
map. A byte 3 other than `0x60` (e.g. a /8 time signature) has never been seen.

---

## Track Headers (0x1000–0x1BFF, 64 × 48 bytes)

Each of the 64 tracks occupies one 48-byte chunk at `0x1000 + (trackIndex × 48)`.
Tracks are **0-indexed** internally; in the file the first track is `Track01`,
and event byte 4 = 1 refers to it.

| Chunk offset | Size | Field         | Notes |
|-------------|------|---------------|-------|
| 0           | 16   | `name`        | Track name, null-padded (e.g. `Track01`, factory `KIK`, `BASS`) |
| 16          | 16   | `pgm_name`    | Associated program name; byte 16 is `0x00`, the name starts at byte 17 (15 chars). `OFF` in unused factory tracks |
| 32          | 2    | [?]           | Zero in observed files |
| 34          | 1    | `midi_channel`| 1 in hardware `.SEQ` files, 0 for unused tracks. Factory `.ALL` tracks use 1–13, often a different value per instrument (e.g. Rock_Taste bass 12, guitar 9); likely the MIDI output channel, unverified |
| 35          | 1    | `active` [?]  | `0x01` on tracks with a name/program, `0x00` on unused ones |
| 36          | 4    | [?]           | `64 00 00 00` = 100 in OS 4.40 `.SEQ` files; zero in the factory `.ALL` (version 1.00). Possibly track volume |
| 38          | 1    | `pgm_change`  | In the factory `.ALL`: the program change number (1-based) of the track's program, matching that program's own byte `0x29D8` (M_Bass 104, Rock_GTR 101, kits 1–8); 0 = none. Overlaps the 4-byte field above, whose second byte is 0 in `.SEQ` files |
| 40          | 2    | [?]           | `1e 00` = 30 in `.SEQ` files; zero in `.ALL` |
| 42          | 4    | [?]           | `6e 00 00 00` = 110 in `.SEQ` files; zero in `.ALL` |
| 46          | 1    | [?]           | Zero |
| 47          | 1    | [?]           | `32` = 50 in `.SEQ` files (possibly pan, 50 = centre); zero in `.ALL` |

**Empty track detection:** if `midi_channel == 0` and `name` is blank, the
track has no content.

### Record at 0x0FD0

The 48 bytes before the track headers are zero in `.SEQ` files. In `.ALL` blocks
they hold a record shaped like a track header: name `(unused)` (or `Track00` in
the blank template), program `OFF`, byte 34 = 1. Its purpose is unknown; the
editor ignores it and preserves it.

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
described in older references does not match real MPC 1000 files). Events are
sorted by tick; records sharing a tick keep their order (factory files put a
track's program change before its first note).

| Byte  | Size | Field      | Notes |
|-------|------|------------|-------|
| 0–3   | 4    | `tick`     | Absolute tick position (uint32 LE, 96 PPQN) |
| 4     | 1    | `track`    | Track number, **1-indexed** (0x01 = Track01); convert to 0-indexed internally |
| 5     | 1    | `status`   | Event type, see below |
| 6     | 1    | `note`     | MIDI note number (0–127), or data byte 1 |
| 7     | 1    | `velocity` | Velocity (0–127), or data byte 2 |
| 8–9   | 2    | `duration` | Note duration in ticks (uint16 LE) |
| 10    | 1    | `var_type` | Note Variation type: 0 = Tune and 1 = Decay are identified (see below) |
| 11    | 1    | `var_value`| Note Variation value, **signed** (Tune: −80…+120 seen) |
| 12    | 1    | [?]        | `0x00`, except in the `ff ff ff ff` case below |
| 13    | 1    | `pad_idx`  | Pad index in recorded files; **not used for playback** (see below) |
| 14–15 | 2    | [?]        | `0x00 0x00`, except in the `ff ff ff ff` case below |

### Event types (byte 5)

| Status | Meaning | Data |
|--------|---------|------|
| `0x90` | NoteOn | note, velocity, duration, variation |
| `0xC0` | Program change | byte 6 = program number − 1 (MIDI's 0-based numbering): M_Bass (program change 104) is sent as 103. Factory sequences put one at tick 0 on the first track that uses each program |
| `0x82` | Unknown | Only on track 1. Byte 6 = 1–13. Each factory sequence has one at tick 0, mostly with its own sequence number; "Standard" switches it 5 → 10 → 5 → 11 → 12 → 5 at or just before bar lines (ticks 1919, 3444, 5760, 8400, 13052: the ends of bars 5, 9, 26 and 38, and the start of bar 18). The `.ALL` file holds 21 `EffectNN` records, so an effect-preset change is a guess |

The editor parses every record type, shows only NoteOn events on the grid,
and writes all records back unchanged (`WriteEvents`).

### Note Variation (bytes 10–11)

The MPC's per-note variation (the "16 Levels" Tune/Decay/Attack/Filter modes)
is stored with each note. Bytes 8–11 are *not* a 32-bit duration: reading them
that way gives durations in the billions for these events. In the factory
Techno sequence the BASS track plays a bass line from one pad (note 91), with
every note carrying type 0 and values in steps of 10 (−80…+120, likely
tenths of a semitone). Rock_Taste 3's hi-hat track uses type 1 (likely Decay,
for open and closed hats from one sample). Types 2 and 3 (presumably Attack and
Filter) have not been seen.

### Bytes 12–15

Bytes 12–15 are `00 00 00 00` in almost every event. In the factory `.ALL`, about
170 of 13,000 note events have `ff ff ff ff` there instead, with ordinary tick,
note, velocity and duration values. Most are the first note on a track at tick 0,
but Techno has them scattered through its 129 bars. Their meaning is unknown, so
parsers must not require these bytes to be zero.

**Byte 13 is not needed for playback.** It is `0x00` in every one of the roughly
13,000 note events in Akai's factory sequences, even though their notes span
35–98 and hit pads in all four banks of the factory kits. If the sequencer played
by byte 13, every factory beat would play pad A1. The hardware therefore plays
events by note, through the program's note tables. Recorded files such as
`Sequence01.SEQ` do fill it in, but whether it then holds the physical pad or
`note - 36` is unverified: in those files both readings give the same number.
When the editor creates a note or changes its pitch it writes `note - 36`.

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
xxd -s 0x1C10 -c 16 path/to/file.SEQ | head -20
```
Each 16-byte row is one event. Check: byte[5] is the type (`0x90` NoteOn,
`0xC0` program change), bytes[0-3] are the tick (LE), bytes[8-9] the duration
(LE) and bytes[10-11] the Note Variation.

To see bar lengths (time signatures) in the clock map:
```bash
xxd -s 0x30 -l 64 -c 4 path/to/file.SEQ
```
