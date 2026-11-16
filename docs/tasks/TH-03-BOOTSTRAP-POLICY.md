# TH-03 — первый список отзывов на новом ПК

Статус: V2-144, 2026-10-24, **реализовано в исходниках, unverified**.
Только PC/Windows. Tests не добавлялись/не менялись/не запускались; compile/build/
typecheck/check/module/gen/SDK/crypto/product CLI/browser/native/live не выполнялись.
`gofmt -w` десяти Go-файлов завершился exit0; это не проверка импорта или VPN.

## Граница доверия

Раньше первая доставка требовала отдельный SHA256 root.json, но могла принять
старый, ещё не просроченный пакет без нового policy target. У чистого ПК нет
локального antirollback/revocation ledger, который доказал бы пропуск отзывов.
Наличие подписей и отсутствие своей истории не доказывают актуальность.

В [TUF 1.0.36 §5.1–5.3](https://theupdateframework.github.io/specification/v1.0.36/)
начальный trusted root предполагается доставленным вне канала обновления;
workflow использует фиксированное время и непрерывность root versions. Вывод
для нашего приложения: при полной потере истории нужен отдельный известный
ориентир отзывов. Это вывод проекта, не новый пункт стандарта TUF. Версии SDK,
зависимости и заявленная область conformance этим изменением не обновлялись.

## Контракт первой установки

Существующие `PublicProfileRequest` и `ProtectedProfileTransferRequest` получают
`approved_pin_policy_sha256` рядом с `approved_root_sha256`. Для `enroll` и
`epoch_enroll` оба значения — обязательные canonical lowercase SHA256 из
независимого доверенного источника. Значение не вычисляется/не заполняется
автоматически из полученного пакета или показанного превью.

Принять можно только точные байты cumulative policy с этим SHA. Проверка идёт
в общем App-пути до SDK, затем в authority preparation после полного private
batch gate и в source fence после retirement/synced staging. Сам SHA не даёт
права подписи: обязательны прежние threshold≥2 offline top-level target proof,
root/profile/epoch/grant/role/time/continuity/credential/source gates.

Реальный подтверждённый snapshot/version/баны сохраняются существующим accepted
checkpoint в fixed4 transaction. Отдельного файла/ключа/ledger не добавлено.
Первое root enrollment для уже настроенного legacy profile тоже требует этот
ориентир и сохраняет его собственные ограничения. Обычный enrolled import,
epoch recovery и recovery не принимают bootstrap approval как сброс истории.

`bootstrap_policy_sha256` в превью означает, что backend проверил отдельный
approval; оно участвует в exact preview comparison. Старые root-only native
entry points оставлены по именам для bindings, но возвращают запрос обновить
клиент до KDF/SDK/записи. Typed public/protected entry points — действующий путь.

## Интерфейс и перенос

Обе существующие формы показывают два поля с явными подписями и связанными labels/
help/error; изменение policy approval инвалидирует прежнее превью и source
draft. Новый UI сверяет enforced echo с введённым SHA и фактическим policy SHA;
ответ старого backend, молча проигнорировавшего новое поле, не предлагает apply.
Generated namespace, native bindings и mock не правились вручную.

Enrolled export теперь требует policy в свежем publisher package. Preview и
результат содержат `destination_pin_policy_sha256`, взятый из того же проверенного
snapshot, что переносится в portable package; итог сравнивается с preview.
Новый UI отвергает отсутствие этого поля уже на стадии export preview до KDF.
После экспорта оба отпечатка остаются видимы рядом с target/profile identity.
Они — публичная информация отправителя, не готовое подтверждение на получателе.
Файл и пароль по-прежнему не назначают себе корень или policy baseline.

У issuer уже есть `pin_policy_sha256`/version в отчёте; новые CLI/артефакты ради
дублирования не создавались. Первичный корень получатель сверяет с фактическим
root его portable package, а не с произвольно выбранным old issuer root. Enrolled
export выводит actual portable root после проверенной rebase; значения report
из недоверенного скачанного файла сами по себе не становятся независимым approval.

## Что этим не доказано

- Ориентир фиксирует конкретный известный владельцу срез, а не математическое
  доказательство отсутствия более новых отзывов. Если получен другой, даже более
  новый snapshot, требуется новая независимая сверка; молчаливого перехода нет.
- Физическая custody, резервные независимые каналы и актуальность сведения у
  владельца требуют операционной процедуры/DR. Невидимая pre-policy/private
  история не клонируется на новый ПК и не объявляется восстановленной.
- Legacy unenrolled import сохранён как отдельный старый контракт: он не получает
  заявленную защиту первой TUF-установки. Его production migration/disable policy
  и старые клиенты остаются отдельным gate до rollout.
- Нужны разрешённые compiled/SDK/crypto/queue/staging/clock/source race/old binding/
  native/UI/DR проверки. Ни одного PASS; все TH-00…08/P0/OS/resources/release/
  comparative обязательства сохраняются. Новый target/EXE не развёрнут.

Далее: latest-known custodian/restore checkpoint при потере publisher history;
не выводить разрешение нового cold start из checksum самого архива. Затем
оставшиеся P0 и TH-02/04…08, без снятия текущего запрета тестирования.

Исходники: `windows/onboarding_{bootstrap,authority,public,transfer,import,
source,delivery,export_proof,export}.go`, `onboarding.go`; frontend `api/{contract,
backend}.ts`, `components/{PublicProfileSource,ProfileTransfer}.vue`.
