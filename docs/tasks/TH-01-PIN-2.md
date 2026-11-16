# TH-01-PIN-2 — сохранение pin-политики и смена сессии

Статус 2026-10-21: код реализуется в V2-111, **не проверен**.
Владелец запретил тестирование; старые результаты V2-110 относятся к прежним входам.
Общая цель сохраняется: все TH-00…08 из PLAN §5.3.

## Решения в коде

- `metadata/accepted_profile.go`: version.txt теперь хранит schema/version,
  SHA256 canonical signed envelope, pin-policy identities и историю отзывов.
  Старый decimal floor читается; первый authenticated installed envelope
  становится content anchor. Ранее неизвестную историю legacy восстановить нельзя.
- Новый профиль с тем же version и иным content запрещён. После signed pins
  нельзя вернуться к manual trust/другому transport под тем же ChannelID:
  старый ID отзывается, новый provisioned отдельно. Удалённый pin-channel
  сохраняется как retired; явные fingerprint revocations действуют между IDs.
- Retention bounded: ≤1024 pin IDs, ≤1024 retired IDs, ≤8192 revocations,
  checkpoint ≤1 MiB. Исчерпание — отказ, история не вытесняется молча.
- Идентичный профиль/секреты не меняют DPAPI ciphertext/audit и не перезапускают
  engine. Runtime reader не перезаписывает одинаковый checkpoint.
- Обновление profile/credentials завершает прежний Manager до commit четырёх
  файлов. При ошибке cleanup/commit новая session не запускается; rollback
  старого engine и автоматические retry заблокированы. Восстановление —
  успешный повтор импорта либо явный Disconnect.
- После успешного commit сохранённое protection intent запускает обычный
  generation-fenced Start с обязательным protected probe; импорт сам по себе
  не публикует running. Concurrent Disconnect инвалидирует отложенный запуск.

## Границы

Это исходный код без новой локальной/runtime приёмки. Не выполнялись tests,
build/vet/harness, live/admin или owner migration. Compile/runtime/concurrency
корректность остаётся непроверенной. Local checkpoint не защищает от атакующего,
который может переписать все local trust files; ACL/reparse/interprocess и
power-loss/crash gates остаются в OFF-04/06/07. Hard bounds для стороннего
Start/Close требуют следующего отдельного process-isolation решения.
