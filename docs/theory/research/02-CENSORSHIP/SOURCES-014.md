# R02-014 — источники и границы чтения

Дата чтения: **27.10.2026**. Сетевых проб не было: запросы к публикациям
и API загружали уже опубликованные тексты и измерения, а не исполняли
измеритель на пути владельца.

## 1. Достоверность acquisition и статусы

`WebFetch` для четырёх начальных ссылок дал timeout; затем публичные HTTPS
страницы получены `curl 8.14.1 / Schannel` с проверкой сертификатов, без `-k`.
Сохранены исходные response body (`*.raw`), извлечённый текст (`*.txt`), URL,
effective URL, время acquisition, HTTP-код и SHA-256 (`*.meta.json`).

Каталог: `.artifacts/research/theory-02/2026-10-27-mobile-operators-res2/`.
Скрипт `fetch_public.py` — только acquisition; `summarize_public_measurements.py`
извлекает поля **опубликованных** JSON, не запускает тестов и не оценивает VPN.
`reading_status: ACQUIRED_NOT_YET_READ` в acquisition-meta намеренно сохранён
как исходный этап; окончательная область чтения — этот реестр и reading-ledger.

Обозначения: FULL_TEXT_READ — прочитан содержательный текст страницы, не
каждый динамический widget/внешний комментарий; PARTIAL_TEXT_READ — только
названные секции/видимые сообщения; DATA_READ — прочитаны названные поля данных;
LINK_ONLY — URL известен, текст не получен; FETCH_FAILED — HTTP/DNS/timeout
не позволил получить нужный материал. Графики в изображениях не оценивались.
Публикация, её пересказ и исходящая ссылка — отдельные источники.

## 2. Проверенные ссылки

### S01 — Первый отдел: исходный материал baseline

- Организация: «Первый отдел»; персональный автор на странице не указан.
- Название: «Может ли государство заблокировать все VPN? А другие способы обхода блокировок?».
- Публикация: **01.04.2026**, явно на странице.
- URL: https://dept.one/memo/blokirovky/.
- Чтение: 27.10.2026; FULL_TEXT_READ, `dept.txt:18–101`.
- Проверенные места: вводная (211, ссылка на Habr); карточка 3 (15–20КБ,
  active probing), 4 (Москва/71), 5 (95/99,5%), 7 (Иран/Psiphon/Ceno),
  9 (закон/1667/8700), 4 (8931 оператор).
- Статус: аналитическая публикация; не собственный отчёт измерителей.
  Методика, экспериментальные данные и лаборатория для 95/99,5% отсутствуют
  на прочитанной странице. 71, 8931, 8700 и Psiphon-цифры не имеют отдельной
  исходной ссылки рядом с утверждением. Обозначение «заявка сервиса» baseline
  тоже не проверено: конкретный заявитель/сервис этих процентов не назван.

### S02 — «Бумага»: интервью четырёх сервисов, не единый measurement report

- Редакция/автор: **Бумага** (авторская строка); собеседники — представители
  Amnezia VPN, BlancVPN, Liberty VPN, Paper VPN, а не один интервьюируемый.
- Название: «“Мы точно знаем, что цензор продолжает поиск слабых мест”.
  Как изменились блокировки VPN и чего еще ждать пользователям от РКН».
- Публикация: **13.10.2026**, явно на странице.
- URL: https://paperpaper.io/my-tochno-znaem-chto-cenzor-prodolzhaet/.
- Чтение: 27.10.2026; FULL_TEXT_READ, `paperpaper.txt:72–112`.
- Секции: «Как изменились…» (ноябрь/май, ссылка на РБК и Медузу),
  «Появились ли…» (сканер: уверенность Amnezia, гипотеза Paper, сомнения других),
  «А как часто…» (DDoS, ссылка на собственный Amnezia report),
  «Что с популярностью…» (Telegram — редакционная оценка),
  «Какие еще ограничения…» (обсуждаемая плата/законопроект).
- Это прямые интервью для **заявлений собеседников**, но не первичные
  пакетные данные и не установленная атрибуция РКН/active probing.

### S03 — Cloudflare: первичный vendor report своих путей

- Авторы: **Michael Tremante, Alissa Starzak / Cloudflare**.
- Название: «Russian Internet users are unable to access the open Internet».
- Публикация: **26.06.2025**; начало наблюдаемого ограничения — **09.06.2025**.
- URL: https://blog.cloudflare.com/russian-internet-users-are-unable-to-access-the-open-internet/.
- Чтение: 27.10.2026; FULL_TEXT_READ текста статьи (`cloudflare.txt:829–856`),
  изображения графиков не прочитаны и численно не оцифрованы.
