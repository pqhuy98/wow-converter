# VPS hosting (`wow.quangdel.com`)

Guide for AI agents to restore or operate production hosting.

## Requirements

- Host `wow-converter` + `wow-data-server` on a VPS behind nginx, managed by **systemd**.
- Live scripts live in **`/root/wow-hosting/`**; this git folder is the source of truth.
- Resilience: boot automatically, restart on failure.
- **Weekly maintenance**: run `clean.sh`, restart both services.
- **Auto-deploy**: poll GitHub `main` every **30s**, pull, rebuild, restart. No GitHub SSH secrets.
- **Security**: never commit `.env` (use `env.example`). No keys in this folder.

## Layout

| Git | VPS |
|-----|-----|
| `deploy/vps/` | copied to `/root/wow-hosting/` via `sync-from-repo.sh` |
| `deploy/vps/clean.sh` | `/root/clean.sh` |
| app source | `/root/wow-converter/` |
| secrets | `/root/wow-converter/.env` only |

Services: `wow-data-server` (:17753 REST) → `wow-converter` (:3001). nginx proxies :443 → 3001.

Timers: `wow-converter-deploy.timer` (30s poll), `wow-hosting-maintenance.timer` (Sun 04:00 UTC).

## Fresh VPS setup

```bash
apt update && apt install -y git nginx certbot python3-certbot-nginx curl python3 make g++
curl -fsSL https://bun.sh/install | bash

cd /root
git clone https://github.com/pqhuy98/wow-converter.git
cd wow-converter
cp deploy/vps/env.example .env
bun install && bun run build:server

bash deploy/vps/sync-from-repo.sh
systemctl start wow-data-server wow-converter
systemctl enable --now wow-converter-deploy.timer wow-hosting-maintenance.timer
```

Configure nginx (`443` → `127.0.0.1:3001`), then `certbot --nginx -d wow.quangdel.com`.

First start may take 1–2 min while remote CASC loads.

## Day-2 ops

```bash
bash /root/wow-converter/deploy/vps/sync-from-repo.sh   # after editing deploy/vps in git
bash /root/wow-hosting/scripts/deploy-app.sh           # manual deploy
FORCE_DEPLOY=1 bash /root/wow-hosting/scripts/deploy-app.sh

systemctl status wow-data-server wow-converter
journalctl -u wow-converter -f
systemctl start wow-hosting-maintenance.service        # manual clean + restart
```

Verify: `curl -sf http://127.0.0.1:3001/` and `curl -sf http://127.0.0.1:17753/rest/getConfig`

## Deploy flow (`deploy-app.sh`)

1. `git fetch` + `reset --hard origin/main`
2. `sync-from-repo.sh`
3. `bun install` + `bun run build:server`
4. Restart `wow-data-server`, then `wow-converter`

## Troubleshooting

**502 / not ready** — CASC load still running; check `journalctl -u wow-data-server` and `journalctl -u wow-converter`.

**Disk** — weekly `clean.sh` removes `.cache/wow-export`, `exported-assets`, `exported-assets-browse`, `recent-exports.json`.
