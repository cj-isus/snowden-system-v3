# Документ перемещён

Каноническая версия: [открыть](../../../theory/research/04-PERFORMANCE/REPORT.md).

Здесь сохранена навигация для прежних ссылок; редактировать следует новый документ.

# R04 — путь трафика и бюджет полезной операции

[Перейти к разделу](../../../theory/research/04-PERFORMANCE/REPORT.md#r04--путь-трафика-и-бюджет-полезной-операции).

## 1. Вопрос и отличие от существующей теории

[Перейти к разделу](../../../theory/research/04-PERFORMANCE/REPORT.md#1-вопрос-и-отличие-от-существующей-теории).

## 2. Входы и воспроизводимость ручного анализа

[Перейти к разделу](../../../theory/research/04-PERFORMANCE/REPORT.md#2-входы-и-воспроизводимость-ручного-анализа).

## 3. R04-01 — фактический путь и правильная граница времени

[Перейти к разделу](../../../theory/research/04-PERFORMANCE/REPORT.md#3-r04-01--фактический-путь-и-правильная-граница-времени).

### 3.1. Windows: два входа, общий worker

[Перейти к разделу](../../../theory/research/04-PERFORMANCE/REPORT.md#31-windows-два-входа-общий-worker).

### 3.2. Android: условный путь, не готовая поставка

[Перейти к разделу](../../../theory/research/04-PERFORMANCE/REPORT.md#32-android-условный-путь-не-готовая-поставка).

### 3.3. Полный latency budget одной операции

[Перейти к разделу](../../../theory/research/04-PERFORMANCE/REPORT.md#33-полный-latency-budget-одной-операции).

## 4. R04-02 — три разных бюджета датаграммы

[Перейти к разделу](../../../theory/research/04-PERFORMANCE/REPORT.md#4-r04-02--три-разных-бюджета-датаграммы).

### 4.1. Постоянная fragmentation граница

[Перейти к разделу](../../../theory/research/04-PERFORMANCE/REPORT.md#41-постоянная-fragmentation-граница).

### 4.2. Почему нельзя просто поднять 1197 или довериться успешному SendDatagram

[Перейти к разделу](../../../theory/research/04-PERFORMANCE/REPORT.md#42-почему-нельзя-просто-поднять-1197-или-довериться-успешному-senddatagram).

## 5. R04-03 — bounded queue не ограничивает receive work и resident memory

[Перейти к разделу](../../../theory/research/04-PERFORMANCE/REPORT.md#5-r04-03--bounded-queue-не-ограничивает-receive-work-и-resident-memory).

## 6. Сводный ресурсный бюджет

[Перейти к разделу](../../../theory/research/04-PERFORMANCE/REPORT.md#6-сводный-ресурсный-бюджет).

## 7. Очерёдность предложений разработчикам

[Перейти к разделу](../../../theory/research/04-PERFORMANCE/REPORT.md#7-очерёдность-предложений-разработчикам).

## 8. Будущие конечные проверки — не запускались

[Перейти к разделу](../../../theory/research/04-PERFORMANCE/REPORT.md#8-будущие-конечные-проверки--не-запускались).

## 9. Предлагаемая вставка в общую теорию

[Перейти к разделу](../../../theory/research/04-PERFORMANCE/REPORT.md#9-предлагаемая-вставка-в-общую-теорию).

<a id="r04-002"></a>
## 10. R04-002 — обслуживание общего HY2 carrier при конкуренции TCP/UDP

[Перейти к разделу](../../../theory/research/04-PERFORMANCE/REPORT.md#10-r04-002--обслуживание-общего-hy2-carrier-при-конкуренции-tcpudp).

### 10.1. Новизна, входы и уровень доказательства

[Перейти к разделу](../../../theory/research/04-PERFORMANCE/REPORT.md#101-новизна-входы-и-уровень-доказательства).

### 10.2. Полный граф обслуживания: разные units и receipts

[Перейти к разделу](../../../theory/research/04-PERFORMANCE/REPORT.md#102-полный-граф-обслуживания-разные-units-и-receipts).

### 10.3. R04-04 — условное исключение STREAM по размеру

[Перейти к разделу](../../../theory/research/04-PERFORMANCE/REPORT.md#103-r04-04--условное-исключение-stream-по-размеру).

### 10.4. R04-05 — ограничения должны пережить handoff ресурса

[Перейти к разделу](../../../theory/research/04-PERFORMANCE/REPORT.md#104-r04-05--ограничения-должны-пережить-handoff-ресурса).

### 10.5. R04-06 — текущий eviction не замыкает borrower lifetime

[Перейти к разделу](../../../theory/research/04-PERFORMANCE/REPORT.md#105-r04-06--текущий-eviction-не-замыкает-borrower-lifetime).

### 10.6. Ручные bounds: что действительно следует из условий

[Перейти к разделу](../../../theory/research/04-PERFORMANCE/REPORT.md#106-ручные-bounds-что-действительно-следует-из-условий).

### 10.7. Готовые подходы: сравнение по месту применения

[Перейти к разделу](../../../theory/research/04-PERFORMANCE/REPORT.md#107-готовые-подходы-сравнение-по-месту-применения).

### 10.8. Windows, Android и маленький VPS

[Перейти к разделу](../../../theory/research/04-PERFORMANCE/REPORT.md#108-windows-android-и-маленький-vps).

### 10.9. Самостоятельный оппонирующий review

[Перейти к разделу](../../../theory/research/04-PERFORMANCE/REPORT.md#109-самостоятельный-оппонирующий-review).

### 10.10. Конечная будущая приёмка и предложения

[Перейти к разделу](../../../theory/research/04-PERFORMANCE/REPORT.md#1010-конечная-будущая-приёмка-и-предложения).

### 10.11. Предлагаемая вставка руководителю

[Перейти к разделу](../../../theory/research/04-PERFORMANCE/REPORT.md#1011-предлагаемая-вставка-руководителю).

### 10.12. Актуальность и сохранность пакетов

[Перейти к разделу](../../../theory/research/04-PERFORMANCE/REPORT.md#1012-актуальность-и-сохранность-пакетов).

<a id="r04-003"></a>
## 11. R04-003 — keepalive, часы сна и цена готовности

[Перейти к разделу](../../../theory/research/04-PERFORMANCE/REPORT.md#11-r04-003--keepalive-часы-сна-и-цена-готовности).

### 11.1. Новый вопрос и граница знания

[Перейти к разделу](../../../theory/research/04-PERFORMANCE/REPORT.md#111-новый-вопрос-и-граница-знания).

### 11.2. Exact source и toolchain reference

[Перейти к разделу](../../../theory/research/04-PERFORMANCE/REPORT.md#112-exact-source-и-toolchain-reference).

### 11.3. R04-07 — часы с одинаковым словом monotonic не взаимозаменяемы

[Перейти к разделу](../../../theory/research/04-PERFORMANCE/REPORT.md#113-r04-07--часы-с-одинаковым-словом-monotonic-не-взаимозаменяемы).

### 11.4. R04-08 — один keepalive не заменяет другой

[Перейти к разделу](../../../theory/research/04-PERFORMANCE/REPORT.md#114-r04-08--один-keepalive-не-заменяет-другой).

### 11.5. Due → queue → wire → refresh — разные события

[Перейти к разделу](../../../theory/research/04-PERFORMANCE/REPORT.md#115-due--queue--wire--refresh--разные-события).

### 11.6. Модель энергии и допустимое объединение работы

[Перейти к разделу](../../../theory/research/04-PERFORMANCE/REPORT.md#116-модель-энергии-и-допустимое-объединение-работы).

### 11.7. Reuse / adapt / reject

[Перейти к разделу](../../../theory/research/04-PERFORMANCE/REPORT.md#117-reuse--adapt--reject).

### 11.8. Оппонирование и falsification

[Перейти к разделу](../../../theory/research/04-PERFORMANCE/REPORT.md#118-оппонирование-и-falsification).

### 11.9. Конечная будущая проверка — не разрешение её выполнять

[Перейти к разделу](../../../theory/research/04-PERFORMANCE/REPORT.md#119-конечная-будущая-проверка--не-разрешение-её-выполнять).

### 11.10. Вставка для руководителя и остаток

[Перейти к разделу](../../../theory/research/04-PERFORMANCE/REPORT.md#1110-вставка-для-руководителя-и-остаток).

### 11.11. Финальная source-сверка и новый внешний review

[Перейти к разделу](../../../theory/research/04-PERFORMANCE/REPORT.md#1111-финальная-source-сверка-и-новый-внешний-review).

