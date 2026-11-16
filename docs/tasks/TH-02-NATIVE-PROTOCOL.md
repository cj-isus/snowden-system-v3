# TH-02 — отдельный Windows-helper для DNS-наблюдений

Статус: V2-147, 2026-10-25, **исходники, unverified**, только PC/Windows.
Тесты не добавлялись/не менялись/не запускались; build/compile/vet/check/module/
generation/SDK/product CLI/helper/native/network/live не выполнялись.
`gofmt` затронутых11 Go-файлов — только форматирование, не приёмка поведения.
Android, обвязка и generated bindings не менялись.

## Назначение и текущая граница

После [V2-146](TH-01-NATIVE-HELPER.md) у приложения есть source owner для отдельного
Job. У parent/worker один EXE AppID с permit, поэтому native DNS из parent не
даёт защищённого маршрута. Добавлен минимальный **другой EXE**, без импорта VPN
engine: `windows/cmd/snowden-native-probe/`. На V2-147 App caller отсутствовал.
V2-148 [image/admission](TH-02-NATIVE-IMAGE.md), V2-149 [manual App lease/report](TH-02-NATIVE-SCOPE.md)
добавлены в исходники; product запуск/compile/native приёмка не выполнялись.

`GetNativeProbeCapabilities` — только read-only compiled facts: protocol/helper
name, наличие canonical SHA256 pin, `image_state=not_checked`,
`execution_state=manual_action` после V2-149. Метод не открывает файл, не отправляет пакеты и не
утверждает наличие helper, исправность сети или защищённость маршрута.

## Переиспользование и отдельное владение

- `backend/netprobe/dns.go` — прежний bounded wire codec/public-IPv4 predicate,
  выделенный из core. Parser остаётся declared `x/net/dns/dnsmessage` той же
  версии; module graph/суммы/зависимости не обновлялись.
- Core DoH wrapper сохраняет ID0, media type и error identities. Native caller
  передаёт отдельный ID. Exact framing/count/RDLENGTH/CNAME/A gates общие.
- `CaptureOptionalHelper` получает свою no-queue lane; protected curl lane не
  занята optional измерением. Обе используют прежние suspended spawn, original
  Job handles, limit1/128MiB **committed memory**, Max+1 stdout и полный retirement
  до выдачи bytes. Engine8 и его environment/memory contract не менялись.
- `ErrProtectedHelper*` сохранены как aliases прежним caller; общий владелец и
  сообщения не объявляют optional helper защищённым.

Отдельная lane — не diagnostic budget: будущий caller ещё обязан заранее
резервировать все попытки/request/reply объёмы, ограничивать rate и повторять
актуальность scope после admission и полного reap.

## Точный публичный протокол

Request — strict canonical raw-URL-base64 JSON≤2048 bytes, schema1, случайный
32-byte nonce,1…4 уникальных network enum, по2 уникальных uint16 wire ID на
каждый network и timeout500…12000ms. Shape проверяется до копирования/entropy
allocation. Только `udp4`, `tcp4`, `udp6`, `tcp6`; произвольных host/name/port/
payload/command/environment selectors нет. Никакие credentials не передаются.

Reply≤56KiB содержит exact schema/nonce, boolean complete и максимум8 результатов
в запрошенном network/source порядке. Пропуск/перестановка/лишний результат,
неверные endpoint/ID, неизвестные поля, duplicate/case alias, null, trailing
JSON и oversized array/body запрещены до typed acceptance. Packet — canonical
standard base64 string≤4096 decoded bytes; JSON byte arrays не принимаются.
Elapsed ограничен запросом; raw stderr/errors не публикуются.

Успех требует response stage и повторного decode исходного packet на стороне
получателя. Ошибки DNS повторно классифицируются общим кодеком; нельзя прислать
`dns_truncated` для валидного пакета или `dial_unavailable` на read stage.
Transport/cancel ошибки не несут packet. Success останавливает fallback данного
network; без success допускается ровно2 источника либо incomplete prefix.
`complete` означает завершённую batch, **не успех всех измерений**.

Nonce и query IDs связывают сообщения/попытки. Это не аутентификация DNS-сервера,
owner или независимое доказательство session/epoch. Итоговый caller ещё должен
отклонять incomplete/stale reply и не повышать отдельные результаты до aggregate
VPN health. Сам wire decoder не проверяет DNSSEC.

## Фиксированные отправители и бюджет

