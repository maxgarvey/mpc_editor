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
| Sequence index | Which sequence to play (0-98) |
| Repeat count | Number of times to repeat the sequence |
| Tempo override | Optional tempo change for this step |

### Binary Format

The byte-level binary layout of .SNG files is **not publicly documented**.
Existing community knowledge comes from the MPC operator's manual and runtime
behavior rather than file-level reverse-engineering.

To build a parser, hex dumps of real .SNG files would need to be analyzed
against known song configurations (known sequence count, step count, tempo
values) to map out the binary structure.

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
