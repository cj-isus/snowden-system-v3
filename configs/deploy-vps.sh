#!/usr/bin/env bash
# deploy-vps.sh — подстановка секретов в серверный шаблон и деплой на VPS.
# Секреты читаются из secrets/channels/ (gitignored), в git и stdout не попадают.
#
# Отличия от старого скрипта (docs/old-core-review.md):
#   - аутентификация по ключу (vps.env: VPS_SSH_KEY_PATH), не по паролю;
#   - post-deploy проверка включена: sing-box check + route.final на сервере;
#   - restart по-прежнему НЕ автоматизирован (не рвать живой туннель).
#
# Использование: bash configs/deploy-vps.sh
# Параметры окружения: VPS_HOST, VPS_PORT, VPS_USER, VPS_SSH_KEY_PATH (vps.env),
# REMOTE_CONFIG_PATH (по умолчанию /etc/sing-box/config.json).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# --- параметры (vps.env, если задан) ---
if [[ -f "$REPO_ROOT/vps.env" ]]; then
  # shellcheck disable=SC1091
  set -a; source "$REPO_ROOT/vps.env"; set +a
fi
VPS_HOST="${VPS_HOST:-203.0.113.10}"
VPS_PORT="${VPS_PORT:-22}"
VPS_USER="${VPS_USER:-root}"
VPS_SSH_KEY_PATH="${VPS_SSH_KEY_PATH:-$HOME/.ssh/vps_key}"
REMOTE_CONFIG_PATH="${REMOTE_CONFIG_PATH:-/etc/sing-box/config.json}"

SECRETS_DIR="$REPO_ROOT/secrets/channels"
TEMPLATE="$REPO_ROOT/configs/singbox/server-combined.json"
WS_PATH="${WS_PATH:-/ws}"
ORIGIN_CERT_PATH="${ORIGIN_CERT_PATH:-/etc/ssl/vpn.example.com.crt}"
ORIGIN_KEY_PATH="${ORIGIN_KEY_PATH:-/etc/ssl/vpn.example.com.key}"

# --- secrets: файлы существуют и непусты ---
missing=()
for f in vless_uuid hy2_password hy2_obfs_password; do
  [[ -s "$SECRETS_DIR/$f" ]] || missing+=("$SECRETS_DIR/$f")
done
if [[ ${#missing[@]} -gt 0 ]]; then
  echo "ERROR: пустые/отсутствующие файлы секретов:" >&2
  printf '  %s\n' "${missing[@]}" >&2
  exit 1
fi

UUID="$(tr -d '\r\n' < "$SECRETS_DIR/vless_uuid")"
HY2_PASS="$(tr -d '\r\n' < "$SECRETS_DIR/hy2_password")"
OBFS_PASS="$(tr -d '\r\n' < "$SECRETS_DIR/hy2_obfs_password")"

# --- temp внутри gitignored-каталога (урок старого проекта) ---
TMPDIR_DEPLOY="$REPO_ROOT/secrets/.deploy-tmp-$$"
mkdir -p "$TMPDIR_DEPLOY"
trap 'rm -rf "$TMPDIR_DEPLOY" 2>/dev/null || true' EXIT

# --- подстановка ({{PLACEHOLDER}} из нового шаблона) ---
substitute() {
  sed \
    -e "s|{{VLESS_UUID}}|$UUID|g" \
    -e "s|{{HY2_PASSWORD}}|$HY2_PASS|g" \
    -e "s|{{HY2_OBFS_PASSWORD}}|$OBFS_PASS|g" \
    -e "s|{{WS_PATH}}|$WS_PATH|g" \
    -e "s|{{ORIGIN_CERT_PATH}}|$ORIGIN_CERT_PATH|g" \
    -e "s|{{ORIGIN_KEY_PATH}}|$ORIGIN_KEY_PATH|g" \
    "$TEMPLATE" > "$TMPDIR_DEPLOY/config.json"
}

assert_no_placeholders() {
  if grep -q '{{' "$1"; then
    echo "ERROR: остались незамещённые плейсхолдеры — деплой отменён:" >&2
    grep -o '{{[A-Z_]*}}' "$1" | sort -u >&2
    exit 1
  fi
}

substitute
assert_no_placeholders "$TMPDIR_DEPLOY/config.json"

# --- деплой ---
SSH_OPTS=(-i "$VPS_SSH_KEY_PATH" -p "$VPS_PORT" -o BatchMode=yes -o ConnectTimeout=10)
echo "Деплой -> $VPS_USER@$VPS_HOST:$REMOTE_CONFIG_PATH"
scp "${SSH_OPTS[@]}" "$TMPDIR_DEPLOY/config.json" "$VPS_USER@$VPS_HOST:$REMOTE_CONFIG_PATH"

# --- post-deploy проверки (fail-closed: битый конфиг не должен ждать рестарта) ---
echo "Проверка конфига на сервере..."
ssh "${SSH_OPTS[@]}" "$VPS_USER@$VPS_HOST" \
  "sing-box check -c $REMOTE_CONFIG_PATH && grep -o '\"final\": \"[a-z]*\"' $REMOTE_CONFIG_PATH"

FINAL=$(ssh "${SSH_OPTS[@]}" "$VPS_USER@$VPS_HOST" \
  "grep -o '\"final\": \"[a-z]*\"' $REMOTE_CONFIG_PATH" || true)
if [[ "$FINAL" != '"final": "direct"' ]]; then
  echo "ERROR: route.final = $FINAL (ожидался direct). Деплой НЕ завершён корректно." >&2
  exit 1
fi

cat <<EOF

Готово. Рестарт ОСОЗНАННО не автоматизирован (не рвать живой туннель).
После ручного рестарта проверь живой канал:

  ssh $VPS_USER@$VPS_HOST 'systemctl restart sing-box'
  curl --proxy socks5h://127.0.0.1:1080 https://api.ipify.org   # -> $VPS_HOST
EOF
