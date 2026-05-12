# MPC 1000/500 Song & Project File Formats

## .SNG Song Files

A song is an ordered playlist of sequences. The MPC plays each step in order,
allowing you to arrange sequences into a full composition.

### Capabilities

- Up to **20 songs** per device
- Up to **250 steps** per song
- Tempo range: **30.0 to 300.0 BPM**
- Sequences referenced by **index** (0-98), not by name

### Song Step Structure

Each step contains:

| Field | Description |
|-------|-------------|
| Sequence index | Which sequence to play (0–98) |
| Repeat count | Number of times to repeat the sequence |
| Tempo override | Optional tempo change for this step (0 = use sequence's own tempo) |

### Binary Format

> **Evidence level:** The header is confirmed by hex analysis of `testdata/test.sng`
> (an empty song captured from MPC 1000 hardware). The step record layout is
> **not yet confirmed** — the test file is all-zero after the header, so step
> encoding cannot be derived from it alone. Fields marked **[?]** are unknown.
> Update this file whenever new evidence (hex dumps of real populated songs)
> clarifies them.

#### File Header

`testdata/test.sng` is 256 bytes. The first 12 bytes are the ASCII magic string
`MPC1000 SNG` followed by a null byte (0x00). All remaining bytes are zero in
this empty-song file.

```
Offset  Size   Field       Notes
------  ----   -----       -----
0x00    11     magic       ASCII "MPC1000 SNG" (no leading null bytes)
0x0B     1     null        0x00 terminator for the magic string
0x0C    [?]   [?]          Unknown; all zero in the observed empty file
```

**Key difference from .SEQ:** SEQ files begin with 4 null bytes before the
magic string (`0x00 0x00 0x00 0x00 "MPC1000 SEQ"`). SNG files begin directly
with the magic string at offset 0x00.

#### Step Records (layout unconfirmed)

Each step likely encodes:

- A sequence index byte (0–98)
- A repeat count byte (number of times to loop the sequence before advancing)
- A tempo field (possibly 2 bytes matching BPM × 10, same encoding as SEQ
  header `bpm` field; 0 = no override)

The total file size for a populated song is unknown. The 256-byte empty stub
does not reveal whether step records are fixed-size (as in SEQ events) or
variable-length.

#### How to Reverse-Engineer This

To map out the binary layout, produce two or three small songs with known
configurations on real MPC 1000 hardware, then diff their hex dumps:

1. Song with 1 step: sequence 0, repeat once, no tempo override
2. Song with 2 steps: sequences 0 and 1, different repeat counts
3. Song with a tempo override on step 2

```bash
xxd MySong.SNG | head -20
```

Compare byte positions that differ between files against the known parameter
values to identify the encoding.

---

## .ALL Project Files

The .ALL format is a container that bundles the complete MPC memory state into
a single file. It captures everything needed to restore a full project.

### Contents

- Up to **99 sequences** (.SEQ data)
- All **songs** (.SNG data)
- All **program references** (.PGM paths)
- Complete device state at time of save

### Binary Format

The .ALL format is a binary container. No byte-level specification is
publicly available from open-source projects. It is known to contain
concatenated/embedded versions of the individual file formats, but the
container structure (headers, offsets, index table) has not been documented.

### Usage Notes

- Loading an .ALL file replaces the entire MPC memory state
- Useful for backing up and restoring complete projects
- The primary way to transfer full projects between devices
- All relative file paths (samples, programs) are preserved

---

## MPC 500 vs MPC 1000 Compatibility

The MPC 500 and MPC 1000 share the same core platform and use the **same file
formats**:

| Format | Compatible | Notes |
|--------|-----------|-------|
| .SEQ | Yes | MPC 500 may support fewer tracks |
| .SNG | Yes | Same format |
| .ALL | Yes | Feature loss possible (see below) |
| .PGM | Yes | Same format |
| .WAV | Yes | Same format (16-bit, mono/stereo) |

### Hardware-Driven Limitations

While the file formats are the same, hardware differences can cause feature
loss when loading files across models:

| Feature | MPC 1000 | MPC 500 |
|---------|----------|---------|
| Pads | 16 (4 banks) | 12 (4 banks) |
| Mixer | Yes | No |
| Filters | Yes | No |
| Tracks per sequence | 64 | Fewer |
| Display | LCD | LED |

Programs using filters or mixer settings created on MPC 1000 will load on
MPC 500 but those parameters will be ignored. Sequences using tracks beyond
the MPC 500's limit may lose data.

### Not Compatible

- **.50k** (Keygroup) and **.50s** (Drum) formats are exclusive to the MPC 5000
  and cannot be read by MPC 500 or MPC 1000