- Секции: «The impact», «Traffic trends», «Client-side reports via Network
  Error Logging», «Corroboration of throttling using internal data».
- Основания: внутренние данные Cloudflare, Radar, browser NEL; серверные потери;
  авторы сами оговаривают, что высокая потеря пакетов не единственная причина
  throttling. 10–14 пакетов в разделе corroboration относятся к **внешнему**
  сообщению и размеру передачи, а не к собственной универсальной packet-cap.
- Указаны Rostelecom, Megafon, Vimpelcom, MTS, MGTS, но **нет** списка per-AS,
  регионов/типов доступа/denominators для каждого оператора.
- HTTP/1.1, HTTP/2 TCP/TLS и HTTP/3 QUIC указаны как затронутые методы;
  raw UDP всех назначений, HY2 или точная стадия QUIC handshake не установлены.
- Проверка русской локали URL `/ru-ru/...` дала404: русский перевод не прочитан.

### S04 — net4people #490: исходное тело и первая страница сообщений

- Автор: **hyperion-cs**; «[Russia] Censor has a new method of blocking».
- Создано **27.06.2025**. Тело редактируемое: даты upd1…upd5 не установлены.
- URL: https://github.com/net4people/bbs/issues/490.
- Чтение: 27.10.2026; PARTIAL_TEXT_READ (`bbs490-web.txt:118–265`).
  Страница показывает **65 remaining items**; полное обсуждение НЕ прочитано.
- Тело: per-TCP server→client ≈15–20КБ; checker-link; позже upd5 утверждает
  25 пакетов в любую сторону TCP/UDP. Это не одно неизменное измерительное правило.
- Комментарий **28.06.2025**, hyperion-cs:
  https://github.com/net4people/bbs/issues/490#issuecomment-3015159747 —
  «no, not all», mostly mobile including t2/Megafon/Beeline/Yota/MTS,
  depends on region. Регион/ASN/размер выборки в этом комментарии не приведены.
- Комментарий **28.06.2025**, hyperion-cs:
  https://github.com/net4people/bbs/issues/490#issuecomment-3016016566 —
  UDP попытка и около10min cooldown; тип UDP приложения/QUIC стадия неизвестны.
- Комментарий **28.06.2025**, hyperion-cs:
  https://github.com/net4people/bbs/issues/490#issuecomment-3014944256 —
  собственное сообщение о неуспехе простой TCP/IP fragmentation.
- Открытый checker не запускался, source implementation не reviewed.
  API GitHub403 сохранён; публичная HTML-страница получена успешно.

### S05 — net4people #546: домашний доступ, отдельный эпизод

- Автор: **0x3mp7y**, создано **14.11.2025**.
- Название: «[Russia] TLS connection-based policing applied on some home ISPs.
  VLESS with most of it variants stopped working as the result».
- URL: https://github.com/net4people/bbs/issues/546.
- Чтение: 27.10.2026; PARTIAL_TEXT_READ (`bbs546-web.txt:118–289`), не всё обсуждение.
- Тело: MTS/MGTS Moscow, JustLan, LanInterCom — с12.11.2025;
  один report RTK Izhevsk с01.11.2025; порт443, активный трафик,
  около60s cooldown, HTTPS curl/browser остаётся работоспособен.
- Автор прямо сообщает «does not affect my own providers directly» и
  трудность получения данных через remote desktop. Политика одновременных
  TLS connections — гипотеза/модель автора, не установленный алгоритм ТСПУ.
- Upd15.11 и22.11 — датированы в теле; значение mitigations менялось.
- Комментарий **22.11.2025**, gohoski:
  https://github.com/net4people/bbs/issues/546#issuecomment-3566897493 —
  Shadowsocks у автора не работает на Beeline (Syktyvkar) с начала2024;
  **тип доступа не указан**, не подставлен как mobile из соседних комментариев.

### S06 — РИА: непосредственная передача заявления РКН, не сетевой аудит

- Организация: РИА Новости; персональный автор не указан.
- «Роскомнадзор ограничил доступ к 469 VPN-сервисам»;
  **26.02.2026 05:34** на странице (часовой пояс не указан здесь).
