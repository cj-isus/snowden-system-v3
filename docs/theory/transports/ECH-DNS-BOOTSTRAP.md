<a id="ech-dns-bootstrap-2026-10-27"></a>
# ECH и DNS: приватность зависит от всего установления соединения

[Оглавление](../README.md) | [Механизмы блокировок](CENSORSHIP-MECHANISMS.md) | [DNS и диагностика](../observability/DNS-AND-DIAGNOSTICS.md)

Дата сверки первоисточников: **2026-10-27**. Основание: RFC 9848 и RFC 9849
(Standards Track, март 2026), RFC 9460 (Standards Track, ноябрь 2023).
Ниже отдельно обозначены нормы стандартов и предлагаемые проектные следствия.
Это не проверка реализации ECH в Snowden, не полевое измерение и не новый
транспортный default. Существующее решение отложить ECH deployment сохраняется.

## 1. Что добавлено к прежней теории

Формулировка «ECH скрывает SNI, но не IP» верна, однако недостаточна для выбора
и диагностики пути. Нужно проверить ещё три границы: **как получена конфигурация,
что клиент делает при отказе и какое имя фактически аутентифицировано**.
RFC 9848 связывает эти вопросы с DNS Service Bindings; одного RFC 9849 недостаточно
для полного DNS-bootstrap контракта. Наличие поля `ech` или TLS extension не
доказывает успешного ECH, а успешный ECH не доказывает защищённого VPN-egress.

## 2. Три имени и один согласованный набор параметров

| Имя / параметр | Роль | Что им нельзя подменять |
|---|---|---|
| Исходное имя сервиса, например `service.example` | Имя, для которого клиент устанавливает прикладное TLS-соединение | Сертификат произвольного DNS target или public name не становится сертификатом исходного сервиса |
| SVCB/HTTPS `TargetName`, `port`, `alpn`, `ech` | Связанный набор параметров одного alternative endpoint | Нельзя произвольно соединить адрес одного пула, ALPN другого и ECHConfig третьего |
| ECH `public_name` | Имя client-facing server, в том числе для аутентификации отказа и получения новых ECH configs | Успешная проверка этого имени не аутентифицирует origin и не разрешает прикладные данные в rejected outer connection |

