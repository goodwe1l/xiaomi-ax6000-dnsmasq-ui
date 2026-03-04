#!/bin/sh

set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
PROJECT_DIR=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)

MODE="deploy"
if [ "$#" -gt 0 ]; then
  case "$1" in
    deploy)
      MODE="deploy"
      shift
      ;;
    install)
      MODE="install"
      shift
      ;;
    uninstall)
      MODE="uninstall"
      shift
      ;;
    uninstall-local|local)
      MODE="uninstall-local"
      shift
      ;;
  esac
fi

ROUTER_HOST="${ROUTER_HOST:-}"
ROUTER_PORT="${ROUTER_PORT:-}"
ROUTER_USER="${ROUTER_USER:-}"
ROUTER_PASS="${ROUTER_PASS:-}"
ROUTER_IP="${ROUTER_IP:-}"

REMOTE_DIR="${REMOTE_DIR:-/data/xiaomi-dnsmasq-gui}"
HTTP_PORT="${HTTP_PORT:-}"
LISTEN_ADDR="${LISTEN_ADDR:-}"
DASHBOARD_PASSWORD="${DASHBOARD_PASSWORD:-}"

TARGET_GOOS="${TARGET_GOOS:-linux}"
TARGET_GOARCH="${TARGET_GOARCH:-arm64}"
GOCACHE_DIR="${GOCACHE_DIR:-/tmp/xiaomi-dnsmasq-gui_go_cache}"
LOCAL_BIN_ARG="${LOCAL_BIN:-}"

GITHUB_REPO="${GITHUB_REPO:-goodwe1l/xiaomi-ax6000-dnsmasq-ui}"
RELEASE_TAG="${RELEASE_TAG:-latest}"

ENABLE_CRON="${ENABLE_CRON:-1}"
SKIP_VERIFY=0
CONFIRM="${CONFIRM:-}"

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

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "缺少命令：$1"
}

is_tty() {
  [ -t 0 ] && [ -t 1 ]
}

prompt_value() {
  var_name="$1"
  prompt_text="$2"
  default_value="${3:-}"
  secret="${4:-0}"

  eval "current_value=\${$var_name:-}"
  if [ -n "$current_value" ]; then
    return
  fi

  if ! is_tty; then
    if [ -n "$default_value" ]; then
      eval "$var_name=\$default_value"
      return
    fi
    die "参数缺失且当前非交互终端：$var_name"
  fi

  if [ "$secret" -eq 1 ]; then
    if [ -n "$default_value" ]; then
      printf '%s [%s]: ' "$prompt_text" "$default_value" >&2
    else
      printf '%s: ' "$prompt_text" >&2
    fi
    stty -echo
    read -r input_value
    stty echo
    printf '\n' >&2
  else
    if [ -n "$default_value" ]; then
      printf '%s [%s]: ' "$prompt_text" "$default_value" >&2
    else
      printf '%s: ' "$prompt_text" >&2
    fi
    read -r input_value
  fi

  if [ -z "$input_value" ]; then
    input_value="$default_value"
  fi
  eval "$var_name=\$input_value"
}

prompt_yes_no() {
  var_name="$1"
  prompt_text="$2"
  default_value="${3:-N}"

  eval "current_value=\${$var_name:-}"
  if [ -n "$current_value" ]; then
    return
  fi

  if ! is_tty; then
    eval "$var_name=\$default_value"
    return
  fi

  printf '%s [%s]: ' "$prompt_text" "$default_value" >&2
  read -r answer
  if [ -z "$answer" ]; then
    answer="$default_value"
  fi
  eval "$var_name=\$answer"
}

