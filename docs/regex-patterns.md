# MPC 1000/500 Binary Format — Regex Pattern Reference

A regex-as-spec representation of the known binary layouts. Every pattern here
is derived from Go source code and confirmed hex analysis of real MPC 1000
hardware files — not guessed. Fields marked **[?]** are observed bytes whose
purpose is unknown; their patterns are included to help you locate and isolate
them, not to claim understanding.

## Purpose

Regex is a useful second notation for binary formats alongside prose specs:

1. **Identification** — confirm a file is what its extension claims.
2. **Extraction** — pull named fields from known byte positions.
3. **Structural anchoring** — orient yourself in an unknown hex dump; each
   `[?]` region is bounded so you know exactly where to focus.
4. **Differential reverse-engineering** — match two files and compare captures
   to see what changed.

These patterns complement [`seq-format.md`](seq-format.md) and
[`song-format.md`](song-format.md). Where those documents describe intent and
context, these patterns describe concrete byte constraints.

## Engine Requirements

All patterns use **PCRE syntax** with `(?s)` (DOTALL flag: `.` matches `\x00`
and newlines). They work on raw bytes.

| Tool | Usage |
|------|-------|
| Python | `re.compile(rb'...', re.DOTALL).search(data)` |
| grep | `grep -Pao --text '...' file.pgm` |
| Perl | `perl -0777 -ne 'print if /pattern/s' file.pgm` |
| ripgrep | `rg -a --no-unicode -P 'pattern' file.pgm` |

Named capture groups (`(?P<name>...)`) work in Python and PCRE. Use `(?x)` to
enable free-spacing mode (whitespace and `#` comments ignored in the pattern).

---

## .PGM (Program Files)

### Fixed size: 10,756 bytes (0x2A04)

All PGM files are exactly this size. The size is also stored as a little-endian
uint32 in the first 4 bytes, so `\x04\x2a\x00\x00` is a constant prefix.

### Identification

```
(?s)^\x04\x2a\x00\x00MPC1000 PGM 1\.00
```

- `\x04\x2a\x00\x00` — file size 10,756 as LE uint32 (constant for all PGMs)
- `MPC1000 PGM 1\.00` — 16-byte ASCII magic at offset 0x04 (`.` escaped)

Shell check:
```bash
grep -Pca '^\x04\x2a\x00\x00MPC1000 PGM 1\.00' --text file.pgm
# prints 1 if valid, 0 otherwise
```

### File layout

```
Offset    Size    Field / Pattern
------    ----    ---------------
0x0000    4       file_size       \x04\x2a\x00\x00  (LE uint32 = 10756)
0x0004    16      version         "MPC1000 PGM 1.00" ASCII
0x0014    4       [?]             unknown header tail
0x0018    10496   pads            64 pads × 164 bytes (see below)
0x2918    64      midi_note_map   one byte per pad: MIDI note number
0x2958    128     note_to_pad     one byte per MIDI note: pad index (64 = no pad)
0x29D8    1       midi_pgm_change MIDI program change value (0=off, 1–128)
0x29D9    13      slider_0        slider 0 (see below)
0x29E6    13      slider_1        slider 1
0x29F3    17      [?]             unknown tail (zero in most files; set in one, see below)
0x2A04    —       EOF
```

Full-file structural pattern (validates all section boundaries):
```python
import re

PGM_LAYOUT = re.compile(
    rb'(?s)^'
    rb'(?P<file_size>\x04\x2a\x00\x00)'   # 0x0000  fixed LE uint32
    rb'(?P<version>MPC1000 PGM 1\.00)'    # 0x0004  magic
    rb'(?P<hdr_unknown>.{4})'             # 0x0014  [?]
    rb'(?P<pads>.{10496})'                # 0x0018  64 × 164 bytes
    rb'(?P<midi_note_map>.{64})'          # 0x2918  MIDI notes
    rb'(?P<note_to_pad>.{128})'           # 0x2958  note → pad
    rb'(?P<midi_pgm_change>.)'            # 0x29D8  program change
    rb'(?P<slider_0>.{13})'              # 0x29D9  slider 0
    rb'(?P<slider_1>.{13})'              # 0x29E6  slider 1
    rb'(?P<unknown_2>.{17})'             # 0x29F3  [?]
    rb'$'
)
```

