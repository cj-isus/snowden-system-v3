<a id="release-evidence-2026-10-28"></a>
# Обновления: время знания, проверенные bytes и первое исполнение

[Оглавление](../README.md) · [Trust continuity](TRUST-CONTINUITY-AND-AGREEMENT.md) · [Решения review](../research/reviews/2026-10-28-DEEP-REVIEW.md)

**2026-10-28, DR27-12.** Уточнение R05-004/005 по первичным RFC и platform API.
Ниже не запускались WinVerifyTrust, installer, candidate DLL/APK или приложение.
Current App guards неизвестны в docs-срезе; нормы API не доказывают их наличие
или отсутствие. Реальные сертификаты и states владельца не читались.

## 1. Время получения evidence не равно времени знания статуса

| Величина | Что означает |
|---|---|
| capturedAt | Локальный момент получения/сохранения evidence |
| OCSP thisUpdate | Момент, когда responder знал сообщаемый статус корректным |
| OCSP producedAt | Момент подписания ответа |
| nextUpdate | Когда ожидается более новая информация по контракту responder |
| TSA genTime | Удостоверенный timestamp с собственными accuracy/policy границами |
| OS/provider decision time | Момент конкретной оценки с выбранными action/flags/store |

Повторное сохранение старого подписанного ответа меняет capturedAt, а не
thisUpdate. OCSP `good` имеет ограниченную семантику и само по себе не
удостоверяет ни выпуск сертификата, ни его историческую неотозванность.
[RFC6960 §§2.2/2.4/3.2](https://www.rfc-editor.org/rfc/rfc6960.html#section-2.4).
Windows `dwRevocationFreshnessTime` относится к CRL.ThisUpdate и имеет
validity flag; недоступное поле не означает нулевой возраст.
[CERT_CHAIN_CONTEXT](https://learn.microsoft.com/en-us/windows/win32/api/wincrypt/ns-wincrypt-cert_chain_context).

### Свежий CRL не обязательно покрывает нужную историю

Контрпример: `revocationTime < signatureTimestamp < certificateExpiry < currentCRL.thisUpdate`.
RFC5280 не требует вечного хранения expired certificate в CRL: запись должна
сохраняться как минимум до соответствующего регулярного CRL после expiry.
RFC6960 предусматривает необязательное архивное хранение и `archiveCutoff`.
Отсутствие записи в новом CRL поэтому не доказывает отсутствие отзыва на
старый timestamp. Нужны подходящий historical coverage и trust policy.
[RFC5280 §3.3](https://www.rfc-editor.org/rfc/rfc5280.html#section-3.3),
[RFC6960 §4.4.4](https://www.rfc-editor.org/rfc/rfc6960.html#section-4.4.4).

Timestamp удостоверяет связанный объект/подпись во времени, но не содержит
автоматически полный снимок trust stores и revocation history.
[RFC3161](https://www.rfc-editor.org/rfc/rfc3161.html),
[Authenticode timestamps](https://learn.microsoft.com/en-us/windows/win32/seccrypto/time-stamping-authenticode-signatures).
Корректный cached evidence может удовлетворять принятой policy; offline
не означает автоматическую недействительность. Конкретный CA/provider может
иметь более сильное архивное правило. Результат WinVerifyTrust нельзя вывести
из этого контрпримера без его фактических inputs/flags.

## 2. Windows: image mapping и одобрение release — не одно свидетельство

Microsoft документирует, что изменённая copy-on-write страница executable
image может продолжать показываться `VirtualQuery` как **MEM_IMAGE**.
Этот признак описывает тип/происхождение mapping, а не неизменность его bytes.
Shared-bit проверка также не сравнивает память с approved release digest.
[VirtualQuery, Remarks](https://learn.microsoft.com/en-us/windows/win32/api/memoryapi/nf-memoryapi-virtualquery).

Поэтому нужно различать signed artifact, проверенный открытый file object,
имя установленного файла, выбранный loader image и текущее содержимое памяти.
Узкая identity lease может быть полезна при доверенном loader и объявленной
threat model; она не становится автоматической whole-process attestation.

**Точная first-execution граница:** обычный `LoadLibrary`/`LoadLibraryEx`
может вызвать DllMain **до возврата**. Схема «загрузить candidate DLL для
проверки, затем решить, запускать ли её функцию» уже допустила initialization.
Data/resource-only modes имеют собственные условия; их handle не означает
независимую проверку подписи и не исключает все path/content races.
[DLL entry point](https://learn.microsoft.com/en-us/windows/win32/dlls/dynamic-link-library-entry-point-function),
[LoadLibraryExW](https://learn.microsoft.com/en-us/windows/win32/api/libloaderapi/nf-libloaderapi-loadlibraryexw).

`LOAD_LIBRARY_REQUIRE_SIGNED_TARGET` документирует signature gate, но не
заведомо нужный release digest, publisher policy, current revocation и полный
dependency closure. `WinVerifyTrust` с return0 относится к subject/action
этого вызова; связь с последующим loader choice — отдельная обязанность.
[WinVerifyTrust](https://learn.microsoft.com/en-us/windows/win32/api/wintrust/nf-wintrust-winverifytrust).

## 3. Android: API-ответ может содержать checksum от installer

`PackageManager.requestChecksums` различает platform-enforced и
installer-provided checksums. Официальный API прямо отмечает, что Android
не может проверить installer-provided значения. Подпись checksum records
удостоверяет их signer, а не то, что ОС вычислила digest установленных APK.
Нужны источник, тип digest, base/split scope и конкретная installation.
[PackageManager](https://developer.android.google.cn/reference/android/content/pm/PackageManager?hl=en),
[ApkChecksum](https://developer.android.google.cn/reference/android/content/pm/ApkChecksum?hl=en).

`Session.openRead(name)` позволяет проверить staged bytes до commit.
Но `openWrite` допускает дальнейшую запись, а sealing происходит при commit
после закрытия streams. В API нет автоматически атомарной операции
«прочитать digest и запечатать». При допускаемом конкурирующем writer
требуется custody интервала между проверкой и sealing.
[PackageInstaller.Session](https://developer.android.google.cn/reference/android/content/pm/PackageInstaller.Session?hl=en).

Противовес: Package Manager самостоятельно проверяет APK signing/integrity;
это существующий механизм, а не необходимость писать свой installer verifier.
Его роль не даёт обычному приложению privileged `PACKAGE_VERIFICATION_AGENT`.
Installed APK set также не всегда идентифицирует текущий process incarnation:
API34 `setDontKillApp` имеет специальный сценарий добавления splits в
`MODE_INHERIT_EXISTING`, тогда как обычный default завершает процессы.
[APK v2](https://source.android.com/docs/security/features/apksigning/v2),
[SessionParams](https://developer.android.google.cn/reference/android/content/pm/PackageInstaller.SessionParams?hl=en).

## 4. Следствие для контракта выпуска

У каждого receipt указываются subject, issuer/provider, policy, время знания,
область bytes и граница применения. Подпись delivery manifest, OS signature,
staging receipt, installed checksum и executing identity не взаимозаменяемы.
Независимый компонент принимает решение до candidate-controlled initialization
в заявленном trust scope, сохраняя связь с выбранным объектом загрузки.

Cases **NOT_RUN**: повтор старого OCSP с новым capturedAt; expired cert без
архивного revocation coverage; approved main с изменённым load-time dependency;
MEM_IMAGE после COW; installer утверждает H(A) для иного signed B; запись
session после hash-read; base/split mismatch и старый процесс после изменения
installed set. Отрицательный и положительный контроль нужны для каждого
свойства, без запуска на рабочей системе по одному наличию API.

Адресат TH-03/08 и release/platform owner. Reuse штатного signing/installer и
existing custody; дополнительные гарантии требуют actual artifacts, OS builds,
flags и конечного разрешённого стенда. Новая approval UX или dependency этим
исследованием не вводится.
