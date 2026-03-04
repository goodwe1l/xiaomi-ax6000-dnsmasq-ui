#!/bin/sh

set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
PROJECT_DIR=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)

ROUTER_HOST="${ROUTER_HOST:-10.0.0.1}"
ROUTER_USER="${ROUTER_USER:-root}"
ROUTER_PORT="${ROUTER_PORT:-22}"
ROUTER_PASS="${ROUTER_PASS:-}"
REMOTE_DIR="${REMOTE_DIR:-/data/dhcp_adv}"
HTTP_PORT="${HTTP_PORT:-8088}"
LISTEN_ADDR="${LISTEN_ADDR:-0.0.0.0:${HTTP_PORT}}"
DASHBOARD_PASSWORD="${DASHBOARD_PASSWORD:-}"
TARGET_GOOS="${TARGET_GOOS:-linux}"
TARGET_GOARCH="${TARGET_GOARCH:-arm64}"
GOCACHE_DIR="${GOCACHE_DIR:-/tmp/dhcp_adv_go_cache}"
LOCAL_BIN_ARG="${LOCAL_BIN:-}"
ENABLE_CRON=0
SKIP_VERIFY=0

log() {
  printf '[INFO] %s\n' "$1"
}

warn() {
  printf '[WARN] %s\n' "$1"
}

die() {
  printf '[ERR ] %s\n' "$1" >&2
  exit 1
}

usage() {
  cat <<'USAGE'
用法：
  ./deploy_oneclick.sh [参数]

参数：
  --host <IP>                    路由器地址，默认 10.0.0.1
  --user <USER>                  SSH 用户，默认 root
  --port <PORT>                  SSH 端口，默认 22
  --password <PASS>              SSH 密码（不建议明文）
  --remote-dir <DIR>             远端目录，默认 /data/dhcp_adv
  --http-port <PORT>             对外访问端口，默认 8088
  --listen-addr <ADDR>           服务监听地址，默认 0.0.0.0:8088
  --local-bin <PATH>             使用指定本地二进制，跳过编译
  --dashboard-password <PASS>    管理页密码（写入 auth.conf）
  --enable-cron                  写入保活 cron（* * * * * ensure + @reboot start）
  --skip-verify                  跳过部署后的 HTTP 验证
  -h, --help                     显示帮助

环境变量：
  ROUTER_HOST / ROUTER_USER / ROUTER_PORT / ROUTER_PASS
  REMOTE_DIR / HTTP_PORT / LISTEN_ADDR / DASHBOARD_PASSWORD / LOCAL_BIN
  TARGET_GOOS / TARGET_GOARCH / GOCACHE_DIR
USAGE
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "缺少命令：$1"
}

ssh_run() {
  sshpass -p "$ROUTER_PASS" ssh \
    -o StrictHostKeyChecking=no \
    -o UserKnownHostsFile=/dev/null \
    -p "$ROUTER_PORT" \
    "$ROUTER_USER@$ROUTER_HOST" "$@"
}

scp_send() {
  sshpass -p "$ROUTER_PASS" scp -O \
    -o StrictHostKeyChecking=no \
    -o UserKnownHostsFile=/dev/null \
    -P "$ROUTER_PORT" \
    "$@"
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --host)
      ROUTER_HOST="$2"
      shift 2
      ;;
    --user)
      ROUTER_USER="$2"
      shift 2
      ;;
    --port)
      ROUTER_PORT="$2"
      shift 2
      ;;
    --password)
      ROUTER_PASS="$2"
      shift 2
      ;;
    --remote-dir)
      REMOTE_DIR="$2"
      shift 2
      ;;
    --http-port)
      HTTP_PORT="$2"
      shift 2
      ;;
    --listen-addr)
      LISTEN_ADDR="$2"
      shift 2
      ;;
    --local-bin)
      LOCAL_BIN_ARG="$2"
      shift 2
      ;;
    --dashboard-password)
      DASHBOARD_PASSWORD="$2"
      shift 2
      ;;
    --enable-cron)
      ENABLE_CRON=1
      shift
      ;;
    --skip-verify)
      SKIP_VERIFY=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      usage
      die "未知参数：$1"
      ;;
  esac
done

require_cmd sshpass
require_cmd ssh
require_cmd scp
require_cmd curl

START_FILE="$SCRIPT_DIR/dhcp_adv_start.sh"
ENSURE_FILE="$SCRIPT_DIR/dhcp_adv_ensure.sh"
API_ENTRY_MAIN="$PROJECT_DIR/cmd/dhcp_adv_api/main.go"
PREBUILT_BIN="$SCRIPT_DIR/dhcp_adv_api"
LOCAL_BUILD_DIR="$PROJECT_DIR/build"
LOCAL_BIN="$LOCAL_BUILD_DIR/dhcp_adv_api_${TARGET_GOOS}_${TARGET_GOARCH}"
BUILD_MODE="source"

[ -f "$START_FILE" ] || die "缺少文件：$START_FILE"
[ -f "$ENSURE_FILE" ] || die "缺少文件：$ENSURE_FILE"

if [ -n "$LOCAL_BIN_ARG" ]; then
  [ -f "$LOCAL_BIN_ARG" ] || die "指定二进制不存在：$LOCAL_BIN_ARG"
  LOCAL_BIN="$LOCAL_BIN_ARG"
  BUILD_MODE="custom"
elif [ -f "$API_ENTRY_MAIN" ]; then
  BUILD_MODE="source"
elif [ -f "$PREBUILT_BIN" ]; then
  LOCAL_BIN="$PREBUILT_BIN"
  BUILD_MODE="prebuilt"
else
  die "未找到源码入口或预编译二进制，请传入 --local-bin"
