# PC-03 — отзыв исходного пути до публикации результата

2026-11-05, V2-211, Windows **source/unverified**. Продолжение
[service refresh](PC-03-ENDPOINT-REFRESH.md) в полном
[PC-00…09](../planning/knowledge-application/PC-PLAN.md). Запрет тестирования
владельца сохраняется; ничего из продукта или сети не запускалось.

## Пробел в исходниках

`netWatcher.fire` доставлял `down`, но `onNetEvent` обрабатывал только up/change.
Down намеренно не создаёт новую health epoch, однако вместе с этим не отзывался
старый pending lifecycle context. Bootstrap сравнивал только номер/start epoch;
probe и positive publication тоже могли сохранить прежний номер при потере пути.
Новая проверка не должна выдумывать новую физическую сеть из события down или
стирать подтверждённую историю. Событие observer — также не kernel attestation.

## Изменения в существующих компонентах

- `networkEpoch` владеет одним private original `networkPathSource` с atomic
  current flag. Down/up/change сначала отзывают старый source под epoch mutex,
  затем отменяют captured operation после Unlock. Callback cancellation не
  выполняется под mutex. Up/change создают другой source; старый не оживает.
- Initial unknown source означает только отсутствие доставленного invalidation,
  не verified network identity. Down сохраняет num/start/class/history и держит
  knownDown. Usable epoch для текущих observations возвращает0; исторический
  `current()` продолжает возвращать retained number. Up/change открывают epoch
  по прежнему правилу, не по догадке из DNS/пустого интерфейса/таймера.
- `bindOperation` прикрепляет actual source к private context key. После down
  новый контекст сразу canceled; release старой операции не удаляет более новую
  регистрацию благодаря прежнему operationSerial. `scopeContext` сохраняет
  существующий source, даже если он отозван: stale ctx не получает новую identity.
- `lifecyclePublicationCurrent` проверяет source атомарно без epoch/App locks.
  Это важно для existing publication sections, уже удерживающих epoch mutex;
  новая проверка не создаёт recursive Lock или callback под value locks.
- `prepareBootstrap` удерживает original source вместе с profile/operation/clock/
  epoch guards. Renderer получает scoped context до descriptor/secrets preparation
  и проверяет его после RenderFrom перед выдачей candidate/session number.
  Истёкшая/старая подсказка не переносится как новый accepted path.
- Probe захватывает source до работы, не запускает измеритель для отозванного
  пути и помечает результат stale при отзыве во время работы. Current-subject
  gate также отвергает known-down source. Existing intent/session/owner/clock
  gates сохранены, public report/wire/generated schema не меняется.
- Positive/retained lifecycle publication и health writer закрыты на knownDown.
  Independent unsafe branch не получает нового positive допуска; прочие
  existing safety/subject gates сохранены. Нет нового forwarding setter или
  permissive fallback, TUN acceptance const по-прежнему false.
- Committed engine имеет отдельный lifetime. Отзыв pending operation не
  запускает здесь Stop/новый engine, не меняет proxy/DNS/WFP/TUN settings и не
  разрывает здоровое соединение по одному observer event. Existing cancellation
  cleanup отвечает за незавершённый lifecycle; up/change сохраняют прежний reprobe.

Scope ограничен **delivered** netwatch event: debounce/polling и отсутствие
физического интерфейса остаются пределом наблюдаемости. Atomic flag не является
жёстким прерыванием syscall/SDK или транзакцией с kernel admission. Обычная
context cancellation распространяется после Unlock; передача полномочий через
native/IPC остаётся отдельной закрытой приёмкой, а не следствием этого флага.

## Источники и оставшиеся требования

Просмотрены current netepoch/netwatch, begin/end operation/cancel cleanup,
bootstrap/render handoff, probe/report/health publication, Manager lifecycle/
lifetime/native consume и App native scope. Existing lock order остаётся runtime
→ Manager → App → epoch → clock/health. Сетевого или native исполнения нет.

Source6 Go/4 docs, gofmt/manual diff/bytes-SHA; evidence
`.artifacts/memory/path-scope-source-evidence.json`. Это не compile, race,
runtime, native, OS, speed, RSS или safety PASS. Tests/fixtures, в том числе
исторические reset helpers, не менялись; новые state fields требуют отдельной
fixture/API приёмки после снятия запрета, старые результаты сюда не переносятся.

Открыты: down/start/select/rollback/cancel/publication interleavings; observer
debounce/flap/unknown/coexistence; реальная отмена и cleanup; DNS flight уже
committed worker при смене сети; one-address vs RRset/IPv6 family fallback;
матрица IPv4/IPv6/UDP/TCP/service endpoint/target/native capture; фактический
Windows route/WFP/DNS/egress, hardware/OS identity и холодный restart. Успешный
HTTPS/DoH/IPv4 report не заполняет эти клетки общим PASS. Следующий source-срез
— явная family/path матрица по существующим фактическим observations.

Полный PC00…09, прежние FR/TH/OFF/COM/RQ/TR и TH00…08, W3b/SDK/interprocess/
durability/UX/release/comparative остаются OPEN. IPC8/native gate/schema2 install
не менялись; TUN unavailable, schema2 unsupported. Android/shared/adaptive/
configs/harness/root/NetGuard и другие чаты не изменялись.
