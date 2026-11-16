# KA-01/W1 — допуск protected payload

V2-190. Owner: `windows`. **Source implemented, unverified**.
[KA-01](../planning/knowledge-application/PLAN.md#ka-01-независимые-p0-платформ),
TH-01 / FR-005…006. Все TH-00…08 и отложенные native gates сохраняются.

## Основание и точная граница

Текущий renderer создавал plain VLESS с detour на ShadowTLS и сначала читал UUID/
пароль. Validator освобождал VLESS с detour от собственного TLS, называя ShadowTLS
шифрующим звеном. Это дефект исходников, не наблюдение об активном профиле/сервере.

Закреплённый `sing-shadowtls v0.2.1` возвращает verifiedConn поверх исходного conn
после donor handshake ([client.go](https://github.com/SagerNet/sing-shadowtls/blob/af3279d7446641d5dd2e66cada24ce4597e98d35/client.go));
его data writer добавляет framing/HMAC к исходному payload
([v3_conn.go](https://github.com/SagerNet/sing-shadowtls/blob/af3279d7446641d5dd2e66cada24ce4597e98d35/v3_conn.go)).
Pinned VLESS подключает TLS только при заданной TLS option
([outbound.go](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/protocol/vless/outbound.go));
обычный no-flow client не добавляет cipher
([client.go](https://github.com/SagerNet/sing-vmess/blob/3aed155119a174c9290a4099841049f0cb275e02/vless/client.go)).
Этого недостаточно для утверждения protected payload confidentiality нынешней
композиции. HTTPS содержимое этим source выводом не объявляется расшифрованным.
Текущий VPS/traffic/cipher/device не исследовались; live leak не заявлен.

## Один descriptor gate до secrets

`metadata.Channel.CheckProtectedPayload` — Windows-local execution policy отдельно
от подписанной структуры. VLESS+ShadowTLS получает стабильные error/restriction
`unsupported_payload_confidentiality`. Nil для остальных комбинаций не доказывает
их доступность/шифрование: прежние schema/TLS/pin/probe/source gates обязательны.

- RuntimeProfile меняет только локальную копию Enabled и добавляет restriction;
  не меняет Envelope, подпись, ID, version/epoch/history, slots или revocations.
- Embedded/legacy/signed preview/runtime descriptor paths используют тот же
  predicate; projection копирует элементы, не меняет вход. Уже выключенный канал
  не даёт повторный disabled count. Остальные каналы сохраняют прежние gates.
- Selectable/default/failover и credential validation исключают композицию.
  Дополнительно getOrErr отказывает до Get/GetByID; прежняя небезопасная render
  branch заменена прямым отказом до UUID/password lookup и создания outbounds.
- Явный requested channel отклоняется до pins/preflight/metrics/render, без
  подмены default. Current/list ChannelView не повышает старый selector tag до
  Enabled при несовместимой capability. Wire DTO/Wails bindings не менялись.

Signed descriptor и сохранённые vault slots остаются для истории/переноса.
Обычный import может хранить предоставленные пользователем slots; это не runtime
credential lookup или разрешение попытки. Export использует прежний общий
Selectable credential collection: неактивные unsupported secrets не требуются.
Никакого разрешения forwarding по configured/live-verified label не добавлено.

## Защита raw config и actual engine boundary

Runtime validator отклоняет любой ShadowTLS node, включая невыбранный/aliased
detour. Даже вручную добавленный TLS flag не активирует неподдержанную композицию.
Прежнее detour TLS exemption удалено; protected candidates проходят обычный
verified TLS gate. Будущая inner-TLS/AEAD композиция требует отдельного согласованного
profile/server/cipher migration, а не скрытого добавления TLS слоя.

Engine Start/Reload/rollback проходят строгий config gate до worker/factory/
listeners/routes/handshake. Используется bounded owned **нормализованный** граф,
прошедший config.Parse, а не повторная трактовка caller JSON другим decoder.
Reload сохраняет прошедшие обе проверки bytes до закрытия старого owner; последующая
мутация caller slice не выбирает другой граф. Это source contract, не проверенный
canonical round-trip/API/ABI/runtime результат. Прежние owner/generation/cancel/
cleanup/probe/clock/pin/source/fixed4 guards не заменены и не снимаются ошибкой.

W3 proxy ownership/cleanup и W4 probe-only/TUN admission ещё не закрыты: этот gate
не доказывает отсутствие всех системных side effects во всём Manager cleanup.
Старые бинарники/hostile owner/kernel также не становятся ограниченными этим кодом.

## Evidence и приёмка

`.artifacts/memory/confidentiality-source-evidence.json`: exact8 Go hashes/sizes,
original7/new1 diff, source trace и primary commit pointers. Gofmt7 + failover1 +
canonical1 captured exit0, manual source review. Один v3_client raw fetch был cache
miss; вместо приписывания ему результата проверены полученные pinned client.go/
v3_conn.go и имеющийся source index. Нет нового protocol/library/dependency.

Tests/fixtures не добавлялись/менялись/запускались. Build/compile/vet/typecheck/
parser/check/SDK/crypto/native/admin/live/network/VPS не выполнялись; реальные
config/vault/credentials/certs/logs не читались. Source-only, не Native PASS.

Будущая конечная приёмка должна покрыть mixed/only-unsupported profile, zero
credential calls, explicit/manual/default/failover, disabled/revoked/expired pins,
all raw ShadowTLS references/fake inner TLS, normalized graph/caller mutation,
candidate rejection/old owner/rollback/Stop, DTO counts и unchanged signed bytes.
W2 per-role prefix, W3/W4, SDK/native/IPC/OS/durability/DR/resources/old-client/
service/release/comparative/all9 остаются открытыми; schema2 unsupported.
