<a id="transport-composition-2026-10-28"></a>
# Композиция транспортов: шифрование, identity и сохранение TLS-контракта

[Оглавление](../README.md) · [Контекст ShadowTLS](CENSORSHIP-MECHANISMS.md#reviewed-transport-confidentiality) · [Решения review](../research/reviews/2026-10-28-DEEP-REVIEW.md)

**Сверка 2026-10-28, DR27-01/02/03.** Source-факты относятся к публичным
закреплённым реализациям; криптографические свойства — к указанным условиям
протокола. Исходники и artifacts приложения в этом docs-срезе отсутствуют.
Смена состава канала не реализована; source review не является wire/native PASS.

## 1. Свойство принадлежит участку, а не названию канала

Обозначим C — клиент, E — ShadowTLS entry, D — donor, X — inner proxy endpoint,
T — прикладной destination. X может совпадать с E, но это отдельная trust-role.
Donor TLS устанавливается между C и D; его ключи не шифруют автоматически
следующие данные C→X. Ранее принятая проблема TLS-less VLESS→ShadowTLS3 остаётся
основанием проверить именно data path.

| Свойство C→X | TLS-less VLESS→ST3 | VLESS→настоящий inner TLS→ST3 | SS2022 AES-GCM→ST3 |
|---|---|---|---|
| Proxy destination | Не шифруется самим VLESS/ST3 | Внутри TLS records до X | Внутри SS AEAD до держателя ключа |
| VLESS UUID | Открытый bearer header | Внутри inner TLS | При замене VLESS отсутствует |
| Payload | Только собственная защита приложения, если она есть | TLS confidentiality/integrity | SS confidentiality/integrity |
| Кто удостоверен | Donor certificate не удостоверяет X | Выбранная PKI identity либо явный key-pin contract | Владение общим SS key, не уникальность сервера среди его держателей |
| PFS payload | Нет от этой композиции | При fresh TLS1.3 (EC)DHE и корректном handshake | Сам SS2022 PFS не даёт |
| Replay между соединениями | VLESS отдельный gate не добавляет | Fresh handshake/traffic keys; не application exactly-once | Timestamp + локальная salt cache; cluster/restart scope отдельно |
| Видимые дополнительные данные | Proxy header; ClientHello HTTPS-приложения | Inner ClientHello и реально отправленные SNI/ALPN без собственной ECH-защиты | Открытый SS TCP salt; application hello внутри ciphertext |

Это не рейтинг camouflage или скорости. Entry IP, размеры и время передачи
сохраняют наблюдаемость. На X→T заканчивается защита proxy transport;
дальнейшая конфиденциальность зависит от приложения и следующего участка.

**Проверенная цепочка inner TLS:** `vlessDialer.DialContext` сначала получает
результат `DialTLSContext`, затем передаёт его в `Client.DialEarlyConn`.
`ClientHandshake` возвращает именно TLS Conn после handshake. `WriteRequest`
пишет UUID/destination/payload в этот writer. На receiver raw-TCP VLESS inbound
выполняет `ServerHandshake` перед `Service.NewConnection`.
[Client caller](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/protocol/vless/outbound.go),
[TLS dialer](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/common/tls/client.go),
[handshake boundary](https://github.com/SagerNet/sing/blob/7c349dacf402256d3a7029746073b05d2ead584a/common/tls/config.go),
[receiver](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/protocol/vless/inbound.go).

**Предпосылки:** raw TCP, `flow=""`, подходящий inbound detour, без произвольного
добавления WS/gRPC/mux. При configured V2Ray transport receiver имеет иной путь;
прямая injection не проходит автоматически через `transport.Serve`.
Возврат исходного `NetConn()` после успешного handshake разрушил бы эту
шифрующую границу. TLS, настроенный только на donor stage, её не создаёт.

## 2. Проверить правильное имя важнее, чем успешно проверить какое-либо имя

В pinned `NewSTDClient` обычная PKI-ветка проверяет chain и выбранное server name.
Но `certificate_public_key_sha256` включает `InsecureSkipVerify` вместе с
собственным `VerifyPeerCertificate`: `verifyPublicKeySHA256` проверяет public-key
hash leaf, а не одновременно SAN, цепочку и срок сертификата.
Для SNI-enabled pin-профиля **правильный pin с неподходящим SAN не обязан быть
отвергнут**. Это иной контракт identity, а не proof обычной PKI-проверки.
Комбинация `disable_sni`/pin имеет дополнительные callbacks и рассматривается
отдельно. [Точные ветви](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/common/tls/std_client.go).

Сильный контрпример: если reference identity inner TLS по ошибке задана как D,
entry может передать этот TLS настоящему D. Проверка D будет корректна,
но VLESS UUID/destination затем отправятся не разрешённому X. Криптография
не исправляет ошибочное назначение роли. Имя для аутентификации выбирается из
доверенного контекста, а не из неаутентифицированного ответа peer:
[RFC9525 §6.1.1](https://www.rfc-editor.org/rfc/rfc9525.html#section-6.1.1).

## 3. MinVersion проходит через preset, а не действует вне его

**R02-16, ограниченный source-вывод:** core `NewUTLSClient` задаёт requested
MinVersion/MaxVersion, но `Client()` создаёт uTLS connection на clone.
`chrome` в рассматриваемом pin соответствует Chrome133. Его preset содержит
SupportedVersions с TLS1.3 и TLS1.2; `TLSVersMin/Max` самого spec равны нулю.
`ApplyPreset→SetTLSVers` выводит диапазон 1.2–1.3 из extension и при
`EncryptedClientHelloConfigList == nil` перезаписывает границы clone.

Поздняя проверка есть: `pickTLSVersion→Config.mutualVersion→supportedVersions`.
Но она читает **уже изменённый** Config, а не исходное requested1.3.
Поэтому контрпример с TLS1.2-only peer, подходящим cipher и корректными
certificate/Finished не отсекается именно этим requested floor. Downgrade
canary, invalid certificate и bad Finished — другие сохраняющиеся причины отказа.

Primary: [core wrapper/options](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/common/tls/utls_client.go),
[Chrome133 preset](https://github.com/MetaCubeX/utls/blob/cf49b0864331e156689feec6363ef9bf518a5ac7/u_parrots.go),
[SetTLSVers/BuildHandshakeState](https://github.com/MetaCubeX/utls/blob/cf49b0864331e156689feec6363ef9bf518a5ac7/u_conn.go),
[client version check](https://github.com/MetaCubeX/utls/blob/cf49b0864331e156689feec6363ef9bf518a5ac7/handshake_client.go),
[Config version filtering](https://github.com/MetaCubeX/utls/blob/cf49b0864331e156689feec6363ef9bf518a5ac7/common.go).

Область: `with_utls`, этот Chrome/non-real-ECH path, без REALITY/kTLS и
дополнительного внешнего version gate. GREASE ECH не равен реальному ECHConfigList.
STD, real ECH, иные presets не получают такой verdict автоматически.
ShadowTLS library `StrictMode=true` дополнительно отвергает non-TLS1.3 после
handshake; core `NewOutbound` это поле не задаёт. Серверный strict-флаг не
передаётся клиенту сам по себе.
[Library](https://github.com/SagerNet/sing-shadowtls/blob/af3279d7446641d5dd2e66cada24ce4597e98d35/client.go),
[core caller](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/protocol/shadowtls/outbound.go).

## 4. Копирование TLS-полей может потерять и capability, и проверку identity

Два разных source-факта:

1. `utlsConnWrapper.ConnectionState` строит новый стандартный
   `crypto/tls.ConnectionState` из публичных полей. Закрытый exporter callback
   `ekm` не переносится. Метод с подходящим именем у типа остаётся, однако
   capability полноценного native state отсутствует. Native `Upstream()`
   возвращает uTLS UConn и не теряет его exporter. Поэтому утверждение
   «exporter недоступен вообще» неверно. `NetConn()` — не этот обход.
   В том же literal не переносится native `ECHAccepted`: нулевое значение
   reconstructed state нельзя считать доказательством реального ECH rejection.
2. В STD `DisableSNI` peer verification сохраняется посредством
   `InsecureSkipVerify=true` **плюс** `VerifyConnection` с исходным DNSName.
   ShadowTLS3 fallback `DefaultTLSHandshakeFunc` копирует первый флаг,
   но не `VerifyConnection`. При отсутствии другого подходящего verifier
   исходная PKI identity-policy не сохраняется. Это не утверждение об
   исчезновении всех остальных ShadowTLS PSK/auth checks.

Primary: [uTLS wrapper](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/common/tls/utls_client.go),
[STD policy](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/common/tls/std_client.go),
[fallback adapter](https://github.com/SagerNet/sing-shadowtls/blob/af3279d7446641d5dd2e66cada24ce4597e98d35/tls_wrapper.go),
[fallback caller](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/protocol/shadowtls/outbound.go).
Это факты адаптации source; установленный клиент с такой конфигурацией,
runtime panic и фактическая утечка не установлены.

**Exporter не универсальный ремонт freshness.** Normal exporter связан с TLS
handshake и его endpoints; early exporter использует другое основание и имеет
replay-ограничения. Повтор вызова с теми же label/context на той же connection
детерминирован, а не создаёт новый nonce. Donor exporter C↔D не становится
разделяемым секретом C↔E у relay. Нужны actual consumer, правильный момент после
handshake, auth state и область connection/epoch.
[RFC8446 §7.5](https://www.rfc-editor.org/rfc/rfc8446.html#section-7.5),
[§8](https://www.rfc-editor.org/rfc/rfc8446.html#section-8),
[RFC9266 §2](https://www.rfc-editor.org/rfc/rfc9266.html#section-2).

## 5. SS2022: отдельные ключи, две реализации и локальный replay scope

В core pin outbound использует **sing-shadowsocks2**
`1f9f20777fd1aedc3aeaebbc08fad00a2e2f8c40`, inbound — **sing-shadowsocks**
`e0612494bafdd1429e9632bc52fd278585d28690`. Для single-user AES-GCM source
подтверждает шифрование variable header с destination и initial payload,
затем AEAD chunks. Response содержит проверяемый request-salt binding.
[Client method](https://github.com/SagerNet/sing-shadowsocks2/blob/1f9f20777fd1aedc3aeaebbc08fad00a2e2f8c40/shadowaead_2022/method.go),
[receiver](https://github.com/SagerNet/sing-shadowsocks/blob/e0612494bafdd1429e9632bc52fd278585d28690/shadowaead_2022/service.go).

`SessionKey` зависит от PSK и открытого salt. Последующая компрометация PSK
раскрывает соответствующие SS transport records: PFS здесь нет.
Знание только независимого ShadowTLS password не означает знания SS key.
Метод `none` не получает свойств AES-GCM по одному имени Shadowsocks.
[SIP022, закреплённый дизайн](https://github.com/shadowsocks/shadowsocks-org/blob/eb7862d19b5d1ade04b89e2d49f8c68d34c8b031/docs/doc/sip022.md),
[SessionKey](https://github.com/SagerNet/sing-shadowsocks2/blob/1f9f20777fd1aedc3aeaebbc08fad00a2e2f8c40/shadowaead_2022/protocol.go).

Два receivers с одним key и отдельными salt caches могут впервые принять один
request каждый, если он доставлен в допустимом timestamp-window. Restart с
потерей cache внутри этого окна — иной instance того же ограничения. Просто
истечение cache не делает пригодным давно expired request при нормальных часах.
Повтор proxy request, dial T и повтор действия приложения — разные события.
[Replay filter](https://github.com/SagerNet/sing/blob/7c349dacf402256d3a7029746073b05d2ead584a/common/replay/simple.go).

**UDP не наследуется от TCP-вывода.** ST outbound принимает только TCP.
Native SS UDP через него не поддерживается; согласованный UoT переносит UDP
внутри SS TCP и использует другой receiver path. Во время исследования найден
отдельный кандидат расхождения native UDP replay-window ordering с SIP022;
он не нужен для доказательства TCP confidentiality и сохраняется для отдельной
приёмки, без blanket SIP022-conformance.

## 6. Решение и будущая приёмка

Для требования confidentiality+identity+PFS приоритетен **условный кандидат
настоящего inner TLS1.3-(EC)DHE до X**, с доказанным сохранением writer и identity.
SS2022 — другой кандидат, если допустимы отсутствие PFS и shared-key identity.
Одновременное добавление обоих слоёв пока deferred: порядок обёрток меняет
наблюдаемость и не доказан одним полем профиля. Это исследовательский выбор,
не переключение production. Существующие TH-01/03/05 gates сохраняются.

Конечные группы future cases, все **NOT_RUN**:

| Группа | Обязательное различие |
|---|---|
| Writer/receiver | Настоящий TLS Conn против raw NetConn; raw TCP против incompatible transport injection |
| Identity | PKI wrong name/chain; wrong pin; correct key с неподходящим SAN; перепутанные D и X |
| Version | Chrome133 min=max1.3 с peer1.3/1.2-only/canary/bad Finished; отдельно STD/real ECH/StrictMode |
| Adapter | STD DisableSNI до/после fallback; native exporter/ECHAccepted против reconstructed state; отсутствие consumer |
| SS | Два AES-GCM метода, неверный key/length, `none`; bad tag/type/time/request salt |
| Replay | Повтор в одном Service, в двух Services и после restart внутри окна; expired timestamp отдельно |
| Network scope | Native SS UDP reject, согласованный UoT, выбранный VLESS UDP encoding, bounded framing errors |

Цена: дополнительный handshake, trust/key delivery, framing/ownership и проверки
обоих концов; выигрыш camouflage/скорости/энергии не измерен. Reuse готовых TLS/SS
примитивов предпочтителен своему cipher. Root licenses sing-box/SS/ShadowTLS
содержат GPL-3.0-or-later notices; transitive artifact/SBOM и совместимость всей
поставки этим обзором не закрываются.