**Норма:** DNS alias/alternative endpoint не меняет исходное имя для проверки
TLS-сертификата ([RFC 9460 §2.3](https://www.rfc-editor.org/rfc/rfc9460.html#section-2.3)).
При ECH SVCB-параметры прикладного ALPN и SNI относятся к **inner** ClientHello,
а outer определяется ECHConfig
([RFC 9848 §5.2](https://www.rfc-editor.org/rfc/rfc9848.html#section-5.2)).
Для HTTP/HTTPS используется RR типа **HTTPS (65)**, а не запрос RR типа SVCB (64)
с тем же именем: общий формат не делает типы взаимозаменяемыми
([RFC 9460 §9](https://www.rfc-editor.org/rfc/rfc9460.html#section-9)).

**Норма deployment:** при публикации `ech` все адреса TargetName должны вести
к серверам, которые либо имеют соответствующий приватный ECH-ключ, либо
авторитетны для public name и могут выполнить процедуру восстановления.
Иначе возможен полный отказ соединения
([RFC 9848 §4](https://www.rfc-editor.org/rfc/rfc9848.html#section-4)).
Поэтому «IPv4 работает, IPv6 не работает» может быть несовпадением конфигураций
двух пулов, а не доказательством фильтрации IPv6.

## 3. Fallback имеет несколько разных смыслов

| Условие | Что действительно требует или допускает стандарт |
|---|---|
| A/AAAA уже получены, SVCB/HTTPS resolution ещё идёт | Для клиентов в DNS-bootstrap процедуре ECH запрещено отправлять TLS ClientHello до завершения SVCB resolution: **MUST NOT**, RFC 9848 §5.3. DNS prefetch и допустимое предварительное TCP-соединение не равны отправке ClientHello |
| SVCB resolution успешно, все alternative endpoints имеют `ech` | ECH-capable SVCB-optional клиент **MUST** перейти к SVCB-reliant установлению: нельзя обойти отказ всех альтернатив обычным non-SVCB подключением, RFC 9848 §5.1 |
| В одном RRset смешаны endpoints с ECH и без ECH | Активный посредник может блокировать первые и вынудить выбор вторых. Такой rollout **NOT RECOMMENDED**; меньший SvcPriority для ECH помогает выбору без активного противника, но не устраняет downgrade, RFC 9848 §8 |
| Защищённый DNS exchange завершился authentication error, SERVFAIL, transport error или timeout при SVCB resolution | Клиенту **SHOULD** отказаться от попытки достигнуть сервиса, даже если он SVCB-optional, RFC 9460 §3.1. Это условная рекомендация против downgrade, не правило «любой DNS failure доказывает атаку» |
| Сервер отверг реально предложенный ECH, outer authentication/handshake не завершились успешно | Вернуть ошибку; **MUST NOT** использовать retry configs или считать отказ безопасным сигналом отключения ECH, RFC 9849 §6.1.6 |
| ECH отвергнут, outer handshake и проверка public_name успешны, получен поддерживаемый retry config | Завершить это соединение с `ech_required` **до application data**; **SHOULD** повторить handshake на новом transport connection с новой конфигурацией, RFC 9849 §6.1.6 |
| После той же успешной outer-аутентификации нет поддерживаемой версии в retry configs, нет ECH EncryptedExtensions либо согласован более ранний TLS | RFC 9849 §6.1.6 допускает считать ECH безопасно отключённым сервером и рекомендует **новое** соединение без ECH. Это не разрешение отключать ECH после произвольного timeout |

Источники таблицы: [RFC 9848 §§5–8](https://www.rfc-editor.org/rfc/rfc9848.html#section-5),
[RFC 9460 §3.1](https://www.rfc-editor.org/rfc/rfc9460.html#section-3.1),
[RFC 9849 §§6.1.6–6.1.7](https://www.rfc-editor.org/rfc/rfc9849.html#section-6.1.6).
Невалидный формат retry extension не является «безопасным отключением».
GREASE не считается предложением настоящего ECH и обрабатывается отдельно.
В rejected outer handshake нельзя предъявлять client certificate: на запрос
отправляется пустой Certificate; session tickets/session IDs игнорируются
(RFC 9849 §6.1.7). В GREASE-ветке синтаксис ответного ECH extension проверяется,
но полученные `retry_configs` **MUST NOT** сохраняться (§6.2.1). Иначе диагностика
и кэш начнут смешивать два разных режима протокола.

**Проектное следствие, PROPOSED:** если выбранный профиль обещает обязательную
ECH-приватность, он должен явно запрещать и аутентифицированное отключение ECH.
Это более строгая application policy, а не универсальное требование RFC.
Обычный профиль без такого обещания нельзя автоматически объявлять дефектным.
Слова `direct connection` в SVCB-спецификации означают подключение без ServiceMode,
а не разрешение Snowden отправить protected traffic мимо VPN. FR-001/FR-009
сохраняются независимо от TLS/SVCB fallback.

В HTTP Alt-Svc ECH тоже не наследуется молча: RFC 9848 §6 рекомендует публиковать
HTTPS RR с `ech` для каждой alt-authority. Иначе следующий handshake может
раскрыть имя; переход на HTTP/3 сам по себе не сохраняет прежний privacy scope.
[Первичный контракт](https://www.rfc-editor.org/rfc/rfc9848.html#section-6).

## 4. Что наблюдать вместо одного флага ECH

**Норма:** GREASE создаёт видимость ECH extension без согласования настоящего ECH.
Поэтому наличие extension в capture не равно сокрытию inner SNI
([RFC 9849 §6, §6.2](https://www.rfc-editor.org/rfc/rfc9849.html#section-6)).
При реальном ECH клиент проверяет acceptance confirmation, затем обычную
аутентификацию и завершение TLS; один ServerHello не является итоговым успехом
([§6.1.4–6.1.5](https://www.rfc-editor.org/rfc/rfc9849.html#section-6.1.4)).

**Предлагаемая модель наблюдения, не новый API:** раздельно хранить происхождение
конфигурации, режим `not_attempted / grease / real_offer / unknown`, результат
ECH negotiation, authentication scope и результат прикладной операции.
Если нижний TLS API не сообщает факт, значение остаётся unknown, а не выводится
из порта, строки ошибки или наличия `ech` в профиле.

Source-уточнение 2026-10-28: рассмотренный core uTLS wrapper реконструирует
стандартный ConnectionState без native `ECHAccepted`. Даже наличие похожего
boolean в возвращённом типе не доказывает, что adapter передал реальный факт.
[Точная граница преобразования](COMPOSITION-AND-TLS-CONTRACTS.md).

| Наблюдение | Допустимый вывод | Недопустимый вывод |
|---|---|---|
| Получен HTTPS RR с `ech` | Resolver сообщил конфигурацию, с указанными trust/freshness границами | ECH уже применён; resolver независим и не мог подменить ответ |
| Отправлен GREASE | Проверяется совместимость с extension | SNI скрыт |
| ECH принят, origin TLS authentication не прошла | Отказ аутентификации данной попытки | Защищённое соединение готово |
| Public-name handshake успешен, ECH отвергнут | Есть аутентифицированный recovery/disable outcome по правилам выше | Origin либо VPN прошёл проверку |
| ECH+origin TLS успешны | Доказан этот TLS handshake в рамках доверия клиента | Доказаны bulk transfer, native DNS/UDP, отсутствие direct leak или доступность всей сети |
| Timeout до аутентифицированного ответа | Причина не установлена | Доказаны ECH-блокировка, DPI или право отключить защиту |

DoH/DoT защищают участок до выбранного resolver; DNSSEC даёт другой контракт
проверки данных, а не шифрование имён. RFC 9849 §10.2 прямо рассматривает
поддельные и удалённые из DNS ответы ECHConfig. RFC 9848 §8 не обещает скрыть
имя от самого resolver. Нужны явные threat/trust boundaries, а не «защищённый
DNS = вся цепочка достоверна».

## 5. Cache, смена сети и цена восстановления

**Нормы RFC 9849:**

- `config_id` имеет длину один байт и может совпадать у разных серверов или
  переиспользоваться. Это не глобальный key fingerprint, версия профиля или
  доказательство одной trust authority ([§4.1](https://www.rfc-editor.org/rfc/rfc9849.html#section-4.1)).
- Retry config связан с предыдущей конфигурацией. Небезопасно соединять его с
  адресами из нового DNS query: они могут принадлежать другому ECH-пулу/public_name
  ([§6.1.6](https://www.rfc-editor.org/rfc/rfc9849.html#section-6.1.6)).
- Persisted ECH-related state **MUST** быть связан с исходным ECHConfig source,
  например конкретным SVCB ServiceMode record; sharing ограничен тем же source.
  Lifetime/scope рекомендуется ограничивать как другие server tracking vectors.
  Более поздняя новая попытка не наследует исчерпанный single-retry limit старой
  операции ([§6.1.8](https://www.rfc-editor.org/rfc/rfc9849.html#section-6.1.8)).
- Повторный retry config немедленно после retry по config рекомендуется не
  принимать: возможны несогласованные серверные узлы. Это **SHOULD NOT**, а не
  доказательство атаки или абсолютный запрет будущего восстановления (§6.1.6).

**Проектное следствие:** retry входит в один owned dial budget; конфигурация,
адреса и наблюдения не переносятся между network/resolver/profile scopes по
одному hostname. Точный cache key и native cancellation требуют отдельного
source review. Не создавать новый глобальный resolver/вечный ECH cache.
Связанные SVCB/proxy/NAT64 ограничения:
[DNS-дополнение](../observability/DNS-AND-DIAGNOSTICS.md#service-binding-observations-2026-10-27).

## 6. Предел криптографического обещания

**Норма/модель стандарта:** ECH не даёт forward secrecy **для inner ClientHello**,
поскольку ECH-ключ сервера статический. Компрометация этого ключа создаёт риск
для записанных ClientHello, защищённых им. Это не означает расшифровку
прикладного TLS-потока или отмену его собственной forward secrecy.
Регулярная ротация рекомендована
([RFC 9849 §10.10.5](https://www.rfc-editor.org/rfc/rfc9849.html#section-10.10.5)).
При этом старые ключи могут ещё требоваться клиентам с кэшированными configs
(§4.1), а слишком частая ротация ограничивает anonymity set (§10.3).
Поэтому TTL, overlap и удаление старых ключей требуют согласованной политики;
универсальный безопасный период в часах из RFC не следует.

Одинаковый IP и public_name не гарантируют большой anonymity set. Раздельные
ECHConfig для каждого имени могут свести его к одному имени. Даже для K имён
вероятность лучшей догадки не обязана равняться 1/K: популярность и видимые
признаки дают наблюдателю дополнительную информацию
([RFC 9849 §10.1](https://www.rfc-editor.org/rfc/rfc9849.html#section-10.1),
[RFC 9848 §8](https://www.rfc-editor.org/rfc/rfc9848.html#section-8)).
Свой ECH-домен может скрыть буквальное поле SNI, но не обязательно скрывает,
какой сервис используется. Это аргумент против обещания неотличимости, не
измерение текущего цензора и не доказательство бесполезности ECH.

## 7. Конечный следующий срез

Адресаты: **TH-02/03/05, E-04/E-06**, владельцы DNS/transport/diagnostics;
для Android те же вопросы передаются его владельцу без изменения AND-статусов.
Сначала нужен exact public/local TLS API и фактический caller: кто получает
ECHConfig, что сообщает negotiation API, кто владеет retry/cache/dial/Close.
В этом Obsidian-срезе исходники приложения отсутствуют, поэтому этот gap открыт.

| Будущий случай | Что необходимо различить |
|---|---|
| 1. Нет ECHConfig, используется GREASE | Не публиковать ECH privacy success; проверять синтаксис ответа и не сохранять его retry configs |
| 2. Корректный real ECH и origin certificate | Negotiation, TLS authentication и application completion раздельны |
| 3. Старый config, доверенный retry, успешная новая попытка | Первый outer не application success, без client certificate/ticket adoption; bounded общий budget |
| 4. Неверный public-name certificate либо недостоверный retry | Ни adoption config, ни automatic disable ECH |
| 5. Аутентифицированное отключение ECH | Обычная RFC-ветка и proposed ECH-required policy дают разные результаты |
| 6. A/AAAA раньше ответа HTTPS RR | ClientHello не отправляется преждевременно |
| 7. Все endpoints с ECH недоступны; отдельно mixed RRset | SVCB-reliant и downgrade-risk mixed rollout не смешиваются |
| 8. Два DNS-пула, ротация и смена network/resolver realm | Retry config не используется с несовместимыми новыми адресами/source |
| 9. IPv6 endpoint с несогласованным ECH config | Configuration failure не становится причинным «IPv6 заблокирован» |
| 10. Alt-Svc к иной authority | Новый lookup/handshake не наследует privacy только по успеху первого |

Все случаи **NOT_RUN**. Это finite contract matrix, не готовый тестовый код и
не достаточная статистическая выборка для РФ. Future lab использует только
согласованные endpoints, синтетические имена/данные, валидное TLS-доверие,
конечные time/byte/retry budgets. До разрешения ничего не запускается.

Reuse: применять нормы к уже выбранному TLS/resolver seam, не писать свой ECH,
HPKE или DNSSEC validator. Конкретная библиотека/версия ECH здесь не выбрана;
license/SBOM, совместимость wrappers и complete artifact acceptance остаются
условиями будущей интеграции. Больше HTTP-запросов и retries могут ухудшить
latency/energy/privacy; численный выигрыш не измерен.
