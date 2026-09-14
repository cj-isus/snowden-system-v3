<script setup lang="ts">
/**
 * Probe-отчёт: пять обязательных проверок. Показывается только фактический
 * результат; нет отчёта — «нет данных» (AGENTS.md §3.4).
 */
import type { ProbeStep } from '@/api/contract'
import { probeStepStatusLabels } from '../api/labels'

// Readonly-форма: стор отдаёт DeepReadonly<AppState> (readonly(state));
// компонент не мутирует отчёт — только отображает.
defineProps<{
  report: { readonly ok: boolean; readonly steps: readonly ProbeStep[] } | null
  running: boolean
}>()
</script>

<template>
  <div v-if="running" class="running">Probe выполняется…</div>
  <div v-else-if="!report" class="empty">Нет данных: probe ещё не запускался</div>
  <ol v-else class="steps">
    <li v-for="(s, i) in report.steps" :key="i" class="step" :class="s.status">
      <span class="name">{{ s.name }}</span>
      <span class="st">{{ probeStepStatusLabels[s.status] }}</span>
    </li>
  </ol>
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

.step.pass .st {
  color: var(--ok);
}
.step.fail .st {
  color: var(--danger);
}
.step.running .st {
  color: var(--accent);
}
.step.skipped .st {
  color: var(--text-faint);
}
</style>
