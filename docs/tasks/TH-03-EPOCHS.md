# TH-03 — восстановление и первая установка profile epoch

2026-10-22, V2-122/123. **Исходники добавлены, компиляция/runtime не проверены.**
Tests не добавлялись/менялись/запускались; build/vet/harness checks/module/native/
worker/live/issuer CLI не выполнялись. Owner keys/vault/runtime config/VPS не
использовались. Gofmt и чтение исходников не являются acceptance.

Контракт: [ADR-002](../decisions/002-profile-epochs.md). Все9 этапов сохраняются.

## Версия и история

- `ProfileVersion{Epoch, Counter}` отделена от network epoch. Schema1 envelope —
  epoch0; schema2 требует ненулевой `profile_epoch`. В одной epoch counter не
  снижается; одинаковая tuple допускает только exact content replay.
- Legacy scalar Validate/Apply/RecordClockObservation/import-floor reader не
  отбрасывают ненулевую epoch. Runtime/render/clock/import/recovery/bindings
  migration используют tuple. Issuer candidate validation не меняет device floor.
- Schema2 checkpoint хранит namespace/last raw grant SHA и последовательные
  boundaries прежних epoch/counter/content/authority/anchor/grant hashes.
  MaxUint64 сохраняется в history; temporary zero checkpoint не подставляется.
- Schema3 дополнительно хранит immutable `EpochOrigin` для действительно первой
  установки сразу в ненулевую epoch. Последующие boundaries начинаются с этой
  начальной epoch, а не с вымышленных прошлых переходов. Schema1/2 сохраняются.
- Все pin revocations, retired channel/publisher IDs, time/issued floors и update
  floor сохраняются. Budget —64 сохранённых перехода на установку, не предел
  глобального номера epoch; uint64 overflow/skip/exhaustion → отказ.
  Runtime checkpoint writer не меняет epoch authority/history. CP/epoch errors
  типизированы как trust fault, не channel FAIL.

## Offline authority

Reserved target `trust/profile-epoch.json` принадлежит непосредственно top-level
targets с threshold≥2 по разному offline public material. Product policy допускает
только этот optional direct target; profiles/updates delegations сохраняются.
Arbitrary top-level targets/новые делегации не разрешены.

Grant имеет ровно10 обязательных fields: schema1,
purpose=`snowden-system/profile-epoch/v1`, from_epoch/to_epoch (ровно+1),
profile_public_hex (новый Ed25519 public key), profile_namespace,
minimum_protocol2, issued_at/expires_at и grant_id (64 lowercase hex).
Нет самостоятельного local signing key: требуются точные bytes private SDK
proof именно offline target. Single profile/recovery key, JSON parsing или
обычная root rotation epoch не увеличивают. Lineage задаёт enrolled proof source.

## Одна проверка SDK для двух артефактов

`PrepareArtifacts` обрабатывает max2 purpose-distinct requests одним upstream
Updater с общими fetch/rotation/budget bounds. Profile/grant proofs имеют общий
source/final retained state. Пока не прошёл весь target set, proofs не выдаются.
Verified prefix возвращается отдельно, включая позднюю cancellation, и сохраняется
прежним4-target/Keep путём независимо от artifact/application отказа.

`CheckEpochProofs` проверяет purpose/path/bytes/expiry/source и одинаковый next
digest. `PrepareEpochRecovery` требует intact accepted checkpoint, enrolled lineage,
new profile signature, schema/tuple/namespace/clock gates. Existing/retired/foreign
key role не продвигается; меняются только profile keys. Exact committed replay
не создаёт ещё одну epoch. Source fences и retirement/blocked-on-failure остаются.

Pinned go-tuf/v2.4.2 `metadata.go:548–592` сопоставляет glob по path components:
`profiles/*` не покрывает nested path. Namespace поэтому flat:
`profiles/epoch-N-`, например `profiles/epoch-1-main.json`. Разделитель после
номера исключает epoch1/epoch10 collision. Своей замены matcher/SDK нет;
это source-based адаптация, не conformance/runtime evidence.

## API и issuer package

