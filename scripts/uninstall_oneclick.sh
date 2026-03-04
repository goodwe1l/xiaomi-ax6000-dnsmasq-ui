#!/bin/sh

set -eu

MODE="remote"
if [ "$#" -gt 0 ]; then
  case "$1" in
    uninstall)
      MODE="remote"
      shift
      ;;
    local)
      MODE="local"
      shift
      ;;
  esac
fi

ROUTER_HOST="${ROUTER_HOST:-}"
ROUTER_PORT="${ROUTER_PORT:-}"
ROUTER_USER="${ROUTER_USER:-}"
ROUTER_PASS="${ROUTER_PASS:-}"
REMOTE_DIR="${REMOTE_DIR:-/data/dhcp_adv}"
CONFIRM="${CONFIRM:-}"

log() {
  printf '[INFO] %s\n' "$1"
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

validate_remote_dir() {
  case "$REMOTE_DIR" in
    ""|"/")
      die "REMOTE_DIR 非法：$REMOTE_DIR"
      ;;
  esac
}

cleanup_local_runtime() {
  app_dir="$1"
  pid_file="$app_dir/dhcp_adv.pid"

  if [ -f "$pid_file" ]; then
    pid="$(cat "$pid_file" 2>/dev/null || true)"
    if [ -n "$pid" ] && kill -0 "$pid" >/dev/null 2>&1; then
      kill "$pid" >/dev/null 2>&1 || true
      sleep 1
    fi
  fi

  pkill -f "$app_dir/dhcp_adv_api" >/dev/null 2>&1 || true

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

usage() {
  cat <<'USAGE'
用法：
  ./uninstall_oneclick.sh [uninstall] [参数]
  ./uninstall_oneclick.sh local [参数]

模式：
  uninstall（默认）  在本地执行：通过 SSH 卸载路由器上的服务
  local              在路由器执行：本机直接卸载

参数：
  --host <IP>        路由器 IP（remote 模式）
  --port <PORT>      SSH 端口（remote 模式，默认 22）
  --user <USER>      SSH 账号（remote 模式，默认 root）
  --password <PASS>  SSH 密码（remote 模式）
  --remote-dir <DIR> 卸载目录，默认 /data/dhcp_adv
  --yes              跳过交互确认，直接卸载
  -h, --help         显示帮助

环境变量：
  ROUTER_HOST / ROUTER_PORT / ROUTER_USER / ROUTER_PASS / REMOTE_DIR / CONFIRM

示例：
  ./scripts/uninstall_oneclick.sh --host 10.0.0.1
  curl -fsSL https://raw.githubusercontent.com/goodwe1l/xiaomi-ax6000-dnsmasq-ui/refs/heads/main/scripts/uninstall_oneclick.sh | sh -s -- local --yes
USAGE
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
      --remote-dir)
        REMOTE_DIR="$2"
        shift 2
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

remote_uninstall() {
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

app_dir="${APP_DIR:-/data/dhcp_adv}"

case "$app_dir" in
  ""|"/")
    echo "[ERR ] APP_DIR 非法: $app_dir" >&2
    exit 1
    ;;
esac

pid_file="$app_dir/dhcp_adv.pid"
if [ -f "$pid_file" ]; then
  pid="$(cat "$pid_file" 2>/dev/null || true)"
  if [ -n "$pid" ] && kill -0 "$pid" >/dev/null 2>&1; then
    kill "$pid" >/dev/null 2>&1 || true
    sleep 1
  fi
fi

pkill -f "$app_dir/dhcp_adv_api" >/dev/null 2>&1 || true

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

local_uninstall() {
  parse_args "$@"

  [ "$(id -u)" = "0" ] || die "local 模式需要 root 权限"

  validate_remote_dir
  confirm_or_exit

  log "开始本机卸载：$REMOTE_DIR"
  cleanup_local_runtime "$REMOTE_DIR"
  log "卸载完成"
}

case "$MODE" in
  remote)
    remote_uninstall "$@"
    ;;
  local)
    local_uninstall "$@"
    ;;
  *)
    usage
    die "未知模式：$MODE"
    ;;
esac
