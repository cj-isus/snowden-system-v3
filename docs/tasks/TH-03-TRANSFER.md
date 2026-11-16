# TH-03 — публичный пакет → превью → общий Windows import

2026-10-23, V2-129. **Исходники unverified.** Тесты не добавлялись/менялись/
запускались; compile/build/vet/harness/module/Wails generator/CLI/listener/network/
native/live не выполнялись. Реальные owner keys/certs/vault/runtime config не
читались. Только ПК; Android и обвязка/их state не менялись. Предыдущая граница —
[publisher read/HTTP](TH-03-SERVING.md). Все9 этапов остаются задачей.

## Слой получения не назначает доверие

`backend/delivery.PublicationHead` стал общим wire type для publisher и reader.
Структура/порядок прежних семи JSON fields сохранены; strict bounded codec
проверяет duplicate/null/unknown/shape/hash/path. **Public current head unsigned**:
его root/candidate hash не становятся enrollment approval или client proof.

`backend/onboarding.FetchPublicProfile` получает только explicit repository,
точный profile target path и route. Current≤16KiB задаёт versioned directory;
client filename выводится как SHA256(`profile/`+target). Public package≤256KiB
должен strict-decode и содержать тот же delivery target. Signatures/pins/version/
epoch/grants/credentials проверяет общий App import, не downloader.

URL без userinfo/query/fragment/percent/backslash/traversal; bounded ASCII path,
HTTPS public-name/literal checks, explicit local loopback для HTTP. Redirects,
environment proxy и decompression отключены; identity encoding разрешён. JSON
media type/status/length/complete read проверяются, oversized/chunked stream
ограничен reader limit. Head не может перенаправить request на другой host/path.
Explicit hostname — выбор владельца; resolved DNS rebinding/address ownership
этой проверкой не доказаны, hostname resolution/bootstrap policy требуют приёмки.

Два complete selections максимум при410 retired generation: current→whole client
package, до любых SDK proof/state writes. Root/history/checkpoint не сбрасываются.
Неверный release header не подменяется latest; caller получает отказ. Ни current
hash, ни successful download не эквивалентны authenticated profile acceptance.

## Маршруты и ограничение операций

- `protected`: только SOCKS5h через endpoint реального renderer, без direct retry.
  Renderer и fetcher используют общие MixedListenAddress/Port; Manager/session/
  owner/generation проверяются до/после I/O и перед выдачей preview. Own EXE в
  applied SplitDirect запрещает protected fetch. Это configured owner evidence,
  не самостоятельный native egress/leak proof. Network epoch/DNS resolution rules,
  malicious local listener и forced cancellation остаются native gates.
- `bootstrap`: explicit HTTPS без local proxy только с `allow_direct=true` от
  отдельного UI согласия. Это запрос **открытого профиля**, не permission для
  direct fallback защищённого traffic или изменения WFP/TUN/proxy/DNS. OS routes
  не меняются и могут сами проходить через другой VPN.
- `local`: только literal loopback HTTP с explicit port, без proxy; выбирается
  отдельно, не находится автоматически. Agent никаких соединений не выполнял.

Один source operation/draft на App без очереди; context download≤30s,
per HTTP≤12s, connect5s/headers8s/TLS8s, максимум4 requests при reselection.
App draft context45s; existing common preview имеет собственные30s/синхронные
file/vault calls. Поэтому45s не hard whole-call deadline и не forced I/O bound.
Payload/copy accounting не измеряет RSS/GC/CPU; процессы/другие App не глобально
ограничены этим mutex. Новые API не меняют diagnostic channel FAIL/health.

## Одноразовый draft и повторная проверка

`PreviewPublicProfileSource(request)` явно получает mode (`profile`, `enroll`,
`recover`, `epoch_recover`, `epoch_enroll`) и independent root approval где нужен.
JSON аргумент пуст: пакет скачивает backend. Затем выполняется **existing**
`importPublicProfile(..., false)` → common SDK/application/credentials gate.
Public profile не несёт credential slots; vault/secret references должны быть
настроены отдельно, иначе common preview отвергает пакет.

