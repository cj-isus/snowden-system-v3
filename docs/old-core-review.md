# Ревью старого ядра (old_snowden/snowden-system_v2) — только изучение

> Дата: 2026-09-09. Правило владельца: старый проект — справочник, НЕ источник
> для копирования. Здесь зафиксировано, что берём как проверенные паттерны,
> что НЕ повторяем и где старое ядро отстаёт от требований плана.

## 1. Инвентаризация (24 Go-файла, ~4 100 строк)

| Пакет/файл | Объём | Роль |
|---|---|---|
| `windows/backend/config` | 418 стр + 6 тест-файлов (вкл. adversarial) | строгий парс + валидация |
| `windows/backend/core` | engine 159 / manager 333 / probe 158 / proxy 153 / preflight 217 | lifecycle |
| `contracts/` | 594 стр + schema JSON | подписанные манифесты каналов (ed25519), Phase 1/2 |
| `configs/singbox/` | 4 шаблона + deploy-vps.sh | серверные/клиентские шаблоны |
| `windows/main.go` | 113 | CLI-сборка: parse → preflight → manager |

Старый модуль: `snowden.system/windows`, sing-box **v1.13.19** (= версия на
сервере), Go 1.24.

## 2. Что берём как эталон (проверено тестами и live)

1. **Строгий парс конфига** (`config.Parse`): `DisallowUnknownFields` +
   отказ от trailing data + `Normalize()` + `Validate()`. Валидатор закрывает:
   плейсхолдеры/`-----BEGIN`, пустые/дубли теги, неизвестные ссылки
   (route/dns/detour), selector `proxy` только из vless/hysteria2 с сервером,
   запрет `direct` в селекторе и в `route.final`, запрет `insecure`,
   требование `server_name`, пин-сертификат = валидный PEM на диске.
   → Переносим как спецификацию поведения (пере-реализуем с тестами).
2. **Менеджер lifecycle**: операции под `mu` + `active *lifecycleOperation`;
   operationCtx (30s) отделён от engine-контекста; cleanup в `defer`+`recover`
   (engine.Stop → restoreProxy, errors.Join); Stop отменяет активный Start и
   всё равно чистит при отменённом caller-контексте; Reload валидирует НОВЫЙ
   конфиг до закрытия старого бокса, при неудаче старта — откат на старый
   конфиг. Тесты: `TestManagerStopsEngineBeforeRestoringProxy`,
   `TestManagerCanceledStopStillCleansUp`, `TestEngineReload*` — образец
   контракта для A1.4.
3. **TLS preflight** (`tlspreflight.go`): классификация x509-ошибок в
   человекочитаемые причины (`ErrTLSUntrusted/HostMismatch/Invalid`),
   redaction endpoint'ов из сообщений, QUIC-каналы честно
   `ErrPreflightUnsupported` вместо ложного негатива, `String()` для лога
   без секретов.
4. **Windows system proxy** (`proxy.go`): snapshot→set→restore реестра
   Internet Settings + `InternetSetOptionW(refresh)`; восстановление в любом
   пути выхода из Start.
5. **Скрипт деплоя** (`deploy-vps.sh`): секреты из файлов, temp внутри
   gitignored-каталога, fail-closed guard «никогда не деплоить с
   `REPLACE_WITH_*`», рестарт осознанно НЕ автоматизирован (не ронять живой
   туннель), инструкция ручной проверки после деплоя.
6. **Тестовая культура**: adversarial-тесты на парсер (вложенные
   плейсхолдеры, неизвестные поля, trailing data), конкурентные
   Start/Stop, отмена контекста на каждом шаге. ~40 тестов — планка для
   нового ядра.
7. **Манифесты каналов** (`contracts/`): подпись ed25519, запрет даунгрейда
   версии, clock skew, структура ManifestChannel (server_ref/credential_ref —
   ссылки, не значения) — это готовый каркас FR-002 для Phase B2.

## 3. Что НЕ копируем (найдено при изучении)

