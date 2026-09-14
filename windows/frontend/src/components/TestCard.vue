<script setup lang="ts">
/**
 * Карточка теста: название, описание, статус «только факт», кнопка запуска.
 * ok=null означает «не запускался/недоступен» — это отдельный честный статус.
 */
import { computed } from 'vue'
import type { TestResult } from '@/api/contract'
import Icon from './Icon.vue'
import type { IconName } from './Icon.vue'
import { fmtDateTime } from '../api/labels'

const props = defineProps<{
  test: TestResult
  running: boolean
}>()

const emit = defineEmits<{ run: [id: string] }>()

const statusIcon = computed<IconName>(() => {
  if (props.running) return 'refresh'
  if (props.test.ok === true) return 'check'
  if (props.test.ok === false) return 'cross'
  return 'info'
})

const statusLabel = computed(() => {
  if (props.running) return 'выполняется…'
  if (props.test.ok === true) return 'пройдено'
  if (props.test.ok === false) return 'не пройдено'
  return 'не запускался'
})

const statusClass = computed(() => {
  if (props.running) return 'running'
  if (props.test.ok === true) return 'pass'
  if (props.test.ok === false) return 'fail'
  return 'idle'
})
</script>

<template>
  <section class="card">
    <div class="card-head">
      <div class="titles">
        <h3>{{ test.id }}</h3>
        <p class="desc">{{ test.detail || 'Без описания' }}</p>
      </div>
      <button class="btn" :disabled="props.running" @click="emit('run', test.id)">
        <Icon :name="statusIcon" :size="14" :class="{ spin: props.running }" />
        {{ props.running ? 'Идёт…' : 'Запустить' }}
      </button>
    </div>

    <div class="status-row">
      <span class="status" :class="statusClass">
        <Icon :name="statusIcon" :size="13" :class="{ spin: props.running }" />
        {{ statusLabel }}
      </span>
      <span class="when">последний запуск: {{ fmtDateTime(props.test.checkedAt) }}</span>
    </div>

    <ol v-if="test.steps && test.steps.length" class="steps">
      <li v-for="(s, i) in test.steps" :key="i" class="step" :class="s.status">
        <span class="step-name">{{ s.name }}</span>
        <span class="step-status">{{ s.status }}</span>
      </li>
  </ol>
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
  flex: 1;
}
h3 {
  font-size: 14px;
}
.desc {
  margin: 2px 0 0;
  color: var(--text-dim);
  font-size: 12.5px;
}

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
  transition: background 0.12s;
}
.btn:hover:not(:disabled) {
  background: var(--bg-hover);
}
.btn:disabled {
  opacity: 0.55;
  cursor: default;
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

.steps {
  margin: 10px 0 0;
  padding: 0;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.step {
  display: flex;
  justify-content: space-between;
  font-family: var(--mono);
  font-size: 11.5px;
  color: var(--text-dim);
  padding: 3px 8px;
  border-radius: 6px;
  background: var(--bg-elevated);
}
.step.pass .step-status {
  color: var(--ok);
}
.step.fail .step-status {
  color: var(--danger);
}
.step.running .step-status {
  color: var(--accent);
}

.spin {
  animation: spin 1s linear infinite;
}
@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
