# Один VPS: минимальная эксплуатация и безопасная поставка

## Краткий вывод

Будущий единственный VPS следует вводить как **runtime endpoint**, а не как новый
control plane, publisher или хранилище root/admin-ключей. Минимально достаточный
первый rollout — один exact-version sing-box, nginx для WS-front и два доказуемых
канала: **VLESS/WS** и **HY2 direct 8444**. REALITY добавляется отдельным stage
после появления полного versioned template; текущий ShadowTLS-контракт нельзя
допускать к protected traffic без inner confidentiality. Port hopping остаётся
выключенным до DNAT/firewall receipt.

Один VPS сохраняет общий отказ: неоплата, provider/account, IP/ASN/prefix,
disk-full, bad rollout и потеря доступа выключают все его transports одновременно.
Ни число протоколов, ни Cloudflare-front WS не делают origin независимым. План
не скрывает это ограничение и не вводит второй VPS, кластер, PostgreSQL,
Prometheus/Grafana или собственный control plane.

## 1. Текущее состояние и реальные расхождения

| Область | Фактическое состояние | Практическое решение |
|---|---|---|
| VPS | `PLAN` и `STATE`: VPS не оплачен; сентябрьский `INFRASTRUCTURE.md` — исторический live-срез | Новый узел считать отсутствующим до отдельной покупки и current receipt. Не наследовать IP/health/listeners |
| Server template | `server-combined.json` содержит только WS+HY2; WS loopback8443 | Не применять как production copy. Первый full render фиксирует edge443→nginx→loopback9443 и HY2:8444 |
| Client demands | descriptors: WS443, HY28444+hopping, REALITY8443, ShadowTLS8445 | Compatibility manifest перечисляет enabled server capabilities. Отсутствующий channel остаётся disabled/configured, не `live-verified` |
| Deploy | `deploy-vps.sh` рендерит secrets через `sed`, затем `scp` прямо в active path; `scp` получает SSH `-p`, хотя порт у scp — `-P` | OFF-09 change required: structured SSH/SCP argv, safe JSON rendering, stage/check/backup/atomic apply/rollback |
| Restart | намеренно ручной; config check выполняется уже после overwrite active file | Сохранить explicit activation, но validation выполнять над staging до замены active path |
| Verify | service/config/final/listeners + `/ip`; `/range` и firewall частично WARN | Разделить structural/activation/path gates; WARN не promoted в Ready. Версия binary и template hash должны входить в receipt |
| Release/profile publisher | Windows tooling уже имеет bounded publisher backup/restore source | Не размещать private publisher/root/signing/admin keys на VPS. Server-config backup и publisher backup — разные процедуры |
| Android | source candidate v1.13.19, approved AAR отсутствует | Server receipt schema можно переиспользовать; Android runtime compatibility остаётся отдельным gate |

`route.final=direct` на сервере и запрет `direct` в protected client selector —
разные роли и должны сохраняться одновременно.

## 2. Минимальные контракты

### 2.1 ServerCompatibilityReceipt — публичный, без секретов

Создаётся offline renderer/deploy owner и относится к конкретному staged/apply:

```text
server_core: version + package/artifact digest
server_template: schema/revision + rendered config digest
fronts: public listener -> nginx/transport -> loopback listener
channel capabilities:
  channel_id, protocol/transport, public/listen ports, path/flow/mux/early-data,
  TLS identity role/pin generation, obfs, hopping range state, CC mode
probe contract: hostname, /ip semantics, optional /range availability
firewall/DNAT policy digest
applied generation, applied_at, previous generation/digest
activation outcome: staged | active | rolled_back | rollback_failed
```

Receipt доказывает versioned server composition, но не reachability, protected
egress или confidentiality. Secret values и private keys в него не входят.

### 2.2 ServerApplyOutcome — транзакционный результат

```text
stage_digest
native_check_result
backup_digest + previous_config_digest
atomic_replace_result
service_restart_result
structural_verify_result
path_verify_result
rollback_attempted/result
active_digest_after_operation
```

`sing-box check`, listener, HTTP marker и client protected probe — разные evidence.
Ошибка post-check после commit не означает автоматически «ничего не изменилось».
Rollback завершён только после возврата previous config и повторной structural
проверки; process start не равен client Ready.

