# R02-006 — framing, оптимизированный IO и ложная атрибуция блокировки

26 октября 2026 · **READY_FOR_REVIEW**.
[Источники](SOURCES-006.md).
Это следующий вопрос после 005: одинаков ли protocol contract у всех IO variants,
через какие callers они доступны и что можно ошибочно принять за DPI?

## 1. R02-13 — разные writers имеют разные HMAC/framing semantics

Actual `sing-shadowtls v0.2.1`, commit
`af3279d7446641d5dd2e66cada24ce4597e98d35`. Текущий `v3_conn.go` совпал с exact raw
capture004. **Source-level candidate interoperability defect**, без запуска HMAC,
fixtures, core или VPN. Не утверждается ни раскрытие AEAD plaintext, ни обход
аутентификации, ни уже измеренная поломка пользовательского канала.

### Порядок вычисления tag

Пусть M — текущее состояние HMAC, P — payload одного record, а
`t=Trunc4(HMAC(M||P))`. Обычный `write(P)` и `WriteBuffer(P)`:

1. Добавляют P в state.
2. Получают t, не меняя state через Sum.
3. Добавляют t в state для следующего record.
4. Отправляют tag t и P.

Receiver `verifyApplicationData` вычисляет тот же t до добавления tag в state.
Но **WriteVectorised** сначала добавляет P, затем добавляет t в state, и только
после этого получает отправляемый `u=Trunc4(HMAC(M||P||t))`. Receiver ожидает t.
Эти recurrences различаются; равенство u=t не обеспечено конструкцией. Нельзя
объявлять их эквивалентными потому, что оба используют HMAC-SHA1. Совпадение
коротких tags для отдельных inputs не устраняет расхождение algorithms.

