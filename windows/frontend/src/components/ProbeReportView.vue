<script setup lang="ts">
/**
 * Shared presentation of the named diagnostic steps and their actual scope.
 * Remote address comparison never becomes a local leak-absence claim.
 */
import { computed } from 'vue'
import type { ProbeMeasurementStatus, ProbeScope, ProbeStep } from '@/api/contract'
import { probeComparisonLabel, probeStepDetail, probeStepName, probeStepStatus, probeStepStatusLabels, isInformationalProbeStep } from '../api/labels'

// Readonly-форма: стор отдаёт DeepReadonly<AppState> (readonly(state));
// компонент не мутирует отчёт — только отображает.
const props = defineProps<{
  report: { readonly ok: boolean; readonly steps: readonly ProbeStep[]; readonly scope?: Readonly<ProbeScope> } | null
  running: boolean
}>()

const scopeStatusLabels: Record<ProbeMeasurementStatus, string> = {
  observed: 'Ответ получен',
  incomplete: 'Не подтверждено',
  unmeasured: 'Не измерено',
}

const scopeRows = computed(() => {
  const scope = props.report?.scope
  if (!scope) return []
  return [
    { id: 'https', name: 'Прикладной HTTPS', status: scope.https, detail: 'Завершённые ответы целевых сервисов через выбранный канал.' },
    { id: 'doh', name: 'DNS через HTTPS', status: scope.doh, detail: 'DNS A-ответ в DoH; не проверка системного DNS.' },
    { id: 'ipv4-tcp', name: 'Системный IPv4 · TCP', status: scope.ipv4TCP, detail: 'Нужен отдельный результат для этого семейства и транспорта.' },
    { id: 'ipv4-udp', name: 'Системный IPv4 · UDP', status: scope.ipv4UDP, detail: 'DoH не измеряет доставку UDP через системный туннель.' },
    { id: 'ipv6-tcp', name: 'Системный IPv6 · TCP', status: scope.ipv6TCP, detail: 'IPv4-ответ не подтверждает путь IPv6.' },
    { id: 'ipv6-udp', name: 'Системный IPv6 · UDP', status: scope.ipv6UDP, detail: 'Нужен отдельный результат IPv6/UDP.' },
    { id: 'local-safety', name: 'Отсутствие локальных утечек', status: scope.localSafety, detail: 'Ответ HTTPS не измеряет исходящие пакеты других приложений.' },
  ]
})
</script>

<template>
  <div v-if="running" class="running">Probe выполняется…</div>
  <div v-else-if="!report" class="empty">Нет данных: probe ещё не запускался</div>
  <div v-else-if="report.steps.length === 0" class="empty">В отчёте нет шагов диагностики</div>
  <div v-else class="report">
    <ol class="steps">
      <li v-for="(s, i) in report.steps" :key="i" class="step" :class="probeStepStatus(s)">
        <div class="step-copy">
          <span class="name">{{ probeStepName(s) }}</span>
          <span v-if="isInformationalProbeStep(s)" class="requirement">Информация · не определяет допуск трафика</span>
          <p class="detail">{{ probeStepDetail(s) }}</p>
          <span v-if="probeComparisonLabel(s)" class="comparison">{{ probeComparisonLabel(s) }} · локальный маршрут не измерен</span>
        </div>
        <span class="st">{{ probeStepStatusLabels[probeStepStatus(s)] }}</span>
      </li>
    </ol>
    <table v-if="scopeRows.length" class="scope-table">
      <caption>Границы измерения в этом отчёте</caption>
      <thead><tr><th scope="col">Проверка</th><th scope="col">Результат</th></tr></thead>
      <tbody>
        <tr v-for="row in scopeRows" :key="row.id">
          <th scope="row">{{ row.name }}<span class="scope-detail">{{ row.detail }}</span></th>
          <td :class="`scope-${row.status}`">{{ scopeStatusLabels[row.status] }}</td>
        </tr>
      </tbody>
    </table>
    <p v-else class="scope-note">Область измерения в отчёте отсутствует. Ответы HTTPS и сравнения IP не подтверждают системный DNS, IPv4/IPv6, UDP или отсутствие локальных утечек.</p>
    <p v-if="report?.scope?.historical" class="scope-note">Отчёт устарел: эти ответы не подтверждают текущее подключение.</p>
  </div>
</template>

<style scoped>
.running,
.empty {
  font-size: 12.5px;
  color: var(--text-faint);
}

.steps {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.step {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding: 5px 10px;
  border-radius: 7px;
  background: var(--bg-elevated);
  font-family: var(--mono);
  font-size: 11.5px;
  color: var(--text-dim);
}

.step-copy { min-width: 0; }
.detail, .comparison, .scope-note, .requirement {
  margin: 4px 0 0;
  color: var(--text-faint);
  font-size: 11.5px;
  line-height: 1.5;
  overflow-wrap: anywhere;
}
.comparison, .requirement { display: block; }
.st { flex: 0 0 auto; }

.step.pass .st {
  color: var(--ok);
}
.step.fail .st {
  color: var(--danger);
}
.step.leak .st {
  color: var(--danger);
}
.step.unavailable .st {
  color: var(--text-faint);
}
.step.running .st {
  color: var(--accent);
}
.step.skipped .st {
  color: var(--text-faint);
}

.report { min-inline-size: 0; }
.scope-table {
  inline-size: 100%;
  table-layout: fixed;
  border-collapse: collapse;
  margin-block-start: 16px;
  font-size: 11.5px;
  line-height: 1.5;
}
.scope-table caption {
  text-align: start;
  color: var(--text-dim);
  padding-block-end: 8px;
  font-weight: 600;
}
.scope-table th, .scope-table td {
  padding: 8px 0;
  text-align: start;
  vertical-align: top;
  border-block-end: 1px solid var(--border);
  overflow-wrap: anywhere;
}
.scope-table th:first-child { inline-size: 70%; padding-inline-end: 12px; }
.scope-table thead th { color: var(--text-faint); font-weight: 400; }
.scope-table tbody th { color: var(--text-dim); font-weight: 500; }
.scope-detail { display: block; margin-block-start: 3px; color: var(--text-faint); font-weight: 400; }
.scope-table .scope-observed { color: var(--ok); }
.scope-table .scope-incomplete { color: var(--text-dim); }
.scope-table .scope-unmeasured { color: var(--text-faint); }
</style>
