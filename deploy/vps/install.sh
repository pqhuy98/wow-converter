#!/usr/bin/env bash
set -euo pipefail

HOSTING="/root/wow-hosting"

cp "${HOSTING}/systemd/"*.service /etc/systemd/system/
cp "${HOSTING}/systemd/"*.timer /etc/systemd/system/
systemctl disable --now wow-data-server.service 2>/dev/null || true
rm -f /etc/systemd/system/wow-data-server.service
systemctl daemon-reload
systemctl enable wow-converter.service
systemctl enable wow-hosting-maintenance.timer wow-converter-deploy.timer
