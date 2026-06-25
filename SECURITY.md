# Security Policy

## Supported versions

Security fixes are applied to the `main` branch only. There are no versioned release tracks at this time.

## Reporting a vulnerability

**Do not open a public GitHub issue for security vulnerabilities.**

Use [GitHub's private security advisory](https://github.com/pawi1/pkgtug/security/advisories/new) to report a vulnerability confidentially.

Please include:
- A description of the issue and its potential impact
- Steps to reproduce or a proof-of-concept
- The version or commit hash you tested against

You can expect an acknowledgement within 72 hours and a resolution plan within 14 days.

## Security model

### Authentication boundaries

| Endpoint | Authentication |
|----------|---------------|
| `POST /tug/fetch/<name>` | None — safe by design; only runs `git fetch` on a local clone |
| `GET /tug/repo/<name>/manifest.json` | None — clients need this to discover available versions |
| `GET /tug/repo/<name>/binaries/...` | None by default; Bearer token if `download_token` is set |
| `GET /tug/packages`, `GET /healthz` | None — informational |
| `GET /tug/build/next`, `POST /tug/build/.../result` | Bearer `worker_secret` |
| `POST /tug/repo/<name>/push` | Bearer `worker_secret` |

### Trust model

**Worker secret** — workers build and upload artifacts that clients download and execute. `worker_secret` is a high-value credential: a compromised secret allows pushing arbitrary binaries to any package on the server. Rotate immediately if exposed.

**Download token** — per-package optional secret restricting binary downloads. Clients must store it in cleartext in `/etc/pkgtug/config.yaml`; it is not a substitute for network-level access control.

**Binary integrity** — every stored binary is SHA256-hashed and the hash is published in `manifest.json`. The client re-computes the hash after download and aborts installation on mismatch.

**Network transport** — pkgtug-server listens on plain HTTP. All production deployments must front it with TLS (reverse proxy, Cloudflare Tunnel, etc.). See [docs/wiki/deployment.md](docs/wiki/deployment.md).

### Notable attack surface

- **Webhook flood** — the webhook endpoint is intentionally unauthenticated. An attacker who can reach it can repeatedly trigger `git fetch`. Mitigated by `webhook_cooldown` (default 10 s per package).
- **Compromised worker** — a malicious worker can inject arbitrary binaries. Workers should run in ephemeral, isolated environments (CI VMs, containers) that are discarded after each build.
- **Post-install commands** — `pkgtug update --all` executes the `post_install` shell commands stored in `/etc/pkgtug/state.json`. Protect that file with root-only read permissions (`chmod 600`).
- **GitHub Releases downloads** — `pkgtug install github:owner/repo` fetches directly from `github.com` and `objects.githubusercontent.com`. Checksum verification is automatic when the release includes a checksum file; there is no verification when no checksum file is present.

## Static analysis

CodeQL analysis runs on every push to `main`. See the [CodeQL workflow](https://github.com/pawi1/pkgtug/actions/workflows/codeql.yml) for current status.
