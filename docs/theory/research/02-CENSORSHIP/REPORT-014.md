# R02-014 — достоверность сведений об ограничениях мобильных и фиксированных сетей РФ

**27 октября 2026 · R02-014 · READY_FOR_REVIEW**.
Не ACCEPTED и не текущий health-check.

## 1. Вопрос, baseline и предел результата

Baseline — [REPORT-012](REPORT-012.md), [SOURCES-012](SOURCES-012.md),
[THREAT-MODEL](../../foundations/THREAT-MODEL.md). Проверены исходные тексты,
на которых основаны наиболее сильные числа/даты/выводы R02-012; вместо
нового списка «работающих VPN» составлена матрица **области наблюдения**.

Главный результат: R02-012 нельзя использовать как подтверждённую
операторскую карту 2026, ranking хостеров/транспорта или оценку успеха 95/99,5%.
Часть текста достоверно передаёт существование публикации/заявления, но
измеряемый механизм, выборка и современная применимость из него не следуют.
Ни одно спорное число не объявлено ложным только из-за отсутствия методики:
правильное заключение — **UNVERIFIED / scope not established**.

Все внешние acquisition/чтения этого пакета — **27.10.2026**; полные URL,
авторы, публикации, разделы и точные границы чтения — [SOURCES-014](SOURCES-014.md).
Ссылки `S01…S19` ниже обозначают записи этого источникового реестра, не
независимые результаты повторных тестов.

### Что выполнено

- Получены реальные тексты Первого отдела, «Бумаги», Cloudflare,
  исходных РБК/РИА-цепочек, официального сообщения Минцифры и Amnezia.
- Прочитаны выбранные методы/ограничения русскоязычного OONI/RKS Global report,
  отдельный OONI Explorer report с update2026 и опубликованные данные:
  CSV279 доменов,5 summary-записей января 2026,3 raw measurement records.
- net4people #490/#546: прочитаны тело и **видимые** комментарии;
  непрочитанный остаток явно отделён.
- Никаких сетевых проб/сканов/измерителей/benchmark/native действий,
  запуска checker или доступа к настоящему пользовательскому профилю.
  Public HTTPS API чтение **уже опубликованного измерения** — не наша проба.

## 2. R02-014-A — lineage и область числа важнее количества цитат

**Вывод:** ведомственный счётчик, журналистская интерпретация, собственное
provider-наблюдение и публичная measurement record дают разные утверждения.
Копирование числа между публикациями не добавляет ни независимости, ни
операторской полноты. Указание «по данным лабораторных тестов» без автора
тестов/протокола/выборки не обосновывает количественное transport ranking.

### Проверка ключевых утверждений baseline

