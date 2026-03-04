#!/bin/sh

set -eu

APP_DIR="${APP_DIR:-/data/xiaomi-dnsmasq-gui}"
PID_FILE="${PID_FILE:-$APP_DIR/xiaomi-dnsmasq-gui.pid}"
START_SCRIPT="${START_SCRIPT:-$APP_DIR/start.sh}"

if [ -f "$PID_FILE" ]; then
  PID="$(cat "$PID_FILE" 2>/dev/null || true)"
  if [ -n "$PID" ] && kill -0 "$PID" >/dev/null 2>&1; then
    exit 0
  fi
fi

[ -x "$START_SCRIPT" ] || exit 1
"$START_SCRIPT"
