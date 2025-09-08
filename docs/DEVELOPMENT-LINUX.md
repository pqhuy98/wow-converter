### Linux Dev Setup (for Cursor background agent)

### Prereqs
- **Docker**: running daemon
- **Node.js 20+** and **npm**
- **Git** with submodule support
- **Python 3** (used by wow.export healthcheck)

### Clone repo (with submodules)
```bash
git clone --recurse-submodules https://github.com/pqhuy98/wow-converter.git
cd wow-converter
# If already cloned: sync submodules
git submodule update --init --recursive
```

### Install dependencies (root + subrepos)
```bash
npm install
```
Notes:
- Runs `postinstall` to install `webui` and `wow.export` automatically.

### Start app in dev
```bash
npm run dev   # runs server + webui concurrently
```

### wow.export (Docker)
- Build context MUST be the `wow.export` subrepo (Docker COPY paths are relative there).

Build image:
```bash
cd wow.export/docker
./build-docker.sh
```

Run container (RPC on localhost:17751 by default):
```bash
# Optional env (override as needed)
export WOWEXPORT_PORT=17751          # host port -> container 17751
export WOWEXPORT_NAME=wow.export     # container name
export WOWEXPORT_ASSET_DIR=/tmp/wow.export  # host dir for profile/cache
export WOWEXPORT_WOW_DIR=/path/to/WoW/installation # optional, mounted read-only

./run-docker.sh
# Waits for RPC readiness and performs a healthcheck automatically.
```

Healthcheck (manual):
```bash
python3 ./healthcheck.py --host 127.0.0.1 --port ${WOWEXPORT_PORT:-17751}
```

Tail logs (runtime or container):
```bash
./tail-logs.sh --lines 200                 # runtime log inside container
./tail-logs.sh --container --lines 200     # docker logs
```

Stop/remove container:
```bash
docker rm -f ${WOWEXPORT_NAME:-wow.export}
```

### CI/Agent Notes
- Use absolute paths where possible.
- Long-running `docker`/dev servers should run detached.
- Network bind is 127.0.0.1 only; use `${WOWEXPORT_PORT}` when calling RPC.

### Quickstart (one-shot)
```bash
git clone --recurse-submodules https://github.com/pqhuy98/wow-converter.git && \
cd wow-converter && npm install && \
cd wow.export/docker && ./build-docker.sh && ./run-docker.sh
```

