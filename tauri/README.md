# tauri

Wraps the `mpc_editor` Go binary in a native desktop application using [Tauri 2](https://tauri.app).

## Purpose

Same goal as the Electron wrapper — dedicated window, dock icon, native menus, no browser or terminal — but using the platform's native WebView (WKWebView on macOS, WebView2 on Windows, WebKitGTK on Linux) instead of a bundled Chromium. Smaller binary, no GPU/EGL noise, lower memory footprint.

## How it works

1. Rust finds a free local port via `TcpListener::bind("127.0.0.1:0")`.
2. Spawns the `mpc_editor` binary with `PORT=<port>` and `NO_BROWSER=1`.
3. Polls `127.0.0.1:<port>` via TCP until the server accepts connections (up to 15 seconds).
4. Opens a `WebviewWindow` with `WebviewUrl::External(http://127.0.0.1:<port>)` — the UI is the Go server rendered in the native WebView.
5. On `RunEvent::Exit`, kills the Go child process cleanly.

Note: the 15-second wait runs synchronously in the Tauri `setup` hook, so the window appears only after the server is ready. In practice startup is under a second; the timeout is a safety net.

## Development

Requires [Rust](https://rustup.rs) and `tauri-cli`:

```bash
cargo install tauri-cli --version "^2"
```

Then, from the project root:

```bash
make build    # compile the Go binary first
make tauri    # launch Tauri dev app (installs tauri-cli if missing)
```

Or manually:

```bash
make build
cd tauri && cargo tauri dev
```

## Packaging

```bash
make tauri-dist          # macOS .dmg / .app (default)
cd tauri && cargo tauri build --target aarch64-apple-darwin   # Apple Silicon
cd tauri && cargo tauri build --target x86_64-apple-darwin    # Intel
cd tauri && cargo tauri build --target universal-apple-darwin # universal
```

On Windows/Linux, `cargo tauri build` produces an NSIS installer / AppImage respectively.

`electron-builder` copies the Go binary into the bundle via `tauri.conf.json → bundle.resources`; the packaged app is fully self-contained.

## Binary Resolution

`go_binary_path()` in `src-tauri/src/lib.rs`:
- **Development** (`tauri::is_dev()`): `CARGO_MANIFEST_DIR/../../mpc_editor` (project root)
- **Packaged**: `app.path().resource_dir()/mpc_editor` (copied by `bundle.resources`)

## Related

- [`cmd/mpc_editor`](../cmd/mpc_editor/README.md) — the Go binary this wrapper spawns; respects `PORT` and `NO_BROWSER` env vars
- [`internal/server`](../internal/server/README.md) — the HTTP server the WebView connects to
- [`electron/`](../electron/README.md) — the Electron wrapper (heavier, but more browser-compatible)
