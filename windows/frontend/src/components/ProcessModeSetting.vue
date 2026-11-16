<script setup lang="ts">
import { computed } from 'vue'
import { isBuildPhase } from '../api/backend'
import { fmtDateTime } from '../api/labels'
import { useBackendState } from '../composables/backendState'

defineProps<{ id: string }>()
const { processModeFacts, processModeLoading, processModeError, processModeReceivedAt, processModeLabel,
  modeMessage, modeChangeReason, canChangeMode, modeRequestPending, initTUNMode, setTUNMode } = useBackendState()
const targetLabel = computed(() => processModeFacts.value?.mode === 'tun' ? 'Перезапустить в режиме прокси' : 'Запросить TUN с перезапуском')
function refresh(): void {
  if (!processModeLoading.value && !isBuildPhase()) void initTUNMode()
}
function change(): void {
  const facts = processModeFacts.value
  if (canChangeMode.value && facts) void setTUNMode(facts.mode === 'socks')
}
</script>

<template>
  <div class="process-mode-setting" :aria-busy="processModeLoading || modeRequestPending">
    <div class="mode-row">
      <div class="mode-copy">
        <h3>Режим текущего процесса</h3>
        <p class="source-note">{{ processModeLabel }}</p>
        <p v-if="processModeFacts" class="source-note">PID {{ processModeFacts.pid }} · сообщение текущего приложения, не результат проверки трафика.</p>
        <p v-if="processModeReceivedAt" class="source-note">Получено экраном: {{ fmtDateTime(processModeReceivedAt) }}</p>
        <p v-if="processModeError" class="source-note problem">{{ processModeError }}</p>
      </div>
      <div class="mode-actions">
        <button type="button" class="btn" :aria-disabled="processModeLoading || isBuildPhase()" :aria-describedby="id + '-help'" @click="refresh">Обновить режим</button>
        <button v-if="processModeFacts" type="button" class="btn" :aria-disabled="!canChangeMode" :aria-describedby="id + '-help ' + id + '-reason'" @click="change">{{ modeRequestPending ? 'Смена режима ожидает ответа…' : targetLabel }}</button>
      </div>
    </div>
    <p :id="id + '-help'" class="source-note">Смена режима требует другого процесса. Выбор режима и ответ команды не подтверждают перезапуск, включённый прокси, готовность TUN или защищённое подключение.</p>
    <p :id="id + '-reason'" class="source-note">{{ modeChangeReason }}</p>
    <p v-if="modeMessage && modeMessage !== modeChangeReason" class="source-note outcome">{{ modeMessage }}</p>
  </div>
</template>

<style scoped>
.process-mode-setting { min-width: 0; padding: 18px 0; border-bottom: 1px solid var(--border); }
.mode-row { display: flex; flex-wrap: wrap; align-items: center; gap: 14px 20px; }
.mode-copy { min-width: 0; flex: 1 1 320px; }
.mode-actions { display: flex; flex-wrap: wrap; min-width: 0; gap: 10px; flex: 0 1 auto; }
h3 { margin: 0 0 8px; font-size: 14px; }
.source-note { margin: 6px 0 0; color: var(--muted); font-size: 12.5px; line-height: 1.65; overflow-wrap: anywhere; }
.problem, .outcome { color: var(--yellow); }
.btn { min-height: 40px; white-space: normal; }
.btn[aria-disabled='true'] { color: var(--muted); cursor: not-allowed; }
</style>