Единственный query — A/IN `example.com.`. Для IPv4 используются1.1.1.1 и8.8.8.8,
для IPv6 —2606:4700:4700::1111 и2001:4860:4860::8888, всегда port53. Адреса
сверены с [Cloudflare](https://developers.cloudflare.com/1.1.1.1/ip-addresses/)
и [Google](https://developers.google.com/speed/public-dns/docs/using).
Это внешние measurement dependencies: разные операторы не доказывают независимый
path/ASN/account/quorum, а outage не доказывает блокировку VPN.

Helper использует literal `net.Dialer` endpoints; OS resolver, HTTP/environment
proxy и direct fallback в нём не выбираются. TCP — exact two-byte length prefix
и bounded ReadFull; UDP — один ответ с Max+1 detector. После ошибочного ответа
может быть только следующая заранее перечисленная попытка, не unbounded retry.
Один context охватывает всю batch. Каждая последовательная попытка получает
не более2s и доли оставшегося общего срока; cancel закрывает connection.
Это cooperative deadline, не доказанный hard OS/loader/cleanup/RSS/CPU bound.
Если elapsed выходит за protocol bound, report не кодируется как валидный.

На Windows entrypoint требует exact arguments, pipe stdin/stdout и membership
в Job. На другой ОС guard возвращает false до сетевого действия. Pipe/Job guard
не аутентифицирует локального вызывающего: другой локальный процесс тоже может
создать Job/pipes. Нельзя считать его заменой image pin, ACL или route lease.

A-ответ через UDP6/TCP6 даёт наблюдение DNS transport поверх IPv6 к конкретному
resolver. Он не доказывает AAAA resolution, IPv6 egress, QUIC/HY2, MTU/bulk или
отсутствие утечек. Сырой port53 — plaintext; отсутствие DNSSEC/endpoint trust
и зависимость fixed query должны оставаться видимыми в будущих result facts.

## Сценарий поставки — тоже исходники

`tools/package.ps1` сначала собирает helper вне Wails build/bin, рассчитывает его
SHA256 и передаёт `-X main.nativeProbeHelperSHA256` приложению. Оба target явно
Windows/amd64, как текущий wintun package; helper CGO=0, task-specific предыдущие
GOOS/GOARCH/CGO восстанавливаются до Wails и при ошибке. Wails
[`-platform`](https://wails.io/docs/reference/cli/) задаёт target явно.
Перед копированием сверяется staging SHA, после — destination SHA.
Custom source NSIS включает sibling helper и exact uninstall entry.

Dev/default build без pin остаётся unavailable. Pin — утверждение producer о
своей паре bytes, **не реализованная проверка файла при запуске**. Сценарии
package/NSIS не выполнялись. Существующие проверки внутри script не запускались.
Нынешний installer ещё использует process-name taskkill; это не owned shutdown.
Atomic pair update/rollback/repair/uninstall и гарантия совместимости старого
updater с новым sibling helper открыты; нельзя продвигать это как готовый release.

## Следующий обязательный срез

V2-148 [image owner/wrapper](TH-02-NATIVE-IMAGE.md) добавлен source-only: retained
handles/ACL/hash/PE/suspended process path и обязательный caller guard. Raw generic
`CaptureOptionalHelper` API удалён; `CaptureNativeDNS` сам резервирует optional
budget. На V2-148 реального App scope/action ещё не было; V2-149 source manual
scope/action/report описан [отдельно](TH-02-NATIVE-SCOPE.md). OS/file/protocol
и App/Wails/runtime приёмка не выполнена.
Список ниже остаётся полным контрактом; наличие source не означает закрытые gates.

1. Owned pinned-image validation/lock: bounded regular local sibling, path/reparse/
   ACL/AppID distinction, exact SHA/architecture, retained handle и запрет замены
   между проверкой и spawn. Empty/mismatch/uncertainty → unavailable.
2. Перед отправкой получить полный intent/config/selector/worker/LUID/network
   epoch/clock lease и optional admission. После capture/reap повторить все fences;
   изменения/cancel/manual Stop не публикуют successor facts или protected PASS.
3. Связать bounded typed family facts с существующим report/API, без raw packets
   в UI/log, без mandatory-gate downgrade или causal DNS/DPI/health claims.
4. До выпуска закрыть pair lifetime/update/rollback и после разрешения владельца
   выполнить отложенную compile/protocol/Win32/WFP/IPv4/IPv6/DNS/clock/race/resource/
   crash/native/release приёмку. Сейчас выполнение запрещено.

Все TH-00…08/P0, independent quorum, full bounded adaptation, owner custody/DR,
client/bindings/migration/rollout и comparative gates остаются открыты.
