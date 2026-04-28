# e2e

Playwright end-to-end tests that run a live server against real fixture files.

## Purpose

Unit tests verify individual packages in isolation. E2E tests verify the full stack — HTTP server, templates, HTMX interactions, and browser-side JavaScript — against a real running instance of `mpc_editor`.

## Running

```bash
make test-e2e          # headless, from project root
cd e2e && npx playwright test              # headed (shows browser)
cd e2e && npx playwright test --ui         # Playwright UI mode
cd e2e && npx playwright test smoke.spec.ts  # single spec file
```

Requires Node.js and a built `mpc_editor` binary. The `make test-e2e` target handles both. Browser binaries are installed automatically by Playwright on first run.

## Test Structure

```
e2e/
  tests/
    helpers.ts          Shared setup/teardown utilities (workspace, server API calls)
    smoke.spec.ts       Basic page load and panel visibility
    layout.spec.ts      Two-panel layout, PGM opens pad grid
    pad-grid.spec.ts    Pad parameter editing
    program.spec.ts     Open / save / new program flows
    sequence.spec.ts    Sequence step grid: insert, delete, move events
    seq-view.spec.ts    Sequence view modes (grid vs piano roll)
    slicer.spec.ts      Audio slicer: load WAV, marker detection, export
    wav-view.spec.ts    WAV detail panel: waveform, audition controls
    file-browser.spec.ts  Workspace panel: listing, search, context menu
    tabs.spec.ts        Tab manager: open multiple files, switch, close
    tags.spec.ts        File tagging UI
    transcode.spec.ts   WAV crop / export
    new-modal.spec.ts   New sequence / new program modal
    sng-view.spec.ts    Song file detail view
    sample_report.spec.ts  Sample assignment report
```

## Helpers (`tests/helpers.ts`)

| Function | Purpose |
|----------|---------|
| `setupWorkspace()` | Creates a temp directory and copies `testdata/` fixtures into it |
| `cleanupWorkspace(dir)` | Removes the temp directory after each test |
| `setWorkspace(page, path)` | POSTs to `/workspace/set` to point the live server at the temp dir |
| `scanWorkspace(page)` | POSTs to `/workspace/scan` to index the fixture files |
| `openProgram(page, path)` | POSTs to `/program/open` to load a `.pgm` |
| `waitForHtmx(page)` | Waits for the next `htmx:afterSettle` event |
| `waitForHtmxOrTimeout(page, ms)` | Same but with a fallback timeout |
| `waitForTabOpen(page, text)` | Waits for a tab's content to appear in the detail panel |

Each test uses `beforeEach` / `afterEach` to create and destroy an isolated workspace, so tests never share state.

## Configuration

`playwright.config.ts` (in `e2e/`) points at `http://127.0.0.1:8080`. The server must already be running when tests execute — `make test-e2e` starts it as part of the target. Set `PORT` if using a non-default port.

## Related

- [`testdata/`](../testdata/README.md) — fixtures copied into each test workspace
- [`internal/server`](../internal/server/README.md) — the running server under test
- [`web/`](../web/README.md) — the templates and JS that tests interact with
- [`.github/workflows/`](.././.github/workflows/README.md) — E2E tests are not run in CI (unit tests and lint only)
