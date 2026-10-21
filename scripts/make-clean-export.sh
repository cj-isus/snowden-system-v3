#!/usr/bin/env bash
# make-clean-export.sh — обновить публичную чистую копию (V2-048).
#
# Что делает:
#   1. Копирует рабочее дерево в .tmp/clean-export (без секретов/артефактов);
#   2. Заменяет инфраструктурные идентификаторы плейсхолдерами (RFC 2606/5737);
#   3. Прогоняет leak-scan по списку реальных значений;
#   4. Собирает и тестирует копию (go build/vet/test);
#   5. Коммитит и пушит в origin (github.com/cj-isus/snowden-system-v3).
#
# Использование:
#   bash scripts/make-clean-export.sh            # обновить + закоммитить
#   bash scripts/make-clean-export.sh --push     # + push в origin
#   bash scripts/make-clean-export.sh --check    # только сборка+scan, без commit
#
# Секреты берутся из локальных файлов (vps.env, .tmp/reality.env,
# secrets/channels/) и НИКОГДА не попадают в копию — только в scan-список.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DST="$ROOT/.tmp/clean-export"
PUSH=0; CHECK=0
for a in "$@"; do
  case "$a" in
    --push) PUSH=1 ;;
    --check) CHECK=1 ;;
    *) echo "unknown flag: $a" >&2; exit 1 ;;
  esac
done

cd "$ROOT"

# ---------- 1. Копия ----------
echo "== [1/5] копия дерева =="
# .git сохраняем между запусками: история экспорта не должна пересоздаваться
if [ -d "$DST/.git" ]; then
  find "$DST" -mindepth 1 -maxdepth 1 -not -name '.git' -exec rm -rf {} +
else
  rm -rf "$DST"
  mkdir -p "$DST"
fi
tar -cf - \
  --exclude='./old_snowden' \
  --exclude='./.tmp' \
  --exclude='./secrets' \
  --exclude='./vps.env' \
  --exclude='./.freebuff' \
  --exclude='./scripts' \
  --exclude='./windows/frontend/node_modules' \
  --exclude='./windows/frontend/dist' \
  --exclude='./windows/build/bin' \
  --exclude='*.log' \
  --exclude='*.db' \
  --exclude='*.exe' \
  --exclude='*.dll' \
  . | tar -xf - -C "$DST"
rm -rf "$DST/windows/build/windows/installer/tmp"
mkdir -p "$DST/windows/frontend/dist" && touch "$DST/windows/frontend/dist/.gitkeep"

# ---------- 2. Подстановки ----------
echo "== [2/5] санитизация =="
VUUID="$(tr -d '\r\n' < "$ROOT/secrets/channels/vless_uuid")"
H2P="$(tr -d '\r\n' < "$ROOT/secrets/channels/hy2_password")"
OBF="$(tr -d '\r\n' < "$ROOT/secrets/channels/hy2_obfs_password")"
CF="$(grep -oE '^CF_API_TOKEN=.*' "$ROOT/vps.env" | cut -d= -f2- | tr -d '\r\n')"
RPRIV="$(grep -oE '^REALITY_PRIVATE_KEY=.*' "$ROOT/.tmp/reality.env" 2>/dev/null | cut -d= -f2- | tr -d '\r\n"')"
RPUB="$(grep -oE '^REALITY_PUBLIC_KEY=.*' "$ROOT/.tmp/reality.env" 2>/dev/null | cut -d= -f2- | tr -d '\r\n"')"
UC="$(grep -oE '^UUID_C=.*' "$ROOT/.tmp/reality.env" 2>/dev/null | cut -d= -f2- | tr -d '\r\n"')"
RSID="$(grep -oE '^REALITY_SHORT_ID=.*' "$ROOT/.tmp/reality.env" 2>/dev/null | cut -d= -f2- | tr -d '\r\n"')"