- URL: https://ria.ru/20260226/roskomnadzor-2076790707.html.
- Чтение: 27.10.2026; FULL_TEXT_READ короткой статьи, `ria469.txt:21–31`.
- Прямая цитата: «…ограничивает доступ к 469 VPN-сервисам».
  Нет списка469, метода счёта, измеренных запросов/абонентов или всех сетей РФ.

### S07 — РБК: основание числа258

- Автор: **Татьяна Зыкина**, РБК.
- «В Роскомнадзоре назвали число заблокированных с начала года VPN-сервисов»;
  **23.10.2025 15:54** на странице.
- URL: https://www.rbc.ru/technology_and_media/23/10/2025/68fa12f19a794735776382b0.
- Чтение: 27.10.2026; FULL_TEXT_READ содержательной статьи (`rbc258.txt:124–150`).
- 258 сообщено Владиславом Смирновым / ГРЧЦ в презентации «Спектр-2025»,
  РБК передаёт ТАСС (ссылка на главную ТАСС, исходный deck не получен).
- Заголовок «с начала года» и тело не определяют тождественный cumulative-set
  с РИА469. **469−258=211** проверяемо арифметически; счёт именно211 новых
  уникальных блокировок за октябрь–февраль из этих текстов не следует без
  одинаковой методики/множества/периода, которых источники не раскрывают.

### S08 — Habr: цепочка перепечаток baseline, не независимые данные

- Автор: **denis-19 / информационная служба Хабра**.
- https://habr.com/ru/news/1003760/ — **26.02.2026**, полный содержательный
  текст прочитан (`habr211.txt:8–63`):469→РИА;258→Habr959718→РБК.
- https://habr.com/ru/news/959718/ — **24.10.2025**, полный содержательный
  текст прочитан (`interfax258.txt:8–21`):258→РБК.
- FULL_TEXT_READ обеих статей. Их повторения не увеличивают число независимых
  измерителей. Изменения правовых актов переданы вторично, не verified law.

### S09 — Amnezia: собственная публикация об инциденте

- Организация/автор: **Amnezia VPN**; индивидуальный автор не указан.
- «О масштабной атаке на нас и том, как мы восстанавливаемся».
- URL: https://amnezia.org/ru/blog/amnezia-vpn-may-june-2026-incident-preliminary-summary.
- Публикация: **точный timestamp не установлен на прочитанной странице**.
  URL содержит2026; текст содержит обновления8/10/12/16/17 июня,
  7/9/16 июля,4 августа; год каждого события самим видимым текстом не маркирован.
- Чтение: 27.10.2026; FULL_TEXT_READ тела (`amnezia-incident.txt:23–46`).
- Прямое сообщение: первые неполадки20мая, массированная IP-блокировка;
  «в понедельник1июня … признаки … DDoS-атаки», расследование продолжается.
- Не содержит per-ISP capture/denominator и не доказывает инициатора атаки.
  «Платёжная система» и 1,5месяца — сообщение интервью S02; в этом тексте
  независимая количественная проверка такой длительности не найдена.

### S10 — официальное сообщение Минцифры о мобильном пилоте

- Автор/организация: **Минцифры России**, верифицированный публичный канал.
- «У россиян появится возможность доступа к востребованным интернет-сервисам
  в периоды ограничений».
- URL: https://t.me/mintsifry/2602?embed=1 (canonical https://t.me/mintsifry/2602).
- Публикация: **05.09.2025**, Telegram widget показывает09:46 без timezone.
  Сообщение помеченоedited; время первой/последней редакции отдельно не установлено.
- Чтение: 27.10.2026; FULL_TEXT_READ (`mintsifry2602.txt:6–21`).
- «Операторы уже обеспечивают доступ … в пилотном режиме»; область —
  **ограничения работы мобильного интернета**, без перечисления регионов/ASN,
  спецификации IP/SNI/протокольного enforcement или матрицы тестов.

### S11 — РБК: комментарии операторов о пилоте

- Автор: **Екатерина Шокурова**, при участии **Дины Отман** / РБК.
- «Появился список сайтов, которые будут работать при отключении интернета».
- Публикация: **05.09.2025 14:57** на странице.
- URL: https://www.rbc.ru/technology_and_media/05/09/2025/68bab3579a79472a69e13def.
- Чтение: 27.10.2026; FULL_TEXT_READ (`rbc-white.txt:124–160`).
- Ссылается на официальный S10; представители МегаФона и ВымпелКома сообщили,
  что механизм работает на их сетях; **МТС отказался от комментария**.
  Отказ от комментария не равен отсутствию режима. Наличие личного кабинета
  T2 в списке не означает измеренную готовность сетиT2.
