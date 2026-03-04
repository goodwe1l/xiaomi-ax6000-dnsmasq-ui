#!/bin/sh

set -eu

HOST="${HOST:-10.0.0.1}"
PORT="${PORT:-8088}"
BASE_URL="${BASE_URL:-}"
DASHBOARD_PASSWORD="${DASHBOARD_PASSWORD:-}"
TIMEOUT="${TIMEOUT:-8}"
CHECK_OLD_URL=1

usage() {
  cat <<'USAGE'
用法：
  ./scripts/api_regression.sh [参数]

参数：
  --host <IP>                  路由器地址，默认 10.0.0.1
  --port <PORT>                管理页端口，默认 8088
  --base-url <URL>             直接指定基础地址，如 http://10.0.0.1:8088
  --dashboard-password <PASS>  管理页登录密码（推荐显式传入）
  --no-old-url-check           跳过旧 URL 404 校验
  --timeout <SEC>              curl 超时秒数，默认 8
  -h, --help                   显示帮助

环境变量：
  HOST / PORT / BASE_URL / DASHBOARD_PASSWORD / TIMEOUT
USAGE
}

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

while [ "$#" -gt 0 ]; do
  case "$1" in
    --host)
      HOST="$2"
      shift 2
      ;;
    --port)
      PORT="$2"
      shift 2
      ;;
    --base-url)
      BASE_URL="$2"
      shift 2
      ;;
    --dashboard-password)
      DASHBOARD_PASSWORD="$2"
      shift 2
      ;;
    --timeout)
      TIMEOUT="$2"
      shift 2
      ;;
    --no-old-url-check)
      CHECK_OLD_URL=0
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

require_cmd curl
require_cmd sed
require_cmd grep
require_cmd tr
require_cmd date

if [ -z "$BASE_URL" ]; then
  BASE_URL="http://$HOST:$PORT"
fi

if [ -z "$DASHBOARD_PASSWORD" ]; then
  DASHBOARD_PASSWORD="admin123456"
  log "未显式提供管理页密码，使用默认值 admin123456 进行回归"
fi

TMP_DIR="$(mktemp -d)"
COOKIE_FILE="$TMP_DIR/cookie.txt"
TAG="api_test_$(date +%s)"
MAC="02:11:22:33:44:55"
IP="10.0.0.250"
NAME="api-test-host"
GW="10.0.0.1"
DNS="9.9.9.9,1.1.1.1"
TEMPLATE_SEC=""

cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

curl_get() {
  path="$1"
  out="$2"
  curl -sS -m "$TIMEOUT" -o "$out" -w '%{http_code}' "$BASE_URL$path"
}

curl_get_cookie() {
  path="$1"
  out="$2"
  curl -sS -m "$TIMEOUT" -b "$COOKIE_FILE" -o "$out" -w '%{http_code}' "$BASE_URL$path"
}

curl_post() {
  out="$1"
  shift
  curl -sS -m "$TIMEOUT" -b "$COOKIE_FILE" -c "$COOKIE_FILE" \
    -X POST -o "$out" -w '%{http_code}' \
    "$BASE_URL/cgi-bin/dhcp_adv_api" "$@"
}

expect_http_200() {
  code="$1"
  step="$2"
  [ "$code" = "200" ] || die "$step 失败，HTTP 状态码：$code"
}

expect_contains() {
  file="$1"
  text="$2"
  step="$3"
  grep -q "$text" "$file" || {
    printf '[ERR ] %s 返回内容异常：\n' "$step" >&2
    cat "$file" >&2
    exit 1
  }
}

extract_template_sec() {
  file="$1"
  one_line="$(tr -d '\n' < "$file")"
  escaped_tag="$(printf '%s' "$TAG" | sed 's/[.[\*^$()+?{|]/\\&/g')"
  printf '%s' "$one_line" | sed -n "s/.*\"sec\":\"\([^\"]*\)\",\"tag\":\"$escaped_tag\".*/\1/p"
}

log "1/10 检查首页可访问"
code="$(curl_get '/' "$TMP_DIR/home.json")"
expect_http_200 "$code" "首页访问"

log "2/10 未登录读取 auth_status"
code="$(curl_get '/cgi-bin/dhcp_adv_api?action=auth_status' "$TMP_DIR/auth_status.json")"
expect_http_200 "$code" "auth_status"
expect_contains "$TMP_DIR/auth_status.json" '"ok":true' "auth_status"

log "3/10 未登录读取 get_state（应返回 UNAUTHORIZED）"
code="$(curl_get '/cgi-bin/dhcp_adv_api?action=get_state' "$TMP_DIR/get_state_unauth.json")"
expect_http_200 "$code" "未登录 get_state"
expect_contains "$TMP_DIR/get_state_unauth.json" '"code":"UNAUTHORIZED"' "未登录 get_state"

