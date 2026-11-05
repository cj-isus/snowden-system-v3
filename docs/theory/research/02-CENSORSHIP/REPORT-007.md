# R02-007 — HRR freshness и effective TLS policy

26 октября 2026 · **READY_FOR_REVIEW**. [Источники](SOURCES-007.md).
Вопрос: какие свойства handshake сохраняются при HelloRetryRequest и совпадает
ли declarative TLS minimum с effective preset?

## 1. R02-15 — первый ServerHello не всегда источник свежего random

**Source/model finding, высокий приоритет независимой рецензии.** Actual
ShadowTLS0.2.1 `af3279d7446641d5dd2e66cada24ce4597e98d35` берёт serverRandom из
первого handshake record типа ServerHello. Client wrapper инициализирует seed
только один раз (`readHMAC==nil`), что прямо поддерживает HRR как первый ServerHello.
Однако TLS1.3 HRR имеет **фиксированное**, а не случайное значение Random.
[RFC8446 §4.1.3](https://www.rfc-editor.org/rfc/rfc8446.html#section-4.1.3).

В service.go это значение используется для initial HMAC data state с направлением
C/S. extractServerRandom не исключает HRR; supported_versions=1.3 в HRR может пройти
проверку strict mode. Следующий настоящий ServerHello с fresh random уже не
переинициализирует client seed. Собственная data-stage seed не включает ClientHello
random/sessionID или полный TLS transcript. Нельзя переносить свежесть donor TLS
handshake на этот отдельный HMAC state.

### Условный ручной witness — без исполнения и без сетевого PoC

Предпосылки: один PSK остаётся действующим; наблюдалась легитимная HRR-сессия;
сохранён её валидно подписанный initial ClientHello и первый post-switch record P;
другая сессия с этим ClientHello получает HRR первым; нет дополнительной внешней
replay-защиты, запрещающей такой input до рассматриваемого Service.

| Свойство | Первая HRR-сессия | Другая HRR-сессия |
|---|---|---|
| ClientHello authentication | Подпись под PSK валидна | Тот же подписанный transcript; проверка stateless |
| Извлечённый initial random | Константа R_HRR | Та же R_HRR |
| Начальный C→E data state | HMAC_K(R_HRR || C) | Тот же state |
| Первый прежний record P и tag | Вычислены из этого state | При том же state tag проверяется так же |

Service переключается на inner handler при первом совпадении data HMAC в
copyByFrameUntilHMACMatches, закрывает donor conn и устанавливает переменную
`handshakeFinished`. Здесь имя означает найденный HMAC switch, **не проверку TLS
Finished на entry**. Task.Group допускает завершение обеих relay tasks после этого
закрытия; cached payload передаётся inner handler. Таким образом, source trace
показывает условную возможность повторного прохождения внешней стадии без знания
PSK через наблюдённый record. Криптооперации и protocol trace не исполнялись.

### Что этим не установлено

- Не проверено, встречается ли HRR у выбранного donor/сети, и может ли противник
  вызвать его для действующего профиля. Нельзя бесшумно менять подписанный ClientHello
  и автоматически сохранять его auth tag. Условие HRR существенно.
- Не доказано работающее соединение до target или повтор прикладного действия.
  У SS2022 inner replay cache/срок и у настоящего inner TLS есть независимые проверки.
  При plain VLESS inner свойства слабее, но отдельная end-to-end эксплуатация не
  проверялась. HTTPS содержимое этим не расшифровывается.
- PSK rotation, внешний admission/replay gate или запрет HRR могут исключить
  условия witness. Наличие таких мер в реальном сервере неизвестно.
- Strict TLS1.3 сам по себе не исключает HRR: это штатная ветка TLS1.3, не TLS1.2.
  Нельзя закрыть этот пробел только `min_version=1.3` или `strict_mode=true`.

**Counterargument:** HRR поддерживается ради совместимости, и одинаковые seeds
нормальны для корректного двустороннего IO. Ответ: согласованность двух peers в
одной сессии и уникальность между разными сессиями — разные требования. Ручной
witness касается второго. Успешный handshake test HRR не доказывает anti-replay.

### Reuse/исправление как предложение, не patch

Для текущего protocol design безопаснее **reject HRR для такого source branch**
до отдельной рецензии или выбрать donor/capability, где допустимость подтверждена
и HRR при появлении приводит к контролируемому отказу. Отказ уменьшает доступность,
может иметь наблюдаемое поведение; это не полноценное совершенствование camouflage.

Поддержать HRR корректно — **adapt обе стороны**: определить реально свежий
binding после HRR/final ServerHello, canonical transcript/state transition и
проверку peer agreement. Простая замена seed только с одной стороны ломает wire
compatibility. Добавление произвольного client random к старой константе не следует
выдавать за доказанное исправление replay: replay может сохранять прежний ClientHello.
Для новой версии нужен самостоятельный proof и migration/capability design.
Зрелые TLS handshake/transcript primitives — reference; новый self-made cipher не нужен.

## 2. R02-16 — версия протокола, strict mode и min_version расходятся

В core sing-box1.13.19 ShadowTLS outbound options не имеют StrictMode. Caller
создаёт ClientConfig без этого поля; bool остаётся false. Inbound strict_mode
передаётся серверу отдельно. Renderer задаёт ShadowTLS version3 и Chrome preset,
но не требует TLS1.3. TLS1.2 compatibility branch client wrapper помечает authorized
после ServerHello иначе, чем TLS1.3 HMAC-modified application records.

Это **не неожиданный cryptographic break всего v3**: авторский протокол документирует
non-strict TLS1.2 как осознанный компромисс. Но название «v3» не позволяет обещать
требуемый strict anti-hijacking режим. Server strict при TLS1.2 переводит поток в
donor relay; это не доказательство готового protected data path. Фактический server
strict config не читался. [Авторский design](https://github.com/ihciah/shadow-tls/blob/master/docs/protocol-v3-en.md).

### Почему одной настройки min_version может оказаться недостаточно

Pinned MetaCubeX/uTLS1.8.4 `cf49b0864331e156689feec6363ef9bf518a5ac7`:

1. sing-box NewUTLSClient записывает options.MinVersion/MaxVersion в Config.
2. Chrome_Auto выбран как Chrome133; preset содержит supported versions1.3/1.2
   и не задаёт собственные explicit TLSVersMin/Max.
3. BuildHandshakeState → applyPresetByID → ApplyPreset → SetTLSVers выводит диапазон
   из preset extensions. При EncryptedClientHelloConfigList=nil **перезаписывает
   Config.MinVersion/MaxVersion** значениями preset.
4. ApplyConfig переносит supported_versions из extension в Hello; прочитанный
   core wrapper отдельно меняет ALPN, но исходный minimum здесь не восстанавливает.

Следовательно, в этом **не-ECH Chrome133 branch** исходное `min_version=1.3` нельзя
считать enforced floor лишь по тому, что оно распарсилось в NewUTLSClient. Preset
путь может вернуть 1.2 в effective config/offered versions. Это source trace,
**не выполненный TLS1.2 handshake и не утверждение о downgrade текущего трафика**.
Для других presets/ECH/STD TLS вывод не переносится без анализа их веток.

Reuse/adapt: согласовать allowed-version policy с preset до генерации wire,
проверять negotiated version перед выдачей data connection, исключать слабый режим
с контролируемой причиной. Полный ClientHello shape меняется при фильтрации версий;
нельзя одновременно обещать неизменный Chrome fingerprint и новую policy без review.
STD verified TLS — reference другой реализации, но не автоматическая camouflage
замена. Failover не должен ослаблять security policy ради успешного handshake.

## 3. Применимость, цена и границы

Windows использует эти pinned dependencies; Actual renderer/caller source
присутствует, но выбранный runtime и donor capabilities неизвестны. Android current
graph ShadowTLS отвергает. Возможное использование uTLS для других Android transports
нуждается в отдельной приёмке; наличие core pin не доказывает AAR/native поведение.

Издержки: HRR rejection — уменьшение совместимых donors/сетей; правильная HRR
поддержка — изменённая state machine и both-end migration; version floor — изменение
ClientHello/доступности и причин отказа. Временные/CPU/байтовые gains не измерены.
PSK/certificate/protection epochs остаются под authority платформ/R05, не R02 policy.

License: ShadowTLS и core GPL-3.0-or-later; MetaCubeX/uTLS BSD-3-Clause, scoped texts
сохранены. Готовые primitives/clients **adapt/reference**, reuse без независимой
рецензии HRR freshness **defer**. No product patches/dependency updates/imports.

## 4. Конечная будущая приёмка — NOT_RUN

1. HRR/no-HRR, same/different PSK, final fresh ServerHello: handoff seed provenance
   и C/S agreement, HRR constant никогда не становится заявленным fresh nonce.
2. Условный replay сохранённого first record в другом допустимом внешнем handshake:
   inner handler invocation отдельно от inner codec acceptance и target effect.
3. Неверный/повторный HRR, second ClientHello, fragmented/coalesced ServerHello,
   upstream close/cancel: state не считается protected только по имени переменной.
4. Chrome133 non-ECH с requested min=max1.3: effective config, offered list,
   negotiated TLS и rejection TLS1.2; отдельные STD/ECH и mismatched peers.
5. Strict client/server capability, policy downgrade/failover и old profile migration;
   затем отдельно native/wire/field при явном разрешении.

Сильнейшая граница доказательства: нет crypto/TLS execution и controlled reproduction;
service.go raw fetch timed out, для него сохранён actual module cache source/hash.
v3_client.go совпал с upstream raw004, версия/пины/manifests сохранены. Это авторский
source/model пакет, не ACCEPTED, не disclosure и не продуктовый/native/live PASS.