- Нет per-region/AS/query observations. Названия списка — source fact списка,
  не proof полного полезного функционирования каждого приложения.

### S12 — Amnezia о белых списках: технический комментарий с оговорками

- «Интернет на минималках: что такое “белые списки” и как с ними жить?»
  (заголовок реально прочитан в capture).
- URL: https://amnezia.org/ru/blog/internet-white-lists-russia-what-they-are-and-how-to-live-with-them.
- Точная дата публикации не установлена; чтение27.10.2026, FULL_TEXT_READ
  (`amnezia-whitelist.txt:23–40`).
- Прямая S10-ссылка найдена здесь. «Судя по сообщениям пользователей»:
  различие операторов/регионов, Wi-Fi может иметь мобильный upstream;
  «универсального способа … не существует» на момент публикации.
  Это provider-commentary, не dataset. Пример МегаФон Москва/Владимир
  иллюстративный, не датированная серия измерений, в матрицу не внесён.

### S13 — OONI/RKS Global: русскоязычная первичная аналитика

- Авторы: **RKS Global; Elizaveta Yachmeneva, Maria Xynou, Mehul Gulati,
  Arturo Filastò (OONI)**.
- «Хроники цензуры: Систематическое давление на независимые медиа в России»;
  **09.12.2024**.
- URL: https://ooni.org/ru/post/2024-russia-report/ — русский текст реально получен.
- Чтение27.10.2026: PARTIAL_TEXT_READ выбранных секций «Основные выводы»,
  «Методы», «Анализ данных OONI», «Методологические ограничения»,
  «Подтвержденные блокировки…» (`ooni-russia-ru.txt:19–177`).
- Данные01.09.2023–01.09.2024, NEWS+WebConnectivity;279 подтверждённых доменов,
  TLS-аномалии рассмотрены отдельно. Исследование явно ограничивает coverage,
  test-list selection и отбор самых сильных сигналов. Это не оценка VPN/HY2
  успеха2026 и не representative mobile/fixed split.
- Опубликованный CSV: https://ooni.org/post/2024-russia-report/2024-09-01-russia-blocked-confirmed-media-domains.csv.
  DATA_READ: header,279 строк (локальный подсчёт CSV-записей), выбранные поля
  www.bbc.com/meduza.io/www.dw.com; `csv-read-summary.json`.
  Все строки механически загружены, **содержательное review каждой строки не заявлено**.

### S14 — OONI: методика интерпретации

- Организация: **OONI**, персональный автор/дата публикации не указаны в
  проверенных секциях; дата чтения27.10.2026.
- «Interpreting OONI data»;
  https://ooni.org/support/interpreting-ooni-data/.
- PARTIAL_TEXT_READ: coverage, outcomes, anomalies/false positives,
  WebConnectivity steps, DNS/TCP/TLS и пример AS12389
  (`ooni-interpret.txt:70–319`).
- Автоматическое confirmed относится к fingerprints blockpage/DNS;
  reset/timeout обычно anomaly. Версии тестов и доступность измерений
  ограничивают вывод; отсутствие измерений не становится success/failure.

### S15 — OONI: блокировка Explorer и последующее уточнение

- Автор: **Maria Xynou / OONI**.
- «Russia blocked OONI Explorer, a large open dataset on Internet censorship»;
  **25.09.2024**.
- URL: https://ooni.org/post/2024-russia-blocked-ooni-explorer/.
- Чтение27.10.2026; FULL_TEXT_READ (`ooni-explorer-block.txt:19–97`).
- Methods, DNS/TLS blocks;195 тестируемых сетей01.08–24.09.2024; это число
  сетей **покрытия**, не число195 одинаково подтверждённых блокировок.
- Обновление: уведомление о разблокировании02.09.2025; по данным авторов
  на январь2026 Explorer остаётся blocked на части сетей. Дата самого
  обновления на странице отдельно не установлена.

### S16 — OONI: опубликованная запись AS12668, не новая проба

- Производитель: **OONI Probe desktop3.22.0 / web_connectivity0.4.3**;
  engine3.22.0, Windows; архивная дата **16.09.2024 04:57:16** (UTC по
  measurement/API conventions; не local-time компьютера владельца).
- https://api.ooni.io/api/v1/raw_measurement?measurement_uid=20240916045716.646385_RU_webconnectivity_1cf1f2dd6e536360.
- DATA_READ всего компактного JSON + выбранные семантические поля;
  `ooni-raw.raw`, `ooni-raw.selected.json`.
