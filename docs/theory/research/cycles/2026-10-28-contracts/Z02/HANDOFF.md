<a id="r28-z02--конечная-передача-сопоставимый-блок-reachability"></a>
# R28-Z02 — сопоставимость наблюдений доступности

28 октября 2026. status: READY_FOR_REVIEW

Основной результат: [REPORT](REPORT.md), точные первоисточники и read depth:
[SOURCES](SOURCES.md).

## 1. Ответ на главный вопрос

Минимальный блок определяется **силой утверждения**, а не постоянным числом
измерений. Один идентифицированный результат может подтверждать, что
измеритель сообщил success конкретной операции. Для сопоставления нужен join
по stage/endpoint/time context; для причинного или популяционного вывода —
отдельные предпосылки контроля и покрытия.

В выбранном fixed corpus есть адресно сопоставимый **TCP** local/TH result,
но нет доказанного совпадения **HTTP** remote endpoint и времени контролей.
Поэтому reported TCP success допустим как узкое свидетельство; «HTTP-таймаут
вызван ISP», «весь оператор недоступен», «16КБ cap», «получено0байт», «этот
VPN работает сейчас» остаются unknown.

## 2. Что нового сверх baseline

1. **DNS consistent не означает одинаковые IP.** Exact historical caller
   принимает common nonzero AS как достаточный критерий; local и control
   DNS sets выбранных 3records не пересекаются.
2. **HTTP-контроль не address-matched TCP-контроль.** THRequest передаёт URL
   отдельно от IP pairs; сохранённый HTTP control result не содержит chosen
   remote endpoint/operation-time. Общий домен/ASN не закрывает этот пробел.
3. **Пустой snapshot при HTTP timeout не счётчик 0 байт.** Saver записывает
   status/headers до чтения тела, а при read error не присваивает partial
   body событию. Это verified source property, не измеренное число bytes.
4. **Успех HTTPS summary не всегда full-body completion.** В условном exact
   source trace saver может сохранить successful prefix, а Runner получить
   ошибку на хвосте. Requests.failure=null и HTTPExperimentFailure!=null
   совместимы; HTTPS shortcut читает первый, не второй. Fixed S19 имеет
   оба failure=null — данный контрпример не объявлен фактически случившимся.
5. Продолжение TASK выполнено: existential positive и universal negative
   требуют разных coverage; unknown strata не становятся национальной rate.

## 3. Паспорта scoped claims

### R28-Z02-A — адресно-временная сопоставимость зависит от операции

| Поле | Ограниченный результат |
|---|---|
| Тип | Published field observation + exact source + primary specification; не собственное измерение |
| Evidence passport | Основной UID20260131203410.740128_RU_webconnectivity_d10431598d631e23; measurement_start31.01.2026 20:33:10UTC; self-reported RU/AS8402; access/city unknown; IPv4 TCP443/HTTP GET; engine3.26.0/test0.4.3; recorded commit e9b4dc98c04d347f0ee5468c2585e32d94812b73 |
| Exact caller | Measurer.Run localDNS/endpoints → THRequest(URL отдельно от IP pairs) → Connects/ComputeTCPBlocking; HTTPGet → Configurer → static DNS cache → Runner/Saver → Requests → Summarize |
| Поддержанный вывод | Record сообщает TCP success к76.76.21.93:443 и66.33.60.194:443; контроль содержит success тех же endpoints. HTTP GET имеет code200 и timeout; negative полного GET относится только к этому request-context |
| Strongest opposing case | Контрольный HTTP GET может выбрать другой CDN front, даже при consistent DNS/common AS. Code200/empty saved body не доказательство нулевого принятого payload; отбрасывание partial snapshot показано source |
| Existing guard | Runner действительно дочитывает whole body и сообщает error; source не объявлен полностью слепым. TCP control comparison адресный. Потеря гарантии возникает при неправильном объединении полей/стадий |
| Граница | TH exact timing/remote HTTP IP, true clock offset/producer truth, backend revision и Android exact source неизвестны. Main не подставлена, бинарник не аттестован |
| Предлагаемый verdict | Принять только source-bound passport/decision-use limitation после review; не current reachability/цензура/диагноз ISP |

### R28-Z02-B — unknown strata ограничивают перенос свидетельства

