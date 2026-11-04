# Дополнительные разборы TLS, QUIC, доверия и платформ

Сводка сохранённых DR27-материалов. Полная [рецензия](reviews/2026-10-28-DEEP-REVIEW.md)
определяет принятую область, предпосылки и контрпримеры. Здесь нет новых решений.

| Предмет | Содержательный результат и граница |
|---|---|
| Шифрующая замена ShadowTLS | Property matrix для bytes/identity/replay/PFS по участкам; identity/UDP и миграционные условия сохраняются. |
| TLS minimum и uTLS | R02-16 принят для exact Chrome/non-real-ECH branch; это не общая гарантия любого preset. |
| Exporter и resumption | R02-19 scoped; готовое универсальное freshness-исправление не установлено. |
| Padding stop/restart | Расход cover может изменяться в обе стороны; fixed prefix скрывает поздний сигнал от модели, не от наблюдателя полного соединения. |
| BBR/Brutal callback | R04-13 scoped accepted; R04-14 требует semantic/compatibility matrix, blind upgrade не обоснован. |
| HY2 reassembly | Scoped arithmetic/ownership не доказывают deployed severity или совокупный RSS bound. |
| Peer timers и suspend | Source/API-условия разделены; energy effects не измерены. |
| Неполная/повреждённая обратная связь | R03-15/16 приняты как условная модель, не как валидность текущего producer. |
| Whole-policy causal evaluation | Randomization полной policy на изолированном домене допускает конечную оценку; marginal propensities не создают отсутствующие joint exposures. |
| TUF durable cut | Различимость историй может требоваться заданной liveness; необходимость нового DTO-поля/БД не доказана. Reset overlap/reuse и минимальность остаются открытыми. |
| Multirepo agreement | Scoped guards/representative constraint findings; hash agreement и authority mapping разделены. |
| Offline revocation | Scoped API/spec; app acceptance и первое исполнение имеют отдельные условия. |
| Corporate proxy/DNS/NAT64 | Actual dial hop определяет область; product mapping остаётся отдельным шагом. |
| Android UID и owner | Capture и lockdown имеют разные UID-наборы; prepare() способен менять owner, declaration underlying network не является egress receipt. |
| Endpoint authenticity | Conditional proof/spec не закрывает whole-path protection. |
| Данные о цензуре | DR27-16 частичный; systematic RF coverage не установлено. |

Основной корпус содержит 15 полных разборов и частичный DR27-16.
Две дополнительные оппонирующие рецензии не дали результата; независимость
такого review не приписывается имеющимся текстам. Source/model acceptance
не является реализованным исправлением, native/device/live или замером эффекта.

Важные уточнения: crash между verify и незавершённым persist сам по себе не
опровергает workflow; recovery necessity не доказывает единственный минимальный
storage layout. Точные source pins и ограничения находятся в полной рецензии.

[Тематические главы](../README.md) · [Реестр решений](REVIEW-REGISTER.md)
· [Ограничения доказательств](LIMITATIONS.md).