### 2.3 RecoveryKitIndex — хранится вне VPS

Содержит ссылки/хэши, а не общий plaintext bundle:

- approved OS/base-image и package/source identity;
- full public server template, nginx config, firewall/DNAT declaration;
- encrypted runtime-secret package и отдельные custody/approval сведения;
- certificate/pin current+next/revoke metadata;
- last known-good config digest и apply receipt;
- restore runbook и SSH host-key expectation;
- publisher backup hash/policy/anchor approvals **отдельно**, если VPS раздаёт
  public artifacts; signing/root/admin keys в runtime backup не входят.

Kit восстанавливает подготовленные inputs, но не доказывает актуальный отзыв,
оплату, доступность IP или post-restore protected path.

## 3. Последовательный operational plan

### Stage 0 — offline compatibility freeze (P0, infra + Windows/profile owners)

1. Выбрать exact sing-box server artifact/version, совместимый с pinned client
   `v1.13.19`; зафиксировать hash и license/SBOM boundary.
2. Сформировать полный declarative render для **выбранных** capabilities, не
   копию исторического VPS.
3. Исправить WS relation на edge443→nginx→127.0.0.1:9443; сохранить forwarding
   `Sec-WebSocket-Protocol` и decoy behavior.
4. HY2:8444 — exact cert identity/current-next pin, auth+obfs. Hopping=false,
   пока нет DNAT/firewall rollback receipt. Не включать session cache/0-RTT auth.
5. REALITY — отдельный optional stage с key/short-id/donor/no-flow pair.
6. ShadowTLS оставить disabled/configured: текущая TLS-less inner VLESS
   composition не имеет принятого protected confidentiality.
7. Сопоставить receipt с `metadata.Channel`/renderer. Mismatch блокирует deploy
   activation/client channel admission, а не лечится probe после факта.

**Gate NOT_RUN:** offline strict parse, exact feature matrix, negative missing-
listener/mux/path/pin/hopping cases. Новый VPS не требуется для Stage0, но
никакие проверки этим документом не запускались.

### Stage 1 — подготовка одного узла, без публикации трафика (P0, infra)

Будущие действия после отдельного разрешения:

1. Проверить SSH host identity; password fallback не считать штатным bootstrap.
2. Установить exact OS/package version; закрыть default firewall до готовности.
3. Создать filesystem layout: active config, same-filesystem staging, bounded
   backups, certificates, logs; права и owners задать до secrets.
4. Ограничить journal/log retention и disk budget. `GOMEMLIMIT`/systemd limits —
   только после измерения; watchdog/DNS/recovery не должны быть вытеснены.
5. Загрузить public template/package отдельно от encrypted runtime secret input.
6. Не размещать publisher/root/admin keys и client vault на узле.

**SPOF:** потеря этого узла означает отсутствие VPN server capacity. Offline kit
может ускорить cold restore, но не даёт warm failover.

### Stage 2 — staging и pre-activation validation (P0)

1. Безопасно декодировать/render secrets в private staging; не использовать
   `sed` как произвольный JSON encoder и не печатать rendered config.
2. Проверить отсутствие placeholders, file owners/modes, ожидаемый artifact hash.
3. Выполнить `sing-box check` над **staged** config.
4. Проверить static composition: exact listeners/fronts, loopback-only inner
   inbounds, `route.final=direct`, nginx path/header, firewall/DNAT declaration,
   probe endpoints, server receipt.
5. До active write создать bounded backup previous config/certs-related metadata
   и manifest. Disk-full/short-write должны завершить операцию до replace.

### Stage 3 — atomic activation и rollback (P0)

1. Повторный source/stage digest fence непосредственно перед commit.
2. Same-filesystem atomic replace active config; записать `committed` отдельно
   от cleanup outcome.
3. Explicit restart/reload по принятому server contract.
4. Structural post-check: exact binary/version, service state, active config
   digest, listeners, route, nginx config, firewall/DNAT.
5. Любой mandatory structural failure → rollback previous bytes → restart →
   повторный structural check. Rollback failure блокирует дальнейшие apply.