- probe AS12668 LLC KomTehCentr; **resolver AS42728 MSK-IX**; DNS answer
  IP принадлежит **AS31483 ER-Telecom**. Три AS — три роли, не один ISP.
- DNS inconsistent, TCP success, TLS eof, accessible=false, blocking=dns;
  control HTTPS200. IP answer связывается с censorship fingerprint **по S15**;
  сам JSON не атрибутирует администратора правила и не описывает mobile/fixed.
- `control.quic_handshake={}`: не positive/negative QUIC тест.

### S17 — OONI: ограниченная API-выборка за январь2026

- Производитель/публикатор: **OONI**, дата чтения27.10.2026.
- https://api.ooni.io/api/v1/measurements?probe_cc=RU&since=2026-01-01&until=2026-02-01&test_name=web_connectivity&domain=explorer.ooni.org&limit=5.
- DATA_READ:5 возвращённых summary-записей, по descending времени,
  **только первая страница**, continuation не пройден.
- UID/датировки в API имеются; verification_status=unverified.
  Три failure не равны confirmed-block. Ни prevalence, ни месячная rate
  из5 последних записей не вычислялись.
- Из страницы выбраны одна anomaly и одна OK для проверки стадий, **не**
  для matched causal сравнения операторов.

### S18 — OONI: AS8402, TLS завершён, HTTP body timeout

- Производитель: OONI Probe desktop3.26.0, engine3.26.0;
  **31.01.2026 20:33:10 UTC**, test web_connectivity0.4.3.
- https://api.ooni.io/api/v1/raw_measurement?measurement_uid=20260131203410.740128_RU_webconnectivity_d10431598d631e23.
- DATA_READ: metadata, DNS, TCP, TLS, requests/response/status/failure,
  контроль; HTML body других целей не нужен/не интерпретирован.
- probe/resolver **AS8402 PJSC Vimpelcom**, platform macos, тип доступа unknown;
  A-resolve, TCP success, TLS1.3/h2 success, HTTP code200 и
  generic_timeout_error с пустым сохранённым body; control200, body_length65947.
- Без stream packet trace нельзя вывести ни16КБ/25packets, ни причинную
  DPI-атрибуцию. Результат API anomaly=true, confirmed=false.

### S19 — OONI: AS8615, положительный HTTP outcome той же даты

- Производитель: OONI Probe android5.3.0, engine3.28.0;
  **31.01.2026 20:21:18 UTC**, web_connectivity0.4.3.
- https://api.ooni.io/api/v1/raw_measurement?measurement_uid=20260131202120.671695_RU_webconnectivity_35145dae4a7e440f.
- DATA_READ: metadata, DNS, TCP, TLS, requests/outcome/control; полная
  HTML-страница ответа не анализировалась как самостоятельная публикация.
- probe/resolver **AS8615 Central Telegraph**, platform android;
  тип mobile/fixed **UNKNOWN**, Android не доказывает сотовый доступ.
- TLS1.3/h2, HTTP200 без failure, accessible=true; сохранённый body непустой
  (`len`строки65744; не объявлен сетевым byte count).
- По тесту конкретный URL в конкретное время accessible; не отсутствие
  блокировок всего оператора. Для сравнения S18/S19 отличаются IP, resolver,
  время/engine, поэтому причинный «эффект ISP» не оценивается.

### S20 — РБК: сообщения о VLESS/SOCKS5/L2TP, не открытый classifier audit

- Автор: **Екатерина Шокурова / РБК**.
- «Роскомнадзор начал менять подход к блокировке запрещенного контента»
  (подзаголовок: «Роскомнадзор усилил блокировки зашифрованного интернет-трафика»).
- Публикация **04.12.2025 07:43** на странице.
- URL: https://www.rbc.ru/technology_and_media/04/12/2025/692edb139a79470afc90db77.
- Чтение27.10.2026: FULL_TEXT_READ содержательной статьи,
  `rbc-vless.txt:124–156`.
- Три источника рынка/комментарии экспертов. Игорь Бедеров передаёт жалобы
  конца ноября в Татарстане, Удмуртии, Нижегородской, Свердловской,
  Новосибирской, Томской, Волгоградской областях и Приморском крае.
  Нет ASN/access/sample/capture. РКН отвечает общим «развитие систем
  безопасности», не подтверждает конкретные signatures/протоколы в ответе.
