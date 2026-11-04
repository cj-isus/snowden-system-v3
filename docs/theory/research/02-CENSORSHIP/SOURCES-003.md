# R02-003 — exact sources и пределы повторного использования

Срез начат в дополнительном окне25 октября UTC, оформлен **26 октября2026**.
Пути captures сохранены по дате старта; [INDEX](../../../../.artifacts/research/theory-02/2026-10-25-r02-003/INDEX.json).

## Исходники

- **Maybenot workspace pin** `51ce20e806f8e8bdffa0c39b464774d79d617234`.
  [lib.rs::get_machine](https://github.com/maybenot-io/maybenot/blob/51ce20e806f8e8bdffa0c39b464774d79d617234/crates/maybenot-machines/src/lib.rs)
  связывает StaticMachine inputs с constructors. Raw capture сохранён.
- [regulator.rs](https://github.com/maybenot-io/maybenot/blob/51ce20e806f8e8bdffa0c39b464774d79d617234/crates/maybenot-machines/src/regulator.rs):
  regulator_client/server, make_regulator_client_queue_machine, get_rate_bins.
  Конкретные timer transitions и seconds→microseconds, не полный audit scheduler.
- [front.rs](https://github.com/maybenot-io/maybenot/blob/51ce20e806f8e8bdffa0c39b464774d79d617234/crates/maybenot-machines/src/front.rs):
  front → gen_front → padding states → LimitReached. Raw fetch не удался;
  сохранён **rendered primary source observation**, не raw file/hash equality.
- [tamaraw.rs](https://github.com/maybenot-io/maybenot/blob/51ce20e806f8e8bdffa0c39b464774d79d617234/crates/maybenot-machines/src/tamaraw.rs):
  padding and soft-stop constructors; counter/timer/tail branches. Raw capture есть.
- Core framework/action/event того же commit повторно прочитан из R02-002 captures:
  from_micros, transition, decrement_limit, events. Те же версии, не новый runtime proof.
- [Core dist.rs](https://github.com/maybenot-io/maybenot/blob/51ce20e806f8e8bdffa0c39b464774d79d617234/crates/maybenot/src/dist.rs):
  дополнительное rendered opposing review: Uniform validate отвергает low>high,
  а sample при low=high возвращает low. Raw API timeout; отдельного raw capture нет.
- License scope: workspace **MIT OR Apache-2.0**, корневые тексты сохранены в002;
  новых production dependencies не вводилось.

**Paper artifact** `maybenot-io/popets-2026.1-ephemeral-defs-paper-artifacts`,
pin `31ff58466742c69b738191bc66789064bac849c2`, commit date2025-09-27:
[README](https://github.com/maybenot-io/popets-2026.1-ephemeral-defs-paper-artifacts/blob/31ff58466742c69b738191bc66789064bac849c2/README.md).
Прочитан reproducibility scope; scripts/experiments/model training не исполнялись.
Root LICENSE/Cargo lookup не дал usable source; **лицензия artifact code не установлена**,
допустимое решение здесь reference, не copy/adapt. Это не вывод об отсутствии
лицензии во всех вложенных компонентах.

## Первичная литература

| Источник | Контекст и использование |
|---|---|
| [Gong/Wang, USENIX2020 FRONT/GLUE](https://www.usenix.org/conference/usenixsecurity20/presentation/gong) | Front-focused randomized padding; отсутствие intentional delay, не гарантия latency под нагрузкой. Результаты автора не прогнозируются для РФ |
| [Holland/Hopper, RegulaTor, arXiv2012.06609v3](https://arxiv.org/abs/2012.06609), revised2021-09-21; PETS2022 | Исходный замысел и packet delay constraint отдельно от конкретной state-machine approximation |
| [Pulls et al., Ephemeral Network-Layer Fingerprinting Defenses, PETS2026(1)](https://www.petsymposium.org/popets/2026/popets-2026-0022.php), DOI10.56553/popets-2026-0022 | §§5.3–5.4 и AppendixE: переносимость обученных атак и трудности портирования; research/deployment setting не наш transport/RF result |
| [Полный PETS2026 paper](https://crysp.petsymposium.org/popets/2026/popets-2026-0022.pdf) | Прочитаны тематические текстовые секции; численные таблицы не заимствуются. Не выполнена репликация artifact |
| [Witwer/Holland/Hopper, WPES2022](https://www-users.cse.umn.edu/~hoppernj/padding_delay_wpes.pdf), DOI10.1145/3559613.3563207 | Padding-only может создавать queueing delays в simulated Tor deployment; не живое измерение нашего VPN |

Бумажные ручные traces/формулы REPORT — вывод автора R02, не результаты этих
исследований. Локальные7 snapshots в INDEX привязывают выводы к сохранённой теории,
renderer/manifest/native lock и неизменённому002; окончательная drift-сверка —
REVIEW.json рядом. Никакого сканирования, тестирования, симуляции, native/live или
чтения профилей/ключей/логов не было. Исправленный путь theory-02 соблюдён;
в ошибочный theory-0X этот чат не писал и чужие файлы не чистил.
