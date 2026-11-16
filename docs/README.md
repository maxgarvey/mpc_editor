# docs

Binary format specifications for MPC file types, derived from reverse-engineering real hardware files.

## Documents

| File | Description |
|------|-------------|
| [`seq-format.md`](seq-format.md) | Byte-level layout of the `.SEQ` sequence file format: header fields, track chunks, event encoding, timing constants, and verification examples from real MPC 1000 files |
| [`all-format.md`](all-format.md) | Layout of the `.ALL` (all sequences and songs) file: directory, effect records, and sequence blocks that use the `.SEQ` layout |
| [`song-format.md`](song-format.md) | Partially-derived layout of the `.SNG` song file format: confirmed header, song step structure, and guidance for further reverse-engineering |
| [`regex-patterns.md`](regex-patterns.md) | Regex-as-spec representation of all known fields in `.PGM`, `.SEQ`, and `.SNG`: identification patterns, field extraction, Python examples, and a consolidated table of every unknown byte region |
| [`references.md`](references.md) | External resources: open-source MPC projects, format documentation links, community forums, and official manuals |

## Status

None of the MPC file formats are officially documented by Akai. All content here comes from community reverse-engineering.

| Format | Documentation level |
|--------|-------------------|
| `.PGM` | Well documented — see [`references.md`](references.md) for the Stephen Norum spec |
| `.SEQ` | Mostly documented — [`seq-format.md`](seq-format.md) covers header, clock map (time signatures), tracks, and events including Note Variation and program changes; a few header and track bytes remain unknown |
| `.SNG` | Header confirmed from hardware file; step record layout unconfirmed (empty test file has no steps) — see [`song-format.md`](song-format.md) |
| `.ALL` | Partially documented from one factory file — [`all-format.md`](all-format.md); sequence blocks fully understood, effect and song areas not decoded |

Fields marked `[?]` in `seq-format.md` are observed in real files but not yet understood. Update the spec whenever new evidence clarifies them.

## Related

- [`internal/seq`](../internal/seq/README.md) — implements the parser and writer described in `seq-format.md`
- [`internal/pgm`](../internal/pgm/README.md) — implements the `.PGM` format; its byte offsets align with the Norum spec
- [`cmd/genseq`](../cmd/genseq/README.md) — generates test `.SEQ` files that exercise the format spec on real hardware
- [`cmd/genpgm`](../cmd/genpgm/README.md) — generates `.PGM` probe files for testing open format questions on real hardware
