# internal/command

High-level operations on top of `pgm`: importing samples and assigning them to pads.

## Purpose

Acts as an application-layer command module — each function corresponds to one user action that touches multiple domain objects. Handlers in `internal/server` call into this package rather than manipulating `pgm` and `audio` directly when the operation involves more than one step.

## Functions

### `assign.go` — Sample-to-pad assignment

**`SimpleAssign(prog, matrix, samples, startPad, mode)`**
Assigns a list of `SampleRef`s to consecutive pad slots starting at `startPad`.
- `AssignPerPad`: one sample per pad (layer 0 only).
- `AssignPerLayer`: fills all 4 layers on each pad before advancing to the next pad.
- Skips slots already occupied in the matrix.
- Sets sensible defaults (level=100, pan=50) for pads that were zeroed out.

**`MultisampleAssign(prog, matrix, samples)`**
Builds a chromatic multisample program via `pgm.MultisampleBuilder`. Computes per-pad MIDI notes and tuning values so the samples spread across the keyboard chromatically.

### `import.go` — Sample validation and import

**`ImportSamples(paths)`** → `([]*SampleRef, ImportResult)`
Validates a list of file paths. Rejects non-`.wav` files. Truncates names longer than 16 characters (MPC limit) and flags them as `SampleRenamed`. Returns valid refs and a summary count.

`ImportResult` reports how many were imported, renamed, or rejected. `Report()` produces a human-readable string for the UI.

## Relationship to Handlers

These functions have no HTTP dependencies. Server handlers construct the inputs (parse form values, resolve paths, load programs) and then call these functions, keeping the application logic testable in isolation.

## Related Modules

| Module | Relationship |
|--------|-------------|
| [`internal/pgm`](../pgm/README.md) | All functions operate on `*pgm.Program` and `*pgm.SampleMatrix` |
| [`internal/server`](../server/README.md) | `handlers_assign.go` calls into this package (`ImportSamples`, `SimpleAssign`, `MultisampleAssign`) |
