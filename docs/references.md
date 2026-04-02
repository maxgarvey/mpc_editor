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
