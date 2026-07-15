# VPS hosting (`wow.quangdel.com`)

Guide for AI agents to restore or operate production hosting.

## Requirements

- Host the bundled Go `wow-converter` (converter + in-process wow-data-server) behind nginx, managed by **systemd**.
- Live scripts live in **`/root/wow-hosting/`**; this git folder is the source of truth.
- Resilience: boot automatically, restart on failure.
- **Weekly maintenance**: run `clean.sh`, restart the bundled service.
- **Auto-deploy**: poll GitHub branch `golang-port` every **30s**, pull, rebuild, restart. Override with `WOW_DEPLOY_BRANCH`; no GitHub SSH secrets.
- **Security**: never commit `.env` (use `env.example`). No keys in this folder.

## Layout

| Git | VPS |
|-----|-----|
| `deploy/vps/` | copied to `/root/wow-hosting/` via `sync-from-repo.sh` |
| `deploy/vps/clean.sh` | `/root/clean.sh` |
| app source | `/root/wow-converter/` |
| secrets | `/root/wow-converter/.env` only |

Service: bundled Go `wow-converter` (:3001). Its wow-data-server and CASC runtime are in-process; nginx proxies :443 → 3001.

Timers: `wow-converter-deploy.timer` (30s poll), `wow-hosting-maintenance.timer` (Sun 04:00 UTC).

## Fresh VPS setup

```bash
apt update && apt install -y git nginx certbot python3-certbot-nginx curl python3 make g++ golang-go
curl -fsSL https://bun.sh/install | bash

cd /root
git clone --branch golang-port https://github.com/pqhuy98/wow-converter.git
cd wow-converter
cp deploy/vps/env.example .env
bun install && bun run build:linux

bash deploy/vps/sync-from-repo.sh
systemctl start wow-converter
systemctl enable --now wow-converter-deploy.timer wow-hosting-maintenance.timer
```

Configure nginx (`443` → `127.0.0.1:3001`), then `certbot --nginx -d wow.quangdel.com`.

First start may take 1–2 min while remote CASC loads. A process/deploy restart reloads CASC; an individual export job timeout does not restart or reset the in-process data server.

## Day-2 ops

```bash
bash /root/wow-converter/deploy/vps/sync-from-repo.sh   # after editing deploy/vps in git
bash /root/wow-hosting/scripts/deploy-app.sh           # manual deploy
FORCE_DEPLOY=1 bash /root/wow-hosting/scripts/deploy-app.sh

systemctl status wow-converter
journalctl -u wow-converter -f
systemctl start wow-hosting-maintenance.service        # manual clean + restart
```

Verify: `curl -sf http://127.0.0.1:3001/` and confirm `cascLoaded: true` from `curl -sf http://127.0.0.1:3001/api/wow-config/status`.

## Deploy flow (`deploy-app.sh`)

1. `git fetch` + `reset --hard origin/${WOW_DEPLOY_BRANCH:-golang-port}`
2. `bun install` + `bun run build:linux` (Next.js static UI + Linux Go bundle)
3. `sync-from-repo.sh` installs the bundled Go systemd unit and disables the obsolete split `wow-data-server` unit
4. Restart `wow-converter`, then wait for HTTP and in-process CASC readiness

## Troubleshooting

**502 / not ready** — bundled CASC load may still be running; check `journalctl -u wow-converter`.

**Disk** — weekly `clean.sh` removes `.cache/wow-export`, `exported-assets`, `exported-assets-browse`, `recent-exports.json`.
