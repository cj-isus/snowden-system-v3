#!/usr/bin/env bash
# verify-server.sh — post-deploy ИНВАРИАНТЫ сервера (V2-076, аудит D12/D11).
#
# Зачем: урок V2-037/§3.2 THEORY («route.final = block → трафик не идёт при
# живом «подключено»») жил только в документации и проверялся руками. Этот
# скрипт превращает инварианты в код: он НЕ копирует серверный конфиг и не
# угадывает его секреты, а проверяет наблюдаемые свойства живого сервера.
#
# Что проверяется (FAIL = выход 1, WARN не влияет на код возврата):
#   FAIL  service:   sing-box active
#   FAIL  config:    sing-box check -c <config>
#   FAIL  route:     route.final == direct (иначе трафик не проходит / режется)
#   FAIL  listeners: обязательные листенеры (по умолчанию: 443/tcp, 8443/tcp,
#                    8444/udp, 9443/tcp, 8445/tcp)
#   WARN  firewall:  ufw/nft содержит ожидаемые порты (политика может быть иной)
#   FAIL  /ip:       self-hosted egress-маркер отвечает 200
#   WARN  /range:    controlled-volume endpoint (если не развёрнут — это не
#                    отказ канала, а «диагностика недоступна»)
#
# Секреты не читаются и не печатаются. Скрипт безопасен: только чтение
# (is-active/check/ss/grep/curl), никаких restart/изменений.
#
# Использование:
#   bash configs/verify-server.sh
# Параметры (vps.env или env):
#   VPS_HOST, VPS_PORT, VPS_USER, VPS_SSH_KEY_PATH, REMOTE_CONFIG_PATH,
#   PUBLIC_DOMAIN (по умолчанию vpn.example.com),
#   EXPECT_TCP (по умолчанию "443,8443,9443,8445"), EXPECT_UDP ("8444").

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

if [[ -f "$REPO_ROOT/vps.env" ]]; then
  # shellcheck disable=SC1091
  set -a; source "$REPO_ROOT/vps.env"; set +a
fi
VPS_HOST="${VPS_HOST:-203.0.113.10}"
VPS_PORT="${VPS_PORT:-22}"
VPS_USER="${VPS_USER:-root}"
VPS_SSH_KEY_PATH="${VPS_SSH_KEY_PATH:-$HOME/.ssh/vps_key}"
REMOTE_CONFIG_PATH="${REMOTE_CONFIG_PATH:-/etc/sing-box/config.json}"
PUBLIC_DOMAIN="${PUBLIC_DOMAIN:-vpn.example.com}"
EXPECT_TCP="${EXPECT_TCP:-443,8443,9443,8445}"
EXPECT_UDP="${EXPECT_UDP:-8444}"

SSH_OPTS=(-i "$VPS_SSH_KEY_PATH" -p "$VPS_PORT" -o BatchMode=yes -o ConnectTimeout=10)
FAILED=0
WARNED=0

pass() { printf 'PASS  %-11s %s\n' "$1" "$2"; }
fail() { printf 'FAIL  %-11s %s\n' "$1" "$2"; FAILED=$((FAILED + 1)); }
warn() { printf 'WARN  %-11s %s\n' "$1" "$2"; WARNED=$((WARNED + 1)); }

remote() { ssh "${SSH_OPTS[@]}" "$VPS_USER@$VPS_HOST" "$1" 2>/dev/null; }

echo "verify-server: $VPS_USER@$VPS_HOST (config $REMOTE_CONFIG_PATH)"
echo

# --- 1. сервис -------------------------------------------------------------
state="$(remote 'systemctl is-active sing-box' || true)"
if [[ "$state" == "active" ]]; then
  pass "service" "sing-box active"
else
  fail "service" "sing-box state=$state (ожидалось active)"
fi

# --- 2. конфиг валиден -----------------------------------------------------
if remote "sing-box check -c $REMOTE_CONFIG_PATH" >/dev/null; then
  pass "config" "sing-box check ok"
else
  fail "config" "sing-box check не прошёл (конфиг отвергнут ядром)"
