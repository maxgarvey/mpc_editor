# electron

Wraps the `mpc_editor` Go binary in a native desktop application using Electron.

## Purpose

The Go binary is a self-contained local web server. The Electron wrapper turns it into a proper desktop app — no browser required, no terminal, dock icon, native menus, and a dedicated window. The Go server runs as a child process; Electron renders its UI in a `BrowserWindow`.

## How it works

1. `main.js` picks a random free port via `net.createServer`.
2. Spawns the `mpc_editor` binary with `PORT=<port>` and `NO_BROWSER=1`.
3. Polls `http://127.0.0.1:<port>` until the server responds (up to 15 seconds).
4. Opens a `BrowserWindow` pointed at the server URL.
5. On `before-quit`, kills the Go process cleanly.

External links (anything not `127.0.0.1`) are routed to the system browser via `shell.openExternal` rather than opening inside the Electron window.

## Development

```bash
cd electron
npm install
npm start          # runs electron . (dev mode, uses ../mpc_editor binary)
```

Requires a built `mpc_editor` binary in the project root (`make build` first).

## Packaging

```bash
npm run dist:mac   # → ../dist-electron/*.dmg  (arm64 + x64 universal)
npm run dist:win   # → ../dist-electron/*.exe  (NSIS installer)
npm run dist:linux # → ../dist-electron/*.AppImage
npm run dist       # all platforms
```

`electron-builder` copies the `mpc_editor` binary into the app bundle via the `extraResources` field in `package.json`. The packaged app is fully self-contained.

## Binary Resolution

`goBinaryPath()` in `main.js` resolves the binary location:
- **Packaged**: `process.resourcesPath/mpc_editor` (copied by electron-builder)
- **Development**: `../mpc_editor` (relative to the `electron/` directory)

## Native Menu

A minimal menu is built at window creation time. Standard roles (`quit`, `undo`, `copy`, `reload`, `toggleDevTools`, `togglefullscreen`) are wired to their platform-native accelerators.

## Related

- [`cmd/mpc_editor`](../cmd/mpc_editor/README.md) — the Go binary this wrapper spawns; respects `PORT` and `NO_BROWSER` env vars
- [`internal/server`](../internal/server/README.md) — the HTTP server the `BrowserWindow` connects to
