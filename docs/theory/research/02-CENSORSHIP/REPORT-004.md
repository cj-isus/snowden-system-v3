# R02-004 — ShadowTLS v3 и отсутствующее шифрование VLESS

26 октября 2026 · **READY_FOR_REVIEW**.
[Источники](SOURCES-004.md) · [Краткий разбор](handoffs/R02-004.md).

## 1. R02-10: конкретная композиция не обеспечивает VPN confidentiality

**Вывод по исходникам, высокий приоритет триажа:** Windows renderer создаёт
`VLESS без TLS → detour ShadowTLS v3` и допускает эту цепочку как protected.
В фактически выбранной dependency `sing-shadowtls v0.2.1` полезные байты после
handshake передаются с HMAC, без шифрования ShadowTLS. VLESS с пустым flow также
не добавляет шифрование. Поэтому этой конкретной композиции нельзя приписывать
конфиденциальность VPN-туннеля. Это source finding, **не утверждение об активном
сервере, выбранном runtime профиле или уже произошедшей утечке**.

Это независимый пробел сверх R02-001–003. Ранее разведённые anti-probing и
fingerprint свойства недостаточны, если отсутствует базовый encrypted data path.

## 2. Версия и достижимый путь

Фактические `windows/go.mod` и `sing-box v1.13.19/go.mod` выбирают
**sing-shadowtls v0.2.1**, tag commit
`af3279d7446641d5dd2e66cada24ce4597e98d35`, 2026-05-17.
Первоначально прочитанная старая pseudo-version `fcd445d3…` — только exploratory;
вывод перепроверен на v0.2.1. Три ключевых raw файла actual tag побайтно совпали
с module cache по SHA-256. Никакая команда Go не запускалась.

| Звено | Наблюдение из исходников | Следствие |
|---|---|---|
| `windows/backend/render/render.go:692–731` | Создаёт ShadowTLS v3 с donor TLS, затем VLESS с UUID и detour, без TLS/flow | Есть маскировочный handshake; отдельное payload encryption не задано |
| `config.go:409–440,579–613` | Для VLESS detour требует ShadowTLS, пропускает собственную TLS-проверку; комментарий приписывает шифрование ShadowTLS | Admission основан на неверном свойстве компонента |
| pinned sing-box `protocol/vless/outbound.go` | Без transport/tlsDialer вызывает detour dialer и DialEarlyConn | Скрытого автоматического TLS для VLESS здесь нет |
| `common/dialer/detour.go` → `protocol/shadowtls/outbound.go` | Получает соединение от ShadowTLS; destination loopback не делает внешний TCP локальным | `127.0.0.1:9444` относится к дальнейшему внутреннему сервису, не защищает участок client→entry |
| `sing-shadowtls/client.go` case3 | После TLS-handshake возвращает `newVerifiedConn` над исходным conn | TLS client object не используется как data encryptor после переключения |
| `v3_conn.go` write/buffer/vectorised и read | HMAC/framing окружают исходное содержимое; read снимает framing | Integrity/authentication и secrecy — разные свойства |
| pinned sing-vmess `vless/client.go`, `protocol.go` | Без flow передаёт VLESS request и последующие данные; UUID сериализуется явно | Заголовок VLESS не становится секретным от наличия UUID |

Ни протокольный код, ни Parse/renderer не исполнялись. Достижимость описывает
ветки исходников при корректных соответствующих inputs, не активность канала.

## 3. Ручная модель видимости

Для обычного post-switch write `P` layout читается из pinned constants/write path:

`TLS-looking header (5 bytes) || HMAC field (4 bytes) || P`

`P` не преобразуется шифром. Пассивному наблюдателю, который видит этот участок
и умеет выделять records, **не нужен ShadowTLS password, чтобы читать P**.
TCP segmentation не меняет это свойство: record/stream reassembly отличается
от границ IP packets. Это ручной source trace; packet capture не проводился.
HMAC и авторизация могут сохранять собственные свойства; их криптостойкость и
полный error-path audit здесь не исследованы.

