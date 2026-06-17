# pkgtug

A self-hosted, generic package manager and auto-updater for binary releases.

pkgtug tracks any git repository, builds its binaries when a new version appears, and distributes them to production hosts — without depending on GitHub/GitLab APIs, package registries, or any specific CI platform.

## How it works

```
git push
    │
    ├─► POST /tug/fetch/<name>   (webhook — no auth)
    │       └─ git fetch on local clone
    │           └─ new tag / new SHA detected → job queued
    │
    └─► pkgtug-worker --once     (CI runner or daemon)
            └─ git clone → build → POST result to server
                    └─ manifest.json updated atomically
                            └─ pkgtug update              (on production hosts)
                                    └─ download → verify SHA256 → swap binary
```

## Binaries

| Binary | Role |
|--------|------|
| `pkgtug-server` | Webhook receiver, job queue, manifest API, binary storage |
| `pkgtug-worker` | Polls server for jobs, builds, uploads result |
| `pkgtug` | Client CLI — installs and updates binaries on production hosts |

The worker can run as a **daemon** on a build machine, or as a **one-shot CI job** (GitHub Actions, GitLab CI, etc.).

## Server

### Configuration

Copy `config.example.yaml` and edit:

```yaml
server:
  listen: ":8080"
  base_url: "https://tug.example.com"
  data_dir: "/var/lib/pkgtug"
  worker_secret: "random-secret-shared-with-workers"

telegram:          # optional — leave empty to disable
  bot_token: ""
  chat_id: ""

packages:
  - name: myapp
    git_url: git@github.com:user/myapp.git
    local_clone: /data/repos/myapp
    version_source:
      type: tag          # track semver/date tags
      pattern: "*-stable"
      # type: branch     # track a branch (version = short SHA)
      # name: main
    build_command: "make build"
    binaries:
      - component: server
        path: dist/myapp
```

`version_source.type`:
- `tag` — version string is the tag name matching `pattern` (e.g. `26.07.02-stable`)
- `branch` — version string is an 8-char commit SHA; any new commit triggers a build

### Run

```sh
pkgtug-server --config /etc/pkgtug/server.yaml
```

### API

| Endpoint | Auth | Description |
|----------|------|-------------|
| `POST /tug/fetch/<name>` | none | Trigger git fetch; safe to use as a native GitHub/GitLab webhook |
| `GET /tug/repo/<name>/manifest.json` | none | Latest manifest for a package |
| `GET /tug/repo/<name>/binaries/<ver>/<platform>/<component>` | none | Download a binary |
| `GET /tug/packages` | none | List tracked packages and their current versions |
| `POST /tug/repo/<name>/push` | Bearer secret | Push a pre-built binary directly (AppImage, etc.) |
| `GET /tug/build/next?platform=<p>` | Bearer secret | Worker: claim next pending job |
| `POST /tug/build/<job_id>/result` | Bearer secret | Worker: submit build result |

Configure the webhook in your forge: `POST https://tug.example.com/tug/fetch/<name>`. No secret needed — it only runs `git fetch`.

### Direct push (pre-built binaries)

For AppImages, release binaries, or any artifact built outside pkgtug — push directly without a worker:

```sh
curl -X POST https://tug.example.com/tug/repo/myapp/push \
  -H "Authorization: Bearer $WORKER_SECRET" \
  -F "version=1.2.3" \
  -F "platform=linux-x64" \
  -F "component=app" \
  -F "file=@myapp-1.2.3-x86_64.AppImage"
```

Multiple platforms: repeat the call with a different `platform` value — each push adds an entry to the manifest without overwriting others. A Telegram notification is sent on each successful push (if configured).

The package must still be declared in `server.yaml` (so the server knows about it), but `build_command` and `binaries` are irrelevant for direct-push packages — they exist only to keep the config schema consistent.

## Worker

### Daemon mode (persistent build machine)

```sh
pkgtug-worker \
  --server  https://tug.example.com \
  --secret  <worker_secret> \
  --platform linux-x64 \
  --work-dir /var/cache/pkgtug-worker \
  --interval 30s
```