6. После успешной структуры выполнить path checks; их failure не разрешает
   direct fallback и не стирает факт committed config.

Cheapest sufficient change — адаптировать существующий deploy/verify, а не
создавать оркестратор. Нужны исправленные argv, staging/backup/apply/rollback и
versioned receipt; новая служба или БД не нужны.

### Stage 4 — активация каналов по одному (P0/P1)

1. **WS**: edge443/nginx/9443, path/header/mux/early-data pair; `/ip` current.
2. **HY2 direct8444**: cert pin/auth/obfs; UDP availability и protected egress.
3. **REALITY no-flow**: только после полного template/keys/donor receipt.
4. Hopping — после explicit DNAT/firewall gate; диапазон40000–41000 не открывать
   автоматически из client descriptor.
5. ShadowTLS — defer до принятого inner confidentiality contract.

Клиент получает server receipt/profile и выполняет обычный protected probe.
Listener/TLS success не повышает status выше своей области; `live-verified`
требует current tuple и дату.

### Stage 5 — минимальная эксплуатация (P1)

Без нового monitoring stack:

- systemd service state/restart count;
- active config/server receipt digest;
- bounded disk/free-space and log/journal bytes;
- certificate/pin deadlines и manual billing/renewal date с provenance;
- listener set и route invariant;
- `/ip` primary marker; `/range` отдельный optional diagnostic fact;
- coarse CPU/RSS, socket/connection counts и UDP drop/resource-stall facts только
  если platform источники доступны; отсутствие данных = unknown;
- последний successful structural/path check и возраст;
- backup age + last restore-drill receipt.

События хранят typed/redacted: никаких raw configs, headers, credentials,
host/IP error dumps сверх необходимого support scope. Автоматическая отправка
не нужна; support export — явное действие владельца.

### Stage 6 — backup и cold recovery (P1)

Разделить три набора:

1. **Server runtime backup:** full public configs/manifests + encrypted runtime
   credentials/certs + last known-good receipt.
2. **Publisher backup:** существующий bounded approved publisher archive;
   исключает signing keys/vault и требует independent archive/state/policy/anchor
   approvals для restore. Не хранить единственную копию на том же VPS.
3. **Client accepted state:** остаётся у клиентов/платформ; server backup не
   сбрасывает profile floors, revocations или pin history.

Cold restore: fresh target → exact kit approvals → stage/validate → activate →
structural verify → client compatibility → protected path. Restore процесса или
HTTP publication не равен client acceptance. Drill сначала на локальном Linux
стенде после разрешения OFF-09; затем отдельный VPS drill после покупки.

## 4. Budgets и ownership

| Budget | Минимальный contract | Owner |
|---|---|---|
| Disk | config backups count/bytes, logs/journal bytes, cert/public artifacts, low-space stop-before-commit | infra |
| Memory/CPU | one sing-box + nginx baseline; connection/session/queue counts; no global cap from receive window; limits only after observation | infra + performance |
| Network | expected ports/protocols; traffic/fair-use manual contract; no hidden hopping range | owner + infra |
| Time | stage/apply/verify/rollback deadlines separately; timeout ≠ completed cleanup | infra |
| Trust | cert/profile/pin/publisher deadlines and approvals; no TOFU reset during outage | profile/security |
| Operations | one writer/apply lock, generation receipt, explicit rollback owner | infra |
| Client | pair receipt, current protected probe, old-client unsupported behavior | Windows/Android owners |

## 5. Сильные противоположные guards и зависимости

- Текущие `sing-box check`, manual restart и `verify-server.sh` полезны; план их
  переиспользует, но меняет порядок вокруг active write.
- Прямой overwrite кажется проще, но при failed check оставляет active path уже
  изменённым; отсутствие автоматического restart не является rollback.
- Один full template может стать слишком сложным. Поэтому capabilities включаются
  stage-by-stage; disabled sections не должны требовать secrets/listeners.
- Один VPS не нуждается в постоянно работающем control plane. Signed/static
  receipts и offline kit достаточны для текущего use case.
- Cloudflare повышает доступность WS-front, но не origin/payment/account trust;
  не считать его вторым VPS или independent exit.
