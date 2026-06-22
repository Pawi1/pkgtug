<img src="https://pawi1.github.io/pkgtug/pkgtug.svg?v=2" alt="pkgtug" width="96">

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
  cors_origins:
    - "*"              # required for the browser UI; restrict to your domain if preferred
  webhook_cooldown: "10s"    # min gap between webhook fetches per package (default 10s)
  # max_upload_size: "100MB" # optional — limit per-file upload (e.g. for Cloudflare Tunnel)
  # keep_versions: 5         # optional — number of old versions to keep per package; 0 = unlimited (default)

telegram:          # optional — leave empty to disable
  bot_token: ""
  chat_id: ""

packages:
  - name: myapp
    git_url: git@github.com:user/myapp.git
    source_url: https://github.com/user/myapp   # optional: shown in manifest and browser UI
    # download_token: "secret"                  # optional: require Bearer token for binary downloads
    # keep_versions: 10                         # optional: override server.keep_versions for this package
    local_clone: /data/repos/myapp
    version_source:
      type: tag          # track semver/date tags
      pattern: "*-stable"
      # type: branch     # track a branch (version = short SHA)
      # name: main
    build_command: "make build"
    poll_interval: "5m"   # optional: re-check for new versions on a schedule
    binaries:
      - component: server
        path: dist/myapp
      - component: systemd   # optional: ship the systemd unit file as a component
        path: contrib/myapp.service
      - component: openrc    # optional: ship the OpenRC init script as a component
        path: contrib/myapp.openrc
```

`version_source.type`:
- `tag` — version string is the tag name matching `pattern` (e.g. `26.07.02-stable`)
- `branch` — version string is an 8-char commit SHA; any new commit triggers a build

`poll_interval` is a backup for missed webhooks. The server also clones the repository automatically on first startup if `local_clone` does not exist.

`max_upload_size` accepts human-readable values: `"100MB"`, `"512MB"`, `"1GB"`, or a plain byte count. Omit or set to `0` for no limit.

### Run

```sh
pkgtug-server --config /etc/pkgtug/server.yaml
```

On startup the server:
1. Clones any repos that are not yet present on disk (`git clone`)
2. Detects the current version for every package
3. Restores version/build state from `data_dir/server-state.json` (survives restarts)
4. Starts background polling goroutines for packages with `poll_interval` set

### API

| Endpoint | Auth | Description |
|----------|------|-------------|
| `GET /healthz` | none | Server health and current version per package |
| `POST /tug/fetch/<name>` | none | Trigger git fetch; safe to use as a native GitHub/GitLab webhook |
| `GET /tug/repo/<name>/manifest.json` | none | Latest manifest (`version`, `source_url`, `auth_required`, `binaries` with `url`/`sha256`/`size`) |
| `GET /tug/repo/<name>/versions` | none | List of stored versions, newest first |
| `GET /tug/repo/<name>/binaries/<ver>/<platform>/<component>` | none (or Bearer token if `download_token` set) | Download a binary; use `:latest` as version alias |
| `GET /tug/packages` | none | List tracked packages and their current versions |
| `POST /tug/repo/<name>/push` | Bearer secret | Push a pre-built binary directly (AppImage, etc.) |
| `GET /tug/build/next?platform=<p>` | Bearer secret | Worker: claim next pending job |
| `POST /tug/build/<job_id>/result` | Bearer secret | Worker: submit build result |

Configure the webhook in your forge: `POST https://tug.example.com/tug/fetch/<name>`. No secret needed — it only runs `git fetch`.

Repeated webhook calls within the cooldown window (`webhook_cooldown`, default 10 s) are rejected with `429 Too Many Requests`.

### Health check

```sh
curl https://tug.example.com/healthz
```

```json
{
  "status": "ok",
  "packages": { "myapp": "26.07.02-stable" },
  "goos": "linux",
  "goarch": "amd64"
}
```

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

Multiple platforms: repeat the call with a different `platform` value. Declare the package in `server.yaml` with `direct_push: true` — `git_url`, `build_command`, and `binaries` are not needed and not validated:

```yaml
packages:
  - name: myapp
    direct_push: true
    source_url: https://github.com/user/myapp   # optional
    # download_token: "secret"                  # optional
```

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

## Client (`pkgtug`)

### Remotes

pkgtug supports multiple package servers (remotes), similar to Flatpak.

```sh
pkgtug remote add main    https://tug.example.com
pkgtug remote add private https://tug.example.com mytoken   # with download token
pkgtug remote set-token main mytoken                         # set/update token later
pkgtug remote set-token main                                 # clear token
pkgtug remote list
pkgtug remote remove nightly
```

Remotes are stored in `/etc/pkgtug/config.yaml` and managed by the CLI.

#### Meta-remotes

A meta-remote is a JSON file hosted anywhere (GitHub Pages, gist, static server) that lists additional remotes. Useful for sharing a public registry or providing fallback mirrors:

```json
{
  "remotes": [
    { "name": "prod",   "url": "https://tug.example.com",        "priority": 1 },
    { "name": "mirror", "url": "https://tug-mirror.example.com", "priority": 2 }
  ]
}
```

```sh
pkgtug remote meta add https://example.github.io/pkgtug-remotes/remotes.json
pkgtug remote meta list
pkgtug remote meta remove <url>
```

Locally-configured remotes always take precedence over meta-provided ones. Failed meta fetches are silently skipped.

### Usage

