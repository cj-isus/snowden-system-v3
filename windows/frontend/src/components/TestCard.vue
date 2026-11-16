<script setup lang="ts">
/**
 * Карточка теста: название, описание, статус «только факт», кнопка запуска.
 * ok=null означает «не запускался/недоступен» — это отдельный честный статус.
 */
import { computed } from 'vue'
import type { DiagnosticContext, TestResult } from '@/api/contract'
import Icon from './Icon.vue'
import ActionButton from './ActionButton.vue'
import type { IconName } from './Icon.vue'
import { diagnosticResultCurrent, fmtDateTime, isSkippedTest, hasUnknownEgressComparison, manualDiagnosticOutcome } from '../api/labels'
import ProbeReportView from './ProbeReportView.vue'

const props = defineProps<{
  test: TestResult
  running: boolean
  disabled?: boolean
  context?: Readonly<DiagnosticContext> | null
}>()

const emit = defineEmits<{ run: [id: string] }>()

// Presentation metadata for the existing Go diagnostic IDs; never a result.
const descriptions: Record<string, { title: string; scope: string; next: string }> = {
  'udp-availability': { title: 'Ответ внешнего UDP-узла', scope: 'QUIC Version Negotiation от 8.8.8.8:443 с проверкой случайных CID. Ответ не аутентифицирует сервер и не проверяет HY2; тишина оставляет результат неизвестным.', next: 'Для HY2 нужен результат подключения самого канала. Не меняйте DNS или защиту по одной UDP-пробе.' },
  'hy2-reachability': { title: 'Ответ объявленного HY2-узла', scope: 'Один endpoint из подходящего дескриптора. UDP-ответ не подтверждает фактический selector, HY2 auth/obfs или защищённый путь.', next: 'Посмотрите результат проверки пути при подключении к этому HY2-каналу.' },
  'vless-config-present': { title: 'Привязки учётных данных', scope: 'Штатный валидатор выбранных каналов и точных SecretID на одном снимке vault. Доступ к серверу не проверяется.', next: 'Сверьте привязки и формат в разделе локальных секретов. Не публикуйте значения.' },
  'elevated-process': { title: 'Права текущего процесса', scope: 'Права администратора текущего процесса Windows; это не проверка TUN или защиты трафика.', next: 'Сверьте требуемый режим. Повышенные права сами по себе не открывают допуск TUN.' },
  'freeze-check': { title: 'Загрузка контрольных объёмов', scope: 'HTTPS-цель активного канала и контрольные запросы по правилам текущего режима. Ошибка загрузки не устанавливает причину блокировки.', next: 'Изучите шаги и ограничения режима. Для вывода о блокировке нужны независимые измерения.' },
  'core-wired': { title: 'Готовность ядра к запуску', scope: 'Сообщённая приложением готовность ядра. Соединение, выходной IP и защита других приложений не измеряются.', next: 'Если запуск недоступен, изучите причину на главной странице.' },
}
const description = computed(() => descriptions[props.test.id] ?? {
  title: props.test.id,
  scope: 'Область этой проверки не описана текущей версией интерфейса.',
  next: 'Уточните область и причину результата перед изменением настроек.',
})
const current = computed(() => diagnosticResultCurrent(props.test, props.context ?? null))
const skipped = computed(() => current.value && isSkippedTest(props.test))
const legacyComparison = computed(() => props.test.steps?.some(hasUnknownEgressComparison) ?? false)
const outcome = computed(() => current.value ? manualDiagnosticOutcome(props.test) : null)

const statusIcon = computed<IconName>(() => {
  if (props.running) return 'refresh'
  if (legacyComparison.value && outcome.value !== false) return 'info'
  if (skipped.value) return 'info'
  if (outcome.value === true) return 'check'
  if (outcome.value === false) return 'cross'
  return 'info'
})

const statusLabel = computed(() => {
  if (props.running) return 'выполняется…'
  if (!current.value && props.test.checkedAt) return 'контекст не подтверждён'
  if (legacyComparison.value && outcome.value !== false) return 'сравнение недоступно'
  if (skipped.value) return 'пропущено'
  if (outcome.value === true) return 'пройдено'
  if (outcome.value === false) return 'не пройдено'
  return props.test.checkedAt ? 'нет результата' : 'не запускался'
})