usage() {
  cat <<'USAGE'
用法：
  ./deploy.sh [deploy] [参数]
  ./deploy.sh install [参数]
  ./deploy.sh uninstall [参数]
  ./deploy.sh uninstall-local [参数]

模式：
  deploy（默认）   在本地执行：编译/上传到路由器并拉起服务
  install          在路由器执行：在线下载 Release 包并本机安装
  uninstall        在本地执行：通过 SSH 卸载路由器上的服务
  uninstall-local  在路由器执行：本机直接卸载

通用参数：
  --remote-dir <DIR>             安装目录，默认 /data/xiaomi-dnsmasq-gui
  --http-port <PORT>             面板端口（外部访问端口），默认 8088
  --listen-addr <ADDR>           服务监听地址，默认 路由IP:面板端口
  --dashboard-password <PASS>    面板登录密码（写入 auth.conf）
  --enable-cron                  启用 cron 保活（默认已启用）
  --disable-cron                 禁用 cron 保活
  --skip-verify                  跳过部署后的 HTTP 验证
  --yes                          跳过卸载确认（仅 uninstall/uninstall-local）

deploy 模式参数（本地执行）：
  --host <IP>                    路由器 IP（SSH 目标）
  --port <PORT>                  SSH 端口，默认 22
  --user <USER>                  SSH 用户，默认 root
  --password <PASS>              SSH 密码
  --local-bin <PATH>             使用指定本地二进制，跳过编译

install 模式参数（路由器执行）：
  --router-ip <IP>               路由器 LAN IP（用于监听和最终访问地址）
  --repo <OWNER/REPO>            Release 仓库，默认 goodwe1l/xiaomi-ax6000-dnsmasq-ui
  --tag <TAG|latest>             Release 标签，默认 latest

uninstall 模式参数（本地执行）：
  --host <IP>                    路由器 IP（SSH 目标）
  --port <PORT>                  SSH 端口，默认 22
  --user <USER>                  SSH 用户，默认 root
  --password <PASS>              SSH 密码

示例：
  ROUTER_PASS='你的SSH密码' ./scripts/deploy.sh --host 10.0.0.1
  ROUTER_PASS='你的SSH密码' ./scripts/deploy.sh uninstall --host 10.0.0.1 --yes
  curl -fsSL https://raw.githubusercontent.com/goodwe1l/xiaomi-ax6000-dnsmasq-ui/refs/heads/main/scripts/deploy.sh | sh -s -- install
  curl -fsSL https://raw.githubusercontent.com/goodwe1l/xiaomi-ax6000-dnsmasq-ui/refs/heads/main/scripts/deploy.sh | sh -s -- uninstall-local --yes
USAGE
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

resolve_router_ip_from_system() {
  if [ -n "$ROUTER_IP" ]; then
    return
  fi

  if command -v uci >/dev/null 2>&1; then
    ROUTER_IP="$(uci -q get network.lan.ipaddr 2>/dev/null || true)"
  fi

  if [ -z "$ROUTER_IP" ] && command -v ip >/dev/null 2>&1; then
    ROUTER_IP="$(ip -4 addr show 2>/dev/null | awk '/inet / && $2 !~ /^127\./ {split($2,a,"/"); print a[1]; exit}')"
  fi

  if [ -z "$ROUTER_IP" ]; then
    ROUTER_IP="10.0.0.1"
  fi
}

parse_args() {
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --host)
        ROUTER_HOST="$2"
        shift 2
        ;;
      --port)
        ROUTER_PORT="$2"
        shift 2
        ;;
      --user)
        ROUTER_USER="$2"
        shift 2
        ;;
      --password)
        ROUTER_PASS="$2"
        shift 2
        ;;
      --router-ip)
        ROUTER_IP="$2"
        shift 2
        ;;
      --remote-dir)
        REMOTE_DIR="$2"
        shift 2
        ;;
      --http-port|--panel-port)
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
      --repo)
        GITHUB_REPO="$2"
        shift 2
        ;;
      --tag)
        RELEASE_TAG="$2"
        shift 2
        ;;
      --enable-cron)
        ENABLE_CRON=1
        shift
        ;;
      --disable-cron)
        ENABLE_CRON=0
        shift
        ;;
      --skip-verify)
        SKIP_VERIFY=1
        shift
        ;;
      --yes)
        CONFIRM="Y"
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
}