fi

if [ -z "$ROUTER_PASS" ]; then
  printf '请输入 %s@%s 的 SSH 密码: ' "$ROUTER_USER" "$ROUTER_HOST" >&2
  stty -echo
  read -r ROUTER_PASS
  stty echo
  printf '\n' >&2
fi

log "开始部署到 $ROUTER_USER@$ROUTER_HOST:$REMOTE_DIR"

if [ "$BUILD_MODE" = "source" ]; then
  require_cmd go
  log "1/7 本地编译 Go 程序（${TARGET_GOOS}/${TARGET_GOARCH}）"
  mkdir -p "$LOCAL_BUILD_DIR" "$GOCACHE_DIR"
  GOOS="$TARGET_GOOS" GOARCH="$TARGET_GOARCH" CGO_ENABLED=0 GOCACHE="$GOCACHE_DIR" \
    go build -o "$LOCAL_BIN" ./cmd/dhcp_adv_api
elif [ "$BUILD_MODE" = "prebuilt" ]; then
  log "1/7 使用 release 包内预编译二进制：$LOCAL_BIN"
else
  log "1/7 使用指定本地二进制：$LOCAL_BIN"
fi

log "2/7 创建远端目录"
ssh_run "set -e; mkdir -p '$REMOTE_DIR'"

log "3/7 上传程序与启动脚本"
scp_send "$LOCAL_BIN" "$ROUTER_USER@$ROUTER_HOST:$REMOTE_DIR/dhcp_adv_api.new"
scp_send "$START_FILE" "$ROUTER_USER@$ROUTER_HOST:$REMOTE_DIR/start.sh"
scp_send "$ENSURE_FILE" "$ROUTER_USER@$ROUTER_HOST:$REMOTE_DIR/ensure.sh"

if [ -n "$DASHBOARD_PASSWORD" ]; then
  log "4/7 写入管理页密码"
  AUTH_TMP_FILE=$(mktemp)
  printf 'password=%s\n' "$DASHBOARD_PASSWORD" > "$AUTH_TMP_FILE"
  scp_send "$AUTH_TMP_FILE" "$ROUTER_USER@$ROUTER_HOST:$REMOTE_DIR/auth.conf"
  rm -f "$AUTH_TMP_FILE"
else
  log "4/7 跳过密码写入（如需可加 --dashboard-password）"
fi

log "5/7 设置权限并重启服务"
ssh_run "set -e; \
  mv -f '$REMOTE_DIR/dhcp_adv_api.new' '$REMOTE_DIR/dhcp_adv_api'; \
  chmod +x '$REMOTE_DIR/dhcp_adv_api' '$REMOTE_DIR/start.sh' '$REMOTE_DIR/ensure.sh'; \
  [ -f '$REMOTE_DIR/auth.conf' ] && chmod 600 '$REMOTE_DIR/auth.conf' || true; \
  APP_DIR='$REMOTE_DIR' DHCP_ADV_LISTEN_ADDR='$LISTEN_ADDR' '$REMOTE_DIR/start.sh'"

if [ "$ENABLE_CRON" -eq 1 ]; then
  log "6/7 写入 cron 保活策略"
  ssh_run "set -e; \
    crontab_file=/etc/crontabs/root; \
    grep -qF '$REMOTE_DIR/ensure.sh' \"\$crontab_file\" || echo '* * * * * APP_DIR=$REMOTE_DIR START_SCRIPT=$REMOTE_DIR/start.sh $REMOTE_DIR/ensure.sh' >> \"\$crontab_file\"; \
    grep -qF '$REMOTE_DIR/start.sh' \"\$crontab_file\" || echo '@reboot APP_DIR=$REMOTE_DIR DHCP_ADV_LISTEN_ADDR=$LISTEN_ADDR $REMOTE_DIR/start.sh' >> \"\$crontab_file\"; \
    /etc/init.d/cron restart >/dev/null 2>&1 || true"
else
  log "6/7 跳过 cron 写入（如需可加 --enable-cron）"
fi

if [ "$SKIP_VERIFY" -eq 1 ]; then
  warn "已跳过 HTTP 验证（--skip-verify）"
  log "部署完成"
  exit 0
fi

log "7/7 在线验证"
HOME_CODE=$(curl -sS -m 8 -o /dev/null -w '%{http_code}' "http://$ROUTER_HOST:$HTTP_PORT/")
API_CODE=$(curl -sS -m 8 -o /dev/null -w '%{http_code}' "http://$ROUTER_HOST:$HTTP_PORT/cgi-bin/dhcp_adv_api?action=auth_status")
OLD_PAGE_CODE=$(curl -sS -m 8 -o /dev/null -w '%{http_code}' "http://$ROUTER_HOST:$HTTP_PORT/cgi-bin/dhcp_adv.sh")
OLD_API_CODE=$(curl -sS -m 8 -o /dev/null -w '%{http_code}' "http://$ROUTER_HOST:$HTTP_PORT/cgi-bin/dhcp_adv_api.sh?action=auth_status")

log "首页状态码: ${HOME_CODE}"
log "API 状态码: ${API_CODE}"
log "旧页面 URL 状态码: ${OLD_PAGE_CODE}（期望 404）"
log "旧 API URL 状态码: ${OLD_API_CODE}（期望 404）"

[ "$HOME_CODE" = "200" ] || die "首页访问失败"
[ "$API_CODE" = "200" ] || die "API 访问失败"
[ "$OLD_PAGE_CODE" = "404" ] || die "旧页面 URL 未按预期下线"
[ "$OLD_API_CODE" = "404" ] || die "旧 API URL 未按预期下线"

log "部署与验证全部成功"
