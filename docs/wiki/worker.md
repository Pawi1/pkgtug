# pkgtug-worker

The worker polls the server for pending build jobs, builds them, and uploads the result.

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--server <url>` | (required) | pkgtug-server base URL |
| `--secret <secret>` | (required) | `worker_secret` from server config |
| `--platform <string>` | auto-detected | Platform string (e.g. `linux-x64`) |
| `--work-dir <path>` | `/var/cache/pkgtug-worker` | Directory for git clones and build artifacts |
| `--interval <duration>` | `30s` | Poll interval in daemon mode |
| `--once` | false | Build one job and exit (CI mode) |
| `--wait <duration>` | `0` | In `--once` mode: keep retrying for this long if no job is ready yet |

## Daemon mode

Runs indefinitely, polling every `--interval`:

```sh
pkgtug-worker \
  --server  https://tug.example.com \
  --secret  $WORKER_SECRET \
  --platform linux-x64 \
  --work-dir /var/cache/pkgtug-worker \
  --interval 30s
```

SIGINT / SIGTERM triggers a clean shutdown after the current job finishes.

## One-shot mode (CI)

```sh
pkgtug-worker \
  --server  https://tug.example.com \
  --secret  $WORKER_SECRET \
  --once \
  --wait 90s
```

Exit codes:

| Code | Meaning |
|------|---------|
| `0` | Built and uploaded successfully |
| `1` | Error |
| `2` | No pending jobs |

In GitHub Actions, treat exit code `2` as success (see below).

## Platform auto-detection

If `--platform` is omitted, the worker detects the platform from `uname`. Supported strings:

| String | OS / arch |
|--------|-----------|
| `linux-x64` | Linux amd64 |
| `linux-arm64` | Linux arm64 |
| `linux-arm` | Linux armv7 |
| `darwin-x64` | macOS amd64 |
| `darwin-arm64` | macOS arm64 |

## Build process

For each claimed job the worker:

1. Clones the repo at the target revision into `--work-dir/<package>/<sha>/`
2. Runs `pre_build_command` in the clone dir (if set)
3. Runs `build_command` in the clone dir
4. For each binary declared in the package config, uploads the file at `path` to the server
5. Removes the clone directory on success

Build environment inherits the worker process environment — set any required tools (Go, Node, Rust, …) in the system PATH.

## GitHub Actions

Copy `contrib/github-actions-worker.yml` to `.github/workflows/pkgtug-build.yml` in your **project repo** (not the pkgtug repo).

Required secrets (GitHub Settings → Secrets → Actions):

| Secret | Value |
|--------|-------|
| `PKGTUG_SERVER` | Base URL of your pkgtug-server |
| `PKGTUG_SECRET` | `worker_secret` from server config |

Required variable (GitHub Settings → Variables):

| Variable | Example |
|----------|---------|
| `PKGTUG_PLATFORM` | `linux-x64` |

Then configure a webhook in your repo (Settings → Webhooks):
- **Payload URL**: `https://tug.example.com/tug/fetch/<name>`
- **Content type**: `application/json`
- **Trigger**: `push` events

The `--wait 90s` flag handles the race between the webhook arriving at the server and the Actions runner starting up.

## GitLab CI

```yaml
pkgtug-build:
  stage: deploy
  image: golang:1.22
  script:
    - go install github.com/pawi1/pkgtug/cmd/pkgtug-worker@latest
    - |
      pkgtug-worker \
        --server  "$PKGTUG_SERVER" \
        --secret  "$PKGTUG_SECRET" \
        --platform linux-x64 \
        --once \
        --wait 90s
      code=$?
      [ "$code" -eq 2 ] && exit 0 || exit "$code"
```

Add `PKGTUG_SERVER` and `PKGTUG_SECRET` as CI/CD variables. Configure the pkgtug webhook in your project's GitLab webhook settings.

## Running as a systemd service

```ini
# /etc/systemd/system/pkgtug-worker.service
[Unit]
Description=pkgtug build worker
After=network.target

[Service]
ExecStart=/usr/local/bin/pkgtug-worker \
  --server  https://tug.example.com \
  --secret  %d/worker-secret \
  --platform linux-x64 \
  --work-dir /var/cache/pkgtug-worker \
  --interval 30s
LoadCredential=worker-secret:/etc/pkgtug/worker-secret
Restart=on-failure
RestartSec=10s
User=pkgtug

[Install]
WantedBy=multi-user.target
```