validate_remote_dir() {
  case "$REMOTE_DIR" in
    ""|"/")
      die "REMOTE_DIR 非法：$REMOTE_DIR"
      ;;
  esac
}

confirm_or_exit() {
  prompt_yes_no CONFIRM "确认执行卸载并删除目录 $REMOTE_DIR 吗？" "N"
  case "$CONFIRM" in
    Y|y|YES|yes)
      ;;
    *)
      die "已取消卸载"
      ;;
  esac
}

cleanup_local_runtime() {
  app_dir="$1"
  pid_file="$app_dir/xiaomi-dnsmasq-gui.pid"

  if [ -f "$pid_file" ]; then
    pid="$(cat "$pid_file" 2>/dev/null || true)"
    if [ -n "$pid" ] && kill -0 "$pid" >/dev/null 2>&1; then
      kill "$pid" >/dev/null 2>&1 || true
      sleep 1
    fi
  fi

  pkill -f "$app_dir/xiaomi-dnsmasq-gui" >/dev/null 2>&1 || true

  crontab_file="/etc/crontabs/root"
  if [ -f "$crontab_file" ]; then
    tmp_file="$(mktemp)"
    grep -vF "$app_dir/ensure.sh" "$crontab_file" | grep -vF "$app_dir/start.sh" > "$tmp_file" || true
    cat "$tmp_file" > "$crontab_file"
    rm -f "$tmp_file"
    /etc/init.d/cron restart >/dev/null 2>&1 || true
  fi

  rm -rf "$app_dir"
}

setup_deploy_inputs() {
  prompt_value ROUTER_HOST "路由器 IP" "10.0.0.1"
  prompt_value ROUTER_PORT "SSH 端口" "22"
  prompt_value ROUTER_USER "SSH 账号" "root"
  prompt_value ROUTER_PASS "SSH 密码" "" 1
  prompt_value HTTP_PORT "面板端口" "8088"
  prompt_value DASHBOARD_PASSWORD "面板密码" "admin123456" 1

  if [ -z "$LISTEN_ADDR" ]; then
    LISTEN_ADDR="${ROUTER_HOST}:${HTTP_PORT}"
  fi
}

setup_install_inputs() {
  resolve_router_ip_from_system
  prompt_value ROUTER_IP "路由器 LAN IP" "$ROUTER_IP"
  prompt_value HTTP_PORT "面板端口" "8088"
  prompt_value DASHBOARD_PASSWORD "面板密码" "admin123456" 1

  if [ -z "$LISTEN_ADDR" ]; then
    LISTEN_ADDR="${ROUTER_IP}:${HTTP_PORT}"
  fi
}

verify_http_online() {
  target_ip="$1"
  home_code=$(curl -sS -m 8 -o /dev/null -w '%{http_code}' "http://$target_ip:$HTTP_PORT/")
  api_code=$(curl -sS -m 8 -o /dev/null -w '%{http_code}' "http://$target_ip:$HTTP_PORT/cgi-bin/xiaomi-dnsmasq-gui_api?action=auth_status")

  log "首页状态码: ${home_code}"
  log "API 状态码: ${api_code}"

  [ "$home_code" = "200" ] || die "首页访问失败"
  [ "$api_code" = "200" ] || die "API 访问失败"
}

apply_cron_local() {
  crontab_file=/etc/crontabs/root
  grep -qF "$REMOTE_DIR/ensure.sh" "$crontab_file" || echo "* * * * * APP_DIR=$REMOTE_DIR START_SCRIPT=$REMOTE_DIR/start.sh $REMOTE_DIR/ensure.sh" >> "$crontab_file"
  grep -qF "$REMOTE_DIR/start.sh" "$crontab_file" || echo "@reboot APP_DIR=$REMOTE_DIR XIAOMI_DNSMASQ_GUI_LISTEN_ADDR=$LISTEN_ADDR $REMOTE_DIR/start.sh" >> "$crontab_file"
  /etc/init.d/cron restart >/dev/null 2>&1 || true
}

