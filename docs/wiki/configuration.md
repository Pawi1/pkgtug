# Configuration reference

## Server config (`server.yaml`)

Default path: `/etc/pkgtug/server.yaml`

Override: `pkgtug-server --config /path/to/server.yaml`

### `server` block

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `listen` | string | `:8080` | TCP address to listen on |
| `base_url` | string | **required** | Public HTTPS base URL (used in manifest download URLs) |
| `data_dir` | string | **required** | Directory for stored binaries and server state |
| `worker_secret` | string | **required** | Shared secret for worker and direct-push authentication |
| `cors_origins` | []string | `[]` | Allowed CORS origins. Use `["*"]` for the browser UI |
| `webhook_cooldown` | duration | `10s` | Minimum gap between webhook-triggered fetches per package |
| `max_upload_size` | size | `0` (unlimited) | Maximum per-file upload size. Accepts `"100MB"`, `"1GB"`, etc. Useful under Cloudflare Tunnel |
| `keep_versions` | int | `0` (unlimited) | Number of old versions to retain per package. Overridable per package |

### `worker` block (embedded worker, optional)

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `enabled` | bool | `false` | Start a worker goroutine inside the server process |
| `work_dir` | string | `./worker-work` | Directory for git clones and build artifacts |
| `interval` | duration | `30s` | Poll interval |

The embedded worker connects to `http://localhost<listen>` and auto-detects platform from `uname`.

### `telegram` block (optional)

| Key | Type | Description |
|-----|------|-------------|
| `bot_token` | string | Telegram bot token from `@BotFather` |
| `chat_id` | string | Target chat ID (use a negative ID for group chats) |

Leave both empty to disable notifications.

### `packages` list

Each entry describes one tracked project.

#### Common fields

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `name` | string | **required** | Unique identifier used in all API paths |
| `source_url` | string | | Project URL shown in manifest and browser UI. Falls back to `git_url` |
| `download_token` | string | | If set, binary downloads require `Authorization: Bearer <token>` |
| `keep_versions` | int | `0` | Per-package override for version retention. `0` = use `server.keep_versions` |
| `compress` | string | | Compress stored binaries: `zstd`, `xz`, or empty (none) |
| `direct_push` | bool | `false` | Accept pre-built binaries via `POST /push` only — no git/build |

#### Git-tracked fields (required when `direct_push` is false)

| Key | Type | Description |
|-----|------|-------------|
| `git_url` | string | Repository URL (`git@…` or `https://…`) |
| `local_clone` | string | Absolute path where the server maintains its local clone |
| `version_source` | object | See below |
| `pre_build_command` | string | Shell command run in the clone directory before `build_command` |
| `build_command` | string | Shell command that produces the output binaries |
| `binaries` | []Binary | See below |
| `poll_interval` | duration | Re-check for new versions on a schedule (backup for missed webhooks). `0` = disabled |

#### `version_source`

| Key | Type | Description |
|-----|------|-------------|
| `type` | string | `tag` or `branch` |
| `pattern` | string | Glob pattern matching tag names (required when `type: tag`) |
| `name` | string | Branch name (required when `type: branch`) |

For `type: branch`, the version string is the 8-character short SHA of the latest commit; any new commit triggers a build.

#### `binaries` list

Each entry describes one output file.

| Key | Type | Description |
|-----|------|-------------|
| `component` | string | **required** — name used in manifest and API paths |
| `path` | string | **required** — relative path to the built file inside the clone directory |
| `install_deps` | []string | Other component names within this package that must be installed first |
| `detect` | string | Shell command; if it exits non-zero the component is skipped (e.g. `which systemctl`) |
| `system_deps` | []SystemDep | System binaries or libraries required at runtime |

#### `system_deps` list

| Key | Type | Description |
|-----|------|-------------|
| `file` | string | Binary name (e.g. `openssl`) or absolute path (e.g. `/usr/lib/libssl.so.3`) |
| `name` | string | Optional human-readable label; defaults to `file` |

### Full example

```yaml
server:
  listen: ":8080"
  base_url: "https://tug.example.com"
  data_dir: "/var/lib/pkgtug"
  worker_secret: "change-me"
  cors_origins:
    - "*"
  webhook_cooldown: "10s"
  max_upload_size: "200MB"
  keep_versions: 5

telegram:
  bot_token: "123456:AAAA…"
  chat_id: "-100…"

packages:
  - name: myapp
    git_url: git@github.com:user/myapp.git
    source_url: https://github.com/user/myapp
    local_clone: /data/repos/myapp
    version_source:
      type: tag
      pattern: "*-stable"
    pre_build_command: "go generate ./..."
    build_command: "make dist"
    poll_interval: "5m"
    keep_versions: 10
    compress: zstd
    binaries:
      - component: server
        path: dist/myapp
        system_deps:
          - file: openssl
      - component: systemd
        path: contrib/myapp.service
        detect: "which systemctl"

  - name: myapp-appimage
    direct_push: true
    source_url: https://github.com/user/myapp
    download_token: "secret"
```

---

## Client config (`config.yaml`)

Default path: `/etc/pkgtug/config.yaml`

### `remotes` list

| Key | Type | Description |
|-----|------|-------------|
| `name` | string | Unique remote name used as a prefix in install commands |
| `url` | string | pkgtug-server base URL |
| `token` | string | Bearer token for protected downloads (`download_token` on the server) |

### `meta_urls` list

A list of URLs pointing to JSON files that provide additional remotes. Each JSON file has the schema:

```json
{
  "remotes": [
    { "name": "prod",   "url": "https://tug.example.com",        "priority": 1 },
    { "name": "mirror", "url": "https://tug-mirror.example.com", "priority": 2 }
  ]
}
```

Lower `priority` = tried first. Local remotes always take precedence.

### `telegram` block (optional)

Same fields as the server config: `bot_token` and `chat_id`. Used by the client to send Telegram notifications on update.

### Full example

```yaml
remotes:
  - name: main
    url: https://tug.example.com
  - name: internal
    url: https://tug.internal.example.com
    token: "secret-token"

meta_urls:
  - https://example.github.io/pkgtug-remotes/remotes.json

telegram:
  bot_token: ""
  chat_id: ""
```