FILES="$(cd "$DST" && find . -type f \( -name '*.go' -o -name '*.md' -o -name '*.json' -o -name '*.sh' -o -name '*.ps1' -o -name '*.vue' -o -name '*.ts' -o -name '*.js' -o -name '*.css' -o -name '*.nsi' -o -name '*.nsh' -o -name '*.html' -o -name '*.sum' \))"
sub() { local val="$1" rep="$2"; [ -n "$val" ] || return 0; (cd "$DST" && printf '%s\n' "$FILES" | xargs -d '\n' sed -i "s|$val|$rep|g" 2>/dev/null); }
sub "203.0.113.10" "203.0.113.10"
sub "vpn.example.com" "vpn.example.com"
sub "legacy-zone" "legacy-zone" # включает и legacy-zone.dpdns.org, и упоминания в тексте
sub "192.0.2.44" "192.0.2.44"
sub "192.0.2.55" "192.0.2.55"
sub "192.0.2.66"    "192.0.2.66"
sub "$VUUID" "00000000-0000-4000-8000-00000000000a"
sub "$UC"    "00000000-0000-4000-8000-00000000000c"
sub "$RPUB"  "$(printf 'A%.0s' $(seq 1 43))"
sub "$RSID"  "0123456c"
sub "$RPRIV" "$(printf 'B%.0s' $(seq 1 43))"
sub "$CF"    "REDACTED-CF-TOKEN"
sub "00000000000000000000000000000000" "00000000000000000000000000000000"
sub "0000000000000000" "0000000000000000"
#anylеftover real-looking secrets in channel files would appear here:
sub "$H2P" "REDACTED-HY2-PASSWORD"
sub "$OBF" "REDACTED-HY2-OBFS"

# ---------- 3. Leak-scan ----------
echo "== [3/5] leak-scan =="
leak=0
while IFS= read -r v; do
  [ -n "$v" ] || continue
  hits=$(cd "$DST" && printf '%s\n' "$FILES" | xargs -d '\n' grep -l -F -- "$v" 2>/dev/null || true)
  if [ -n "$hits" ]; then echo "LEAK FOUND (${v:0:8}...):" >&2; echo "$hits" >&2; leak=1; fi
done <<VALUES
$VUUID
$H2P
$OBF
$CF
$RPRIV
$RPUB
$UC
$RSID
203.0.113.10
vpn.example.com
legacy-zone
192.0.2.44
192.0.2.55
192.0.2.66
00000000000000000000000000000000
0000000000000000
VALUES
[ "$leak" = "0" ] || { echo "ABORT: leaked values found" >&2; exit 1; }
echo "scan clean"

# ---------- 4. Сборка/тесты ----------
echo "== [4/5] go build/vet/test в копии =="
( cd "$DST/windows" && go build ./... && go vet ./... && go test ./... >/dev/null && echo "tests ok" )

[ "$CHECK" = "1" ] && { echo "== check-only режим: готово =="; exit 0; }

# ---------- 5. Commit/push ----------
echo "== [5/5] commit =="
cd "$DST"
if [ ! -d .git ]; then
  git init -q -b main
  git remote add origin https://github.com/cj-isus/snowden-system-v3.git
  printf '* text=auto eol=lf\n*.png binary\n*.ico binary\n' > .gitattributes
  git add -A
  git -c user.name="SNOWDEN-SYSTEM" -c user.email="owner@local" \
    commit -q -m "Начальный коммит: VPN-клиент Windows (Wails + sing-box), sanitized"
fi
git add -A
if git diff --cached --quiet; then
  echo "нет изменений с прошлым экспортом — коммит не нужен"
else
  git -c user.name="SNOWDEN-SYSTEM" -c user.email="owner@local" \
    commit -q -m "Sync: обновление из рабочего дерева ($(date +%F)), sanitized"
fi
if [ "$PUSH" = "1" ]; then
  git push -q origin main && echo "pushed to origin/main"
fi
echo "== готово: $DST =="