deploy_mode() {
  require_cmd sshpass
  require_cmd ssh
  require_cmd scp
  require_cmd curl

  START_FILE="$SCRIPT_DIR/start.sh"
  ENSURE_FILE="$SCRIPT_DIR/ensure.sh"
  API_ENTRY_MAIN="$PROJECT_DIR/cmd/api/main.go"
  PREBUILT_BIN="$SCRIPT_DIR/xiaomi-dnsmasq-gui"
  LOCAL_BUILD_DIR="$PROJECT_DIR/build"
  LOCAL_BIN="$LOCAL_BUILD_DIR/xiaomi-dnsmasq-gui_${TARGET_GOOS}_${TARGET_GOARCH}"
  BUILD_MODE="source"

  [ -f "$START_FILE" ] || die "缺少文件：$START_FILE"
  [ -f "$ENSURE_FILE" ] || die "缺少文件：$ENSURE_FILE"

  parse_args "$@"
  setup_deploy_inputs

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

  log "开始部署到 $ROUTER_USER@$ROUTER_HOST:$REMOTE_DIR"

  if [ "$BUILD_MODE" = "source" ]; then
    require_cmd go
    log "1/7 本地编译 Go 程序（${TARGET_GOOS}/${TARGET_GOARCH}）"
    mkdir -p "$LOCAL_BUILD_DIR" "$GOCACHE_DIR"
    (
      cd "$PROJECT_DIR"
      GOOS="$TARGET_GOOS" GOARCH="$TARGET_GOARCH" CGO_ENABLED=0 GOCACHE="$GOCACHE_DIR" \
        go build -o "$LOCAL_BIN" ./cmd/api
    )
  elif [ "$BUILD_MODE" = "prebuilt" ]; then
    log "1/7 使用脚本同目录预编译二进制：$LOCAL_BIN"
  else
    log "1/7 使用指定本地二进制：$LOCAL_BIN"
  fi

  log "2/7 创建远端目录"
  ssh_run "set -e; mkdir -p '$REMOTE_DIR'"

  log "3/7 上传程序与启动脚本"
  scp_send "$LOCAL_BIN" "$ROUTER_USER@$ROUTER_HOST:$REMOTE_DIR/xiaomi-dnsmasq-gui.new"
  scp_send "$START_FILE" "$ROUTER_USER@$ROUTER_HOST:$REMOTE_DIR/start.sh"
  scp_send "$ENSURE_FILE" "$ROUTER_USER@$ROUTER_HOST:$REMOTE_DIR/ensure.sh"

  log "4/7 写入管理页密码"
  AUTH_TMP_FILE=$(mktemp)
  printf 'password=%s\n' "$DASHBOARD_PASSWORD" > "$AUTH_TMP_FILE"
  scp_send "$AUTH_TMP_FILE" "$ROUTER_USER@$ROUTER_HOST:$REMOTE_DIR/auth.conf"
  rm -f "$AUTH_TMP_FILE"

  log "5/7 设置权限并重启服务"
  ssh_run "set -e; \
    mv -f '$REMOTE_DIR/xiaomi-dnsmasq-gui.new' '$REMOTE_DIR/xiaomi-dnsmasq-gui'; \
    chmod +x '$REMOTE_DIR/xiaomi-dnsmasq-gui' '$REMOTE_DIR/start.sh' '$REMOTE_DIR/ensure.sh'; \
    chmod 600 '$REMOTE_DIR/auth.conf'; \
    APP_DIR='$REMOTE_DIR' XIAOMI_DNSMASQ_GUI_LISTEN_ADDR='$LISTEN_ADDR' '$REMOTE_DIR/start.sh'"

  if [ "$ENABLE_CRON" -eq 1 ]; then
    log "6/7 写入 cron 保活策略"
    ssh_run "set -e; \
      crontab_file=/etc/crontabs/root; \
      grep -qF '$REMOTE_DIR/ensure.sh' \"\$crontab_file\" || echo '* * * * * APP_DIR=$REMOTE_DIR START_SCRIPT=$REMOTE_DIR/start.sh $REMOTE_DIR/ensure.sh' >> \"\$crontab_file\"; \
      grep -qF '$REMOTE_DIR/start.sh' \"\$crontab_file\" || echo '@reboot APP_DIR=$REMOTE_DIR XIAOMI_DNSMASQ_GUI_LISTEN_ADDR=$LISTEN_ADDR $REMOTE_DIR/start.sh' >> \"\$crontab_file\"; \
      /etc/init.d/cron restart >/dev/null 2>&1 || true"
  else
    log "6/7 跳过 cron 写入（已指定 --disable-cron）"
  fi

  if [ "$SKIP_VERIFY" -eq 1 ]; then
    warn "已跳过 HTTP 验证（--skip-verify）"
    log "部署完成"
  else
    log "7/7 在线验证"
    verify_http_online "$ROUTER_HOST"
    log "部署与验证全部成功"
  fi

  printf '访问地址: http://%s:%s\n' "$ROUTER_HOST" "$HTTP_PORT"
}

