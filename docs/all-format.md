# MPC 1000 All-in-One File Format (.ALL)

A `.ALL` file is the MPC 1000's "save all sequences and songs" file. It holds a
directory, global settings and effect presets, and then every sequence in the
same layout as a standalone [`.SEQ`](seq-format.md) file.

> Derived from one file: Akai's official MPC 1000 factory
> `ALL_SEQS&SONGS.ALL` (352,628 bytes, akaipro.com legacy downloads), which holds
> 9 sequences and no songs. Layout facts below were checked against every block in
> that file; anything marked **[?]** is observed but not understood. The editor
> does not read `.ALL` files yet.

All multi-byte values are little-endian.

---

## File Layout

```
Offset     Size           Section
------     ----           -------
0x0000     24 B           Header
0x0018     99 × 24 B      Sequence directory
0x0960     64 B           [?] global settings
0x09A0     21 × 268 B     Effect records ("Effect00" … "Effect20")
0x1FA0     7200 B         Sequence block "(unused)" (empty)
0x3BC0     7200 B         Sequence block "Sequence" (empty template)
0x57E0     ~11 KB         Song area (starts with "Song"; no songs in this file) [?]
…          variable       Sequence blocks, in any order, located via the directory
```

The factory file also contains about 100 KB that nothing points to (from
`0x1B5F4`, right after Techno's block): more event records resembling Techno's,
a second end terminator, then zeros. It is probably leftover memory and can be
ignored.

## Header (0x0000–0x0017)

| Offset | Size | Field | Notes |
|--------|------|-------|-------|
| 0x00 | 4 | `file_size` | Equals the file length |
| 0x04 | 16 | `version` | `MPC1000 ALL 1.00`, null-padded |
| 0x14 | 4 | [?] | Zero |

## Sequence Directory (0x0018, 99 × 24 bytes)

One entry per sequence slot, in slot order:

| Entry offset | Size | Field | Notes |
|--------------|------|-------|-------|
| 0 | 16 | `name` | Sequence name, or `(unused)` |
| 16 | 4 | [?] | Zero |
| 20 | 4 | `offset` | Absolute file offset of the sequence's block; 0 for unused slots |

Blocks are not stored in directory order (in the factory file Techno, slot 4,
comes first).

## Sequence Blocks

Each block is byte-for-byte the [`.SEQ`](seq-format.md) layout, with two
differences:

- Bytes 4–19 hold the **sequence name** instead of the `MPC1000 SEQ x.xx`
  version string.
- The 48-byte record at `0x0FD0` is filled in (a track-shaped record named
  `(unused)`, program `OFF`); `.SEQ` files leave it zero.

The block's size field (bytes 0–3) equals the block length, and header byte
`0x15` holds the slot number. A block can therefore be cut out and saved as a
`.SEQ` file by writing a version string over bytes 4–19 (and, to match
hardware `.SEQ` files exactly, zeroing `0x0FD0–0x0FFF`). That a block saved
this way loads on the MPC is untested.

Track headers in these blocks carry fields that OS 4.40 `.SEQ` files leave at
defaults, notably the program change number at byte 38. See
[`seq-format.md`](seq-format.md#track-headers-0x10000x1bff-64--48-bytes).

## Effect Records (0x09A0, 21 × 268 bytes)

Each record starts with 4 bytes [?] and then a 16-byte name (`Effect00` …
`Effect20`), followed by parameter values, mostly 16-bit (e.g. `64 00` = 100,
`f4 01` = 500, `40 1f` = 8000). All 21 records in the factory file share the
same parameters. The layout is not decoded. Sequence events with status `0x82`
take values 1–13 and might select one of these presets; that is a guess.

## Song Area (0x57E0)

Starts with the ASCII name `Song` followed by zeros and structured data. The
factory file has no songs, so the layout is unknown; see
[`song-format.md`](song-format.md) for the standalone `.SNG` format.

## Related

- [`seq-format.md`](seq-format.md): layout of each sequence block
- [`regex-patterns.md`](regex-patterns.md): unknown-region table