### Pad structure (164 bytes at `0x18 + N × 0xA4`)

Relative offsets within a pad. Fields marked **[MPC500 firmware]** come from
the Open500 projects' reading of the MPC500 OS 1.31 Program-menu code (see
[`references.md`](references.md#open500-mpc500-firmware-add-ons)). The MPC500
keeps programs in memory with this same pad layout, and every other offset that
code reads matches the fields below. These three are not yet confirmed in
MPC1000 files: they are zero in every public program examined.

```
+0x00   24  layer_0         (see layer structure below)
+0x18   24  layer_1
+0x30   24  layer_2
+0x48   24  layer_3
+0x60   2   [?]
+0x62   1   voice_overlap   0=Poly, 1=Mono
+0x63   1   mute_group      0-32 (0=off)
+0x64   2   [?]
+0x66   1   env_attack      0-100
+0x67   1   env_decay       0-100
+0x68   1   env_decay_mode  0=End, 1=Start
+0x69   1   env_vel_attack  velocity → attack (0-100) [MPC500 firmware]
+0x6A   1   env_vel_start   velocity → start (0-100) [MPC500 firmware]
+0x6B   1   env_vel_level   velocity → level (0-100)
+0x6C   4   [?]
+0x70   1   vel_tune        velocity → tune (0-100) [MPC500 firmware]
+0x71   1   filter1_type    0=Off, 1=LP, 2=BP, 3=HP
+0x72   1   filter1_freq    0-100
+0x73   1   filter1_res     0-100
+0x74   4   [?]
+0x78   1   filter1_vel_f   velocity → freq (0-100)
+0x79   1   filter2_type    0=Off, 1=LP, 2=BP, 3=HP, 4=Link
+0x7A   1   filter2_freq    0-100
+0x7B   1   filter2_res     0-100
+0x7C   4   [?]
+0x80   1   filter2_vel_f   velocity → freq (0-100)
+0x81   14  [?]
+0x8F   1   mixer_level     0-100
+0x90   1   mixer_pan       0-49=L, 50=Center, 51-100=R
+0x91   1   mixer_output    0=Stereo, 1=1-2, 2=3-4
+0x92   1   fx_send         0=Off, 1=1, 2=2
+0x93   1   fx_send_level   0-100
+0x94   1   filter_atten    0=0dB, 1=-6dB, 2=-12dB
+0x95   15  [?]
```

Pattern for a single pad (164 bytes):
```python
PAD_RE = re.compile(
    rb'(?s)'
    rb'(?P<layer_0>.{24})'                 # +0x00
    rb'(?P<layer_1>.{24})'                 # +0x18
    rb'(?P<layer_2>.{24})'                 # +0x30
    rb'(?P<layer_3>.{24})'                 # +0x48
    rb'.{2}'                               # +0x60 [?]
    rb'(?P<voice_overlap>[\x00\x01])'      # +0x62
    rb'(?P<mute_group>[\x00-\x20])'        # +0x63
    rb'.{2}'                               # +0x64 [?]
    rb'(?P<env_attack>[\x00-\x64])'        # +0x66
    rb'(?P<env_decay>[\x00-\x64])'         # +0x67
    rb'(?P<env_decay_mode>[\x00\x01])'     # +0x68
    rb'(?P<env_vel_attack>.)'              # +0x69 [MPC500 fw]
    rb'(?P<env_vel_start>.)'               # +0x6A [MPC500 fw]
    rb'(?P<env_vel_level>[\x00-\x64])'     # +0x6B
    rb'.{4}'                               # +0x6C [?]
    rb'(?P<vel_tune>.)'                    # +0x70 [MPC500 fw]
    rb'(?P<filter1_type>[\x00-\x03])'      # +0x71
    rb'(?P<filter1_freq>[\x00-\x64])'      # +0x72
    rb'(?P<filter1_res>[\x00-\x64])'       # +0x73
    rb'.{4}'                               # +0x74 [?]
    rb'(?P<filter1_vel_f>[\x00-\x64])'     # +0x78
    rb'(?P<filter2_type>[\x00-\x04])'      # +0x79
    rb'(?P<filter2_freq>[\x00-\x64])'      # +0x7A
    rb'(?P<filter2_res>[\x00-\x64])'       # +0x7B
    rb'.{4}'                               # +0x7C [?]
    rb'(?P<filter2_vel_f>[\x00-\x64])'     # +0x80
    rb'.{14}'                              # +0x81 [?]
    rb'(?P<mixer_level>[\x00-\x64])'       # +0x8F
    rb'(?P<mixer_pan>[\x00-\x64])'         # +0x90
    rb'(?P<mixer_output>[\x00-\x02])'      # +0x91
    rb'(?P<fx_send>[\x00-\x02])'           # +0x92
    rb'(?P<fx_send_level>[\x00-\x64])'     # +0x93
    rb'(?P<filter_atten>[\x00-\x02])'      # +0x94
    rb'.{15}'                              # +0x95 [?]
)
```

### Layer structure (24 bytes at `pad_base + M × 0x18`)

```
+0x00   16  sample_name     null-padded ASCII, max 16 chars (no extension)
+0x10   1   [?]             byte between name and level fields
+0x11   1   level           0-100
+0x12   1   range_lo        velocity range low (0-127)
+0x13   1   range_hi        velocity range high (0-127)
+0x14   2   tuning          int16 LE: value = semitones × 100 (-3600 to +3600)
+0x16   1   play_mode       0=One Shot, 1=Note On
+0x17   1   [?]             padding byte
```

Pattern for a single layer:
```python
LAYER_RE = re.compile(
    rb'(?s)'
    rb'(?P<sample_name>[^\x00]{0,16})\x00{0,16}'  # name + null pad (16 bytes total)
    rb'.'                                           # +0x10 [?]
    rb'(?P<level>[\x00-\x64])'                     # +0x11  0-100
    rb'(?P<range_lo>[\x00-\x7f])'                  # +0x12  0-127
    rb'(?P<range_hi>[\x00-\x7f])'                  # +0x13  0-127
    rb'(?P<tuning>.{2})'                           # +0x14  int16 LE
    rb'(?P<play_mode>[\x00\x01])'                  # +0x16
    rb'.'                                           # +0x17 [?]
)
```

> **Note:** the sample_name field is always exactly 16 bytes; the name is
> null-terminated within that field. Use `data[off:off+16].split(b'\x00')[0]`
> for extraction rather than the regex above, which is fragile on names that
> fill all 16 bytes with no null.

### Sample name scan (Python)

List every assigned sample across all pads and layers:

```python
PAD_SECTION = 0x18
PAD_SIZE    = 0xA4   # 164 bytes
LAYER_SIZE  = 0x18   # 24 bytes

with open('file.pgm', 'rb') as f:
    data = f.read()

for pad in range(64):
    bank = 'ABCD'[pad // 16]
    pos  = pad % 16 + 1
    for layer in range(4):
        off = PAD_SECTION + pad * PAD_SIZE + layer * LAYER_SIZE
        name = data[off:off+16].split(b'\x00')[0]
        if name:
            print(f'{bank}{pos:02d} layer {layer}: {name.decode()}')
```

### MIDI note→pad map (0x2918, 64 bytes)

One byte per pad (pads 0-63). The factory default assigns note 35+N to pad N,
though programs can remap arbitrarily.

```python
MIDI_MAP_OFFSET = 0x2918
midi_notes = list(data[MIDI_MAP_OFFSET:MIDI_MAP_OFFSET+64])
# midi_notes[0] = MIDI note for pad 0 (A01), etc.
```

Pattern for the default (factory) note map:
```
(?s)^.{10520}[\x23-\x62]{64}
```
(10520 = 0x2918; notes 35-98 = `\x23` to `\x62`)

### Slider structure (13 bytes at `0x29D9 + N × 13`)

```
+0x00   1   pad             0=off, 1-64=pad number
+0x01   1   [?]
+0x02   1   parameter       0=Tune, 1=Filter, 2=Layer, 3=Attack, 4=Decay
+0x03   2   tune_range      int8 pair: lo, hi (-120 to +120)
+0x05   2   filter_range    int8 pair: lo, hi (-50 to +50)
+0x07   2   layer_range     uint8 pair: lo, hi (0-127)
+0x09   2   attack_range    uint8 pair: lo, hi (0-100)
+0x0B   2   decay_range     uint8 pair: lo, hi (0-100)
```

---

## .SEQ (Sequence Files)

### Variable size: `0x1C10 + (N_events + 1) × 16` bytes

The event count varies; everything before 0x1C10 is fixed size.

### Identification

Quick check (magic only):
```
(?s)^.{4}MPC1000 SEQ 4\.40
```

Structural check (also validates clock map):
```
(?s)^.{4}MPC1000 SEQ 4\.40.{28}\x00\x00\x00\x60\x80\x01\x00\x60
```
The 28-byte gap is `0x30 - 0x14 = 0x1C`: the remainder of the header after
the magic. Clock entries 0 and 1 appear at 0x30 (`\x00\x00\x00\x60`) and 0x34
(`\x80\x01\x00\x60` = tick 384).

### File layout

```
Offset    Size     Field / Pattern
------    ----     ---------------
0x0000    4        file_size       LE uint32 = actual file length
0x0004    16       version         "MPC1000 SEQ 4.40" ASCII
0x0014    1        [?]             0x00 in all observed files
0x0015    1        seq_number      sequence slot number (1–9 in the factory .ALL; 1 in .SEQ files)
0x0016    1        [?]             0x01, except 0x00 in the factory "Standard" sequence
0x0017    1        loop_flag [?]   0x00=off, 0x01=loop "1-End" (unconfirmed on hardware)
0x0018    1        [?]             0x01 in all observed files
0x0019    1        [?]             0x00 in all observed files
0x001A    2        [?]             0xe8 0x03 = 1000 LE (probably the clock map entry count)
0x001C    2        bars            bar count uint16 LE
0x001E    2        [?]             0x00 0x00 in all observed files
0x0020    2        bpm_x10         BPM × 10 as uint16 LE (120.0 → 0xb0 0x04 = 1200)
0x0022    14       [?]             all zero in observed files
0x0030    4000     clock_map       bar start ticks, 1000 entries × 4 bytes (see below)
0x0FD0    48       [?]             zero in .SEQ; a track-shaped "(unused)" record in .ALL blocks
0x1000    3072     track_headers   64 tracks × 48 bytes (see below)
0x1C00    16       separator       ff ff ff ff ff 00 ff ff ff ff ff ff ff ff ff ff
0x1C10    N×16     events          NoteOn, program change and other records (see below)
EOF-16    16       terminator      ff ff ff 7f ff ff ff ff ff ff ff ff ff ff ff ff
```

> **BPM storage:** the code writes `PutUint16` at 0x0020 (2 bytes), not a
> uint32. The `seq-format.md` description of "uint32 LE" reflects that bytes
> 0x0022–0x0023 are always zero and were assumed padding; the canonical storage
> is 2 bytes at 0x0020.

Full-layout pattern:
```python
SEQ_LAYOUT = re.compile(
    rb'(?s)^'
    rb'(?P<file_size>.{4})'              # 0x0000
    rb'(?P<version>MPC1000 SEQ 4\.40)'   # 0x0004
    rb'.'                                 # 0x0014 [?]
    rb'(?P<seq_number>.)'                # 0x0015
    rb'.'                                 # 0x0016 [?]
    rb'(?P<loop_flag>[\x00\x01])'        # 0x0017
    rb'.{2}'                              # 0x0018 [?] constants
    rb'(?P<unknown_1000>.{2})'           # 0x001A [?] constant 1000
    rb'(?P<bars>.{2})'                   # 0x001C
    rb'.{2}'                              # 0x001E [?]
    rb'(?P<bpm_x10>.{2})'                # 0x0020
    rb'.{14}'                             # 0x0022 [?]
    rb'(?P<clock_map>.{4048})'           # 0x0030  4000 data + 48 padding
    rb'(?P<track_headers>.{3072})'       # 0x1000
    rb'(?P<separator>\xff\xff\xff\xff\xff\x00\xff{10})'  # 0x1C00
    rb'(?P<events>.+?)'                  # 0x1C10
    rb'(?P<terminator>\xff\xff\xff\x7f\xff{12})'
    rb'$'
)
```

Header field extraction:
```python
import re, struct

HEADER_RE = re.compile(
    rb'(?s)^'
    rb'(?P<file_size>.{4})'
    rb'(?P<version>.{16})'    # 0x0004
    rb'.{3}'                   # 0x0014 [?]
    rb'(?P<loop_flag>.)'       # 0x0017
    rb'.{4}'                   # 0x0018 [?]
    rb'(?P<bars>.{2})'         # 0x001C
    rb'.{2}'                   # 0x001E [?]
    rb'(?P<bpm_x10>.{2})'      # 0x0020
)

with open('file.SEQ', 'rb') as f:
    data = f.read()

m = HEADER_RE.match(data)
if m:
    file_size = struct.unpack('<I', m.group('file_size'))[0]
    version   = m.group('version').rstrip(b'\x00').decode()
    loop      = bool(m.group('loop_flag')[0])
    bars      = struct.unpack('<H', m.group('bars'))[0]
    bpm       = struct.unpack('<H', m.group('bpm_x10'))[0] / 10.0
    print(f'{version} | BPM={bpm} Bars={bars} Loop={loop}')
```

### Clock map (0x0030–0x0FCF, 1000 entries × 4 bytes)

Entry N holds the tick at which bar N starts, as a 3-byte little-endian integer
followed by `\x60`; entry `bars` holds the end tick. Bar lengths are the only
record of time signatures (384 ticks = 4/4, 288 = 3/4). See
[`seq-format.md`](seq-format.md#clock-map-0x00300x0fcf).

```
byte[0..2] = bar start tick, 24-bit LE
byte[3]    = 0x60 (96; probably ticks per beat)
```

MPC500 firmware code calls byte 3 the bar's time signature. It is `0x60` on
both the 3/4 and the 4/4 bars of the factory "Standard" sequence, so it can
only carry the denominator, most likely as ticks per beat (`0x60` = a quarter
note; `0x30` predicted for /8, unverified).

A 4/4 sequence (byte[0] alternates `\x00` / `\x80` because 384 = 0x180):

| Entry | Tick  | Bytes             |
|-------|-------|-------------------|
| 0     | 0     | `00 00 00 60`     |
| 1     | 384   | `80 01 00 60`     |
| 2     | 768   | `00 03 00 60`     |
| 3     | 1152  | `80 04 00 60`     |

A 3/4 bar adds 288 (`0x120`) instead, e.g. `a0 0e 00 60` (3744) after
`80 0d 00 60` (3456). Pure 4/4 hardware files keep counting through all 1000
entries; Akai's mixed-meter "Standard" zero-fills the entries after the end tick.

4/4 fingerprint (first four entries):
```
\x00\x00\x00\x60\x80\x01\x00\x60\x00\x03\x00\x60\x80\x04\x00\x60
```

Single-entry pattern (any bar length; the all-zero form marks unused entries):
```
(?s)[\x00-\xff]{3}\x60|\x00{4}
```

### Event section markers

**Separator** (fixed at 0x1C00):
```
\xff\xff\xff\xff\xff\x00\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff
```
Byte 5 = `\x00` distinguishes it from the surrounding `\xff` fill.

**Terminator** (final 16 bytes of file):
```
\xff\xff\xff\x7f\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff
```
Byte 3 = `\x7f` (not `\xff`) is the unique distinguishing marker.

### Event record (16 bytes, starting at 0x1C10)

```
Byte   Size  Field       Constraints
----   ----  -----       -----------
0-3    4     tick        uint32 LE — absolute tick
4      1     track       0x01–0x40 (1-indexed; subtract 1 for 0-indexed)
5      1     status      0x90 NoteOn, 0xC0 program change, 0x82 unknown
6      1     note        0x00–0x7F (MIDI note; data byte 1 for other types)
7      1     velocity    0x01–0x7F for notes (data byte 2 for other types)
8-9    2     duration    uint16 LE — ticks held
10     1     var_type    Note Variation type (0 = Tune, 1 = Decay seen)
11     1     var_value   Note Variation value, signed
12     1     [?]         0x00 usually; see ff ff ff ff note below
13     1     pad_idx     0x00 in all factory events; not used for playback
14-15  2     [?]         0x00 0x00 usually; see ff ff ff ff note below
```

Program change: byte 6 = program number − 1, other bytes zero, e.g.
`00 00 00 00 07 c0 67 00 00 00 00 00 00 00 00 00` (track 7, program 104).

Pattern for a single NoteOn event:
```
(?s)(?P<tick>.{4})(?P<track>[\x01-\x40])\x90(?P<note>[\x00-\x7f])(?P<velocity>[\x01-\x7f])(?P<dur>.{2})(?P<var_type>.)(?P<var_value>.)(?P<tail>\x00[\x00-\x7f]\x00\x00|\xff{4})
```

Bytes 12–15 are `ff ff ff ff` in about 1% of the events in Akai's factory
`.ALL` file, so the tail accepts that as well as the usual zeros.
See [`seq-format.md`](seq-format.md#events-0x1c10-onward-16-bytes-each).

```
```

Extract all events (Python):
```python
EVENT_RE = re.compile(
    rb'(?s)'
    rb'(?P<tick>.{4})'
    rb'(?P<track>[\x01-\x40])'
    rb'\x90'
    rb'(?P<note>[\x00-\x7f])'
    rb'(?P<velocity>[\x01-\x7f])'
    rb'(?P<dur>.{2})'
    rb'(?P<var_type>.)'
    rb'(?P<var_value>.)'
    rb'(?:\x00(?P<pad>[\x00-\x7f])\x00\x00|\xff{4})'
)

for m in EVENT_RE.finditer(data, 0x1C10):  # start past headers
    tick     = struct.unpack('<I', m.group('tick'))[0]
    track    = m.group('track')[0] - 1
    note     = m.group('note')[0]
    velocity = m.group('velocity')[0]
    dur      = struct.unpack('<H', m.group('dur'))[0]
    var      = struct.unpack('<b', m.group('var_value'))[0]
    print(f't={tick:6d}  track={track:2d}  note={note:3d}  vel={velocity:3d}  dur={dur}  var={m.group("var_type")[0]}:{var}')
```

> **Important:** start `finditer` at offset 0x1C10, not 0. The clock map and
> track header regions contain byte sequences that can spuriously match this
> pattern.

### Track header (48 bytes at `0x1000 + N × 48`)

```
+0x00   16  name            track name, null-padded (e.g. "Track01")
+0x10   16  pgm_name        PGM filename; byte[0] \x00, name at bytes 1-15 ("OFF" = none)
+0x20   2   [?]             zero in observed files
+0x22   1   midi_channel    1-16 = active, 0 = unused/empty track (factory .ALL: 1-13, likely MIDI out channel)
+0x23   1   active_flag     0x01 in active tracks, 0x00 in empty tracks
+0x24   1   [?]             0x64 = 100 in .SEQ files (volume?); 0 in .ALL
+0x25   1   [?]             zero in observed files
+0x26   1   pgm_change      program change number (1-based) in factory .ALL tracks; 0 = none
+0x27   1   [?]             zero in observed files
+0x28   1   [?]             0x1e = 30 in active tracks
+0x29   1   [?]             zero in observed files
+0x2A   1   [?]             0x6e = 110 in active tracks
+0x2B   3   [?]             zero in observed files
+0x2E   1   [?]             zero in observed files
+0x2F   1   [?]             0x32 = 50 in active tracks (pan=center?)
```

Pattern to extract track name and MIDI channel:
```python
TRACK_RE = re.compile(
    rb'(?s)'
    rb'(?P<name>.{16})'        # +0x00
    rb'(?P<pgm_name>.{16})'    # +0x10
    rb'.{2}'                    # +0x20 [?]
    rb'(?P<midi_channel>.)'    # +0x22
    rb'(?P<active_flag>.)'     # +0x23
    rb'.{24}'                   # +0x24 [?]
)

TRACK_SECTION = 0x1000
TRACK_SIZE = 48

for i in range(64):
    off = TRACK_SECTION + i * TRACK_SIZE
    m = TRACK_RE.match(data, off)
    if m and m.group('midi_channel')[0] > 0:
        name = m.group('name').rstrip(b'\x00').decode()
        pgm  = m.group('pgm_name').strip(b'\x00').decode()
        ch   = m.group('midi_channel')[0]
        print(f'Track {i:2d}: {name!r:12s} pgm={pgm!r} ch={ch}')
```

---

## .SNG (Song Files)

### Identification

```
^MPC1000 SNG\x00
```

Unlike SEQ and PGM, SNG files start at byte 0 with the magic string — **no
leading file-size uint32**.

Confirmed from a hardware-captured empty song file (256 bytes):

```
Offset  Size  Field       Notes
------  ----  -----       -----
0x00    11    magic       "MPC1000 SNG" ASCII
0x0B    1     null        \x00 terminator
0x0C    ?     [?]         all zero in empty file; step records expected here
```

Step record layout is not confirmed — the test file has no populated steps.
See [`song-format.md`](song-format.md) for the reverse-engineering approach.

---

## Unknown Regions Summary

Collecting all `[?]` fields in one place to guide future reverse-engineering.
The most productive method: create two files differing in exactly one parameter,
then `diff <(xxd a) <(xxd b)` to isolate which bytes changed.

### PGM

| Offset | Size | Location | Notes |
|--------|------|----------|-------|
| 0x0014 | 4 B  | file header | constant in all observed files |
| 0x29F3 | 17 B | file tail | after slider 1. Zero in nearly all files; stigi/mpc1k-node `Program01.PGM` (origin OS unknown) has 0x29F4 = 100, 0x29F6 = 100, 0x29F8 = 1, 0x29FA/0x29FB = −120/120, 0x29FD = 1, 0x29FF/0x2A00 = −120/120, which looks like two more slider-shaped records (a third slider, or MPC2500/JJOS settings?) |
| pad+0x60 | 2 B | per-pad | int16 LE, non-zero on ~100 factory pads; roughly layer 1 tuning ÷ 10 but often not, and set on some untuned pads |
| pad+0x64 | 2 B | per-pad | between mute group and envelope |
| pad+0x6C | 4 B | per-pad | before vel→tune (+0x69, +0x6A and +0x70 are now named from MPC500 firmware) |
| pad+0x74 | 4 B | per-pad | between filter1 resonance and vel→freq |
| pad+0x7C | 4 B | per-pad | between filter2 resonance and vel→freq |
| pad+0x81 | 14 B | per-pad | before mixer section |
| pad+0x95 | 15 B | per-pad | pad tail |
| layer+0x10 | 1 B | per-layer | byte between name and level |
| layer+0x17 | 1 B | per-layer | layer tail padding |
| slider+0x01 | 1 B | per-slider | between pad and parameter |

### SEQ

| Offset | Size | Location | Notes |
|--------|------|----------|-------|
| 0x0014 | 1 B  | header | zero in all observed files |
| 0x0016 | 1 B  | header | 0x01, except 0x00 in the factory "Standard" sequence |
| 0x0018 | 2 B  | header | constants 0x01, 0x00 |
| 0x001A | 2 B  | header | constant 1000 (0xe8 0x03 LE); probably the clock map entry count |
| 0x001E | 2 B  | header | zero in all observed files |
| 0x0022 | 14 B | header | zero in all observed files |
| 0x0FD0 | 48 B | before tracks | "track 0" slot (MPC500 firmware addresses track n at 0xFD0 + n × 0x30); zero in .SEQ, "(unused)"/"OFF" record in .ALL blocks |
| track+0x20 | 2 B  | per-track | zero in observed files |
| track+0x24 | 2 B  | per-track | 100 in MPC1000 .SEQ files (volume?), 0 in .ALL. **Conflict:** MPC500 firmware reading (unconfirmed) puts track type at +0x24 (0 drum, 1 MIDI) and MIDI channel at +0x25 |
| track+0x27 | 7 B  | per-track | 30, 110, 50 in .SEQ files (pan at +0x2F?); 0 in .ALL |
| track+0x2E | 1 B  | per-track | zero |
| event+12 | 4 B  | per-event | bytes 12–15: `00 00 00 00` (byte 13 = pad_idx) or `ff ff ff ff` |
| event status 0x82 | — | per-event | track 1 only, data 1–13; effect preset change? |

### SNG

| Offset | Size | Location | Notes |
|--------|------|----------|-------|
| 0x000C | all  | step records | layout unknown; only empty file observed. MPC500 in-memory song: +0x11 in use, +0x14 sequence per step (256 × 1 B, 0 = end), +0x114 repeats per step (−1 = HOLD); whether the file mirrors it is unknown |