1. **DNS-конфиг клиентского шаблона = утечка.** Правило
   `"outbound": "any" → server: "local"`, а `local` — `https://1.1.1.1/dns-query`
   **с `detour: direct`**: DNS уходит МИМО туннеля. Противоречит требованию
   probe «DNS через туннель». В новом шаблоне: резолвер внутри туннеля
   (detour: proxy), hijack-dns, никаких «any→direct».
2. **Probe без expected egress.** Старый probe проверяет только
   *консистентность* IP между целями, но не равенство **ожидаемому** egress
   канала. Улучшение: `ExpectedEgress` — обязательное поле из дескриптора
   канала; не совпало → probe fail (fail-closed).
3. **HY2 попадает в селектор без UDP-проверки.** Требование плана (A1.3):
   HY2 входит в селектор только после live-verify из UDP-сети. В старом
   клиентском шаблоне hysteria2 вписан безусловно. Новое: селектор собирается
   из валидированных дескрипторов, а не из статического шаблона.
4. **`sniff: false` на TUN** — оставляем решение открытым: sniff нужен для
   domain-роутинга в TUN, но отключён в шаблоне; в новом ядре это конфиг-политика,
   а не случайность.
5. **Module path `snowden.system/windows`** — не переносим; в V3 уже
   `github.com/snowden-system/windows`.
6. **CLI main.go не переносится** — у нас композиция идёт через Wails app.go;
   CLI-режим останется отдельной сборкой (tags), если понадобится.
7. `.workbuddy-ai/`, дубли документов и `docs/cloudflare-ip-restriction.md`
   (там был токен!) — в новый репозиторий не переносить никогда.

## 4. Соответствие контракту PLAN (FR-001..007)

- Строгая валидация (FR-005) — реализована в старом config, соответствует
  плану почти дословно; берём как спецификацию + добавляем expected-egress
  и UDP-gate.
- Селектор только из валидированных каналов (FR-002/FR-003) — в старом ядре
  реализован частично: селектор проверяется структурно, но «валидирован» =
  «формат+TLS», не «live-verified». Новое ядро обязано связать
  validation_status из дескриптора с составом селектора.
- Сериализация lifecycle и разделение контекстов (FR-004) — реализовано и
  протестировано; паттерн подтверждён live (V2-004).
- Серверный baseline `route.final: direct` (FR-006) — в server-combined.json
  корректен; live-проверка сегодня: `final: direct` на сервере.
- Отчёт probe, структурно валидируемый (FR-003) — ProbeReport в старом ядре
  простой; в новом — json-схема + шаги в UI (уже есть ProbeReportView).

## 5. Вердикт готовности к разработке

**Достаточно для старта Phase A1 сегодня.** Все входы изучены:
- сервер жив и проверен сегодня (sing-box/nginx active, final=direct, 101 WS);
- секреты канала A в файлах + DPAPI-хранилище (5/5 verify=ok), сверка хешей
  с сервером автоматизирована (TestSecret);
- SSH-ключ доступа работает, host-ключи закреплены;
-Sing-box версия зафиксирована (1.13.19, совпадает с сервером);
- паттерны ядра изучены и сведены в §2; анти-паттерны — в §3;
- инструментарий (Go 1.26.7, Wails 2.13, Node 22) работает.

Блокеры не-кодового типа (решения владельца): git-инициализация V3, судьба
CF-токена (ACTIVE), Rotация секретов после acceptance, TUN/HY2 live-этапы на
машине владельца. Ни один не блокирует написание ядра и юнит-контрактов A1.4.

**Порядок нового ядра** (следующий шаг): config (строгий парс + expected-egress
в probe-типах) → engine (обёртка sing-box v1.13.19) → manager (паттерн §2.2)
→ probe (DNS-through-tunnel + 2 egress-маркера + expected-egress + direct-leak)
→ CLI-смок SOCKS → reload/selector контракт-тест (A1.4).
