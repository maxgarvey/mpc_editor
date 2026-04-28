# cmd/genseq

Generates a curated set of `.SEQ` test fixture files for MPC 1000 hardware verification.

## Purpose

The MPC `.SEQ` binary format cannot be fully verified by unit tests alone — the ultimate ground truth is whether the hardware plays back the file correctly. `genseq` produces a set of named files covering common patterns, edge cases, and flag combinations that can be loaded onto a physical MPC 1000 to confirm parser and writer correctness.

## Usage

```bash
go run ./cmd/genseq
# or after building:
./genseq
```

Files are written to `testdata/seq/`. Each filename describes what it tests:

| Category | Files | What to check on hardware |
|----------|-------|--------------------------|
| `general_*` | Single note, quarter notes, all 16 steps, all bank A pads, two bars | Basic timing and note mapping |
| `boundary_*` | First/last step, all pads simultaneous, velocity range (1–127), 90 BPM, 4 bars, duration 1 tick, duration 383 ticks | Edge-case timing, velocity response, multi-pad polyphony |
| `verify_loop_on` / `verify_loop_off` | Identical 1-bar patterns with `loop=true` / `loop=false` | Loop flag byte `0x17` (hardware should loop or stop accordingly) |

## Workflow

```
1. Run: go run ./cmd/genseq
2. Copy testdata/seq/*.SEQ onto the MPC 1000 (via USB or CF card)
3. Load each file into a sequence slot and play it back
4. Compare playback against the expected behaviour listed in the filename
```

## Related

- [`internal/seq`](../../internal/seq/README.md) — `seq.Create` is the only function called here; all binary encoding lives there
- [`testdata/`](../../testdata/README.md) — destination directory for generated files
- [`docs/seq-format.md`](../../docs/seq-format.md) — byte-level spec the generated files exercise
