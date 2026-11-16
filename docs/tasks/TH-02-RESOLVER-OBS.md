# TH-02 — ограниченные DoH wire-наблюдения с резервным источником

Статус: V2-145, 2026-10-24, **исходники, unverified**, PC/Windows only.
Тесты не добавлялись/не менялись/не запускались. Compile/build/vet/check/module/
generation/SDK/codec/product CLI/browser/native/live не выполнялись. `gofmt -w`
одиннадцати Go-файлов завершился exit0; протокол, сеть и скорость этим не проверены.
Реальные DNS/DoH-запросы к измерителям не отправлялись; прочитана только документация.

## Формат и переиспользование

Один обязательный resolver step раньше зависел от фиксированного Cloudflare
JSON API. [Cloudflare описывает отсутствие общего стандарта JSON и рекомендует
wire-format для критичных случаев](https://developers.cloudflare.com/1.1.1.1/encryption/dns-over-https/make-api-requests/dns-json/).
Использован DNS wire через HTTPS POST с `application/dns-message`; JSON fallback
не добавлен. HTTP exchange коррелирует запрос/ответ, DNS ID выбран равным нулю;
это реализация ограниченного probe, не полный рекурсивный DNS-клиент.
[RFC8484](https://www.rfc-editor.org/rfc/rfc8484.html).

Переиспользован `golang.org/x/net/dns/dnsmessage`: версия0.54.0 уже закреплена
в go.mod как declared dependency, изменена только классификация indirect→direct. Ни версии, ни go.sum
не обновлялись; module tooling/скачивание/установка не запускались. Прочитаны
локальные cache-исходники Message/Pack/Unpack/resource codecs с этим именем версии;
module verification и совместимость компиляции остаются непроверенными.

## Источники и владение

`Channel.dns_probe_targets` — необязательные2…3 distinct-host HTTPS URL. Empty
сохраняет compiled defaults: прежний `https://1.1.1.1/dns-query` и дополнительный
`https://dns.google/dns-query`. Wire POST документирован у
[Cloudflare](https://developers.cloudflare.com/1.1.1.1/encryption/dns-over-https/make-api-requests/dns-wireformat/)
и [Google](https://developers.google.com/speed/public-dns/docs/doh).
Разные hostname не доказывают независимость ASN/аккаунта/маршрута; quorum этим
не появился. Оба источника остаются внешними measurement dependencies.

Validator ограничивает URL≤2048, ASCII DNS/exact IP, порт1…65535 и distinct
normalized hosts. Credentials/query/fragment/zone/opaque/ambiguous hosts запрещены;
другие port/path на одном host не становятся вторым источником. Неверный explicit
список не заменяется defaults. Новое поле omitted при legacy nil/empty; новые
подписанные payloads требуют migration старых strict decoders до rollout.

Channel.Clone → selected runtime config → ClientConfig.Clone → App probe → Run
передают owned список. В существующем dependency graph добавлены фактические
resolver measurement endpoints с configured/compiled provenance. Это не изменение
настроек системного DNS, TUN/WFP или связи resolver endpoint с обычным DNS потоком.

## Единый бюджет

Shared DiagnosticProtected admission резервирует все возможные HTTP попытки,
response Max+1 и до512 bytes DNS query на каждую resolver попытку до сети.
По-прежнему максимум8 egress targets,3 resolver источника и1 OS control, один
общий timeout≤30s. Resolver источники идут последовательно в одной существующей
goroutine; каждый child context получает долю оставшегося срока, затем остаток
переходит следующему. У первого источника нет права занять весь срок второго.
После cancellation новые попытки не начинают; child contexts освобождаются.

Это cooperative application budget, не измеренный hard RSS/CPU/OS/latency bound.
Response headers/TLS/kernel/retransmissions в payload budget не превращаются.
SOCKS5h client/no redirects/no environment proxy/ordinary TLS verification
сохранены. DNS step не открывает direct HTTP, UDP или OS resolver fallback.

## Корреляция и границы парсинга

Требуются HTTP200, правильный media type, отсутствие unsupported Content-Encoding,
полный body≤4KiB. Header counts ограничены до decode: ровно1 question, не более
32 answer/authority/additional records в каждой секции. Exact frame не допускает
trailing data, неверный RDLENGTH fixed/name-only codecs или выход за body.
Name/pointer/resource decoding остаётся в существующем dnsmessage, не в новом
самодельном DNS parser. Frame helper только задаёт дополнительные bounds.

Response QR/opcode/ID, question name/type A/class IN должны соответствовать
константному `example.com.`. Truncated response, отсутствие recursion/NOERROR/A,
неправильный EDNS extended status/version, loop/conflicting CNAME и несвязанные
A-ответы не дают pass. Разрешённая CNAME цепочка≤8 переходов; принимаемые IPv4
должны пройти общий с preflight Go global-unicast/non-private gate. Старый
preflight не исключал multicast; predicate выделен и исправлен source-only.
Это не проверка полной IANA
special-purpose таблицы, DNSSEC или содержимого авторитетной зоны.

## Outcome и наблюдения

Успех одного разрешённого источника подтверждает **один** DoH step. Предыдущие
недоступные попытки сохраняются в fixed value slots `resolver_attempts` с
`resolver_attempt_count`≤3; unused slots не являются измерениями. Копирование
ProbeResult/Report не создаёт shared mutable slice. Основной `Observation` —
успешный источник либо последняя выполненная попытка, с redacted endpoint,
stage/bytes/completion/elapsed и типизированной ошибкой. Body/raw error не выводятся.

Обязательные≥2 HTTPS egress/consistency/expected-IP/OS-control/leak gates не
ослаблялись. Endpoint fallback не превращает неверный egress или leak в успех.
Failed endpoint attempt не добавляется как failed mandatory step рядом с успешным
resolver step; это сохраняет прежний контракт «один валидный resolver-HTTPS ответ».
Ошибка всех источников остаётся unavailable, а не причинным доказательством DPI.

## Остаток

V2-147: общий pure codec выделен в `backend/netprobe`, core wrappers сохраняют
DoH ID0/ошибки/media type. [Отдельный native helper](TH-02-NATIVE-PROTOCOL.md)
добавляет source transport/протокол, но ещё не подключён к App/session lease.
Наблюдения этого документа остаются DoH, новый native PASS ими не заявляется.

Native DNS/UDP/IPv6 и data-plane family proof, независимый quorum egress/endpoints,
shared failure domains, full controller actions/backoff, compile/race/clock/
malformed DNS/HTTP/timeout/cancel/native/UI/old-client/SDK/module/SBOM/rollout gates
не выполнены. Нет измеренного выигрыша скорости/безотказности. Все TH-00…08/P0/
OS/DR/resources/bindings/release/comparative обязательства сохраняются.

Исходники: metadata `{channel,resolver_probes}`; render `{render,dependency_graph}`;
core `{probe,probe_http_observation,probe_dns,probe_dns_wire,preflight}`; App
`{probe_observation,failure_observation}`; windows/go.mod. Тестовые/генерируемые
файлы, Android и harness не менялись.
