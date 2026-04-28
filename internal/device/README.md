# internal/device

Polls for MPC 1000 devices connected as USB mass storage and exposes their mount path.

## Purpose

When an MPC 1000 is connected over USB in "Storage" mode, it appears as a mounted volume. This package detects that volume by scanning `/Volumes` (macOS) for directories that look like an MPC: an `AUTOLOAD` directory and/or `.pgm` files in the root. Once detected, the UI shows a "Use as workspace" shortcut.

## Key Types

| Type | Role |
|------|------|
| `Detector` | Background poller; holds a `sync.RWMutex`-protected `*MPCDevice` |
| `MPCDevice` | Detected device: `VolumeName`, `MountPath`, `HasAutoload`, `PGMCount` |

## Workflow

```
1. device.New() → creates Detector (default base path "/Volumes", 3s interval)
2. go detector.Start(ctx) → polls on a ticker
3. Each tick: scans base path for mounted volumes matching MPC heuristics
4. detector.Current() → returns *MPCDevice (nil if none connected)
5. On cancel(ctx): goroutine exits cleanly
```

## Detection Heuristics

A volume is considered an MPC if it has an `AUTOLOAD` subdirectory or at least one `.pgm` file in its root. The first matching volume wins; only one device is tracked at a time.

## Configuration

`WithBasePath(path)` and `WithInterval(d)` options let tests override the defaults without touching the real filesystem.

## Platform Notes

Currently only meaningful on macOS (`/Volumes`). On Linux, the base path would need to be overridden (e.g. `/media/user` or `/mnt`). No kernel USB events are used — pure filesystem polling.