В данной композиции первое `P` включает VLESS version, UUID, additions, command,
destination и, возможно, первый payload. Следующие writes содержат данные потока.
Из этого следуют разные уровни раскрытия:

- VLESS UUID и адрес назначения не защищены шифрованием этой композиции.
- Нешифрованный прикладной TCP payload остаётся читаемым на client→entry.
- Вложенный HTTPS сохраняет собственную криптографию: этот вывод **не расшифровывает
  HTTPS-содержимое**. Однако VLESS header и доступный ClientHello остаются отдельной
  metadata/fingerprint поверхностью. ECH нельзя предполагать для всех приложений.
- Захваченный UUID не равен ShadowTLS PSK; он сам по себе не разрешает пройти
  внешнюю аутентификацию и не доказывает доступ к другим каналам. Повторное
  использование UUID в других профилях не проверялось.

Для цензора это потенциально более сильное наблюдение, чем статистический traffic
shape. Для нашей адаптации низкая задержка такого канала не компенсирует отсутствие
требуемого свойства защиты. Доступность, throughput, anti-probing и confidentiality
должны иметь разные eligibility условия. Никаких заявлений о deployed RF detector.

## 4. Самооппонирование

**«TLS enabled=true уже есть».** Есть у маскировочного handshake. Проверен переход
к data stage и фактический writer; boolean не доказывает криптографию data path.

**«В v3 есть XOR».** Проверено: XOR в `v3_client.go`/`v3_server.go` относится к
обработке donor handshake application records. Он не появляется в post-switch
`verifiedConn` payload writer. Нельзя переносить свойство между стадиями.

**«Внутренний адрес loopback».** Loopback расположен после удалённого entry;
удалённый carrier всё равно проходит через наблюдаемую сеть.

**«VLESS UUID шифрует».** В выбранной реализации UUID включается в request bytes;
криптографического преобразования payload в traced no-flow branch нет.

**«Приложения обычно HTTPS».** Это уменьшает содержимое раскрытия, но не создаёт
универсальную конфиденциальность VPN и не скрывает VLESS credentials/metadata.

**«Сервер может добавить шифрование».** Одностороннее шифрование после приёма
не скрывает байты, которые клиент уже отправил на client→entry. Другой внешний
шифрующий overlay мог бы изменить условия; renderer этой ветки его не задаёт.
Фактическая серверная композиция и пользовательский runtime не исследовались.