log "4/10 登录"
code="$(curl_post "$TMP_DIR/login.json" \
  --data-urlencode 'action=login' \
  --data-urlencode "password=$DASHBOARD_PASSWORD")"
expect_http_200 "$code" "登录"
expect_contains "$TMP_DIR/login.json" '"ok":true' "登录"

log "5/10 新增临时模板"
code="$(curl_post "$TMP_DIR/template_upsert.json" \
  --data-urlencode 'action=template_upsert' \
  --data-urlencode "template_tag=$TAG" \
  --data-urlencode "template_gateway=$GW" \
  --data-urlencode "template_dns=$DNS")"
expect_http_200 "$code" "模板新增"
expect_contains "$TMP_DIR/template_upsert.json" '"ok":true' "模板新增"

code="$(curl_get_cookie '/cgi-bin/dhcp_adv_api?action=get_state' "$TMP_DIR/state_after_tpl_add.json")"
expect_http_200 "$code" "读取状态（模板新增后）"
TEMPLATE_SEC="$(extract_template_sec "$TMP_DIR/state_after_tpl_add.json")"
[ -n "$TEMPLATE_SEC" ] || die "无法从 get_state 提取模板 section"

log "6/10 新增临时静态租约"
code="$(curl_post "$TMP_DIR/lease_upsert.json" \
  --data-urlencode 'action=lease_upsert' \
  --data-urlencode "name=$NAME" \
  --data-urlencode "mac=$MAC" \
  --data-urlencode "ip=$IP" \
  --data-urlencode 'gateway=' \
  --data-urlencode 'dns=' \
  --data-urlencode "tag=$TAG")"
expect_http_200 "$code" "租约新增"
expect_contains "$TMP_DIR/lease_upsert.json" '"ok":true' "租约新增"

code="$(curl_get_cookie '/cgi-bin/dhcp_adv_api?action=get_state' "$TMP_DIR/state_after_lease_add.json")"
expect_http_200 "$code" "读取状态（租约新增后）"
expect_contains "$TMP_DIR/state_after_lease_add.json" "$MAC" "租约新增后状态检查"

log "7/10 删除临时静态租约"
code="$(curl_post "$TMP_DIR/lease_delete.json" \
  --data-urlencode 'action=lease_delete' \
  --data-urlencode "mac=$MAC")"
expect_http_200 "$code" "租约删除"
expect_contains "$TMP_DIR/lease_delete.json" '"ok":true' "租约删除"

code="$(curl_get_cookie '/cgi-bin/dhcp_adv_api?action=get_state' "$TMP_DIR/state_after_lease_del.json")"
expect_http_200 "$code" "读取状态（租约删除后）"
if grep -q "$MAC" "$TMP_DIR/state_after_lease_del.json"; then
  die "租约删除后仍然存在：$MAC"
fi

log "8/10 删除临时模板"
code="$(curl_post "$TMP_DIR/template_delete.json" \
  --data-urlencode 'action=template_delete' \
  --data-urlencode "template_sec=$TEMPLATE_SEC")"
expect_http_200 "$code" "模板删除"
expect_contains "$TMP_DIR/template_delete.json" '"ok":true' "模板删除"

log "9/10 登出并校验未登录态"
code="$(curl_post "$TMP_DIR/logout.json" --data-urlencode 'action=logout')"
expect_http_200 "$code" "登出"
expect_contains "$TMP_DIR/logout.json" '"ok":true' "登出"

rm -f "$COOKIE_FILE"
code="$(curl_get '/cgi-bin/dhcp_adv_api?action=get_state' "$TMP_DIR/get_state_unauth_after_logout.json")"
expect_http_200 "$code" "登出后未登录 get_state"
expect_contains "$TMP_DIR/get_state_unauth_after_logout.json" '"code":"UNAUTHORIZED"' "登出后未登录 get_state"

log "10/10 校验旧 URL 下线"
if [ "$CHECK_OLD_URL" -eq 1 ]; then
  old_page_code="$(curl_get '/cgi-bin/dhcp_adv.sh' "$TMP_DIR/old_page.txt")"
  [ "$old_page_code" = "404" ] || die "旧页面 URL 应为 404，实际：$old_page_code"

  old_api_code="$(curl_get '/cgi-bin/dhcp_adv_api.sh?action=auth_status' "$TMP_DIR/old_api.txt")"
  [ "$old_api_code" = "404" ] || die "旧 API URL 应为 404，实际：$old_api_code"
fi

log "回归测试通过"
