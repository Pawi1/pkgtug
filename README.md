<img src="https://pawi1.github.io/pkgtug/pkgtug.svg?v=2" alt="pkgtug" width="96">

# pkgtug [![Test](https://github.com/pawi1/pkgtug/actions/workflows/test.yml/badge.svg)](https://github.com/pawi1/pkgtug/actions/workflows/test.yml) [![CodeQL](https://github.com/pawi1/pkgtug/actions/workflows/codeql.yml/badge.svg)](https://github.com/pawi1/pkgtug/actions/workflows/codeql.yml) [![Go](https://img.shields.io/github/go-mod/go-version/pawi1/pkgtug)](go.mod) [![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A self-hosted, generic package manager and auto-updater for binary releases.

pkgtug tracks any git repository, builds its binaries when a new version appears, and distributes them to production hosts — without depending on GitHub/GitLab APIs, package registries, or any specific CI platform.

```sh
curl -fsSL https://pawi1.github.io/pkgtug/install.sh | sh
```

## How it works

```
git push
    │
    ├─► POST /tug/fetch/<name>   (webhook — no auth)
    │       └─ git fetch on local clone
    │           └─ new tag / new SHA detected → job queued
    │
    └─► pkgtug-worker --once     (CI runner or daemon)
            └─ git clone → pre-build? → build → POST result to server
                    └─ manifest.json updated atomically
                            └─ pkgtug update              (on production hosts)
                                    └─ download → verify SHA256 → swap binary
```

## Components

| Binary | Role |
|--------|------|
| `pkgtug-server` | Webhook receiver, job queue, manifest API, binary storage |
| `pkgtug-worker` | Polls server for jobs, builds, uploads result |
| `pkgtug` | Client CLI — installs and updates binaries on production hosts |

The worker runs as a **daemon** on a build machine or as a **one-shot CI job** (GitHub Actions, GitLab CI, etc.).

## Quick start

**Server** — copy `config.example.yaml`, set `base_url`, `data_dir`, `worker_secret`, and declare your packages, then:

```sh
pkgtug-server --config /etc/pkgtug/server.yaml
```

**Worker** — daemon on a build machine:

```sh
pkgtug-worker --server https://tug.example.com --secret <worker_secret> --interval 30s
```

One-shot in CI (`--once --wait 90s` handles the webhook/runner race):

```sh
pkgtug-worker --server https://tug.example.com --secret <worker_secret> --once --wait 90s
```

Copy `contrib/github-actions-worker.yml` to `.github/workflows/pkgtug-build.yml` for a ready-made GitHub Actions setup.

**Client** — add a remote and install:

```sh
pkgtug remote add main https://tug.example.com
pkgtug install myapp/server
pkgtug install github:cli/cli          # or straight from GitHub Releases
pkgtug update --all
```

## Documentation

- [Architecture](docs/architecture.md)
- [Server reference](docs/wiki/server.md)
- [Worker reference](docs/wiki/worker.md)
- [Client reference](docs/wiki/client.md)
- [Deployment guide](docs/wiki/deployment.md)
- [Configuration reference](docs/wiki/configuration.md)

## Building

```sh
make all              # server + worker + client → dist/
make build-linux-amd64
make build-linux-arm64
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup. For security issues see [SECURITY.md](SECURITY.md).

## License

MIT
