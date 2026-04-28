# internal/audio

WAV file I/O, energy-based beat detection, slice marker management, waveform downsampling, and audio crop/export utilities.

## Purpose

Provides the audio pipeline for the sample slicer feature: read a WAV, detect transients, present a list of editable markers, and export individual slices back to disk. Also provides server-side waveform downsampling for canvas rendering in the browser.

## Key Types

| Type | Role |
|------|------|
| `Sample` | Decoded PCM audio: `Format` (rate/channels/bits), `FrameLength`, raw `Data []byte` |
| `Format` | `SampleRate`, `Channels`, `BitsPerSample` |
| `Slicer` | Owns a `Sample` + `Markers`; runs beat detection and exposes slice export |
| `Markers` | Ordered list of frame-position markers with select/nudge/insert/delete |
| `Marker` | One slice boundary: frame position + display time |

## Workflow

```
1. Load:    OpenWAV(path) → Sample
2. Slice:   NewSlicer(sample) → auto-detects transients → Slicer.Markers
3. Edit:    markers.Select(i), markers.Nudge(delta), markers.Insert(pos), markers.Delete(i)
4. Export:  slicer.ExportSlices(dir, prefix) → writes "prefix0.wav", "prefix1.wav", ...
            slicer.GetSlice(i) → Sample (in-memory region, not written)
```

## Beat Detection Algorithm (`beatdetect.go`)

Energy-based onset detection:
1. Compute RMS energy for each overlapping window across the stereo signal.
2. For each window, compare its energy against the local average of the surrounding `localEnergyWindowSize` windows.
3. If `energy > sensitivity/100 × localEnergy`, it is an onset.
4. Snap the onset position to the nearest zero crossing within `windowSize` samples (reduces clicks on export).

| Parameter | Default | Effect |
|-----------|---------|--------|
| `windowSize` | 1024 | Analysis frame size |
| `overlapRatio` | 1 | Window step = windowSize / overlapRatio |
| `localEnergyWindowSize` | 43 | Context window for local average |
| `sensitivity` | 130 | Higher = more sensitive (more markers) |

Sensitivity can be adjusted post-detection via `slicer.SetSensitivity(n)` without reloading the sample.

## WAV I/O

- `OpenWAV` / `ReadWAV`: reads RIFF/WAVE, PCM format 1 only (16-bit LE assumed). Skips unknown chunks.
- `ReadWAVHeader`: reads format + frame count without loading PCM data into memory (used by the workspace scanner).
- `Sample.SaveWAV` / `WriteWAV`: writes a standard RIFF/WAVE file.
- `Sample.SubRegion(from, to)`: slices the PCM data by frame range (used for slice export).

## Waveform Downsampling (`waveform.go`)

`DownsamplePeaks(sample, numBins)` reduces a full PCM waveform to `numBins` (min, max) pairs for efficient canvas rendering. The server sends these peak pairs as JSON; the browser draws them directly without needing the full audio file.

## Transcode Utilities (`transcode.go`)

`CropWAV(src, dst, startFrame, endFrame)` writes a region of a WAV file to a new file, used when exporting a specific region to disk.
