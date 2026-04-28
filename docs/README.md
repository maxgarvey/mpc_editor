# docs

Binary format specifications for MPC file types, derived from reverse-engineering real hardware files.

## Documents

| File | Description |
|------|-------------|
| [`seq-format.md`](seq-format.md) | Byte-level layout of the `.SEQ` sequence file format: header fields, track chunks, event encoding, timing constants, and verification examples from real MPC 1000 files |
| [`references.md`](references.md) | External resources: open-source MPC projects, format documentation links, community forums, and official manuals |

## Status

None of the MPC file formats are officially documented by Akai. All content here comes from community reverse-engineering.

| Format | Documentation level |
|--------|-------------------|
| `.PGM` | Well documented — see [`references.md`](references.md) for the Stephen Norum spec |
| `.SEQ` | Partially documented — [`seq-format.md`](seq-format.md) covers header, tracks, and events; several header fields remain unknown |
| `.SNG` | Undocumented at byte level |
| `.ALL` | Undocumented at byte level |

Fields marked `[?]` in `seq-format.md` are observed in real files but not yet understood. Update the spec whenever new evidence clarifies them.

## Related

- [`internal/seq`](../internal/seq/README.md) — implements the parser and writer described in `seq-format.md`
- [`internal/pgm`](../internal/pgm/README.md) — implements the `.PGM` format; its byte offsets align with the Norum spec
- [`cmd/genseq`](../cmd/genseq/README.md) — generates test `.SEQ` files that exercise the format spec on real hardware
