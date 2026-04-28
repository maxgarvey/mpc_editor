# web

Frontend assets and templates for the MPC Editor browser UI.

## Purpose

Implements the entire user interface as server-rendered HTML with HTMX dynamic updates. No JS framework — Go templates generate the initial HTML, HTMX swaps in partials on user interaction, and vanilla JS handles audio playback, canvas rendering, and the sequence editor.

## Directory Structure

```
web/
  embed.go              Go embed directives (exposes TemplateFS, StaticFS to server)
  templates/
    layout.html         Application shell: header, workspace panel, detail panel
    sequence_page.html  Full sequence editor page wrapper
    slicer_page.html    Audio slicer page wrapper
    partials/           HTMX swap targets — each file is one partial:
      detail_*.html     Pad detail, WAV detail, SEQ detail, empty state
      file_browser_nav  Workspace file tree
      sequence_grid     Sequence step grid (grid + piano roll views)
      device_status     USB device detection badge
      ...
  static/
    css/style.css       Single stylesheet for all UI components
    js/
      htmx.min.js       HTMX 2.0.4 (vendored)
      app.js            Tab manager, workspace panel collapse, modal dialogs
      audio.js          Web Audio API engine: decode WAV, schedule pad playback
      sequencer.js      Sequence player + step editor (SequencePlayer, SequenceEditor)
      tabs.js           Tab bar management across file opens
      wav_detail_player.js   WAV file audition controls
      wav_waveform.js        Canvas waveform rendering from server peak data
      waveform.js            Lower-level canvas drawing primitives
```

## HTMX Integration

The app uses HTMX 2.0 (`hx-get`, `hx-post`, `hx-target`, `hx-trigger`). Key patterns:

- Most interactions are `hx-post` or `hx-get` with `hx-target` pointing at a container `div`.
- `hx-include` passes sibling form fields with a request (e.g. the PGM/time-sig selectors are included with every step-grid action).
- HTMX 2.0 fires `htmx:afterSwap` on the **swapped-in elements** (not the target container). JS hooks use `evt.target.closest('#sequence-grid')` to detect relevant swaps.

## JavaScript Modules

### `sequencer.js`
Two IIFEs: `SequencePlayer` and `SequenceEditor`.

**`SequencePlayer`**
- Fetches event JSON from `/sequence/events`
- Uses Web Audio API to schedule pad samples ahead of time (look-ahead scheduler pattern)
- Animates the playhead via `requestAnimationFrame`
- Manages loop/mute/solo state, bank expand/collapse, view mode (grid vs. piano roll)
- `renderContinuousView(data)`: builds the piano roll DOM from scratch — beats ruler + seconds ruler + per-pad event blocks

**`SequenceEditor`**
- `mode`: `'view'` (preview on click) | `'insert'` (toggle cells) | `'edit'` (drag to move)
- Multi-select: `selectedCells` Set keyed by `"pad:globalStep"`. Ctrl+click toggles selection; Delete key bulk-deletes; right-click opens bulk-edit popover; drag in edit mode moves the entire selection by the step/pad delta.
- Posts to `/sequence/event/edit` with actions: `toggle`, `move`, `delete`, `update`, `multi_delete`, `multi_move`, `multi_update`.
- `postEdit` replaces `#sequence-grid` innerHTML, then restores mode buttons, bank state, view layout, loop sync, and event data.

### `app.js`
- `TabManager`: tracks open files as browser tabs; reopens the last viewed file on load.
- `WorkspacePanel`: collapse/expand the left file browser; persisted in `localStorage`.
- Modal helpers: new-program dialog, save confirmation, settings modal.

### `audio.js`
- `AudioPlayer`: decodes WAV bytes (fetched from `/audio/stream`) into `AudioBuffer`; caches decoded buffers by pad index; plays samples at exact Web Audio API times for accurate sequencer timing.

## CSS

`style.css` is a single file covering the full UI:
- `.browser-layout`: CSS Grid for the two-panel layout (file browser + detail panel); `grid-template-columns` animated for the collapsible panel.
- `.step-grid`: the sequence step table. Cell states: `.step-active`, `.step-playing`, `.step-selected`, `.step-dragging`, `.step-drop-target`.
- `.seq-cont-*`: piano roll (continuous view) layout — rulers, track rows, event blocks, gridlines.
- Color convention: blue = low velocity, green = medium, red = high.

## Template Functions (registered in `server.go`)

| Function | Signature | Use |
|----------|-----------|-----|
| `add` | `(a, b int) int` | Arithmetic in templates |
| `mul` | `(a, b int) int` | Arithmetic in templates |
| `mod` | `(a, b int) int` | Step/beat modulo for CSS classes |
| `seq` | `(n int) []int` | Generate `[0..n-1]` for `range` |
| `padBankLabel` | `(bank int) string` | `"A"`, `"B"`, etc. |
| `velocityColor` | `(vel byte) string` | `#4488cc` / `#44aa44` / `#cc4444` |
| `velocityOpacity` | `(vel byte) float64` | 0.5–1.0 mapped from 0–127 |