```sh
# search / install from pkgtug server
pkgtug search myapp
pkgtug install myapp/server               # auto-discovers remote
pkgtug install main:myapp/server          # explicit remote
pkgtug install --autoupdate myapp/server  # mark for daemon auto-update

# install from GitHub Releases
pkgtug install github:cli/cli             # auto-detects asset for current platform
pkgtug install github:cli/cli/gh          # filter assets by name hint
pkgtug install --autoupdate github:cli/cli

# update
pkgtug update myapp/server
pkgtug update --all                       # skips pinned packages

# daemon — background process that auto-updates marked packages
pkgtug daemon --interval 15m

# mark / unmark for daemon
pkgtug autoupdate myapp/server            # enable
pkgtug autoupdate --remove myapp/server   # disable
pkgtug autoupdate                         # list marked packages

# pin / unpin
pkgtug pin myapp/server                   # lock version, skip in update --all
pkgtug pin --unpin myapp/server

# status, rollback, uninstall
pkgtug status
pkgtug rollback myapp/server
pkgtug uninstall myapp/server
pkgtug uninstall --remove-binary myapp/server
```

### GitHub Releases source

pkgtug can install and update binaries directly from GitHub Releases — no pkgtug server needed for these packages.

```sh
pkgtug install github:cli/cli
pkgtug install github:goreleaser/goreleaser/goreleaser
```

Asset selection:
- pkgtug tries to **auto-detect** the right asset based on OS and architecture (`linux-amd64`, `darwin-arm64`, etc.)
- If the match is ambiguous, an **interactive picker** is shown
- A name hint (`github:owner/repo/hint`) narrows the list before matching

Checksum verification is automatic when the release includes a companion file (`checksums.txt`, `SHA256SUMS`, `<asset>.sha256`, etc.).

Set `GITHUB_TOKEN` in the environment for private repositories and higher API rate limits (5000 req/h vs 60).

GitHub-sourced packages are stored in state alongside pkgtug-server packages. `pkgtug update --all`, `pkgtug check`, and `pkgtug daemon` handle both source types transparently.

### Interactive install

`pkgtug install` is fully interactive. For each component it asks:

1. **Binary path** — where to place the file on disk
2. **Post-install command** — shell command run after every update (e.g. `systemctl daemon-reload`). pkgtug suggests a command based on the target path:
   - `/etc/systemd/system/*.service` → `systemctl daemon-reload && systemctl enable <unit>`
   - `/etc/init.d/*` → `rc-update add <name> default`
3. **Service** — name of the service to stop before and start after the binary is replaced. A picker lists all services detected from `systemctl` / `rc-service`. Type `e` to open `$EDITOR` for manual entry.
4. **Health check** — URL or shell command verified after start (retried up to 5×)
5. **Backup directory** — where to keep the previous binary for `pkgtug rollback`
6. **Dependencies** — other installed `package/component` keys that must be updated before this one

### Service files as components

A package can ship its own daemon configuration as separate components alongside the binary:

```
pkgtug install myapp/systemd
  Binary path: /etc/systemd/system/myapp.service
  Post-install: systemctl daemon-reload && systemctl enable myapp  ← suggested

pkgtug install myapp/server
  Dependencies: 1) myapp/systemd   ← choose
```

`pkgtug update --all` then respects the dependency order: unit file first, binary second.

### Service file conflict detection

pkgtug records the SHA256 of every file it installs. On update it compares:

- **on-disk SHA256** — did the user edit the file since last install?
- **new SHA256** — does the server have a different version?

If both changed, pkgtug reports a conflict. In an interactive terminal:

```
⚠  conflict: myapp/systemd
   /etc/systemd/system/myapp.service was modified locally and a new version is available.

--- current
+++ new
@@ -5,6 +5,7 @@
 ExecStart=/usr/local/bin/myapp
+Restart=on-failure

  (u) use new      — overwrite with new version
  (k) keep current — save new as .pkgtug-new
  (e) edit         — open $EDITOR, new saved as .pkgtug-new
  (a) abort        — skip update for this package
  choice [k]:
```

In non-interactive mode (cron, daemon): automatically keeps the current file, saves the new version as `<path>.pkgtug-new`, and logs a warning — the update is not blocked.

### Update flow

1. Fetch `manifest.json` from the package's remote
2. Compare installed version (string equality — works for both tags and SHAs)
3. Download new file to temp location
4. Verify SHA256
5. **Conflict check** — compare on-disk SHA256 to baseline; prompt or auto-resolve
6. Backup current file to `backup_dir`
7. Stop service (`systemctl` or `rc-service`, auto-detected at runtime)
8. Atomic replace (`rename`)
9. Run `post_install` command if set
10. Start service
11. Health check
12. On failure → restore backup, restart service

### Cron / systemd timer

```
*/15 * * * * pkgtug update --all
```

No daemon required. When stdout is not a terminal (cron, CI), all TUI output is replaced with plain log lines and conflicts are auto-resolved non-destructively.

## Building

```sh
make all              # build all three binaries into dist/
make build-linux-amd64
make build-linux-arm64
```

Requires Go 1.26+. Produces fully static binaries.

## Security model

- The webhook endpoint has **no authentication** — it only runs `git fetch` on a local clone. Safe to expose publicly. Repeated calls within the cooldown window are rate-limited (default 10 s).
- Worker endpoints require `Authorization: Bearer <worker_secret>`. Workers write data that is later served to all clients, so they must be trusted.
- Manifest and package list endpoints are always public — clients need them to discover what's available.
- Binary downloads are **public by default**. Set `download_token` on a package to require `Authorization: Bearer <token>` (or `?token=` query param). The manifest will advertise `auth_required: true` so clients know a token is needed.
- GitHub Releases downloads go directly to `github.com` / `objects.githubusercontent.com`. Set `GITHUB_TOKEN` to authenticate private repos. Checksums are verified automatically when provided by the release.

## License

MIT
