#!/usr/bin/env bash
# Read-only black-box check for channel A anti-probing. It never sends valid
# credentials and therefore cannot prove the protected channel; the normal
# protected probe remains the positive acceptance gate.
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
if [[ -f "$REPO_ROOT/vps.env" ]]; then
  # shellcheck disable=SC1091
  set -a; source "$REPO_ROOT/vps.env"; set +a
fi

PUBLIC_DOMAIN="${PUBLIC_DOMAIN:-vpn.example.com}"
URL="https://$PUBLIC_DOMAIN/ws"
FAILED=0

check_response() {
  local name="$1"; shift
  local headers body code server content_type
  headers="$(mktemp)"
  body="$(mktemp)"
  code="$(curl -sS --max-time 15 -D "$headers" -o "$body" -w '%{http_code}' "$@" "$URL" 2>/dev/null || true)"
  server="$(awk 'BEGIN{IGNORECASE=1} /^server:/{sub(/\r$/,""); print; exit}' "$headers")"
  content_type="$(awk 'BEGIN{IGNORECASE=1} /^content-type:/{sub(/\r$/,""); print; exit}' "$headers")"

  case "$code" in
    200|301|302|403|404) ;;
    *) echo "FAIL  $name status=${code:-no-response}"; FAILED=$((FAILED + 1)); rm -f "$headers" "$body"; return ;;
  esac
  if grep -Eqi 'sing-box|vless|websocket.*(bad|invalid)|early.?data' "$headers" "$body"; then
    echo "FAIL  $name leaked upstream/protocol fingerprint"
    FAILED=$((FAILED + 1))
    rm -f "$headers" "$body"
    return
  fi
  echo "PASS  $name status=$code ${content_type:-content-type=unknown} ${server:-server=hidden}"
  rm -f "$headers" "$body"
}

check_response "ordinary GET"
check_response "invalid WS" \
  -H 'Connection: Upgrade' \
  -H 'Upgrade: websocket' \
  -H 'Sec-WebSocket-Version: 13' \
  -H 'Sec-WebSocket-Key: AAAAAAAAAAAAAAAAAAAAAA==' \
  -H 'Sec-WebSocket-Protocol: invalid-probe'

if [[ "$FAILED" -gt 0 ]]; then
  echo "ИТОГ: anti-probing FAIL=$FAILED"
  exit 1
fi
echo "ИТОГ: неавторизованный /ws не раскрывает upstream fingerprint"