fi

# --- 3. route.final (ключевой инвариант) -----------------------------------
final="$(remote "grep -o '\"final\": *\"[a-z-]*\"' $REMOTE_CONFIG_PATH" | head -1 || true)"
case "$final" in
  *'"direct"'*) pass "route" "route.final=$final" ;;
  "")           warn "route" "route.final не найден через grep — проверить вручную" ;;
  *)            fail "route" "route.final=$final (ожидался direct; block/direct-mismatch режет трафик)" ;;
esac

# --- 4. листенеры ----------------------------------------------------------
tcp_listen="$(remote "ss -tlnH | awk '{print \$4}'" | tr -d '\r' || true)"
udp_listen="$(remote "ss -ulnH | awk '{print \$4}'" | tr -d '\r' || true)"

check_port() { # proto port
  local proto="$1" port="$2" list="$3"
  # ss может печатать IPv4 (0.0.0.0:443), IPv6 ([::]:443), wildcard
  # и CRLF через ssh/Windows Git Bash. Ищем границу порта, а не строковый
  # суффикс — иначе живые listeners давали ложный FAIL (V2-077 live-audit).
  if grep -qE "(^|:)${port}([[:space:]]|$)" <<<"$list"; then
    pass "listener" "$proto/$port слушает"
  else
    fail "listener" "$proto/$port НЕ слушает (ожидался по документированному baseline)"
  fi
}
IFS=',' read -r -a tcp_ports <<<"$EXPECT_TCP"
for p in "${tcp_ports[@]}"; do [[ -n "$p" ]] && check_port tcp "$p" "$tcp_listen"; done
IFS=',' read -r -a udp_ports <<<"$EXPECT_UDP"
for p in "${udp_ports[@]}"; do [[ -n "$p" ]] && check_port udp "$p" "$udp_listen"; done

# --- 5. firewall (информационно) -------------------------------------------
if remote 'command -v ufw >/dev/null && ufw status' | grep -qi 'inactive'; then
  warn "firewall" "ufw inactive — проверить, чем фильтруется вход (nft/облако)"
else
  missing_fw=""
  for p in "${tcp_ports[@]}"; do
    [[ -n "$p" ]] || continue
    if ! remote "command -v ufw >/dev/null && ufw status | grep -q '$p'"; then
      missing_fw="$missing_fw $p/tcp"
    fi
  done
  if [[ -n "$missing_fw" ]]; then
    warn "firewall" "в ufw нет правил:$missing_fw (возможно, порт открыт иначе)"
  else
    pass "firewall" "ufw содержит ожидаемые порты"
  fi
fi

# --- 6. self-hosted egress-маркер -----------------------------------------
ip_code="$(curl -sS -o /dev/null -w '%{http_code}' --max-time 15 "https://$PUBLIC_DOMAIN/ip" 2>/dev/null || true)"
if [[ "$ip_code" == "200" ]]; then
  pass "/ip" "https://$PUBLIC_DOMAIN/ip → 200 (self-hosted маркер жив)"
else
  fail "/ip" "https://$PUBLIC_DOMAIN/ip → ${ip_code:-no response} (primary probe-цель каналов)"
fi

# --- 7. /range (controlled volume) ----------------------------------------
range_code="$(curl -sS -o /dev/null -w '%{http_code}' --max-time 15 \
  -H 'Range: bytes=0-8191' "https://$PUBLIC_DOMAIN/range" 2>/dev/null || true)"
case "$range_code" in
  200|206) pass "/range" "Range-запрос → $range_code (freeze-диагностика доступна)" ;;
  404)     warn "/range" "404: endpoint не развёрнут — freeze-check честно недоступен (не отказ канала)" ;;
  *)       warn "/range" "код ${range_code:-no response} — проверить вручную" ;;
esac

echo
if [[ "$FAILED" -gt 0 ]]; then
  echo "ИТОГ: FAIL=$FAILED WARN=$WARNED — сервер НЕ соответствует инвариантам (см. выше)."
  exit 1
fi
echo "ИТОГ: инварианты выполнены (WARN=$WARNED)."