resolve_release_tag() {
  if [ "$RELEASE_TAG" != "latest" ]; then
    printf '%s' "$RELEASE_TAG"
    return
  fi

  tag=$(curl -fsSL "https://api.github.com/repos/$GITHUB_REPO/releases/latest" \
    | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' \
    | head -n 1)
  [ -n "$tag" ] || die "无法获取最新 Release 标签，请改用 --tag 指定"
  printf '%s' "$tag"
}

install_mode() {
  require_cmd curl
  require_cmd tar

  [ "$(id -u)" = "0" ] || die "install 模式需要 root 权限"

  parse_args "$@"
  setup_install_inputs

  RELEASE_REAL_TAG=$(resolve_release_tag)
  ARCHIVE_NAME="xiaomi-dnsmasq-gui_${RELEASE_REAL_TAG}_arm64.tar.gz"
  DOWNLOAD_URL="https://github.com/$GITHUB_REPO/releases/download/$RELEASE_REAL_TAG/$ARCHIVE_NAME"

  TMP_DIR=$(mktemp -d)
  PKG_DIR="$TMP_DIR/pkg"
  mkdir -p "$PKG_DIR"

  log "1/6 下载 Release 包：$DOWNLOAD_URL"
  curl -fsSL "$DOWNLOAD_URL" -o "$TMP_DIR/$ARCHIVE_NAME"

  log "2/6 解压安装包"
  tar -xzf "$TMP_DIR/$ARCHIVE_NAME" -C "$PKG_DIR"

  for f in xiaomi-dnsmasq-gui start.sh ensure.sh; do
    [ -f "$PKG_DIR/$f" ] || die "安装包缺少文件：$f"
  done

  log "3/6 安装程序与脚本到 $REMOTE_DIR"
  mkdir -p "$REMOTE_DIR"
  cp "$PKG_DIR/xiaomi-dnsmasq-gui" "$REMOTE_DIR/xiaomi-dnsmasq-gui.new"
  cp "$PKG_DIR/start.sh" "$REMOTE_DIR/start.sh"
  cp "$PKG_DIR/ensure.sh" "$REMOTE_DIR/ensure.sh"

  log "4/6 写入面板密码"
  printf 'password=%s\n' "$DASHBOARD_PASSWORD" > "$REMOTE_DIR/auth.conf"

  log "5/6 启动服务"
  mv -f "$REMOTE_DIR/xiaomi-dnsmasq-gui.new" "$REMOTE_DIR/xiaomi-dnsmasq-gui"
  chmod +x "$REMOTE_DIR/xiaomi-dnsmasq-gui" "$REMOTE_DIR/start.sh" "$REMOTE_DIR/ensure.sh"
  chmod 600 "$REMOTE_DIR/auth.conf"
  APP_DIR="$REMOTE_DIR" XIAOMI_DNSMASQ_GUI_LISTEN_ADDR="$LISTEN_ADDR" "$REMOTE_DIR/start.sh"

  if [ "$ENABLE_CRON" -eq 1 ]; then
    log "附加：写入 cron 保活"
    apply_cron_local
  else
    log "附加：跳过 cron 写入（已指定 --disable-cron）"
  fi

  if [ "$SKIP_VERIFY" -eq 1 ]; then
    warn "已跳过 HTTP 验证（--skip-verify）"
    log "安装完成"
  else
    log "6/6 在线验证"
    verify_http_online "$ROUTER_IP"
    log "安装与验证全部成功"
  fi

  rm -rf "$TMP_DIR"
  printf '访问地址: http://%s:%s\n' "$ROUTER_IP" "$HTTP_PORT"
}

uninstall_mode() {
  require_cmd sshpass
  require_cmd ssh

  parse_args "$@"

  prompt_value ROUTER_HOST "路由器 IP" "10.0.0.1"
  prompt_value ROUTER_PORT "SSH 端口" "22"
  prompt_value ROUTER_USER "SSH 账号" "root"
  prompt_value ROUTER_PASS "SSH 密码" "" 1

  validate_remote_dir
  confirm_or_exit

  log "开始卸载：$ROUTER_USER@$ROUTER_HOST:$REMOTE_DIR"

  sshpass -p "$ROUTER_PASS" ssh \
    -o StrictHostKeyChecking=no \
    -o UserKnownHostsFile=/dev/null \
    -p "$ROUTER_PORT" \
    "$ROUTER_USER@$ROUTER_HOST" \
    "APP_DIR='$REMOTE_DIR' sh -s" <<'REMOTE_SH'
set -eu

app_dir="${APP_DIR:-/data/xiaomi-dnsmasq-gui}"

case "$app_dir" in
  ""|"/")
    echo "[ERR ] APP_DIR 非法: $app_dir" >&2
    exit 1
    ;;
esac

pid_file="$app_dir/xiaomi-dnsmasq-gui.pid"
if [ -f "$pid_file" ]; then
  pid="$(cat "$pid_file" 2>/dev/null || true)"
  if [ -n "$pid" ] && kill -0 "$pid" >/dev/null 2>&1; then
    kill "$pid" >/dev/null 2>&1 || true
    sleep 1
  fi
fi

pkill -f "$app_dir/xiaomi-dnsmasq-gui" >/dev/null 2>&1 || true

crontab_file="/etc/crontabs/root"
if [ -f "$crontab_file" ]; then
  tmp_file="$(mktemp)"
  grep -vF "$app_dir/ensure.sh" "$crontab_file" | grep -vF "$app_dir/start.sh" > "$tmp_file" || true
  cat "$tmp_file" > "$crontab_file"
  rm -f "$tmp_file"
  /etc/init.d/cron restart >/dev/null 2>&1 || true
fi

rm -rf "$app_dir"
REMOTE_SH

  log "卸载完成"
}

uninstall_local_mode() {
  parse_args "$@"

  [ "$(id -u)" = "0" ] || die "uninstall-local 模式需要 root 权限"

  validate_remote_dir
  confirm_or_exit

  log "开始本机卸载：$REMOTE_DIR"
  cleanup_local_runtime "$REMOTE_DIR"
  log "卸载完成"
}

case "$MODE" in
  deploy)
    deploy_mode "$@"
    ;;
  install)
    install_mode "$@"
    ;;
  uninstall)
    uninstall_mode "$@"
    ;;
  uninstall-local)
    uninstall_local_mode "$@"
    ;;
  *)
    usage
    die "未知模式：$MODE"
    ;;
esac