- Это исходная журналистская цепочка для S02, не независимо measured
  «тестирование classifier» и не установленный protocol-wide запрет.

## 3. Неудачные обращения / отрицательный поиск

- GitHub API net4people/490:403; HTML200 прочитан отдельно.
- net4people/490 first page:65 remaining items не загружены.
- NTC https://ntc.party/t/17013: DNS timeout; source body не прочитан.
- Censored Planet https://censoredplanet.org/tspu и guessed
  https://censoredplanet.org/assets/tspu.pdf:404. IMC2022 paper full text
  этим пакетом не получен, не цитируется как независимо reviewed.
- https://github.com/apernet/hysteria/issues/1447:404; возможный перенос
  репозитория/удаление не означает, что claim никогда не существовал.
  Но CCA→IP-block attribution baseline **не подтверждена этим acquisition**.
- official1667: https://publication.pravo.gov.ru/document/0001202511060014
  и `/Document/View/0001202511060014`:DNS timeout. Нормативная редакция не прочитана;
  дата акта27.10.2025 и предполагаемое вступление01.03.2026 переданы в S08,
  **не** verified-law вывод. 282-ФЗ отдельно по официальному тексту не reviewed.
- RKN https://rkn.gov.ru/press/news/news74975.htm:DNS timeout.
- Google429, DuckDuckGo202 challenge, Bing200 выдал нерелевантные результаты;
  поиск не использован как доказательство71/даты/географии.
- Предполагаемые `/ru-ru/` Cloudflare и `/articles/2026/03/14/` Новой газеты404;
  это **не найденная оригинальная публикация**, а не опровержение события.
- OONI post-index прочитан как список ссылок: найден S13/S15; исчерпывающего
  поиска всех публикаций2025–2026 не заявлено.

## 4. Что не читалось и не проверялось

Реальные профили, secrets/env/vault и логи пользователя не читались.
Исходники приложения/failclass/adaptive не исследовались этим пакетом:
вопрос — внешняя достоверность ограничений, не source/API implementation.
Наличие файлов основного репозитория не превращает публикации в runtime evidence.
Нет network/QUIC/UDP probes, builds/tests/security scans, разрешения на
packet-capture владельца или оценки current VPS. Материалы других авторов и
общие главы/STATE сохранены без записи.

## 5. FUP-2-01 — exact engine outcome path и границы API compatibility

Область A и реестр S01–S20 сохранены точным префиксом. Новых network
records не получено: использованы S16/S18/S19 и уже сохранённый S17.
Источники ниже — публичные исходники по revision из самой записи,
не текущий main.

### S21 — inventory прежних raw/API captures

Локальное статическое извлечение: **27.10.2026**.
`fup2-field-inventory.json` содержит presence/value root/test_keys и сохранённые
API summaries. Новых измерений или переоценки API flags нет.

- S16: app/engine3.22.0, test_version0.4.3, data_format0.2.0;
  recorded commit **97f1df33611097fdcd8e0cc53fe096c0c7727487**;
  vcs_modified="false". network_type ключ отсутствует.
- S18: app/engine3.26.0, test_version0.4.3, data_format0.2.0;
  recorded commit **e9b4dc98c04d347f0ee5468c2585e32d94812b73**;
  vcs_modified="false". network_type ключ отсутствует.
- S19: app5.3.0, engine3.28.0, test_version0.4.3, data_format0.2.0;
  vcs_revision/vcs_time/vcs_modified/vcs_tool **присутствуют, пусты**;
  **annotations.network_type="wifi" присутствует**. Прежний S19 текст о
  неизвестном access не учёл этот локальный type; уточнение append-only.
  Fixed/cellular upstream по этому полю не определяется.
- Root/test_keys трёх raw не имеют anomaly/confirmed/failure booleans.
  Это отсутствующие поля, не false. Stage-specific failure/null,
  test_keys.blocking/accessible/x_status присутствуют.
- S17 API содержит anomaly/confirmed/failure/verification_status для S18/S19,
  но не engine/backend classifier revision. У S16 matching summary capture
  отсутствует. Scores/flags нельзя реконструировать из current main.
- Raw hashes при follow-up совпали с initial source receipts:
  S16 `0ad7bacc87e98a490a6456c5b3bc273b2742180aa733a72ee1984e6ab76ba149`,
  S18 `b75507e5fe70ef19d29e9ef6db9311ca04342413e695983b49c80786cbb8204e`,
  S19 `522ada3e0904798ca53cc4f0944fcc732f6c693a37148ed04aa1d97e2a2dd45f`.
  Равенство saved bytes не аттестует бинарник производителя или truth outcome.

