# pkgtug-server

## Starting the server

```sh
pkgtug-server --config /etc/pkgtug/server.yaml
```

If the config file does not exist, `pkgtug-server` generates an example config at that path and exits.

On startup the server:
1. Validates the config (required fields, no duplicate package names, etc.)
2. For each non-`direct_push` package: clones the repo if `local_clone` does not yet exist
3. Detects the current version for every package
4. Restores job/build state from `data_dir/server-state.json`
5. Starts background polling goroutines for packages with `poll_interval` set
6. Starts the HTTP listener

## Configuration

See [configuration.md](configuration.md) for the full reference.

Minimum required fields:

```yaml
server:
  base_url: "https://tug.example.com"
  data_dir: "/var/lib/pkgtug"
  worker_secret: "<random secret>"

packages:
  - name: myapp
    git_url: git@github.com:user/myapp.git
    local_clone: /data/repos/myapp
    version_source:
      type: tag
      pattern: "v*"
    build_command: "make build"
    binaries:
      - component: server
        path: dist/myapp
```

## API reference

All endpoints are HTTP. Use a reverse proxy for TLS.

### Public endpoints (no auth)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/healthz` | Health check; returns JSON with `status`, per-package versions, `goos`, `goarch` |
| `GET` | `/tug/packages` | List all tracked packages and their current version |
| `POST` | `/tug/fetch/<name>` | Trigger `git fetch` for the named package; safe to use as a webhook |
| `GET` | `/tug/repo/<name>/manifest.json` | Latest manifest for the package |
| `GET` | `/tug/repo/<name>/versions` | List of stored versions, newest first |
| `GET` | `/tug/repo/<name>/binaries/<version>/<platform>/<component>` | Download a binary |

Use `:latest` as `<version>` to always get the current version.

Binary downloads require `Authorization: Bearer <download_token>` (or `?token=<token>`) when `download_token` is set for that package.

### Worker endpoints (Bearer `worker_secret`)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/tug/build/next?platform=<p>` | Claim the next pending build job |
| `POST` | `/tug/build/<job_id>/result` | Submit a build result (multipart: `component`, `platform`, file) |
| `POST` | `/tug/repo/<name>/push` | Push a pre-built binary (multipart: `version`, `platform`, `component`, file) |

## Webhook setup

In your forge, create a webhook that `POST`s to:

```
https://tug.example.com/tug/fetch/<name>
```

No secret is required. The endpoint only runs `git fetch` on the local clone.

Repeated calls within the `webhook_cooldown` window (default 10 s) return `429 Too Many Requests`.

## manifest.json schema

```json
{
  "version": "26.07.01-stable",
  "source_url": "https://github.com/user/myapp",
  "auth_required": false,
  "binaries": {
    "server": {
      "linux-x64": {
        "url": "https://tug.example.com/tug/repo/myapp/binaries/26.07.01-stable/linux-x64/server",
        "sha256": "abc123…",
        "size": 12345678
      }
    }
  }
}
```

`auth_required: true` signals to clients that a `download_token` must be configured for this remote.

## Version pruning

Set `keep_versions: N` (global or per-package) to automatically delete old binaries after a new version is stored. `0` (default) means unlimited retention.

Global setting:
```yaml
server:
  keep_versions: 5
```

Per-package override:
```yaml
packages:
  - name: myapp
    keep_versions: 10
```

## Telegram notifications

```yaml
telegram:
  bot_token: "123456:AAAA…"
  chat_id: "-100…"
```

The server sends a message when a new build is stored. Leave both fields empty to disable.

## Embedded worker

To run a worker in the same process as the server (single-machine setup):

```yaml
worker:
  enabled: true
  work_dir: /var/cache/pkgtug-worker
  interval: 30s
```

The embedded worker connects to `http://localhost<listen>` using the configured `worker_secret`. Platform is auto-detected from `uname`.

## File layout under `data_dir`

```
data_dir/
  server-state.json           job and version state (survives restarts)
  repos/
    <name>/
      <version>/
        <platform>/
          <component>         binary file
```