| Утверждение R02-012 | Что реально прочитано | Решение для использования |
|---|---|---|
| 211 новых VPN за окт 2025–фев 2026, почти столько же, сколько раньше | S01 ссылается на S08; S08 ведёт к РИА 469 (S06) и РБК 258 (S07). Число 211 получается как 469−258. Но S07 говорит «с начала года», а S06 — «ограничивает доступ»; списки/стабильная методика/сопоставимость не опубликованы | Сохранять 211 как **интерпретацию S01**, не как измеренные 211 новых уникальных блокировок/частоту отказов.469 и258 — атрибутированные заявления ведомства, не наш аудит |
| VLESS≥95% / REALITY99,5%, «D1 лучше всех» | S01 карточка 5 действительно содержит проценты и слова «лабораторных тестов»; лаборатория, методика, выборка, продукт и дата измерений не указаны | Числа **не использовать для ranking**, вероятностей policy или обещаний; даже «заявки сервисов» не подтверждены — конкретный сервис не назван |
| Ноябрь 2025 ТСПУ «научился» VLESS, Сибирь/Дальний Восток | S02 интервью пересказывает РБК. Прочитанная РБК-публикация 04.12.2025 (S20) передаёт три market-source и комментарии экспертов; РКН подтверждает только общее развитие безопасности | Жалобы/атрибутированное экспертное объяснение; не установленный packet-classifier и не пооператорская серия |
| Весна 2026: active probing штатно, «один IP → сотня» | S01 говорит Russia since2025 без capture; S02 передаёт уверенность Amnezia, гипотезу Paper VPN и сомнения других сервисов. Сканер/скоринг не тождественны сетевому active probing | Наличие scanning/probing **гипотеза/заявление**, причинная атрибуция РКН не установлена. Подсетевое ограничение — provider reports, не доказанный общий алгоритм |
| 15–20 КБ для Hetzner/DO/OVH, «дешёвый VPS перестал быть классом выживания» | S01 карточка 3 передаёт общее утверждение без собственных данных; S03 Cloudflare различает **свои данные** и внешний report о других поставщиках; S04 — редактируемое полевое сообщение | Исторический per-path симптом, не universal ASN rule/все IP указанных хостеров и не class-wide запрет покупать бюджетный VPS |
| Москва март 2026;71 регион | S01 карточка 4 содержит обе формулировки, но без списка регионов, метода/периода подсчёта и исходной ссылки рядом. S10 непосредственно подтверждает **мобильный пилот 05.09.2025**, не масштаб 71/Москву марта 2026 | Факт существования официального пилота подтверждён **как заявление**.71 и география/продолжительность московского эпизода остаются UNVERIFIED, не ложные |
| VK-видеозвонки: обход «белого списка» | S01 карточка 5 утверждает, но не раскрывает конкретный transport/оператор/дату/данные. S12 прямо отрицает универсальность и описывает variation по сообщениям пользователей | Кандидат/сообщение о реализации, не доказанная стойкость D1–D5 и не гарантия работоспособности текущей сети |
| Telegram «апрель2026 полностью»;18–20 сентября пик | В S02 апрель — редакционная оценка со ссылкой, без nationwide measurement.18–20 сентября — **даты выборов**,25 сентября — peak по письму неназванного VPN, пересказанному «Версткой» | Не смешивать дату события/письма/наблюдения. Nationwide block Telegram не подтверждён этим пакетом |
| DDoS Amnezia, платёжная система,1,5 месяца | S02 — интервью; S09 собственный обновляемый текст с «признаками DDoS», IP-блокировками и продолжающимся расследованием. Точная publication timestamp S09 не найдена | Provider incident report — первичный для опыта сервиса, не доказательство инициатора/механизма на всей РФ. Платёжка и длительность атрибутируются интервью; не independent quantitative verification |
|1667 вступает 01.03.2026, «полный контроль» | S01 цитирует номер; S08 указывает акт **27.10.2025 №1667** и официальную ссылку. Official-text retrieval DNS timeout | Не называть 01.03.2026 датой издания акта; действующая нормативная область не verified. Юридический review отдельно |
|282-ФЗ,8700 сайтов,8931 оператор | S01 содержит, но official law/list/definition/count not read этим пакетом | Не использовать как verified legal advice/измеренный deployment census; числа сохранены как атрибутированные непроверенные сведения |
| Psiphon1,5м/330 ТБ и Ceno при allowlist | S01 карточка 7 без однозначно датированной методики рядом с числами; разные системы/метрики | Не объединять Psiphon/Ceno в одну measured success-rate и не переносить Иран на РФ; primary dataset не установлен |
| CCA → IP-блокировка «становится причиной» | Baseline опирается на paper summary и форумное подозрение. Issue#1447 при чтении 404; full-text paper не reviewed этим пакетом | Причинность **не установлена**; не закрывать loop D7→D2 как факт на основании подозрения |

**Сильнейший контраргумент:** интервью может содержать настоящий опыт
измерителей, а проценты могут происходить из реального теста. Это возможно,
но не даёт читателю воспроизводимого estimand/denominator. Поэтому verdict —
недостаточная идентификация источника/области, **не опровержение результата**.

## 3. R02-014-B — матрица наблюдений с незаполненными ячейками

**Вывод:** по прочитанным материалам допустима матрица scoped observations,
а не MTS/МегаФон/Билайн/T2/Yota league table. Бренд не определяет ASN,
ASN не доказывает radio/access-type, платформа Android не означает cellular,
а тип TLS/HTTP ошибки не устанавливает общий протокол фильтра.

`UNKNOWN` — источник не даёт нужное поле; оно **не** заполняется нулём,
«вся РФ», географией штаб-квартиры или типичным AS бренда.