- Hard memory cap до измерений может убить DNS/watchdog/recovery. Сначала bounded
  admission/logs и наблюдения, затем controlled systemd resource profile.
- Server config backup не включает publisher trust authority; publisher backup
  не содержит keys и не доказывает reachability.
- Android source candidate не блокирует Windows/one-VPS rollout, но Android channel
  compatibility остаётся unknown до approved AAR/device gates.

## 6. Future acceptance — всё NOT_RUN

| Gate | Положительный случай | Сильный отрицательный случай |
|---|---|---|
| Render compatibility | receipt совпадает с client channel/profile | template валиден, но WS9443/mux/path/flow не совпадают |
| Secret rendering | quotes/control characters не ломают JSON, ничего не логируется | sed-инъекция/placeholder/пустой secret |
| SSH/SCP | non-default port и pinned host identity используют правильные argv | scp получает `-p`, host-key check ослаблен |
| Staging | active path неизменён до successful staged check | check падает после overwrite active config |
| Disk-full | backup/stage fail до commit | partial active file или удалённый previous backup |
| Atomic apply | active digest = staged digest; previous manifest сохранён | cross-filesystem rename/late source drift |
| Restart/health | exact binary, service, config digest/listeners/fronts | process active, но старый config/не тот binary |
| Rollback | previous bytes + service + structural verify restored | rollback fails/unknown; new apply blocked |
| WS | 443→nginx→9443 + header/path/mux receipt | listener открыт, backend port/path drift |
| HY2 | exact pin/password/obfs, protected egress | stale cert pin, UDP unavailable, fake open port |
| Hopping | every advertised port has DNAT/firewall receipt | client uses unopened range or port443 conflict |
| REALITY | key/short-id/donor/no-flow matching pair | only historical config or unsupported flow |
| Recovery | fresh target restored from independently held kit | kit exists only on failed VPS; revoked credential reused |
| Publisher | archive/state/policy/anchor independently approved | backup self-authorizes trust or keys stored on VPS |
| Monitoring | stale/unknown distinct from fail/pass | missing metric stored as zero/healthy |
| Client Ready | current pair + protected path + no leak | service/listener alone promoted to live |

## 7. Что не стоит делать

- Не добавлять PostgreSQL, Kubernetes, Redis, cluster manager или второй daemon.
- Не покупать второй VPS и не обещать независимость в этом one-VPS plan.
- Не копировать исторический production config или `server-combined.json` в
  active path как authoritative render.
- Не хранить root/publisher/admin keys и единственный recovery kit на VPS.
- Не включать все четыре транспорта одновременно ради parity с descriptor.
- Не включать port hopping без DNAT/firewall/rollback receipt.
- Не использовать ShadowTLS TLS-less inner VLESS для protected user traffic.
- Не обновлять quic-go/sing-quic ради предполагаемой скорости до coupled
  compatibility/resource acceptance; base HY2 не зависит от такого upgrade.
- Не считать оплату, открытый порт, active service, `sing-box check` или HTTP200
  взаимозаменяемыми доказательствами health.

## 8. Остаток и передача владельцам

| Deliverable | Existing owner / ID | Статус после этого crosscheck |
|---|---|---|
| Full server render + receipt | infra + owner, OFF-09/FR-006 | change required |
| Transactional deploy/rollback | infra, OFF-09 | change required; current script direct-overwrite/argv defect |
| Pair compatibility before channel admission | Windows profile/render, OFF-04/FR-005/TH-03 | reuse/adapt existing seams |
| Minimal monitoring/billing/cert/storage facts | infra+UI, COM-15/TH-07 | planned; no monitoring stack required |
| Publisher backup/restore | Windows tools/profile-security, TH-03 | source exists, native/DR NOT_RUN; keep off VPS |
| Android pair acceptance | Android AND-01/04/07/11/15 | source candidate/unverified |
| Live VPS activation/drill | infra+owner, OFF-09 | blocked until purchase and explicit permission |

План сохраняет one-VPS SPOF и минимальную инфраструктуру. Production readiness,
current provider state, costs, restore time, throughput и protected egress этим
source-only crosscheck не проверялись.
