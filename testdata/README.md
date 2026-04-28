# testdata

Static test fixtures used by Go unit tests and end-to-end tests.

## Contents

| File / Directory | Type | Purpose |
|-----------------|------|---------|
| `test.pgm` | MPC1000 program | Reference `.pgm` for parser round-trip tests |
| `chromatic.pgm` | MPC1000 program | Program with chromatic pad-to-note mapping; used in multisample assignment tests |
| `test.seq` | MPC1000 sequence | Reference `.seq` for parser tests |
| `test.sng` | MPC1000 song | Reference `.sng` for song-format tests |
| `test.wav` / `chh.wav` / `myLoop.wav` | PCM audio | WAV fixtures for slicer, waveform, and audition tests |
| `test_audio.mp3` | MP3 | Non-WAV fixture used to verify rejection of unsupported audio formats |
| `seq/` | Directory | Generated `.SEQ` files from [`cmd/genseq`](../cmd/genseq/README.md) for hardware verification |

## The `seq/` Subdirectory

Files in `testdata/seq/` are produced by running `go run ./cmd/genseq`. They are not used by automated tests — they exist to be loaded onto a physical MPC 1000 to verify that the binary writer produces valid output. See [`cmd/genseq`](../cmd/genseq/README.md) for the full fixture list and what each file verifies.

## Usage in Tests

Go unit tests reference these files via relative paths from the package under test, e.g.:

```go
prog, err := pgm.OpenProgram("../../testdata/test.pgm")
```

End-to-end tests (in [`e2e/`](../e2e/README.md)) copy the top-level fixtures (not `seq/`) into a temporary workspace directory for each test run, keeping tests isolated and repeatable.

## Adding Fixtures

- Binary fixtures (`.pgm`, `.seq`, `.wav`) should be minimal — small enough to keep `git clone` fast, large enough to exercise the feature.
- Regenerate `seq/` fixtures after changing `seq.Create` by running `go run ./cmd/genseq` and committing the updated files.

## Related

- [`internal/pgm`](../internal/pgm/README.md) — parses `test.pgm`, `chromatic.pgm`
- [`internal/seq`](../internal/seq/README.md) — parses `test.seq`
- [`internal/audio`](../internal/audio/README.md) — reads `*.wav` fixtures
- [`cmd/genseq`](../cmd/genseq/README.md) — generates `seq/*.SEQ`
- [`e2e/`](../e2e/README.md) — copies top-level fixtures into per-test workspaces