| ID | Дата наблюдения / дата источника | Регион | ISP/AS и роль | Доступ | Протокол/цель | Наблюдение и уровень |
|---|---|---|---|---|---|---|
| M01 | с09.06.2025 /26.06.2025 | РФ, per-region UNKNOWN | Cloudflare перечисляет Ростелеком/МегаФон/ВымпелКом/МТС/МГТС; per-AS UNKNOWN | UNKNOWN, не разделено по оператору | Cloudflare-protected web; HTTP1.1/2 TCP/TLS, HTTP3 QUIC | Provider telemetry/NEL/Radar: ограничения около 16 КБ, resets/timeouts и h3.protocol.error. Не universal rawUDP/HY2 handshake outcome (S03) |
| M02 | сообщение 28.06.2025 / комментарий того же дня | UNKNOWN; автор говорит depends on region | t2/МегаФон/Билайн/Yota/МТС; AS UNKNOWN | mobile — по словам автора | foreign-datacenter TCP/TLS, per-connection server→client | Примерно 15–20 КБ, не все операторы; field report без denominators (S04 #3015159747) |
| M03 | дата UDP-опыта UNKNOWN / сообщение 28.06.2025 | UNKNOWN | ISP/AS UNKNOWN | UNKNOWN | UDP к неназванному endpoint; приложение UNKNOWN | Автор сообщает cooldown около 10min. Это не QUIC handshake test и не nationwide UDP ban (S04 #3016016566) |
| M04 | upd5 недатирован / issue27.06.2025 | UNKNOWN | UNKNOWN | UNKNOWN | TCP/UDP any direction |25packets — поздний изменяемый body-claim; дата/packet-trace UNKNOWN, не смешивается с M01/M02 (S04) |
| M05 | с12.11.2025 /14.11.2025+updates | Москва | МТС/МГТС, AS UNKNOWN | home по теме source, технология UNKNOWN | VLESS+REALITY(+Vision), TCP443 | partial, иногда router reboot; cooldown≈60s; normal HTTPS работает. Remote/policing hypothesis, не established DPI mechanism (S05) |
| M06 | с12.11.2025 /14.11.2025 | UNKNOWN | JustLan/LanInterCom; AS UNKNOWN | home, технология UNKNOWN | тот же класс issue#546 | Partial/unknown число пользователей; не переносится на mobile и не одинаковый statewide режим (S05) |
| M07 |01.11.2025 /14.11.2025 | Ижевск | RTK (как написано автором), AS UNKNOWN | home по теме source | VLESS/TLS family issue#546 | Один report; no prevalence/causal attribution (S05) |
| M08 | с начала 2024 по словам автора /22.11.2025 | Сыктывкар | Beeline, AS UNKNOWN | **UNKNOWN** | Shadowsocks, точная версия UNKNOWN | «Не работает» одного автора; соседний комментарий про mobile не уточняет его access (S05 #3566897493) |
| M09 | пилот к05.09.2025 /05.09.2025 | РФ, конкретные регионы UNKNOWN | Минцифры/операторы, per-ISP/AS UNKNOWN | **mobile — официально** | востребованные сервисы, protocol UNKNOWN | Объявлен whitelist-like доступ во время ограничений, без spec/results; policy declaration (S10) |
| M10 | к05.09.2025 /05.09.2025 | UNKNOWN | МегаФон/ВымпелКом; AS UNKNOWN | mobile | ресурсы списка | Представители сообщили о работающем механизме; МТС не комментировал. Это не контрольный тест каждого ресурса/порта (S11) |
| M11 |16.09.2024 04:57:16Z | РФ, город UNKNOWN | probe AS12668 KomTehCentr; resolver AS42728 MSK-IX; answer AS31483 ER-Telecom | **UNKNOWN**, Windows client не classifier | explorer.ooni.org, A→TCP443→TLS | DNS inconsistent, TCP success, TLS eof, control200; blocking=dns; S15 связывает answer-IP с fingerprint. Прочитанная архивная record (S16) |
| M12 |31.01.2026 20:33:10Z | РФ, город UNKNOWN | probe/resolver AS8402 Vimpelcom | **UNKNOWN**, macos client не fixed-proof | explorer.ooni.org, IPv4 TLS1.3/h2 | TCP/TLS success; response code200, HTTP generic_timeout_error, saved-body пуст; control200. API anomaly=true, confirmed=false (S18) |
| M13 |31.01.2026 20:21:18Z | РФ, город UNKNOWN | probe/resolver AS8615 Central Telegraph | **UNKNOWN**, Android client не mobile-proof | тот же URL, IPv4 TLS1.3/h2, другие IP | TCP/TLS success, HTTP200 nonempty saved-body, accessible=true, nofailure. Positive URL outcome того момента, не whole-network availability (S19) |

Для M12/M13 API summary выбраны из 5 последних записей января 2026 (S17).
**Это целевой разбор одного negative и одного positive record**, не sample
для rate и не matched causal comparison: разное время/IP/резолвер/engine.

### Разделение stages, обязательно для интерпретации

1. **UDP reachability:** сообщённые отправка/ответ UDP; handshake QUIC не
   следует из факта UDP ответа. Его отсутствие может иметь endpoint/ACL/NAT/
   transport причины, а не общую UDP-фильтрацию.
2. **QUIC handshake:** отдельный negotiated результат. M01 содержит
   HTTP3 errors, но не открытый per-ISP handshake corpus; у S16–S19
   `control.quic_handshake={}` — не отрицательный результат, а пустой объект.
3. **TCP establishment:** успешный connect не подтверждает data path;
   M11 и M12 дают конкретные примеры разных последующих отказов.
4. **Volume vs time vs count:**15–20 КБ,25packets,≈60s и≈10min принадлежат
   разным заявлениям/эпизодам. Одна ось не заменяет другую; 25 TCP packets
   не переводятся в16 КБ без payload/ACK/направления/MTU/retransmission definition.
5. **CGNAT/IPv6:** в рассмотренной матрице **не установлены**. A-record и
   IPv4 dial не доказывают отсутствие IPv6/CLAT, shared public IP — не
   доказательство CGNAT без network facts. Не объявляем IPv6 надёжным обходом.
6. **Allowlist:** официальное сообщение о разрешённом наборе не определяет
   IP/SNI/портовый enforcement. HTTP index/UI списка не гарантирует полный
   dependency graph разрешённого приложения. Wi-Fi не тождественен fixed:
   у AP может быть мобильный upstream (S12, комментарий, не измерение каждого AP).

## 4. R02-014-C — область источника и наблюдаемость ограничивают диагноз

**Вывод:** отсутствие свежих измерений — UNKNOWN; историческая блокировка
не постоянный health status. Browser/probe error доказывает стадию сбоя, но
не автоматически DPI/ТСПУ/allowlist. Для причинного вывода нужны контроль и
область, соответствующие утверждению, а не количество одинаковых перепечаток.

### Основания

- S14 (OONI) отделяет OK/confirmed/anomaly/failed. Автоматическое confirmed
  в проверенной методике WebConnectivity — known blockpage/DNS fingerprints;
  reset/timeout обычно anomaly и требуют анализа coverage/consistent-errors.
- S13 — настоящий **русскоязычный отчёт измерителей**, с разделами Methods,
  dataset/CSV и limitations; анализ 01.09.2023–01.09.2024 ограничен NEWS и
  volunteer coverage. Его 279 доменов не превращаются в279 недоступных VPN,
  representative mobile-count или текущую вероятность успеха 2026.
- В CSV S13 `www.bbc.com`:36674 observations/289 AS coverage,22 AS с
  confirmed fingerprint. **Остальные 267 AS не означают 267 доступных сетей**:
  blocking other than fingerprint остаётся anomaly; coverage/denominators
  различны. Сам подсчёт строкCSV=279 — адресная сверка публичного корпуса,
  не повторное доказательство блокировки каждой строки.
- S15: block notice11.09.2024 и spike-анализ; later unblock notice02.09.2025,
  но авторы сообщают признаки блокировки части сетей в январе 2026. Юридическое
  уведомление и actual rollout — разные события. Наша S18/S19 детализация
  показывает different outcomes, но не самостоятельно подтверждает intent.
- M11: AS-probe, AS-resolver и AS-answer различны. DNS return censorship-IP
  нельзя приписывать именно probe ISP только потому, что test `probe_cc=RU`.
  Source S15 поддерживает fingerprint-интерпретацию; точный механизм
  доставки ответа/администратор правила остаются unknown.

### Сильнейшие альтернативные объяснения

- HTTP generic_timeout после TLS может происходить из-за congestion/server/
  CDN branch, проблемы body чтения, устройства/измерителя или фильтра.
  M12 control success увеличивает содержательность, но не устраняет все причины.
- Согласованные failure нескольких пользователей могут разделять common CDN/
  resolver/router outage; число сообщений не даёт независимый sample.
- Один положительный большой ответ M13 опровергает лишь «этот URL всегда
  недоступен на всех путях РФ в этот период»; он не опровергает scoped
  Cloudflare paths июня 2025, mobile allowance или filtering других SNI.
- Отсутствие опубликованной методики процентов не доказывает, что эксперимента
  не было. Работа с UNKNOWN не допускает замены «не проверено» на «никогда».

## 5. Применение к системе без изменения продукта

**Windows и Android:** external report должен быть датированным ограниченным
свидетельством, а не Ready/permit текущего пути. Требования TH-02 наблюдений,
TH-03 dependency/provenance и TH-06 сравнительной оценки — адресаты,
не закрытые product statuses. Платформенный producer остаётся владельцем
actual network facts/observation. Не вводим prior95/99,5%, current-MTS verdict
или IPv6 ranking из чужого текста.

Минимальные поля внешнего свидетельства (предложение модели, не API):

```text
source_id, source_url, publisher, publication_time, observation_interval,
reading_scope, evidence_kind, derivation_parent[],
probe_asn?, resolver_asn?, endpoint_asn?, region?, access_type?,
target, address_family?, protocol_stage, workload?,
outcome, classifier/version?, control?, coverage?, missing_fields[],
mechanism_hypothesis?, attribution_limit, applicability_limit
```

Эта схема не выдаёт identity/device-secret и не требует нового telemetry-stack.
`reading_scope` не подменяет `evidence_kind`: FULL_TEXT_READ журналистики
остаётся журналистикой; DATA_READ не становится собственным опытом.

**Сервер:** один origin/ASN может быть общим риском, но сам public report
не определяет current reachability нашего origin. Нет оснований обещать
«болгарский VPS безопасен», «бюджетники мертвы» или «HY2 не страдает IP-filter».
Рекомендации менять страну/ASN/порт требуют собственного ограниченного evidence
и разрешённой проверки; эта работа ничего не развёртывала.

**Reuse и цена:** используем уже опубликованные OONI measurement formats и
проверенную логику stage/control/coverage; не импортируем OONI engine/API client
в продукт. Лицензия на код/данные для будущей интеграции отдельно NOT_REVIEWED,
наличие footer Creative Commons не является permission/full licensing review.
Цена — сохранять provenance и UNKNOWN, не строить красивую, но ложную карту.

<a id="6-адресные-предложения-руководителю-без-записи-в-общие-файлы"></a>
## 6. Предложения по применению

1. У R02-012 сохранить исторический текст, но пометить 95/99,5% и «D1 лучше
   всех» как неподтверждённые количественные утверждения; убрать их из оснований
   policy, не заменяя собственной оценкой.
2. Отделить число 211 как интерпретацию счётчиков от измеренных новых блокировок;
   раскрыть chain S01→S08→S06/S07, даты и неизвестную сопоставимость.
3. «Весь риск платформенного камуфляжа — только D6» не следует из источников:
   остаются transport reachability, dependency/relay behaviour и изменение
   enforcement. «HY2 выживает D3» не доказано QUIC label. «Нет технического
   кандидата» может быть **условной моделью** лишь при явно недостижимом
   endpoint/непригодном permitted relay, а не nationwide field conclusion.
4. CCA-подозрение не делать установленной причиной IP-block. Full-text paper/
   actual classifier/causal data — отдельный gate другого тематического пакета.
5. Даты выборов 18–20 сентября не peak25 сентября; publication time не время
   происшествия; official1667 date не предполагаемая effective date.
6. Каноническую операторскую карту вести с UNKNOWN-ячейками и stage-specific
   observations; эту матрицу не считать актуальным состоянием 27.10.2026.

## 7. Конечные непроведённые cases / остаток

Никакие execution cases не созданы как тесты и не выполнены; ниже только
предел текущего desk research и адресные условия следующего этапа:

- V01: найти named primary лабораторию 95/99,5% и методику либо окончательно
  оставить источник unidentified; не измерять «по пересказу».
- V02: получить исходный deck/состав sets258/469 для проверки comparable census;
  арифметика сама не докажет 211 новых блокировок.
- V03: источник 71 регион/московского эпизода с observation dates и
  классификацией ограничения; пресс-заявление не заменит technical outcomes.
- V04: полнота/контроли OONI на одном **заданном** временном/ASN/target scope;
  five-latest sampling не станет monthly rate.
- V05: разрешённая future matched программа `ISP/AS × регион × access ×
  family × stage × workload × время`, с DNS/TCP/TLS/QUIC и контролями отказов;
  **NOT_RUN**, текущий запрос её не разрешает.
- V06: CGNAT/IPv6/CLAT facts и useful-task/protected-egress, не одни A/TCP probes;
  **NOT_RUN**, privilege/контроль/privacy согласуются отдельно.
- V07: официальные правовые редакции 1667/282-ФЗ и отдельный review,
  прежде чем использовать их для legal/UI advice; **FETCH_FAILED/NOT_REVIEWED**.

Окончательный статус — **READY_FOR_REVIEW** только A/B/C в указанной
source/method области. Fresh РФ measurements, current VPS reachability,
эксплуатационная надёжность/ускорение/обход/правовая оценка не установлены.

## 8. FUP-2-01 / R02-014-D — сопоставимость raw outcomes и summary labels OONI

[Область A](../reviews/NINE-2026-10-27-R02-014-A.md) принята отдельно;
B/C и дополнительная D остаются **READY_FOR_REVIEW**. Дополнение использует
три прежние raw записи S16/S18/S19 и сохранённый summary S17,
без новых сетевых измерений.

### 8.1 Ответ на вопрос сопоставимости

**Частично сопоставимы поля и проверенный участок engine-классификации,
но не доказано тождество всей классификации трёх записей/backend labels.**
У всех записей `data_format_version="0.2.0"`, `test_name="web_connectivity"`,
`test_version="0.4.3"`; это не тождество app/engine/backend implementations.
Два desktop raw records раскрывают exact VCS revision; Android раскрывает
engine version, но revision пуст. Версия классификатора API-summary в
сохранённом ответе не предоставлена.

| Запись | App / engine | Exact source revision из annotations | Raw test_keys | Summary API из прежнего capture |
|---|---|---|---|---|
| S16 / AS12668,16.09.2024 | desktop3.22.0 / engine3.22.0 | `97f1df33611097fdcd8e0cc53fe096c0c7727487`, vcs_modified="false" | accessible=false; blocking="dns"; control_failure=null; HTTP eof_error; x_status=8480 | Соответствующего summary **нет** в сохранённой выборке; anomaly/confirmed/failure не достроены |
| S18 / AS8402,31.01.2026 | desktop3.26.0 / engine3.26.0 | `e9b4dc98c04d347f0ee5468c2585e32d94812b73`, vcs_modified="false" | accessible=false; blocking="http-failure"; control_failure=null; HTTP generic_timeout_error; x_status=8704 | anomaly=true; confirmed=false; failure=false; verification_status="unverified" |
| S19 / AS8615,31.01.2026 | Android app5.3.0 / engine3.28.0 | vcs_revision/vcs_time/vcs_tool/vcs_modified **присутствуют как пустые строки** | accessible=true; blocking=false (boolean); control_failure=null; HTTP failure=null; x_status=1 | anomaly=false; confirmed=false; failure=false; verification_status="unverified" |

`vcs_revision` — self-reported provenance в опубликованной записи. Чтение
файлов этого commit устанавливает source-path, но не аттестует бинарник автора
измерения. Android app5.3.0 нельзя подменять engine release3.28.0 или текущим
main без проверки exact mapping. Main-ветка не использована.

### 8.2 Какие поля относятся к разным слоям

**1. Raw operation observations.** `queries[].failure`,
`tcp_connect[].status.failure/success`, `tls_handshakes[].failure` и
`network_events[]` с operation/tags, `requests[].failure`/response code,
`http_experiment_failure`, `control_failure`,
`control.http_request.failure`. Failure строка/null относится к конкретной
операции/эксперименту, а не к boolean статуса всей записи. TCP status.blocked
уже содержит сравнение с control, а не только первичное наблюдение.

**2. Engine analysis в `test_keys`.** `dns_consistency`, сравнения HTTP,
`accessible`, `blocking`, `x_status` — производные поля измерителя.
`blocking` имеет union-тип **null | false | string**, а не boolean;
`accessible` — **null | bool**. Проверка отсутствия/null/false обязательна.
В трёх сохранённых raw records **нет** `anomaly`, `confirmed`, `failure`
ни в root, ни непосредственно в test_keys. Это не значения false:
поля отсутствуют. `blocking`/`accessible` присутствуют только внутри test_keys.

**3. Backend/API summary.** У двух имеющихся summary S17 есть
`anomaly`, `confirmed`, `failure`, `scores`, `verification_status`;
`blocking`/`accessible` как верхние поля отсутствуют.
У S18 `scores.analysis.blocking_type="http-failure"`, у S19
`analysis` отсутствует. API `failure=false` совместимо с raw HTTP timeout
S18 — поля относятся к разным уровням. Для точного правила получения API
boolean/score нужен backend revision, **не раскрытый этим capture**.
`blocking_general=1.0` не назван нами вероятностью цензуры;
`verification_status="unverified"` не превращает операционные поля в null
и не означает, что никакой тест не проводился.

**4. Интерпретация автора/публикации.** «DNS interference», «confirmed по
fingerprint», «полезный HTTP outcome», «mobile/fixed», «DPI-причина» —
разные утверждения поверх записи. Например, в S16 raw `blocking="dns"`
не является raw полем `confirmed=true`. Связь answer-IP с known blockpage
в прежнем C происходит из OONI-публикации S15; API confirmed для S16
в имеющихся captures неизвестен. Не присваиваем его по сходству.

### 8.3 Exact source путь для S18: HTTP timeout → http-failure

Все перечисленные файлы прочитаны по commit
**e9b4dc98c04d347f0ee5468c2585e32d94812b73** из S18, не по main;
конкретные URL и hash — SOURCES §5 и artifacts `fup-*.meta.json`.

1. `webconnectivity/webconnectivity.go:17–18` объявляет 0.4.3.
   `Measurer.Run:128–129` связывает `TestKeys` с `measurement.TestKeys`.
   `:160–175` сохраняет control result/failure; `:189–213` собирает
   TCP/TLS probes; `:216–229` выполняет HTTPGet, сохраняет
   HTTPExperimentFailure/Requests и вызывает **Summarize(tk)**.
2. `webconnectivity/httpget.go:48–69` вызывает
   `urlgetter.Getter.Get` и возвращает его Failure/TestKeys.
   `urlgetter/getter.go:43–81` нормализует error, читает saver events,
   получает Requests через `tracex.NewRequestList`; `:69–73` явно
   оговаривает: **последний запрос находится первым**. Поэтому
   `Requests[0]` — не автоматически первая попытка redirect-chain.
3. В S18 есть **один** Requests element: failure=generic_timeout_error,
   URL=https://explorer.ooni.org/, response code200. ControlFailure=null,
   control HTTP failure=null; dns_consistency="consistent";
   TCP records success, TLS1.3/h2 завершены. Отдельные TCP/TLS probes
   в `network_events` tagged `tcptls_experiment`; их TLS success не
   автоматически receipt HTTPGet-потока — это разные измерительные операции.
4. `summary.go:118–129`: shortcut secure-success не срабатывает, так как
   Requests[0].Failure ненулевой. Control-unreachable guard не срабатывает.
   DNS failure branch `:167–173` не срабатывает; connect-failure branch
   `:178–195` не соответствует сохранённым успешным TCP attempts.
   Control HTTP failure/no requests guards `:199–206` не срабатывают.
5. `summary.go:211–245`: branch FailureGenericTimeoutError выставляет
   BlockingReason="http-failure", Accessible=false и
   StatusExperimentHTTP | StatusAnomalyUnknown. DNS-override `:264–267`
   не срабатывает из-за consistent DNS. Flags **8192|512=8704** —
   согласованы с сохранённым x_status S18 (ручная арифметика, не run).
6. `summary.go:98–102` defer вызывает DetermineBlocking;
   `:84–89` возвращает pointer reason, если Accessible не true.
   JSON Summary.Blocking становится строкой"http-failure".
   Текст ошибки **не устанавливает причину цензуры**: это classifier label.
7. `webconnectivity.go:251–272` содержит **локальный** summary adapter:
   `IsAnomaly = BlockingReason != nil`, а Anomaly() возвращает это поле.
   Это объясняет engine-side возможность anomaly, но не аттестует
   backend-классификатор, который сформировал API response S17. Здесь есть
   дополнительная потеря состояния: локальный SummaryKeys содержит bool
   Accessible, и `:265` превращает raw Accessible=nil вfalse; пустой
   BlockingReason даёт summary Blocking="" и IsAnomaly=false, хотя
   x_status может содержать flag control-unreachable. Это engine adapter,
   **не доказанный mapper полей сохранённого API-summary**.

Дополнительный source факт: `netxlite/classify.go:93–103` может нормализовать
разные error suffixes (`context deadline exceeded`, `i/o timeout`,
`TLS handshake timeout`) в один FailureGenericTimeoutError.
`ErrWrapper` сохраняет и failure, и operation; `errwrapper.go:84–105,130–131`
не заменяет классификацию уже wrapped error. Не восстанавливаем из одной
строки timestamp/слой/механизм исходного timeout.

### 8.4 Проверка S16 и ограничение сравнения версий

Полученные `summary.go` и `webconnectivity.go` по **двум recorded commits**
3.22.0/3.26.0 совпали побайтно. SHA256 summary:
`26327ff540f3b93eb1152a962df14e2ff162c78bdf6bc84a9cebaabb5f23776d`;
SHA256 caller:
`0d93b452533223f0f9449f8f424a51b5039c9a2e9b9504c58cd8cdf62afb91b4`.
Это **не whole-engine parity**: остальные dependency/caller semantics не
сравнивались целиком, source Android exact revision неизвестен, backend
version unknown. Одинаковый test_version не заменяет такую проверку.

Для S16 (один request=eof_error, inconsistent DNS, control доступен)
тот же Summarize сначала даёт HTTP read/write anomaly (`:234–239`), затем
one-request DNS override (`:264–267`) меняет reason на"dns".
Flags **8192|256|32=8480**, как в raw. Поэтому `blocking="dns"`
не утверждает, что TCP connect провалился: connect в S16 успешен, TLS eof;
это **приоритет анализа**, основанный на согласованности DNS.

### 8.5 Противоположный пример: одна строка timeout — разные labels

**Ручной условный source trace, не четвёртая запись и не synthetic fixture.**
Сохраняем Requests[0].Failure=generic_timeout_error в рассмотренном
historical Summarize. ВариантI соответствует guards S18: control reachable,
HTTP control succeeds, consistent DNS, requests есть — получается
Accessible=false / Blocking="http-failure" / Status8704.

ВариантII: **при том же request failure** control не получен
(`ControlFailure != nil`). Guard `summary.go:126–128` возвращает раньше
timeout switch: Accessible=nil, BlockingReason=nil, а deferred
DetermineBlocking даёт JSON **blocking=null**, статус control-unreachable
(8), не"http-failure". ВариантIII: control reachable, один request,
DNS inconsistent — one-request override меняет reason на"dns".
Ни один вариант не доказывает цензуру или неисправность измерителя;
не предсказываем boolean API failure для вариантовII/III без backend pin.

### 8.6 Адресное уточнение прежнего B/C: явный Wi-Fi annotation

В полном inventory S19 найдено `annotations.network_type="wifi"`.
В прежних §3 M13/S19 SOURCES это поле не было учтено: **локальный network type
не отсутствует**. У S16/S18 ключ network_type отсутствует; у S19 присутствует
как wifi. Это добавка-уточнение к ранее сданному материалу, не исправление A.
Wi-Fi сообщает локальное подключение приложения, но fixed/cellular upstream,
CGNAT/IPv6/операторский тип канала по-прежнему UNKNOWN. Android-record не
назван сотовым. Исходные raw bytes всех трёх records совпадают с initial
receipts — факт найден в прежнем capture, новых измерений нет.

### 8.7 Итог D и остаток

R02-014-D READY_FOR_REVIEW: три слоя (operations → engine analysis → API)
разделены; для S18 выполнен **один exact-version source trace**. Указанный
участок двух desktop versions одинаков; полная взаимозаменяемость трёх
engine/backend labels не доказана. Сохранять тип, наличие поля, scope/control,
engine revision и неизвестный backend pin; не унифицировать их в bool
«blocked». Сетевых probes, backend replay, tests/build/fixtures нет.

Остаток ограничен exact Android revision и backend version/caller для
API anomaly/confirmed/failure/scores. Они не раскрыты в используемых записи
и summary: **UNKNOWN**, не main-based reconstruction. Новая массовая
выборка и дальнейший обзор операторов для этого результата не нужны.