- Public `mode=epoch_recover`: confirm + canonical target SHA из preview.
  Нельзя одновременно enroll root/использовать single-key permit. Ordinary
  mode с epoch grant отвергается. Все credential/render/vault/lifecycle/4-target
  gates общие с прежним импортом.
- Public `mode=epoch_enroll`: отдельные root SHA и canonical target SHA + confirm;
  только первая установка. Обычный enroll/recover не получает этот режим неявно.
- `epoch_grant` переносится exact bytes/base64. В delivery-pack plan это optional
  relative public JSON file, только для profile; SDK проверяет оба target.
  Epoch/signer/namespace должны совпасть. Нужны уже signed TUF inputs; полного
  authoring/signing/custody workflow новый command не создаёт.
- Report различает raw artifact SHA и canonical profile content SHA; tuple DTO
  сериализуется decimal strings. Preview `bundle_identity/current_identity`
  сохраняет MaxUint64 без JS rounding. Numeric compatibility fields не authority.
  `GetProfileRecoveryContextJSON` отдаёт backend JSON string: UI не должен
  parse/stringify его числовые поля перед экспортом.
- Envelope reader regular/Lstat/SameFile/≤1MiB; TUF/public transfer budgets256KiB
  сохранены. Duplicate/alias/null/unknown/trailing envelope properties отвергаются.
  Canonical epoch0 fields без новых свойств сохраняются; rollout не проверен.

## Первая установка в текущую epoch — V2-123

`CheckFreshEpochSource` требует отсутствия файлов version/envelope и прежнего
delivery enrollment. Даже существующий decimal `0`, symlink, повреждённый файл
или недоступный путь не считаются чистой установкой. Импорт сначала проверяет
исходный journal; после retirement повторяет отсутствие файлов и точную source
authority под прежней сериализацией. Это не Windows interprocess CAS/ACL proof.

`PrepareEpochEnrollment` использует тот же private SDK grant/profile pair и
проверку подписи, что recovery. Отдельно одобренный root совпадает с baseline
bytes/hash; последующие verified root rotations остаются частью SDK chain.
Разрешён только один точный delivered profile signer; остальные seed profile
keys сохраняются retired, update/recovery roles не меняются. Foreign/retired key
не продвигается и не возрождается. Существующая принятая identity не сбрасывается.

Origin записывает реальные начальные epoch/counter/content/grant/anchor/source
trust hashes. Последующий replay/ordinary import/same-epoch recovery/clock write
сохраняют origin; последующий epoch recovery добавляет только реальную source
boundary. Runtime writer запрещает изменение origin/history. Clock preparation
теперь клонирует также вложенные pin revocations, retired IDs/history/origin;
публичный recovery context возвращает owned origin и число сохранённых переходов.

Credential/render/vault/clock/source/lifecycle и four-target commit остаются
общими. До commit нужны свежие proof/expiry/source gates; отсутствие прежнего
enrollment не разрешает частично сохранять root. Новый режим пока не подключён
к UI/generated Wails bindings; реально установленный EXE этим не обновлён.

## Остаток и следующий срез

Fresh nonzero-epoch enrollment реализуется только в source V2-123, **unverified**.
Existing-device recovery по-прежнему требует intact source checkpoint. Повреждение
или потеря истории не превращаются в разрешение первой установки/сброса.

Full TUF publisher/custody/signing/publication, recovery-plan automation,
UI/native Wails bindings/export/old-client rollout открыты. Старые strict клиенты
отвергнут новые schema/reserved target; direct/legacy fallback не добавлен.
Все compile/module/SBOM/conformance/fixtures/native ACL/reparse/interprocess/
crash/power-loss/suspend/lifecycle/release/comparative gates открыты. Context/budgets
не hard OS/CPU/RSS deadline и не доказательство независимости custodians.

Следующий срез — полный publisher workflow/custody/signing/publication, затем
product UI/export/rollout. После разрешения тестирования нужны отдельные матрицы
fresh install/current high epoch/decimal0/pending journal/source replacement/
role conflict/origin preservation/64 transitions/MaxUint64; текущих доказательств
компиляции и поведения нет. Тестирование и реальные rollouts отложены.
