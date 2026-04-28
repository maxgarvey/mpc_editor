# cmd/mpc_editor

Main entry point for the MPC Editor web application.

## What it does

1. Opens (or creates) the SQLite database via [`internal/db`](../../internal/db/README.md).
2. Constructs the HTTP server via [`internal/server`](../../internal/server/README.md), passing the embedded template and static filesystems from [`web`](../../web/README.md).
3. Binds a TCP listener on `127.0.0.1:8080` (or `$PORT`).
4. On macOS, opens the browser automatically (`open http://...`) unless `NO_BROWSER=1`.
5. Serves until killed.

## Configuration

| Env var | Default | Effect |
|---------|---------|--------|
| `PORT` | `8080` | Listening port |
| `NO_BROWSER` | _(unset)_ | Set to `1` to suppress auto-open (used by the Electron wrapper) |

## Running

```bash
make run          # build + start, opens browser on macOS
make build        # build only → ./mpc_editor
./mpc_editor      # run the compiled binary
PORT=9090 make run
```

## Related

- [`electron/`](../../electron/README.md) — wraps this binary in a native desktop app; sets `NO_BROWSER=1` and dynamically assigns a free port
- [`internal/server`](../../internal/server/README.md) — the HTTP server constructed here
- [`internal/db`](../../internal/db/README.md) — the database opened here at startup
