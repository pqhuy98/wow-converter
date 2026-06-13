#!/usr/bin/env bash
set -euo pipefail

HOSTING="/root/wow-hosting"

cp "${HOSTING}/systemd/"*.service /etc/systemd/system/
cp "${HOSTING}/systemd/"*.timer /etc/systemd/system/
systemctl daemon-reload
systemctl enable wow-data-server.service wow-converter.service
systemctl enable wow-hosting-maintenance.timer wow-converter-deploy.timer
