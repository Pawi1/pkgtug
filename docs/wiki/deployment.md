# Deployment

## System requirements

- Linux (amd64 or arm64)
- A dedicated user (`pkgtug`) with write access to `data_dir` and the git clone directories
- Git installed (required by the server for cloning and fetching repos)
- SSH key or HTTPS credentials for the repos you want to track
- A reverse proxy for TLS termination

## User and directories

```sh
useradd -r -s /sbin/nologin pkgtug
mkdir -p /var/lib/pkgtug /data/repos /etc/pkgtug
chown pkgtug:pkgtug /var/lib/pkgtug /data/repos
chmod 750 /var/lib/pkgtug /data/repos
```

Config and credentials:

```sh
# server config
install -m 640 -o pkgtug -g pkgtug config.server.yaml /etc/pkgtug/server.yaml

# SSH key for git clones (if using git@ URLs)
install -d -m 700 -o pkgtug /home/pkgtug/.ssh
install -m 600 -o pkgtug /path/to/deploy_key /home/pkgtug/.ssh/id_ed25519
```

## systemd service

```ini
# /etc/systemd/system/pkgtug-server.service
[Unit]
Description=pkgtug server
After=network.target

[Service]
Type=simple
User=pkgtug
ExecStart=/usr/local/bin/pkgtug-server --config /etc/pkgtug/server.yaml
Restart=on-failure
RestartSec=5s

# Harden (adjust as needed for git SSH access)
PrivateTmp=true
ProtectHome=read-only
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
```

```sh
systemctl daemon-reload
systemctl enable --now pkgtug-server
```

## Reverse proxy

pkgtug-server listens on plain HTTP. Front it with TLS.

### nginx

```nginx
server {
    listen 443 ssl http2;
    server_name tug.example.com;

    ssl_certificate     /etc/letsencrypt/live/tug.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/tug.example.com/privkey.pem;

    # Increase for large binary uploads
    client_max_body_size 500M;

    location / {
        proxy_pass         http://127.0.0.1:8080;
        proxy_set_header   Host              $host;
        proxy_set_header   X-Real-IP         $remote_addr;
        proxy_set_header   X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header   X-Forwarded-Proto $scheme;
        proxy_read_timeout 600s;
        proxy_send_timeout 600s;
    }
}
```

### Caddy

```
tug.example.com {
    reverse_proxy localhost:8080 {
        transport http {
            read_timeout  600s
            write_timeout 600s
        }
    }
}
```

### Cloudflare Tunnel

Cloudflare Tunnel imposes a 100 MB upload limit. Set `max_upload_size` in server config to match:

```yaml
server:
  max_upload_size: "99MB"
```

## Cloudflare Tunnel setup

```sh
cloudflared tunnel create pkgtug
cloudflared tunnel route dns pkgtug tug.example.com
```

`~/.cloudflared/config.yml`:

```yaml
tunnel: <tunnel-id>
credentials-file: /home/pkgtug/.cloudflared/<tunnel-id>.json

ingress:
  - hostname: tug.example.com
    service: http://localhost:8080
  - service: http_status:404
```

## SSH key for git clones

When using `git@github.com:` or `git@gitlab.example.com:` URLs, the server process must have a working SSH key:

```sh
# Generate a deploy key
ssh-keygen -t ed25519 -C "pkgtug@$(hostname)" -f /home/pkgtug/.ssh/id_ed25519 -N ""

# Add the public key to GitHub / GitLab as a read-only deploy key
cat /home/pkgtug/.ssh/id_ed25519.pub
```

Configure `known_hosts`:

```sh
su - pkgtug -s /bin/sh -c "ssh-keyscan github.com >> ~/.ssh/known_hosts"
su - pkgtug -s /bin/sh -c "ssh-keyscan gitlab.example.com >> ~/.ssh/known_hosts"
```

## Production checklist

- [ ] `worker_secret` is a random, high-entropy string (e.g. `openssl rand -hex 32`)
- [ ] `server.yaml` is readable only by the `pkgtug` user (`chmod 640`)
- [ ] `data_dir` and clone dirs are owned by `pkgtug` and not world-readable
- [ ] TLS is terminated at the reverse proxy; pkgtug-server is not exposed directly
- [ ] `download_token` is set on packages whose binaries should not be public
- [ ] Workers run in isolated environments (CI VMs, containers), not on the server host
- [ ] `state.json` on client hosts is `chmod 600` (contains `post_install` commands)
- [ ] `keep_versions` is set if disk space is a concern

## Upgrading pkgtug-server

pkgtug-server can manage its own binary if you add it as a package:

```yaml
packages:
  - name: pkgtug
    direct_push: true
```

Then push a new binary via CI and install on the server host with `pkgtug update pkgtug/server`.