Это символический source trace, не вычисленный cryptographic counterexample.
Семантика Hash.Sum подтверждена официальным интерфейсом: Sum не меняет state.
[Go hash.Hash](https://pkg.go.dev/hash#Hash).

### Размер record зависит от варианта API

Обычный Write режет P на части не больше 16384 bytes. WriteBuffer и WriteVectorised
записывают длину всего buffer/aggregate в16-bit поле и не повторяют эту нарезку.
Поэтому есть два отдельных условия, которые нельзя заменить одним «buffer valid»:

- Для payload L=20000 buffer path может объявить record body20004 bytes. Это
  больше предела TLS1.3 ciphertext16640. ShadowTLS reader этот предел отдельно
  не проверяет; поток может быть принимаем своим peer и всё же иметь наблюдаемое
  несоответствие TLS camouflage. [RFC8446 §5.2](https://www.rfc-editor.org/rfc/rfc8446.html#section-5.2).
- Для L=65532 `4+uint16(L)` даёт 0 по modulo65536, хотя далее идёт 65536 body bytes.
  Это уже рассогласование собственного framing. Для vector aggregate аналогично.
  Размеры — ручные граничные модели, buffers не создавались/не исполнялись.

Первый пример не доказывает реальный RF classifier: нужны правильная stream
reassembly, phase knowledge и достижимость данного размера в конкретном adapter.
Сам факт корректного AEAD внутри record не исправляет внешний TLS-looking header.

## 2. Достижимость: отвергнутый и условный caller

### Отвергнутая первоначальная гипотеза

`sing-shadowsocks v0.2.8` serverConn.WriteVectorised вызывает **свой**
shadowaead.Writer.WriteVectorised. Этот codec собирает encrypted chunks в buffer
и вызывает upstream.Write, а не upstream.WriteVectorised. Client library2 использует
Write/WriteBuffer после своего AEAD. Следовательно, **наличие vectorised API у SS
server не доказывает вызов ошибочного ShadowTLS vectorised branch**.

Текущий source не обосновывает утверждение «SS2022 replacement обязательно
ломается из-за HMAC vectorised bug». Эта гипотеза явно отклонена; other framing,
buffer size и error paths по-прежнему требуют своей приёмки.

### Условно достижимый путь plain VLESS

В текущем core no-flow VLESS outbound возвращает DialEarlyConn. После отправки
request `WriterReplaceable()` становится true. `sing v0.8.13` CopySession отмечает
окончание lazy handshake и refreshUnwrap; count-writer unwrap проходит VLESS и
ExtendedConn wrapper к ShadowTLS writer. ShadowTLS сам не помечен replaceable,
поэтому его framing не должен сниматься обычным unwrap.

Затем `bufio.copyWaitWithPool`, **если source поддерживает соответствующий read
wait path**, после DefaultIncreaseBufferAfter=`512*1000` copied bytes включает
IncreaseBuffer. При наличии vectorised read waiter и writer выбирается
copyWaitVectorisedWithPool. CreateVectorisedWriter распознаёт ShadowTLS interface;
это путь к отличающемуся HMAC branch. ReadWaitOptions может выделять buffer65535.

Не установлено, что конкретный Windows TUN stack, socket wrappers, source reader,
forceSTDIO/LowMemory и направление в пользовательском runtime выберут эту ветку.
Число 512000 — source threshold copy loop, **не измеренный порог блокировки**.
Обычный local proxy/TCP source и native TUN path нельзя объявить одинаковыми.

Уже отклонённая по confidentiality композиция 004 не становится приемлемой после
одного исправления IO; её quarantine proposal сохраняет приоритет. Здесь source
gap полезен для приёмки любых будущих wrappers/candidates, не для легализации 004.

## 3. R02-14 — ошибка раннего chunk может исчезнуть из результата

verifiedConn.Write разбивает большой input и для каждого chunk присваивает
`err=c.write(...)`, но не выходит при ошибке. В конце, если **последний** err=nil,
возвращает полную исходную длину и nil. Ручной control-flow witness:

| Шаг | Допущенный результат нижнего writer | Результат wrapper |
|---|---|---|
| Input L>16384, первый chunk | Ошибка, например часть bytes не принята | err установлен; выполнение продолжается |
| Второй chunk | Успешная запись | Предыдущий err перезаписан nil |
| Выход | Первый chunk не доставлен полностью | Возвращены n=L, err=nil |

Это условная последовательность легального нижнего IO; не все real socket errors
восстанавливаются на следующем вызове. Если любая ошибка terminal и последующие
записи тоже fail, именно сокрытие err может не проявиться. Наличие такого runtime
сценария не проверялось. Но wrapper не может выводить полный успех лишь из
последнего chunk. HMAC state уже изменён до actual IO; после partial write нужны
явные правила termination, а не продолжение изменённого authenticated stream.
[Go io.Writer contract](https://pkg.go.dev/io#Writer).

Не следует считать успешный return подтверждением доставки до application peer;
даже правильный Write этого не гарантирует. Здесь дополнительно нарушается
локальная достоверность ошибки/accepted byte count при указанной последовательности.

## 4. Цена и reuse решения

**Adapt/review upstream**, а не новый транспорт: одна record encoder/state machine
для всех публичных write variants, с согласованными tag recurrence, bound и bytes/
error semantics. Передача через vectors может сохранить несколько корректных
records в одном syscall; «один syscall» не требует «один большой TLS-looking record».
Однопоточность или serialization state+wire порядка также должна быть явно задана;
concurrent caller behavior здесь не доказан и оставлен отдельным вопросом.

Для K разбиений на records внешний framing стоит `9*K` bytes поверх передаваемого
P:5 header+4HMAC на record. При scalar cap16384 и без иных boundaries
`K=ceil(L/16384)` для L>0. Увеличение числа records добавляет headers/IO bookkeeping;
batch syscall может сократить системные вызовы без изменения valid record cap.
Это dimensional accounting, не CPU/throughput benchmark и не сумма всего VPN overhead.

| Вариант | Решение / причина |
|---|---|
| Уже имеющийся scalar Write codec | **Reference** правильной tag recurrence, но его error loop тоже требует review |
| Existing vector APIs и sing bufio | **Adapt** при единой protocol semantics; сами по себе не запрещать все fast paths |
| Отключить HMAC/AEAD или молча raw-copy при ошибке | **Reject**: разрушает security property, не приемлемый recovery |
| Повысить buffer / сменить port / дать failover score штраф | **Reject как доказанное исправление**: не устраняет codec mismatch |
| Назначить любой bad-record-mac доказательством DPI | **Reject attribution**: есть внутренний source counterexplanation |

Все рассматриваемые Go implementation notices: GPL-3.0-or-later. Изменения upstream,
обновление pin, patches/dependencies/product import не выполнялись. Нужен отдельный
адресный developer review и минимальная авторизованная проверка.

## 5. Применимость и будущая конечная приёмка — NOT_RUN

**Windows:** relevant core closure присутствует; actual runtime branch неизвестен.
**Android:** тот же core pin не доказывает наличие AAR/flow, а current graph вообще
отвергает ShadowTLS. Source finding не превращается в текущий Android fault.
**R03/R04/R06:** protocol phase, selected IO variant, record/body length и first
local error — useful bounded evidence. Их следует отделять от transport/endpoint
unreachability. Raw content/UUID/PSK не нужны для диагностики; privacy scope у R05.

Будущие cases после разрешения, без выполнения сейчас:

1. Один P≤16384 через Write/WriteBuffer/WriteVectorised: receiver accepts и decoded
   bytes равны P; не требовать bitwise wire equality при разных record boundaries.
2. Boundary L=0/16384/16385/20000/65531/65532 и multi-buffer aggregates: cap,
   no uint16 wrap, все tags следуют одному определённому recurrence.
3. Первая/средняя/последняя partial/error запись; ошибка не теряется; повреждённый
   поток закрывается в защищённом lifecycle без plaintext/direct fallback.
4. Реальный caller trace до/после lazy handshake и batch threshold на каждой
   платформе; scalar versus fast source-read capabilities, SS codec negative path.
5. Затем controlled wire/native/fault comparison. Ни build/parser, ни unit alone
   не доказывают отсутствие DPI или полную runtime пригодность.

Источники/границы: full sing tag commit и raw Go1.26 captures получить не удалось;
используются exact module version+go.sum и cached interface docs. Нулевые failed
capture placeholders отмечены как unusable. Core/ShadowTLS pins и previous raw
comparison сохранены. Это ограничение provenance, не повод выдумать SHA или PASS.