Независимое подтверждение замысла: автор ShadowTLS v3 прямо исключает шифрование
и forward security из целей. [Design goal, revision 2023-02-12](https://github.com/ihciah/shadow-tls/wiki/V3-Protocol/e30756384998f064e5b51e0ffd016286622777b2).

## 5. Готовый reuse и цена исправления

Новые самописные transport/crypto не нужны. Два кандидата требуют сравнения:

| Кандидат | Что можно переиспользовать | Существенные ограничения |
|---|---|---|
| Shadowsocks AEAD-2022 поверх ShadowTLS | В том же pinned sing-box есть Shadowsocks outbound с detour и method.DialEarlyConn; `sing-shadowsocks2 v0.2.1` поддерживает 2022 AES-GCM | Нужны совместимые server inbound, профиль, собственный SS key и новая admission support. TCP carrier сам не даёт native UDP; UoT/mux — отдельный контракт. AEAD-2022 **не даёт forward secrecy** |
| VLESS с настоящим внутренним verified TLS | Существующий tlsDialer VLESS и проверка сертификата origin | Обе стороны должны поддержать TLS/identity. ClientHello внутреннего TLS остаётся видимым в нешифрующем ShadowTLS framing: metadata/fingerprint надо оценить повторно. Дополнительный handshake/очередь, не обещанный performance gain |

Первый вариант использует готовую encrypted proxy композицию; второй тоже может
обеспечить содержательную конфиденциальность при корректном TLS, но camouflage
и exposure отличаются. Donor certificate не заменяет проверку identity внутреннего
TLS endpoint. Нельзя исправить ситуацию только именем outbound, новым портом,
добавлением padding или переключением UI-индикатора.

SIP022 описывает AEAD и отсутствие forward secrecy; его общее обещание обхода DPI
не переносится на все сети РФ. [Спецификация SIP022](https://shadowsocks.org/doc/sip022.html).
License scope Go implementations sing-box/sing-shadowtls/sing-vmess/sing-shadowsocks2:
**GPL-3.0-or-later**, сохранены тексты/пины; новый dependency не добавлялся.
Original Rust ShadowTLS и его отдельная лицензия не подменяют лицензию Go port.

## 6. Применимость и порядок передачи

**Windows:** предложить руководителю сначала исключить эту конкретную TLS-less
VLESS→ShadowTLS композицию из protected eligibility до выбора и проверки encrypted
replacement. Проверить selector default/failover/profile migration согласованно;
иначе исключённый кандидат может сохраниться через старое состояние. Это предложение
разработчику, не выполненная правка и не разрешение менять runtime.

**Android:** текущий `NativeGraphSecurity` принимает VLESS/Trojan/HY2, требует TLS
каждому, а ShadowTLS type отвергает. Поэтому PC-исключение нельзя объявить текущим
Android-дефектом. При будущем расширении не переносить его admission assumption.
Source subset не доказывает native execution, AAR availability или protected egress.

**R03/R05:** зафиксировать security eligibility до оптимизации score: у цепочки
должна быть проверенная защита реального data path и identity scope. В графе
недостаточно существования detour, валидной подписи профиля и TLS boolean на
произвольном соседе. Владельцы определяют контракт; здесь SDK не менялся.

Будущая конечная приёмка — **NOT RUN**, только после явной авторизации:

1. Проверка отказа admission для исходной композиции и обоих selector/default путей.
2. Для replacement trace client/server codec boundaries, key/identity checks,
   replay/errors/partial writes/reconnect и отсутствие downgrade к старому пути.
3. Изолированный deterministic canary payload и wire observation: plaintext marker
   не появляется на внешнем участке. Само отсутствие marker недостаточно для
   доказательства криптостойкости; нужны code/protocol proof и корректный AEAD/TLS.
4. Трафик HTTPS и plaintext рассматривать отдельно; DNS/UDP/TCP и long-lived flows
   проверять в заявленном scope. Это не просьба запустить live сейчас.
5. Затем отдельные runtime/native/platform/fault gates и конкретные RF измерения.

## 7. Предложенная вставка в общую теорию

> ShadowTLS v3 даёт маскировочный handshake и аутентифицированное framing, но
> не шифрует собственный post-switch payload. Поэтому plain VLESS внутри ShadowTLS
> нельзя считать конфиденциальным VPN transport: его UUID, destination и содержимое
> без отдельной прикладной криптографии остаются наблюдаемыми. Шифрование должно
> обеспечиваться отдельным совместимым inner protocol; его identity, replay, PFS
> и camouflage свойства проверяются отдельно. До этой проверки канал не допускается
> в защищённый набор только за счёт успешного handshake/probe или высокой скорости.

Общая теория и product source не изменялись. До решения руководителя — только
READY_FOR_REVIEW; live leak не заявлен. Captures, raw/cache equality, local hashes
и ручной opposing review находятся в собственном artifacts namespace.

### Актуальность входов при завершении

REVIEW.json фиксирует **один drift из семи входов**: общий CENSORSHIP-THEORY.md
изменён параллельным владельцем. По адресному повторному поиску ShadowTLS там
по-прежнему только в общем списке кандидатов; новая finding не выдаётся за
доказательство актуальности всего сборника. Product inputs Windows и Android
в этом наборе сохранили hashes. Руководитель должен повторно согласовать вставку
со своей текущей редакцией; старый hash общего документа не переносится как PASS.
