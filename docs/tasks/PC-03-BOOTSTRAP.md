# PC-03 — bootstrap DNS, scoped cache и завершение запросов

2026-11-04, V2-208, Windows **source/unverified**. Часть полного
[PC-00…09](../planning/knowledge-application/PC-PLAN.md), продолжение
[egress/failure semantics](PC-03-EGRESS-SEMANTICS.md). Не приёмка работающего VPN.

## Найденные дефекты

Прежний `core/preflight.go` сохранял `{host: ip, at}` на семь дней без DNS TTL,
profile/endpoint/network identity. Будущий timestamp, IPv6 или непубличный адрес
из кеша проходил отдельный, более слабый gate. File read/JSON/map/write не имели
собственных byte/count/original-I/O owners. Отмена caller context разрешала
cache fallback. Resolver возвращал первый результат, не дожидаясь остальных
стратегий. JSON DoH не проверял точный DNS question/CNAME/TTL; стандартный HTTP
transport мог использовать окружение и redirect. Hostname-keyed renderer map
позволяла последнему каналу с тем же именем заменить подстановку другого.

## Изменённый источник данных

- Расширен существующий Windows `backend/netprobe/dns.go`: bounded ASCII host
  question, ID/type/class/name correlation, прежний exact DNS framing до codec,
  CNAME loop/conflict gate и минимум TTL всей использованной CNAME/A цепочки.
  Fixed protected/native measurement API сохранён wrappers, не создан второй
  DNS parser. TTL=0 используется только как свежий адрес текущего candidate,
  не сохраняется в кеш. PublicIPv4 остаётся существующим pure gate, не полной
  IANA special-purpose таблицей и не подтверждением принадлежности имени.
- UDP задаёт один вопрос фиксированному connected resolver с новым 16-bit ID.
  TC допускает один TCP повтор тому же resolver с тем же question/ID. Ответ
  TCP length-framed, 12…4096 bytes; без системного resolver fallback.
- Две фиксированные DoH стратегии используют wire POST, HTTPS verification,
  media type, status и limit+1 body gate. Нет environment proxy, redirects,
  keepalive или автоматического replay. Порт/IP/успех DNS не становятся
  доказательством работоспособности туннеля, DNSSEC или независимости маршрутов.
  Legacy JSON decoder оставлен только как source-compatible helper; runtime
  bootstrap его не вызывает.
- Время пригодного DNS ответа фиксируется до deferred Close. Медленное закрытие
  проигравших запросов не начинает TTL заново. Remaining context budget
  переводится в существующий logical-clock domain, а не сравнивается с ним
  как отдельное абсолютное wall time.

## Scope и cache

Root `bootstrap_resolver.go` получает тот же coherent DescriptorSnapshot:
profile digest (source/version/epoch/envelope/pin policy/channels), отдельный
endpoint digest и compiled `tunnel_vless_ws_tls` role. Захвачены actual App
operation/clock instance, network epoch/start и profile expiry. Current/clock
проверяются вокруг работы без App/epoch locks над сетевым или файловым I/O.

Cache key содержит profile/endpoint/role/hostname/network scope. Epoch0 не даёт
reuse. Свой случайный process namespace исключает перенос одинакового E1 между
процессами/перезагрузками. **Cross-restart reuse намеренно не принимается:**
сравнимой физической network identity пока нет; старый семидневный кеш не
превращается в trusted seed. Наличие файла само по себе не cache hit.

Срок записи — минимум DNS/CNAME TTL, 5 минут и profile expiry. Future/expired/
malformed/чужой scope отвергается. После I/O повторяется clock/freshness gate.
Существующие process-lived `filetxn.BoundedReadSlot/BoundedWriteSlot` сохранены
для отдельного cache domain; uncertain original owner не заменяется. Confirmed
retired legacy/corrupt bytes могут быть заменены только свежим DNS результатом.
Cache write — optional hint, не durable trust checkpoint и не условие Ready.
Действующий hint используется до запуска DNS workers. Чтение не обновляет At,
TTL, health или service-authentication status; ускорение запуском не измерялось.

`ResolveAddress` передаёт scoped expiring value. Root исключает истёкшие hints
после обработки остальных каналов. Новый renderer option копирует map по channel
ID, проверяет public IPv4 до secret lookup и сохраняет исходные TLS SNI/WS Host.
Это свежесть адреса при построении candidate, **не lease активного трафика**:
DNS TTL не отзывает уже принятый поток и не продлевает trust/forwarding permit.
Последующее обновление адреса/повторные dial и safe reload — отдельный runtime
сценарий PC-03/05, ещё без приёмки.

## Владение и реальные ограничения

Source limits: один bootstrap owner, 6 workers, до 10 оригинальных connections
(4 UDP + 4 TCP fallback + 2 HTTPS), до 2 response bodies. Оригинальные объекты
регистрируются до dial/чтения body; cancel закрывает losers, все worker results
joined. First Close outcome sticky; repeated Close не подменяет ошибку успехом.
При неизвестном retirement сохраняется исходный owner и `ErrPreflightCleanup`;
root не переходит к cache/hostname fallback, existing local-retirement guard
блокирует автоматического опасного преемника. Panic/clock fault не теряются
при удачном ответе другой стратегии. Это собственность Go API объектов, не
проверенная аттестация Win32 handles/stdlib внутренних задач.

Cache — максимум 64 KiB и 64 записей; count gate перед декодированием очередной
записи. Один retained reader/writer domain, без unbounded path→owner map. Root
имеет local batch6s/не более8 eligible WS names; query context не больше4s.
Это cooperative budgets: синхронные file/Close/native calls могут задержать
Stop/возврат сверх deadline; **hard wall-time/RSS/OS bounds не доказаны**.
Устранение задержек optional cache I/O без потери original owner остаётся PC-06.

Caller cancellation/stale scope/clock/cleanup — не cache success и не отрицательная
health запись. После подтверждённого retirement обычное отсутствие DNS результата,
TTL-expired hint или только истечение local batch budget сохраняют прежнее
настроенное разрешение имени в engine; это bootstrap endpoint, не direct fallback
пользовательского трафика. Parent Stop/network/profile change не получает этот
fallback. TLS/auth/probe/private worker/native acceptance gates сохранены.

## Evidence и незакрытая приёмка

8 Go/4 docs, оригиналы 12 путей (4 новых/absent), gofmt/manual diff/bytes-SHA:
`.artifacts/memory/bootstrap-scope-source-evidence.json`.
Никаких tests/fixtures/build/compile/vet/typecheck/parser/check/SDK/helper/native/
browser/admin/live или исполнения DNS/кеша не было. Исходники не скомпилированы.
Codec/TTL/CNAME/TC/UDP/TCP/DoH/PKI, cancel/panic/late dial/Close, concurrent process,
file ABI/ACL/sharing/durability/disk-full, complete stdlib goroutine/resource/time
bounds, source/clock changes и fixture/API migration требуют будущей приёмки.

Далее: engine bootstrap DNS rules/capture scope, IPv4/IPv6/UDP/TCP/user DNS клетки,
refresh/reconnect с истечением hint, действительная client-server пара; PC-04 UX,
W3b/SDK и полный PC/old9 объём сохраняются. Native acceptance const=false/IPC8
не менялись: TUN unavailable, schema2 install unsupported. Это source progress,
не разрешение включить TUN или выпустить EXE.