Platform is auto-detected from `uname` if omitted. Supported values: `linux-x64`, `linux-arm64`, `linux-arm`, `darwin-x64`, `darwin-arm64`.

### One-shot mode (GitHub Actions / GitLab CI)

```sh
pkgtug-worker --server <url> --secret <secret> --once --wait 90s
```

- `--once` — build one job and exit
- `--wait 90s` — retry polling for up to 90 s if no job is ready yet (handles the race between the webhook and CI startup)

Exit codes: `0` = built, `1` = error, `2` = no pending jobs.

#### GitHub Actions

Copy `contrib/github-actions-worker.yml` to `.github/workflows/pkgtug-build.yml` in your project repo and add two secrets:

| Secret | Value |
|--------|-------|
| `PKGTUG_SERVER` | Base URL of your pkgtug-server |
| `PKGTUG_SECRET` | `worker_secret` from server config |

Add a variable `PKGTUG_PLATFORM` (e.g. `linux-x64`) in GitHub Settings → Variables.

The workflow triggers on every push, so it runs in parallel with the webhook — the `--wait` window covers any timing gap.

## Client (`pkgtug`)

### Remotes

pkgtug supports multiple package servers (remotes), similar to Flatpak. Each remote is a named pkgtug-server instance.

```sh
# add a server
pkgtug remote add main    https://tug.example.com
pkgtug remote add nightly https://tug-nightly.example.com

# list configured servers
pkgtug remote list

# remove a server
pkgtug remote remove nightly
```

Remotes are stored in `/etc/pkgtug/config.yaml`. The file is managed by the CLI — no manual editing required.

### Configuration

`/etc/pkgtug/config.yaml` (managed by `pkgtug remote`):

```yaml
remotes:
  - name: main
    url: https://tug.example.com
  - name: community
    url: https://tug.community.example.com

telegram:          # optional — leave empty to disable
  bot_token: ""
  chat_id: ""
```

> **Migration:** the legacy `server_url` key is still accepted and is automatically treated as a remote named `"default"`.

### Usage

```sh
# search across all remotes
pkgtug search myapp

# search a specific remote
pkgtug search --remote community myapp

# install — auto-discovers which remote has the package
pkgtug install myapp/server

# install from a specific remote
pkgtug install main:myapp/server

# check for update
pkgtug check myapp/server

# update one package
pkgtug update myapp/server

# update everything installed
pkgtug update --all

# show installed packages (includes remote column)
pkgtug status

# restore previous binary from backup
pkgtug rollback myapp/server
```

`install` is interactive: it prompts for binary path, service name (for stop/start during updates), health check URL or command, and backup directory. All prompts except the path have defaults or can be skipped with Enter.

### Cron / systemd timer

```
*/15 * * * * pkgtug update --all
```

No daemon required. When stdout is not a terminal (cron, CI), all TUI output (spinner, progress bar) is automatically replaced with plain log lines.

### Update flow

1. Fetch `manifest.json` from the package's remote
2. Compare installed version (string equality — works for both tags and SHAs)
3. Download binary to temp file
4. Verify SHA256
5. Backup current binary to `backup_dir` (only the binary — config files are never touched)
6. Stop service (`systemctl` or `rc-service`, auto-detected at runtime)
7. Atomic replace (`rename`)
8. Start service
9. Health check (URL or shell command, retried up to 5×)
10. On failure → restore backup, restart service

## Building

```sh
# build all three binaries into dist/
make all

# cross-compile
make build-linux-amd64
make build-linux-arm64
```

Requires Go 1.22+. Produces fully static binaries.

## Security model

- The webhook endpoint has **no authentication** — it only runs `git fetch` on a local clone. Safe to expose publicly.
- Worker endpoints require `Authorization: Bearer <worker_secret>`. Workers write data that is later served to all clients, so they must be trusted.
- Client endpoints (manifest, binary download) have **no authentication** — treat them like a package mirror.

## License

MIT
