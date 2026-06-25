# Contributing to pkgtug

## Prerequisites

- Go 1.22+
- `make`
- Git

## Building

```sh
git clone https://github.com/pawi1/pkgtug
cd pkgtug
make all           # builds all three binaries into dist/
```

Cross-compile:

```sh
make build-linux-amd64
make build-linux-arm64
```

Embed a version string:

```sh
make all VERSION=26.07.01-stable
```

## Running tests

```sh
go test ./...
go vet ./...
```

Tests live alongside the code they cover (e.g. `internal/server/server_test.go`). There are currently no tests for `internal/client` — new tests there are welcome.

## Project layout

```
cmd/
  pkgtug/               client CLI
  pkgtug-server/        server binary (HTTP API + job queue)
  pkgtug-worker/        worker binary (builds and uploads)
internal/
  client/               install, update, rollback, state, GitHub source
  compress/             zstd / xz streaming compression
  config/               YAML config types + validation
  gitops/               git clone / fetch helpers
  manifest/             manifest.json type shared by server and client
  notify/               Telegram notifications
  server/               HTTP handlers, job queue, build dispatch, state persistence
  tui/                  interactive terminal prompts and progress bars
  worker/               worker polling loop
contrib/
  github-actions-worker.yml   GitHub Actions template for running pkgtug-worker as CI
```

## Code style

- Format with `gofmt`/`goimports`.
- `go vet ./...` must pass.
- No external linter config is enforced.
- Keep binary sizes small — avoid large dependencies.
- Default to writing no inline comments; add one only when the *why* is non-obvious.

## Adding a client command

1. Add `cmd/pkgtug/cmd_<name>.go`.
2. Register it in `cmd/pkgtug/main.go`.
3. Add usage to the command's help text.

## Adding a server endpoint

1. Add the handler in `internal/server/`.
2. Register the route in `internal/server/server.go`.
3. Update the API table in `docs/wiki/server.md` and `README.md`.

## Pull requests

1. Fork and create a feature branch.
2. Keep commits focused — one logical change per commit.
3. `go vet ./...` and `go test ./...` must pass before opening the PR.
4. Reference any related issue in the PR description.

## Reporting issues

Open an issue at <https://github.com/pawi1/pkgtug/issues>.  
For security vulnerabilities, see [SECURITY.md](SECURITY.md).
