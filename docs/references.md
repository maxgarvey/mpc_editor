# MPC File Format References

## Open-Source Projects

### mpc1k-seq
Python CLI utility for MPC 1000 .SEQ sequence files. Best available source
of binary format details for the sequence format. Originally written for
JJOS firmware; assumed compatible with MPC 2500.

- Repository: https://github.com/JOJ0/mpc1k-seq
- Blog post: https://blog.jojotodos.net/mpc1k-seq/

### MPC Maid
Java GUI editor for MPC 1000, MPC 500, and MPC 2500 .PGM program files.
The original inspiration for this project (mpc_editor is a Go rewrite).

- Repository: https://github.com/cyriux/mpcmaid
- SourceForge: https://sourceforge.net/projects/mpcmaid/

### pympc1000
Python module for loading, editing, and exporting MPC 1000 .PGM files.

- Repository: https://github.com/stephenn/pympc1000

### Open500 (MPC500 firmware add-ons)
Native-code add-ons for the MPC500 (OS 1.31), loaded as WAV files. Their source
headers document the OS's in-memory structures, read from the firmware and
partly tested on hardware. The MPC500 keeps programs and sequences in memory in
the same layout as the MPC1000 `.PGM` and `.SEQ` files, so these notes name
fields in our formats. Addresses and offsets apply to the MPC500 OS 1.31 only.
The authors label each one as tested on hardware or read from firmware only.

- Repository: https://github.com/M45K/open500 (SDK, `sdk/README.md` table of OS
  facts; `sdk/addons/wav_grid.py` and `wav_seqtools.py` for the sequence layout,
  tested on hardware)
- Repository: https://github.com/LA-2500/Open500-LA (`sdk/examples/program.py`
  pad fields, `seqedit.py` clock map, `track.py` track header, `song.py` song
  structure)

### Other public example files
Programs examined for format evidence (all 10,756-byte `MPC1000 PGM 1.00`):

- seaweedfactory/MPC1000 `KEYMAP3.PGM`: a JJOS-saved chromatic keymap and the
  only public program with two pads on one note (pad indices 62 and 63 both on
  note 98; note→pad points to 62). https://github.com/seaweedfactory/MPC1000
- stigi/mpc1k-node `data/Program01.PGM` (blank, drum layout, layer levels 100,
  non-zero tail bytes at 0x29F3) and `data/E_Kit.PGM`.
  https://github.com/stigi/mpc1k-node
- antonhornquist/pgm-rb `test/original.pgm`. https://github.com/antonhornquist/pgm-rb
- JOJ0/mpc1k-seq ships four JJOS `FunkBG_*.SEQ` files (all 4/4).

---

## Format Documentation

### PGM Format
- Stephen Norum's specification (most complete public doc):
  https://www.mybunnyhug.org/fileformats/pgm/

### General MPC Formats
- Chickensys Translator format reference:
  http://chickensys.com/translator/documentation/formatinfo/akaimpc.html

### MPC 2000XL File Spec
- Detailed exploration of the older MPC 2000XL format (related but different):
  https://www.minimumviableparagraph.com/blog/20250801-mpc2000xl-specs-reference/

---

## Community Resources

### Forums
- MPC Forums — file format discussion:
  https://www.mpc-forums.com/viewtopic.php?f=15&t=40898

### Guides
- MPC Samples file compatibility guide:
  https://www.mpc-samples.com/article/mpc-file-compatibility-1
- MPC-Tutor — MPC 500 file loading:
  https://www.mpc-tutor.com/akai-mpc-500-loading-files/

### Official Manuals
- Akai MPC 1000 Operator's Manual v2.0 (PDF) — available from Akai/inMusic
- JJOS 128XL Operations Manual (PDF) — covers extended OS features

---

## Key Caveat

None of the MPC file formats are officially documented by Akai. All binary
layout details in this project come from community reverse-engineering.

| Format | Documentation Level |
|--------|-------------------|
| .PGM | Well documented (Stephen Norum spec, multiple parsers) |
| .SEQ | Partially documented (mpc1k-seq header + events, many unknown fields) |
| .SNG | Undocumented at byte level (behavior known from manual) |
| .ALL | Undocumented at byte level (known to be a container) |

To fully document .SNG and .ALL formats, hex dumps of real files with known
contents would need to be analyzed.
