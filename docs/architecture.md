# Architecture

pkgtug consists of three independent binaries that communicate over HTTP.

## Components

```
┌─────────────────────────────────────────────────────────────────┐
│  Your forge (GitHub / GitLab / Gitea / …)                       │
│  POST /tug/fetch/<name>  on git push                            │
└───────────────────────────┬─────────────────────────────────────┘
                            │ webhook
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│  pkgtug-server                                                   │
│                                                                  │
│  ┌─────────────┐   ┌──────────────┐   ┌───────────────────┐    │
│  │  git clones  │   │  job queue   │   │  data_dir/        │    │
│  │  (local FS)  │   │  (in-memory) │   │  binaries + state │    │
│  └──────┬───────┘   └──────┬───────┘   └─────────┬─────────┘    │
│         │ fetch            │ next/result           │ manifest     │
└─────────┼──────────────────┼───────────────────────┼────────────┘
          │                  │                        │
          │          ┌───────▼────────┐               │
          │          │ pkgtug-worker  │               │ GET manifest.json
          │          │ (build + push) │               │ GET binary
          │          └───────┬────────┘               │
          │  git clone       │ POST result             │
          └──────────────────┘              ┌──────────▼──────────┐
                                            │  pkgtug (client)     │
                                            │  production hosts    │
                                            └─────────────────────┘
```

## Data flow: git-tracked package

1. A `git push` fires a webhook to `POST /tug/fetch/<name>`.
2. The server runs `git fetch` on the local clone.
3. If a new tag or commit is detected, a build job is enqueued.
4. A worker calls `GET /tug/build/next?platform=<p>` (polling or on-demand).
5. The worker clones the repo at the target revision, optionally runs `pre_build_command`, then runs `build_command`.
6. The worker `POST`s the result binary to `POST /tug/build/<job_id>/result`.
7. The server writes the binary to `data_dir/`, updates `manifest.json` atomically, prunes old versions if `keep_versions` is set, and optionally sends a Telegram notification.
8. On production hosts, `pkgtug update <package>/<component>` polls the manifest, compares versions, downloads the new binary, verifies SHA256, and swaps it in.

## Data flow: direct push

For pre-built binaries (AppImages, release artifacts):

1. CI or a local script calls `POST /tug/repo/<name>/push` with the binary as a multipart file.
2. The server stores it and updates `manifest.json`.
3. Clients update identically to the git-tracked flow.

## State persistence

**Server** — persists job history and per-package version state to `data_dir/server-state.json`. Survives restarts. Written atomically (temp file + rename).

**Client** — persists installed-package metadata to `/etc/pkgtug/state.json`. Maps `package/component` keys to `InstallEntry` structs containing binary path, service name, health check, backup dir, post-install command, SHA256 of last-written file, and dependency order.

## Version tracking

| `version_source.type` | Version string | Trigger |
|-----------------------|---------------|---------|
| `tag` | Tag name (e.g. `26.07.01-stable`) | New tag matching `pattern` |
| `branch` | 8-char commit SHA | Any new commit on `name` |

The client compares version strings with simple equality — no semver parsing. Both tag-based and SHA-based versions work transparently.

## Job queue

Jobs are in-memory only. A worker claims a job by calling `GET /tug/build/next`; the server marks it `in-progress`. If the server restarts while a job is in-progress, it is lost (the next webhook or poll will re-enqueue it).

## Embedded worker

`pkgtug-server` can optionally run a built-in worker in the same process via the `worker:` config block. This is useful for single-machine setups where a separate worker process is inconvenient. The embedded worker connects to `http://localhost<listen>` and uses the same `worker_secret`.

## Platform strings

The worker auto-detects platform from `uname`. Supported values:

| String | OS / arch |
|--------|-----------|
| `linux-x64` | Linux amd64 |
| `linux-arm64` | Linux arm64 |
| `linux-arm` | Linux armv7 |
| `darwin-x64` | macOS amd64 |
| `darwin-arm64` | macOS arm64 (Apple Silicon) |

Platform is stored in the manifest; clients filter binaries by their own platform when installing.

## Compression

Binaries can be compressed at rest with `compress: zstd` or `compress: xz` (per package). The server transparently decompresses on download — clients do not need to be aware of compression.
