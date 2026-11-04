<a id="probe-proof-boundaries-2026-10-28"></a>
# Что доказывает probe: endpoint, путь и отсутствие утечек

[Оглавление](../README.md) · [Метод измерения](../methodology/RESEARCH-METHOD.md) · [Решения review](../research/reviews/2026-10-28-DEEP-REVIEW.md)

**2026-10-28, DR27-15.** Это логическая граница доказательства и нормативная
методика, не новый probe API или измерение сети. Работоспособность и отсутствие
утечек текущей реализации не установлены этим исследованием.

## 1. Свежая подпись доказывает конкретное утверждение signer

Если trusted verifier выбирает свежий непредсказуемый nonce n, independently
trusted signer аутентифицирует n, контекст и все используемые поля, а verifier
проверяет связь с незавершённой попыткой и её временным окном, то получено
свидетельство участия signer после появления challenge. При доверии к signer
**как измерителю** можно также принять его утверждение о наблюдавшемся запросе
и transport peer. Подлинность и честность измерения — разные предпосылки.

Unsigned echo может вернуть любой увидевший n посредник. Signed echo может
быть честно получен через relay. Во втором случае криптография не нарушена.
Это не доказательство пути от конкретного локального процесса и не непрерывного
состояния канала. Подпись должна покрывать используемые fields; подписанный
Content-Digest дополнительно требует сверки digest фактического body.
[RFC9421 §§1.4/7.2.1/7.2.2/7.2.8](https://www.rfc-editor.org/rfc/rfc9421.html#section-7.2.2).

Даже имя режима `authenticated` не определяет покрытие всех данных. В базовом
TWAMP authenticated mode HMAC покрывает первые16 bytes reflected test packet;
timestamps расположены далее. Encrypted mode имеет другое покрытие.
[RFC5357 §4.2.1](https://www.rfc-editor.org/rfc/rfc5357.html#section-4.2.1).
Это не рекомендация внедрить TWAMP: урок — проверять конкретные защищённые fields.

## 2. Два разных локальных мира могут дать один remote transcript

Пусть B — локальная граница защиты, N — общий внешний NAT, R — responder.

```text
W_safe: C → VPN через B → V → N → R
W_leak: C → direct через B → N → R
```

При допустимой топологии пути сходятся до N. R может получить одинаковые
challenge/request/source IP и выдать одинаковый аутентифицированный ответ.
Во втором мире нарушена VPN route policy, хотя end-to-end HTTPS до R исправен.
STUN reflexive address также описывает наблюдаемый NAT mapping относительно
сервера, не весь предыдущий маршрут:
[RFC8489 §2](https://www.rfc-editor.org/rfc/rfc8489.html#section-2).

Формально, если `O_R(W_safe)=O_R(W_leak)`, а safety различна, никакая функция
только от O_R не различит эти два исполнения. Свежая подпись не устраняет эту
неразличимость. Аналогичный пример получается с общим relay вместо NAT.

Egress IP позволяет **условный** более сильный вывод, если независимо известно
полное множество D возможных direct-egresses в этом scope и честно наблюдённый
x не принадлежит D. Но один direct-control даёт одну точку, не доказывает полноту
D при multi-WAN/translation/relay. Подписанный `Forwarded` также не делает
недоверенный исходный header независимым сетевым измерением:
[RFC7239 §8.1](https://www.rfc-editor.org/rfc/rfc7239.html#section-8.1).

Ещё проще оставить весь проверенный echo неизменным, но вывести **другой** DNS,
IPv6 или application flow вне туннеля. Все remote receipts сохранятся даже
при нескольких честных responders. Исследование TunnelCrack показывает
практическую значимость route-exception класса, а не дефект нашего приложения:
[Xue et al., USENIX2023](https://www.usenix.org/conference/usenixsecurity23/presentation/xue),
[primary ServerIP explanation](https://tunnelcrack.mathyvanhoef.com/details.html#serveripdetails).

| Свойство | Предмет доказательства |
|---|---|
| Endpoint authenticity | Кто аутентифицировал конкретное утверждение |
| Path evidence/attestation | Где прошла связанная с evidence копия пакета при доверии свидетелям |
| Absence of leaks | Ни одна запрещённая отправка в объявленной области не произошла |

Транзит одной копии через правильный узел не исключает другой копии вне него.

## 3. Нулевое число обнаружений требует полноты наблюдателя

Логически корректно:

`(любая утечка в S ⇒ обнаружение M) AND (обнаружений нет) ⇒ утечек в S нет`.

Содержательная работа — обосновать первую предпосылку: observation boundary,
все относящиеся к S интерфейсы/families/transports/процессы, интервал, отсутствие
capture gaps и корректная атрибуция permitted outer traffic против bypass.
Один достоверный запрещённый выход опровергает safety; ноль без полноты не
подтверждает её. [RFC7799 §3.6](https://www.rfc-editor.org/rfc/rfc7799.html#section-3.6),
[RFC2330 §10.2](https://www.rfc-editor.org/rfc/rfc2330.html#section-10.2),
[RFC8468 §§4/6](https://www.rfc-editor.org/rfc/rfc8468.html#section-4).

- TUN-only capture не видит пакеты, обошедшие TUN.
- Plaintext на внутренней стороне TUN может быть штатным.
- Host enqueue event до enforcement не обязательно означает выход на провод.
- Одинаковая policy shape до/после допускает промежуточный bypass и восстановление
  — ABA. Нужен инвариант или подходящее непрерывное evidence, не две равные картинки.
- Полный конечный capture не доказывает безопасность будущего lifecycle transition.

## 4. Cache и browser completion — отдельные события

Правильно аутентифицированный свежий nonce исключает старый receipt другого
nonce, но не свежий receipt через CDN/relay. HTTP freshness разрешает reuse
представления; отсутствие Age не доказывает обращение к origin.
[RFC9111 §5.1](https://www.rfc-editor.org/rfc/rfc9111.html#section-5.1).
Service worker может сформировать Response и имеет отдельный Cache API:
[W3C Service Workers, CRD2026-09-17](https://www.w3.org/TR/2026/CRD-service-workers-20260917/).
Browser/mock fetch completion поэтому не становится native path proof.

## 5. Практический контракт без лишней телеметрии

Для каждого claim задаются объект, scope, observation point, layer/family,
временное окно и привязка к current owner/epoch. Requested route не называется
observed route. Endpoint authenticity/freshness/egress/local safety остаются
разными полями знания; подписанный обратно opaque generation не удостоверяет ОС.

Invalid signature/correlation → invalid evidence; смена scope → stale;
timeout → отсутствующее наблюдение; подтверждённый leak → отдельный safety outcome.
TTL ограничивает reuse, а не обещает непрерывную безопасность до истечения.
Выводить достаточно необходимых reason/scope/provenance и counts; raw body,
nonce, реальные адреса и IDs не становятся обязательной долговременной telemetry.

Future cases **NOT_RUN**: неверная подпись/чужой nonce/непокрытое поле; старый
cached против fresh relayed receipt; VPN/direct через общий NAT; healthy
responders при ином DNS/IPv6 leak; TUN-only/family-only/gapped capture; ABA
policy и late result после нового owner. Сначала проверяется различимость
конечных моделей и scope, затем отдельно разрешённый native стенд.
Адресаты TH-01/02/04; existing probe/owner machinery сохраняется.
