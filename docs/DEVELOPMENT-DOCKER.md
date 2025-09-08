### wow.export Docker workflow (Linux/WSL2)

Compact guide for an AI agent to build, run, validate, and tail logs for the wow.export Docker image. Scripts can be invoked from any directory; use repo-root relative paths or set `REPO=/abs/path/to/repo` and call `$REPO/...`. The run script uses your current working directory to create/mount `./exports` → `/exports`.

### Prereqs
- **Docker** and **Python 3** available on PATH.
- Build context is the `wow.export` subrepo.
 - Publishing images is human-only; agents must not publish.

### Build
```bash
# from repo root
./wow.export/docker/build-docker.sh

# from anywhere
$REPO/wow.export/docker/build-docker.sh
```
- Builds `wow.export:latest` using `wow.export/Dockerfile` and the `wow.export` context.
 
 - Rebuild when app code changes under `wow.export/src/` (e.g., any `*.js`) or other files copied into the image. No rebuild is needed for editing host scripts under `wow.export/docker/*.sh`.

### Run (idempotent)
```bash
# from repo root
./wow.export/docker/run-docker.sh

# from anywhere
$REPO/wow.export/docker/run-docker.sh
```
- Starts container `wow.export` on `127.0.0.1:17751` by default and waits for RPC readiness, runs healthcheck, then selects CASC.
- Environment overrides if necessary (prefer defaults):
  - `WOWEXPORT_PORT` (default `17751`)
  - `WOWEXPORT_NAME` (default `wow.export`)
  - `WOWEXPORT_ASSET_DIR` (default `/tmp/wow.export`)
  - `WOWEXPORT_WOW_DIR` (optional host WoW path; will be mounted read-only at same path)
  - `WAIT_SECONDS` (default `60`)

### Tail logs (bounded)
- Runtime log inside container (default path `/tmp/wow.profile/Default/runtime.log`):
```bash
# from repo root
./wow.export/docker/tail-logs.sh --duration 15

# from anywhere
$REPO/wow.export/docker/tail-logs.sh --duration 15
```
- Docker container logs instead of runtime log:
```bash
# from repo root
./wow.export/docker/tail-logs.sh --container --duration 15

# from anywhere
$REPO/wow.export/docker/tail-logs.sh --container --duration 15
```
- Options: `--name NAME`, `--lines N`, `--profile-log PATH`.

### Validate readiness (manual check if needed)
```bash
# from repo root
python3 wow.export/docker/helpers/healthcheck.py --host 127.0.0.1 --port 17751

# from anywhere
python3 $REPO/wow.export/docker/helpers/healthcheck.py --host 127.0.0.1 --port 17751
```
- Expect HTTP 200 and OK. The run script already performs this check; use manually after changes.

### Agent workflow for edits
1) Edit app code in `wow.export/src/` (e.g., `*.js`) or other files bundled into the image.
2) If app code changed (e.g., any `wow.export/src/**/*.js`), rebuild the image; if only host shell scripts changed (`wow.export/docker/*.sh`), skip rebuild.
3) Start fresh container: run script (removes old, maps port, waits, healthchecks, selects CASC).
4) Tail logs briefly to verify behavior.
5) If applicable, run manual healthcheck.

### Troubleshooting
- Port in use: set `WOWEXPORT_PORT` to a free port and re-run.
- Stale NW.js locks: auto-removed on run; verify in `WOWEXPORT_ASSET_DIR`.
- CASC selection fails: set `CASC_LOCAL_WOW`/`CASC_LOCAL_PRODUCT` or `CASC_REMOTE_REGION`/`CASC_REMOTE_PRODUCT` envs (consumed by helper `select-casc.py`).
- Container name conflicts: the run script removes any existing container with the same name.
