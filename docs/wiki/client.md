# pkgtug client

## Installation

```sh
curl -fsSL https://pawi1.github.io/pkgtug/install.sh | sh
# or
wget -qO- https://pawi1.github.io/pkgtug/install.sh | sh
```

The script detects your OS and architecture, downloads a bootstrap binary, adds the public remote, and installs pkgtug via `pkgtug install` so it self-updates. Requires `sudo`, `doas`, or `run0`, or run as root.

## Config and state files

| File | Purpose |
|------|---------|
| `/etc/pkgtug/config.yaml` | Remotes, meta-remote URLs, Telegram config |
| `/etc/pkgtug/state.json` | Installed packages, versions, paths, SHA256 checksums |

Both files require root permissions to modify. `state.json` is written atomically (temp file + rename).

## Remotes

A remote is a named pointer to a pkgtug-server base URL.

```sh
pkgtug remote add main    https://tug.example.com
pkgtug remote add private https://tug.example.com mytoken   # with download token
pkgtug remote set-token main mytoken                         # add/update token
pkgtug remote set-token main                                 # clear token
pkgtug remote list
pkgtug remote remove nightly
```

Tokens are stored in `/etc/pkgtug/config.yaml` in cleartext. Protect the file with `chmod 600`.

### Meta-remotes

A meta-remote is a JSON file hosted anywhere that lists additional remotes. Useful for sharing a public registry or providing fallback mirrors:

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

Lower `priority` = tried first. Local remotes always take precedence over meta-provided ones. Failed meta fetches are silently skipped.

## Installing packages

### From a pkgtug server

```sh
pkgtug install myapp/server               # auto-discovers remote
pkgtug install main:myapp/server          # explicit remote
pkgtug install --autoupdate myapp/server  # mark for daemon/cron auto-update
```

### From GitHub Releases

```sh
pkgtug install github:cli/cli             # auto-detects asset for current platform
pkgtug install github:cli/cli/gh          # narrow by name hint before auto-detect
pkgtug install --autoupdate github:cli/cli
```

Asset selection: pkgtug tries to auto-detect the right asset based on OS and architecture. If the match is ambiguous, an interactive picker is shown. A name hint narrows the list before matching.

Checksum verification is automatic when the release includes a companion file (`checksums.txt`, `SHA256SUMS`, `<asset>.sha256`, etc.).

Set `GITHUB_TOKEN` in the environment for private repos and higher API rate limits (5 000 req/h vs 60).

### Interactive install prompts

For each component, the installer asks:

1. **Binary path** — destination path on disk
2. **Post-install command** — shell command run after every update. Suggested automatically:
   - Path under `/etc/systemd/system/*.service` → `systemctl daemon-reload && systemctl enable <unit>`
   - Path under `/etc/init.d/*` → `rc-update add <name> default`
3. **Service** — stop before / start after binary replacement (interactive picker from `systemctl`/`rc-service`)
4. **Health check** — URL or shell command verified after service start (retried up to 5×)
5. **Backup directory** — where to keep the previous binary for `pkgtug rollback`
6. **Dependencies** — other `package/component` keys that must be updated first

## Update flow

```
1. GET manifest.json from the package's remote
2. Compare installed version (string equality)
3. Download new binary to a temp file beside the target path
4. Verify SHA256 against manifest — abort on mismatch
5. Conflict check — compare on-disk SHA256 to post-install baseline
6. Backup current file to backup_dir
7. Stop service (systemctl / rc-service, auto-detected)
8. Atomic rename (temp → target)
9. Run post_install command (e.g. systemctl daemon-reload)
10. Start service
11. Health check — retried up to 5×
12. On failure — restore backup, restart service
```

## Conflict resolution

When a file has been modified locally *and* a new version is available upstream, pkgtug reports a conflict.

In an interactive terminal:

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

In non-interactive mode (cron, daemon): keeps the current file, saves the new version as `<path>.pkgtug-new`, and logs a warning. The update is not blocked.

## Commands reference

```sh
# Search and install
pkgtug search <query>
pkgtug install [<remote>:]<package>/<component>
pkgtug install github:<owner>/<repo>[/<hint>]
pkgtug install --autoupdate <spec>

# Update
pkgtug update <package>/<component>
pkgtug update --all                  # updates all non-pinned packages

# Status and rollback
pkgtug status
pkgtug rollback <package>/<component>
pkgtug uninstall <package>/<component>
pkgtug uninstall --remove-binary <package>/<component>

# Remotes
pkgtug remote add <name> <url> [token]
pkgtug remote remove <name>
pkgtug remote list
pkgtug remote set-token <name> [token]
pkgtug remote meta add <url>
pkgtug remote meta list
pkgtug remote meta remove <url>

# Pin / unpin
pkgtug pin <package>/<component>
pkgtug pin --unpin <package>/<component>

# Auto-update marks
pkgtug autoupdate <package>/<component>
pkgtug autoupdate --remove <package>/<component>
pkgtug autoupdate                    # list marked packages

# Daemon
pkgtug daemon --interval 15m
```

## Auto-update: daemon vs cron

**Daemon** — long-running process; useful on servers where a systemd service or OpenRC service is not practical.

```sh
pkgtug daemon --interval 15m
```

**Cron** — simpler; no persistent process needed.

```
*/15 * * * * pkgtug update --all
```

When stdout is not a terminal, all TUI output is replaced with plain log lines and conflicts are resolved non-destructively.

## Service files as components

A package can ship its systemd unit or OpenRC init script as a separate installable component. This allows pkgtug to manage updates to both the binary and its service definition with correct ordering:

```sh
pkgtug install myapp/systemd
#   Binary path: /etc/systemd/system/myapp.service
#   Post-install: systemctl daemon-reload && systemctl enable myapp  ← suggested

pkgtug install myapp/server
#   Dependencies: 1) myapp/systemd   ← choose
```

`pkgtug update --all` then updates `myapp/systemd` before `myapp/server`.

## State file schema

`/etc/pkgtug/state.json` is a JSON object keyed by `<package>/<component>`:

```json
{
  "myapp/server": {
    "remote": "main",
    "installed_version": "26.07.01-stable",
    "updated_at": "2026-07-01T12:00:00Z",
    "binary_path": "/usr/local/bin/myapp",
    "service_name": "myapp",
    "health_check": "https://localhost:8080/healthz",
    "backup_dir": "/var/backups/pkgtug/myapp",
    "pinned": false,
    "auto_update": true,
    "depends_on": ["myapp/systemd"],
    "post_install": "systemctl daemon-reload",
    "installed_sha256": "abc123…"
  }
}
```