const statusClass = computed(() => {
  if (props.running) return 'running'
  if (legacyComparison.value && outcome.value !== false) return 'idle'
  if (skipped.value) return 'idle'
  if (outcome.value === true) return 'pass'
  if (outcome.value === false) return 'fail'
  return 'idle'
})
</script>

<template>
  <section class="card">
    <div class="card-head">
      <div class="titles">
        <h2>{{ description.title }}</h2>
        <p class="check-id mono">{{ test.id }}</p>
      </div>
      <ActionButton :disabled="props.running || props.disabled" :busy="props.running" :aria-label="`Запустить проверку: ${description.title}`" @click="emit('run', test.id)">
        <Icon :name="statusIcon" :size="14" :class="{ spin: props.running }" />
        {{ props.running ? 'Идёт…' : 'Запустить' }}
      </ActionButton>
    </div>

    <p class="desc"><strong>Сообщение проверки:</strong> {{ test.detail || 'Подробный результат не передан.' }}</p>
    <p class="scope"><strong>Область:</strong> {{ description.scope }}</p>

    <div class="status-row">
      <span class="status" :class="statusClass">
        <Icon :name="statusIcon" :size="13" :class="{ spin: props.running }" />
        {{ statusLabel }}
      </span>
      <span class="when">последний запуск: {{ fmtDateTime(props.test.checkedAt) }}</span>
    </div>

    <p class="next-step"><strong>Следующий шаг:</strong> {{ props.running ? 'Ожидается ответ приложения. Остановка VPN доступна на главной странице.' : description.next }}</p>
    <p class="when">{{ current ? 'Контекст сопоставлен с последними сведениями приложения.' : 'Актуальность контекста не подтверждена.' }} Срок действия измерения не сообщён; совпадение контекста его не продлевает.</p>

    <details v-if="test.steps?.length" class="probe-report">
      <summary>Шаги проверки · {{ test.steps.length }}</summary>
      <ProbeReportView :report="{ ok: outcome === true, steps: test.steps }" :running="false" />
    </details>
  </section>
</template>

<style scoped>
.card {
  padding: 14px 16px;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--bg-panel);
}

.card-head {
  display: flex;
  align-items: flex-start;
  gap: 12px;
}
.titles {
  flex: 1 1 14rem;
  min-width: 0;
}
h2 {
  font-size: 14px;
}
.desc {
  margin: 2px 0 0;
  color: var(--text-dim);
  font-size: 12.5px;
}
.check-id { margin: 3px 0 0; color: var(--muted); font-size: 11px; }
.scope, .next-step { margin: 8px 0; color: var(--text-soft); font-size: 12.5px; line-height: 1.6; }
.scope strong, .next-step strong { font-weight: 600; }
.card { min-width: 0; overflow-wrap: anywhere; }
.card-head, .status-row { flex-wrap: wrap; }
.card-head .btn { flex: 0 0 auto; }
.probe-report summary { cursor: pointer; padding: 8px 0; color: var(--text-soft); }

.btn {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 7px 12px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--bg-elevated);
  color: var(--text);
  font-size: 12.5px;
  cursor: pointer;
  min-height: 40px;
  transition: background 0.12s;
}
.btn:hover:not([aria-disabled='true']) {
  background: var(--bg-hover);
}
.status-row {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 10px;
}
.status {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  padding: 2px 9px;
  border-radius: 999px;
}
.status.pass {
  color: var(--ok);
  background: var(--ok-soft);
}
.status.fail {
  color: var(--danger);
  background: var(--danger-soft);
}
.status.running {
  color: var(--accent);
  background: var(--accent-soft);
}
.status.idle {
  color: var(--text-faint);
  background: var(--bg-hover);
}

.when {
  font-size: 11.5px;
  color: var(--text-faint);
}

.probe-report { margin-top: 10px; }

.spin {
  animation: spin 1s linear infinite;
}
@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
