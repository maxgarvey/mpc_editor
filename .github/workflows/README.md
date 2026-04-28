# .github/workflows

GitHub Actions CI configuration.

## Workflows

### `ci.yml` — Continuous Integration

Runs on every push and pull request to `main`.

| Step | Command | Purpose |
|------|---------|---------|
| Build | `go build ./...` | Verifies the project compiles cleanly |
| Test | `go test -race ./...` | Runs all unit tests with the race detector |
| Vet | `go vet ./...` | Catches common Go mistakes |
| Lint | `golangci-lint` v2.11.4 | Enforces code style and catches additional issues |

## What is not in CI

- **End-to-end tests** (`make test-e2e`) — Playwright tests require a running server and a display; run them locally before merging. See [`e2e/`](../../e2e/README.md).
- **Electron packaging** — built locally via `npm run dist`. See [`electron/`](../../electron/README.md).

## Running the same checks locally

```bash
make check     # vet + lint + tests (matches CI exactly)
make test-race # tests with race detector only
```

## Related

- [`e2e/`](../../e2e/README.md) — end-to-end tests run locally, not in CI
- [`CLAUDE.md`](../../CLAUDE.md) — full list of `make` targets