Только успешное preview сохраняет один in-memory public-only draft≤256KiB:
exact package bytes, explicit intent/root approval, scalar approval snapshot,
CSPRNG nonce32 bytes и deadline подтверждения≤2min/SDK proof lifetime. Показанный
UI object не alias retained approval fields. Request ID — correlation/cancel
ownership; он не signing authority/authorization nonce. Draft не пишется на disk.

`ApplyPublicProfileSource(token, approvedTarget, confirm)` consumes nonce once,
в том числе при ошибке. Deadline проверяется в момент consumption; это срок
подтверждения, не обещание завершить filesystem/lifecycle до этой секунды.
Применяются **те же downloaded bytes**, без нового HTTP/refetch. Recovery modes
требуют отдельный exact displayed target SHA; обычный import его не получает.
Первичная установка требует SHA256 всего `root.json` по независимому каналу,
а не SSH-hostkey, HY2 certificate pin или fingerprint profile signer.

Common import повторяет весь SDK/application/epoch/profile/pins/credentials/
clock/source/vault/lifecycle/journal gate. New optional expected-preview check:

1. Exact initial policy digest сравнивается **до** собственной SDK prefix retention.
   Иначе обновление clock/cache самим apply могло бы сделать approval stale.
2. Перед подготовкой vault/commit сверяются target/signer/fingerprint/mode,
   current/bundle identity, delivery anchor/epoch grant и initial policy digest.
3. Затем сохраняются existing retirement→four-target commit→fresh Start/probe
   gates и source fences. Late verification failure может отдельно удержать
   authenticated prefix по прежнему contract, не превращая artifact в PASS.

Source request не получает нового root/role/history через unsigned routing head.
Ordinary API с nil expected preview сохраняет прежние проверки. Token не заменяет
полномочий пользователя; это binding его подтверждения к exact preview/intent.

`CancelPublicProfileSource(requestID)` отменяет только owned fetch/ready draft.
Отменённый/заменённый reply не публикует nonce. Cancellation не прерывает forcibly
неподдерживающий его common I/O и не отменяет уже явно подтверждённый apply;
Stop/Disconnect сохраняют прежнее владение lifecycle.

## UI и честная граница Wails

`PublicProfileSource.vue` расширяет existing ProfileTransfer section, без нового
menu/tab/card framework. Source/route/intent и independent approval explicit;
bootstrap требует отдельного checkbox. Preview показывает signer/anchor/точные
string epoch:counter identities и deadline, recovery target подтверждается отдельно.
Generation/request-ID guards отвергают stale replies; edits/unmount отменяют
owned draft, controls не блокируют внешнее отключение VPN. Ошибки/processing/
success/unavailable раздельны, field errors только после action, labels/help/
aria-describedby/role status/alert и focus styles. Не утверждается визуальная
или accessibility приёмка — browser/native запусков не было.

Применён local modern-web guide `accessible-error-announcement`: visual/error
state и aria-invalid возникают после действия. Новые CSS :user-invalid/browser
APIs/polyfills не требуются. Existing tokens/layout сохранены; narrow layout
сводит preview rows в один столбец. Mobile/DPI/keyboard/native gates отложены.

Facade использует optional capabilities **только generated module namespace**
через Reflect.get. В текущих generated files новых exports нет; UI честно
недоступен до нормальной генерации. `wailsjs` вручную не патчился, raw window.go
вызов/новый bridge обход generator не добавлен. Генератор требует выполнения
инструментов проекта; в текущем source-only режиме это отложено, **не PASS**.

Общий mapper старого encrypted transfer переиспользован, optional new facts
сохранены. UI предпочитает exact identity strings; unsafe JS counters не выдаются
за точные версии или0. Применение не объявляет работающий VPN/успешный probe.
Enrolled legacy export остаётся намеренно отклонён. V2-130 добавляет отдельный
[fresh issuer-proof encrypted export/recipient path](TH-03-EXPORT.md) в source,
unverified; current generated capabilities по-прежнему отсутствуют.

## Остаток

Normal binding generation + frontend/backend/native acceptance, persisted draft/
reconnect/endpoint policy refinement, issuer-proof export runtime acceptance,
durable root/archive/old-client compatibility и publisher time/storage lifecycle.
Все crypto/module/license/concurrency/ACL/reparse/crash/OS/rollout/EXE/comparative
gates TH-00…08 остаются открыты. Новые тесты не создавались и не запускались.
