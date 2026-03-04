#!/bin/sh

set -eu

APP_DIR="${APP_DIR:-/data/xiaomi-dnsmasq-gui}"
BIN="${BIN:-$APP_DIR/xiaomi-dnsmasq-gui}"
PID_FILE="${PID_FILE:-$APP_DIR/xiaomi-dnsmasq-gui.pid}"
LISTEN_ADDR="${XIAOMI_DNSMASQ_GUI_LISTEN_ADDR:-0.0.0.0:8088}"
AUTH_FILE="${XIAOMI_DNSMASQ_GUI_AUTH_FILE:-$APP_DIR/auth.conf}"
SESSION_FILE="${XIAOMI_DNSMASQ_GUI_SESSION_FILE:-/tmp/xiaomi-dnsmasq-gui_session}"
LISTEN_PORT="${LISTEN_ADDR##*:}"

[ -x "$BIN" ] || exit 1
mkdir -p "$APP_DIR"

if [ -f "$PID_FILE" ]; then
  OLD_PID="$(cat "$PID_FILE" 2>/dev/null || true)"
  if [ -n "$OLD_PID" ] && kill -0 "$OLD_PID" >/dev/null 2>&1; then
    kill "$OLD_PID" >/dev/null 2>&1 || true
    sleep 1
  fi
fi

# 兼容旧版本，清理占用同端口的 uhttpd 进程。
for PID in $(ps w | grep '[u]httpd' | grep ":$LISTEN_PORT" | awk '{print $1}'); do
  kill "$PID" >/dev/null 2>&1 || true
done

XIAOMI_DNSMASQ_GUI_LISTEN_ADDR="$LISTEN_ADDR" \
XIAOMI_DNSMASQ_GUI_AUTH_FILE="$AUTH_FILE" \
XIAOMI_DNSMASQ_GUI_SESSION_FILE="$SESSION_FILE" \
start-stop-daemon -S -b -m -p "$PID_FILE" -x "$BIN"

sleep 1
NEW_PID="$(cat "$PID_FILE" 2>/dev/null || true)"
[ -n "$NEW_PID" ] || exit 1
kill -0 "$NEW_PID" >/dev/null 2>&1