| Поле | Ограниченный результат |
|---|---|
| Тип | Ручной логический вывод для конечного designated D; не statistical estimator/новая rate |
| Основной аргумент | Один достоверный success внутри D свидетельствует exists-success. Все-failure утверждение требует отрицательного evidence всех designated cells; хотя бы одна unknown оставляет universal negative unknown |
| Конечная формула | При known finite N=s+f+u>0, равных весах и корректных stage classifications: s/N≤theta_D≤(s+u)/N. Края — заполнение всех unknown0 или 1; независимость/MAR не нужны |
| Strongest opposing case | Same published block совместим с неограниченно различающимися unobserved populations. Если N неизвестно либо ошибочны классификации/принадлежностьD, bound не вычисляется/не действует. API5latestrows не denominator РФ |
| Temporal limit | Snapshot/DNS TTL и общий день не гарантируют stable endpoint/time validity. ОтветDate/Age сообщает возраст representation, не выполнение TH. Publisher/collector timestamp не timestamp каждой network operation |
| Предлагаемый verdict | Принять только условный finite-D аргумент; отклонить national probability/universal outage по partial sample |

### R28-Z02-C — готовое reuse не равно бесконтрольной интеграции dataset

| Поле | Ограниченный результат |
|---|---|
| Ready reuse | OONI spec pin8a68ed35b1dfeaeec737c0f1e8985865828d75d7: presence/null/types/transaction scopes; BSD3 LICENSE прочитан. Это modern normalisation reference, не historical instrument |
| Data licence | OONI licence pince329e1d6a2a0d0922d27557123e07e463b7c1e9: CC BY-NC-SA4.0, полный лицензирующий текст прочитан; notices/NC/SA и exceptions отдельно |
| Code reference | Probe-cli e9b4dc98… reference-only; GPL3 header, не full legal dependency audit и не embedding engine |
| Стоимость | Versioned reader/type-presence и source/provenance index; closed missing fields; licence/legal basis перед commercial-data use. Не новый collector/telemetry stack, численный RAM/CPU/cost benefit не измерен |
| Контрпример | Licence/data availability не permission commercial ingestion или endorsement. Current schema optional fields не восстанавливают отсутствующие historical IDs |
| Предлагаемый verdict | Reuse schema reasoning и frozen scoped research; defer production ingest/code reuse до соответствующего legal/platform review |

## 4. Versioned crosswalk и несовместимые уровни

- Raw operation failures/null/TCP success — stage-specific observations.
- Engine test_keys.blocking/accessible/x_status — условный анализ измерителя;
  не backend confirmed и не useful-task outcome.
- API anomaly/confirmed/failure/scores — отдельная summary проекция;
  её revision отсутствует в selected capture, не реконструирована по main.
- Reported observation time, local event t, test_start набора,
  upload/collector-time и HTTP cache Date/Age — разные времена.
- Android network_type=wifi означает local association, не fixed upstream.
- Three UID records не повторения одного comparable trial; S18/S19 разные
  AS/time/IP/engine. Их нельзя использовать как matched estimate ISP effect.

Полная таблица и7missing premises — REPORT §§2–4/9. Нормализация хранит
field absence/null/false, origin, raw-ref/hash, stage, source revision и
capability limits; не теряет различия в одном bool blocked.

## 5. Paired contract и последствие для нашей системы

На review-stage C02 HANDOFF был уже доступен и прочитан **только по
связанному action/validation claimA**. Не ожидали соседей, не писали им.
Его ECH source claims не независимо reviewed. Согласие scope — не
corroborating measurement и не scientific ACCEPTED.

Reachability/cause unknown допускает новую **безопасную конечную попытку**
у platform owner, но не выдаёт credentials/user-traffic permit, не снижает
confidentiality/identity/floors и не заменяет actual owned validation.
Windows/Android producers сохраняют источник/время/операцию/unknown;
portable policy не объявляет старую публикацию нынешним health-check.
Новых product IDs/status/API/source changes этим заданием нет.

<a id="6-work-stages-и-конечная-сдача"></a>
## 6. Область выводов и незавершённые проверки

| Stage | Фактический результат |
|---|---|
| Baseline/source availability | Named sections доступны;3scoped public records сохранены; exact desktop pin свежо перечитан. Android/backend missing явно |
| Основной argument | Один TCP/HTTP comparability passport, typed crosswalk, joins проверены чтением исходников; limits явно отделены |
| Strong opposing | Same timeout/different controls; DNS AS vs IP; discarded partial body и успешный prefix/ошибка tail; HTTPS content denial; dashboard/upload/clock paths |
| Продолжение внутри TASK | Positive/negative quantifier strength при unknown strata; finite-D bound с предпосылками; primary temporal-DNS/source изменяет границу вывода |
| Reuse/licence/cost | Schema/data/code licence scopes разделены и pinned; no new dependencies/collector/privacy harvest |
| Handoff | Все claims scoped, author READY_FOR_REVIEW; closed missing list и будущие cases NOT_RUN |

Дальнейшее требует нового exact input или платформенного измерения.

Не выполнены tests/fixtures/build/benchmarks/simulation/crypto/scans/
native/device/live/admin/install/OS network/VPS/SSH/deploy; не читались
secrets/env/product vault/real profiles/raw user logs. No agents/schedules,
provider settings changes, git mutations, соседние/общие записи.