### S22 — OONI Probe engine source по recorded commit S18

Организация/авторы: **OONI / contributors probe-cli**, source snapshot
**e9b4dc98c04d347f0ee5468c2585e32d94812b73**.
Дата commit в annotations S18: **2025-06-12T15:43:40Z** (recorded metadata,
не новая независимая проверка Git timeline); чтение **27.10.2026**.
Все acquisition — HTTPS/TLS verified, raw source/URL/time/hash в
`fup-*.meta.json`. Код не исполнялся, зависимости не устанавливались.

- [webconnectivity.go](https://github.com/ooni/probe-cli/blob/e9b4dc98c04d347f0ee5468c2585e32d94812b73/internal/experiment/webconnectivity/webconnectivity.go),
  **FULL_TEXT_READ** source; raw capture `fup-wc326`.
  `:17–18` test_version0.4.3; `:128–129` TestKeys↔Measurement;
  `:160–175` ControlFailure, `:198–213` TCP records/success counts,
  `:216–229` HTTPGet/Requests/HTTPAnalysis/Summarize.
  `:235–245` TCP status.Blocked — уже comparison with control.
  `:251–272` local SummaryKeys bool Accessible/string Blocking + IsAnomaly;
  raw nil Accessible сводится кfalse в adapter, а не сохраняется какnil.
- [summary.go](https://github.com/ooni/probe-cli/blob/e9b4dc98c04d347f0ee5468c2585e32d94812b73/internal/experiment/webconnectivity/summary.go),
  **FULL_TEXT_READ**; `fup-summary326`.
  `:42–60` raw summary fields; `:84–89` union DetermineBlocking;
  `:98–129` deferred mapper, HTTPS success shortcut, ControlFailure guard;
  `:167–206` DNS/connect/control/no-request guards;
  `:211–269` request-failure switch + inconsistent-DNS override;
  `:240–245` timeout→http-failure and flags.
  SHA256 `26327ff540f3b93eb1152a962df14e2ff162c78bdf6bc84a9cebaabb5f23776d`.
- [httpget.go](https://github.com/ooni/probe-cli/blob/e9b4dc98c04d347f0ee5468c2585e32d94812b73/internal/experiment/webconnectivity/httpget.go),
  **FULL_TEXT_READ**, `fup-httpget326`; `:48–69` Getter.Get result→Failure/TestKeys.
- [urlgetter/getter.go](https://github.com/ooni/probe-cli/blob/e9b4dc98c04d347f0ee5468c2585e32d94812b73/internal/experiment/urlgetter/getter.go),
  **FULL_TEXT_READ**, `fup-getter326`; `:52–81` saver→Requests/Error,
  `:69–73` «the last request appears first», `:141–146` Runner caller.
  tracex.NewRequestList body не reviewed: collector call/contract — граница,
  не полный сетевой source-closure audit.
- [connects.go](https://github.com/ooni/probe-cli/blob/e9b4dc98c04d347f0ee5468c2585e32d94812b73/internal/experiment/webconnectivity/connects.go),
  **FULL_TEXT_READ**, `fup-connects326`; `:31–53` endpoints Multi.Collect и
  success/total из TCPConnect entry.Status.Success. Count не восстановлен
  только по полю no-TLS-failure.
- [httpanalysis.go](https://github.com/ooni/probe-cli/blob/e9b4dc98c04d347f0ee5468c2585e32d94812b73/internal/experiment/webconnectivity/httpanalysis.go),
  **FULL_TEXT_READ**, `fup-httpanalysis326`; `:34–39` comparison,
  `:46–71` len/body/null checks, `:77–105` status-match.
  У S18 `status_code_match=true`, `headers_match=true`,
  `body_length_match=null`, `title_match=null`; наличие matching headers/code
  не отменяет request.Failure в приоритетах Summarize.
- [netxlite/classify.go](https://github.com/ooni/probe-cli/blob/e9b4dc98c04d347f0ee5468c2585e32d94812b73/internal/netxlite/classify.go),
  **PARTIAL_TEXT_READ** `:18–120`, `fup-classify326`;
  suffixes context-deadline/i-o-timeout/TLS-handshake-timeout могут дать
  один FailureGenericTimeoutError. Конкретная Go ошибка S18 не восстановлена.
- [netxlite/errwrapper.go](https://github.com/ooni/probe-cli/blob/e9b4dc98c04d347f0ee5468c2585e32d94812b73/internal/netxlite/errwrapper.go),
  **FULL_TEXT_READ**, `fup-wrap326`; `:54–67` Error/JSON failure string;
  `:84–105,130–131` preserve existing wrapped failure/major operation.

**Pin/caller обоснование:** commit взят из annotations raw S18 (не по latest
release); test_version source совпадает с raw. Это trace source-consistent
ветви до одного label, не исполнение/аттестация whole EXE и не API backend audit.

### S23 — историческое сравнение source по recorded commit S16

Организация/авторы: OONI/contributors; commit
**97f1df33611097fdcd8e0cc53fe096c0c7727487**, recorded VCS date
**2024-05-22T09:08:46Z**. Чтение27.10.2026.

- [summary.go](https://github.com/ooni/probe-cli/blob/97f1df33611097fdcd8e0cc53fe096c0c7727487/internal/experiment/webconnectivity/summary.go),
  source acquired `fup-summary322`; **FULL_TEXT_READ через byte-equality**
  с прочитанным S22 (содержательное сравнение scoped branches, не запуск).
- [webconnectivity.go](https://github.com/ooni/probe-cli/blob/97f1df33611097fdcd8e0cc53fe096c0c7727487/internal/experiment/webconnectivity/webconnectivity.go),
  `fup-wc322`, **FULL_TEXT_READ через equality** с S22.
- Две пары совпали целиком bytes, summary hash выше;
  caller SHA256 `0d93b452533223f0f9449f8f424a51b5039c9a2e9b9504c58cd8cdf62afb91b4`.
  Следует только equality этих файлов на двух named source snapshots,
  **не** whole-engine parity, влияние всех зависимостей или same backend.
- S16 eof/read-write + inconsistent-DNS override согласованы с
  blocking=dns/x_status8480; это source/ручной trace, не новый DNS experiment.

### S24 — ошибки/границы дополнительного source поиска

- Guessed filenames analysis.go/experiment.go/testkeys.go в exact
  webconnectivity snapshot404; исправлены через **exact commit directory**,
  откуда найдены summary.go/httpget.go/httpanalysis.go/connects.go.
- Guessed netxlite/errors.go и tracex/request.go404.
  Directory listings `fup-dir326`, `fup-error-dir326`, `fup-trace-dir326`
  — **DISCOVERY_ONLY**, не outcome/evidence source.
- `internal/model/experiment.go` acquired (`fup-summarymodel326`), но
  нужный summary interface definition не найден/полностью не reviewed;
  **LINK/ACQUISITION_ONLY**, не доказательство backend contract.
- netxlite netx.go/operations.go/quirks.go acquisition для negative search;
  нужная timeout literal definition не найдена в просмотренном диапазоне.
  **не claimed full-text source proof**; не расширяет основной trace.
- Backend API source/version **не раскрыт** в прежнем response; current main
  не бралась. Это bounded missing provenance, не ошибка API/измерителя.
- Android engine3.28.0 revision пуст; release tag/main не используется как
  подмена исторического точного commit. Отдельная source compatibility
  Android не доказана — **UNKNOWN**.

### S25 — LICENSE и limits reuse

[probe-cli LICENSE](https://github.com/ooni/probe-cli/blob/e9b4dc98c04d347f0ee5468c2585e32d94812b73/LICENSE),
получен `fup-license326`; прочитан заголовок GPL version3, **PARTIAL_TEXT_READ**
лицензии, не полный правовой аудит files/dependencies. Код в продукт не
переносился. Reference алгоритма stage/control и его priced UNKNOWN —
не решение об интеграции GPL кода/полном лицензировании всех зависимостей.

### Ручной противоположный пример и сохранность

REPORT §8.5: **та же** Requests[0].Failure=generic_timeout_error,
но ControlFailure ненулевой → guard раньше switch → accessible=null,
blocking=null/status control-unreachable. Или consistent→inconsistent DNS
при одном request → reason=dns. Это conditional source reasoning;
не четвёртая запись, не fixture, не выполненный classifier тест.

Исходные prefixes REPORT/SOURCES/handoff сохранены в
`fup2-baseline-*.snapshot` и `fup2-baseline-manifest.json`. Точные URLs,
raw hashes и время получения source — в `fup-*.meta.json`; body кодовых страниц
не исполнялся. Scope принятия A сохраняется без расширения на B/C/D.

