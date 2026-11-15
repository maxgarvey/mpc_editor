# cmd/genpgm

Generates `.PGM` probe programs, matching WAV samples and `.SEQ` files for answering open questions about the MPC 1000 program format on real hardware. It also compares two `.pgm` files byte by byte with field names, for analysing what the hardware saved.

## Purpose

Some format questions can only be answered by the hardware: which of the two MIDI note tables it trusts, what the "always 1" bytes do, where unmodelled settings are stored. `genpgm` writes programs that are deliberately crafted (some deliberately inconsistent) so that each answer can be heard.

## Usage

```bash
go run ./cmd/genpgm OUTDIR              # write the probe set into OUTDIR
go run ./cmd/genpgm -diff A.PGM B.PGM   # list differing bytes with field names
```

Write the set somewhere outside the repo (for example `/tmp/PGMTEST`) and copy that folder to the CF card. Generated files are not committed.

| File | What it probes |
|------|----------------|
| `P1_CTRL.PGM` | Control: consistent tables, A1 = 35 … D16 = 98, bank A pads play `TONE_A01`–`TONE_A16` |
| `P2_SWAP.PGM` | The tables disagree: pad→note is the control layout, but note→pad says 36 → A6 and 40 → A2 |
| `P3_DUPE.PGM` | A2 and A3 both on note 36, note→pad 36 → A3 (not the lowest pad), note 37 has no pad |
| `P4_PAD0.PGM` | Pad byte `+0x65` = 0 on all pads |
| `P5_SLD0.PGM` | Slider byte `+0x01` = 0 on both sliders |
| `P6_REMAP.PGM` | Consistent layout A1 = 60 … D16 = 123, for checking what a recorded `.SEQ` stores |
| `TONE_Ann.WAV` | 16-bit 44.1 kHz mono tones: the number of beeps gives the column, the pitch gives the row |
| `S1_CTRL.SEQ` / `S2_SWAP.SEQ` / `S3_DUPE.SEQ` | Note 36 on beat 1 and note 40 on beat 3, with track 1 pointing at P1 / P2 / P3 |

Every probe differs from `P1_CTRL` only in the bytes it targets. The tests check this, and also check that each probe has the intended (in)consistency and still parses with `pgm.OpenProgram`.

## Related

- [`internal/pgm`](../../internal/pgm/README.md): `OpenProgramRaw`, `SetNoteMapEntry`, `SetUnknownConstByte` and `DiffBytes` exist for this tool
- [`internal/seq`](../../internal/seq/README.md): `seq.Create` builds the `.SEQ` files
- [`internal/audio`](../../internal/audio/README.md): `Sample.SaveWAV` writes the tones
- [`cmd/genseq`](../genseq/README.md): the companion generator for `.SEQ` hardware tests
