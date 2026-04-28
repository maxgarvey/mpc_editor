# internal/midi

Generates and reads Standard MIDI Files (SMF Type 0) from sequence or marker data.

## Purpose

Converts audio slicer markers or sequence events into a `.mid` file for use in a DAW or for import back into an MPC. The primary use case is: slice a WAV → get markers → export as MIDI with each slice on a successive chromatic note.

## Key Types

| Type | Role |
|------|------|
| `Sequence` | A list of `Event` structs with a PPQ (pulses per quarter note) value |
| `Event` | Tick position + MIDI status byte + key + velocity |

## Workflow

```
From markers:
  BuildFromMarkers(locations, tempo, sampleRate, ppq) → Sequence
  locations: []int  — frame positions of each slice marker
  Returns: NoteOn/NoteOff pairs at the equivalent tick positions

Write:
  sequence.WriteSMF(path)  — writes SMF Type 0 with one track
  format.WriteSMF(w, seq)  — lower-level writer to an io.Writer

Read:
  format.ReadSMF(path) → Sequence
```

## Timing

Default PPQ is 96 (matches the MPC's native resolution). Notes are assigned starting from MIDI note 35 (`DefaultStartKey`) and increment chromatically — A1 pad = note 35, A2 = 36, etc. Default note length is 32 ticks (1/8th note at 96 PPQ).

## Related Modules

| Module | Relationship |
|--------|-------------|
| [`internal/audio`](../audio/README.md) | `Slicer.Markers` provides the frame positions that `BuildFromMarkers` converts to MIDI ticks |
| [`internal/seq`](../seq/README.md) | Shares the 96 PPQN resolution and the factory note mapping (bank A = notes 36–51) |
| [`internal/server`](../server/README.md) | `handlers_slicer.go` calls `midi.WriteSMF` during slice export |

## References

- MIDI 1.0 Standard MIDI File specification
- MPC factory MIDI note mapping: bank A = notes 36–51 (see [`docs/seq-format.md`](../../docs/seq-format.md))
