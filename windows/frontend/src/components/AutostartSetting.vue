<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { api, isBuildPhase } from '../api/backend'
import { fmtDateTime } from '../api/labels'

const props = defineProps<{ refreshVersion: number }>()
const value = ref<boolean | null>(null)
const reading = ref(false)
const writing = ref(false)
const receivedAt = ref<string | null>(null)
const outcome = ref('')
const problem = ref(false)
let active = true
let revision = 0
let pendingRead: Promise<void> | null = null
let refreshQueued = false

onBeforeUnmount(() => { active = false; revision++; refreshQueued = false })
onMounted(() => { void refresh() })
watch(() => props.refreshVersion, () => { void refresh() })

function current(owner: number): boolean { return active && owner === revision }

function failureMessage(error: unknown): string {
  const message = typeof error === 'string' ? error : error instanceof Error ? error.message : ''
  if (message.includes('содержит другую команду')) return 'Обнаружена другая команда автозапуска. Она не перезаписывается.'
  if (message.includes('поддерживаемом формате')) return 'Формат записи автозапуска не поддерживается. Состояние неизвестно.'
  if (message.includes('ещё не завершена')) return 'Предыдущая операция автозапуска ещё не завершена. Изменение недоступно; перечитайте состояние позже.'
  return 'Состояние или изменение автозапуска не подтверждено. Обновите состояние перед новой попыткой.'
}

function unavailable(error: unknown): void {
  value.value = null
  receivedAt.value = null
  problem.value = true
  outcome.value = failureMessage(error)
}

function publish(actual: boolean): void {
  value.value = actual
  receivedAt.value = new Date().toISOString() // UI receipt time, not registry/logon proof
  problem.value = false
}

// One original read or write/readback at a time. Only an explicit refresh can
// queue one follow-up; no polling, timeout-as-completion or automatic write retry.
function refresh(): Promise<void> {
  if (!active || isBuildPhase()) return Promise.resolve()
  if (writing.value) { refreshQueued = true; return Promise.resolve() }
  if (pendingRead) {
    refreshQueued = true
    revision++ // discard this older read before starting the requested follow-up
    return pendingRead
  }
  const owner = ++revision
  reading.value = true
  value.value = null
  receivedAt.value = null
  problem.value = false
  outcome.value = ''
  pendingRead = Promise.resolve().then(async () => {
    if (!current(owner)) return
    const actual = await api.getAutostart()
    if (!current(owner)) return
    publish(actual)
    outcome.value = 'Состояние автозапуска прочитано.'
  }).catch(error => {
    if (current(owner)) unavailable(error)
  }).finally(() => {
    pendingRead = null // the original promise settled; unmount did not cancel Wails
    if (!active) return
    reading.value = false
    if (refreshQueued) { refreshQueued = false; void refresh() }
  })
  return pendingRead
}

function refreshFromButton(): void {
  if (reading.value || writing.value || isBuildPhase()) return
  void refresh()
}

async function toggle(): Promise<void> {
  if (!active || isBuildPhase() || reading.value || pendingRead || writing.value || value.value === null) return
  const wanted = !value.value, owner = ++revision
  writing.value = true
  value.value = null
  receivedAt.value = null
  outcome.value = ''
  problem.value = false
  try {
    await api.setAutostart(wanted)
    if (!current(owner)) return
    const actual = await api.getAutostart() // belongs to this write, never a shared old read
    if (!current(owner)) return
    publish(actual)
    problem.value = actual !== wanted
    outcome.value = actual === wanted ? 'Повторное чтение соответствует запросу.' : 'После команды получено другое состояние; изменение не подтверждено.'
  } catch (error) {
    if (current(owner)) unavailable(error)
  } finally {
    if (current(owner)) {
      revision++ // no read captured during this mutation can publish afterwards
      writing.value = false
      if (refreshQueued) { refreshQueued = false; void refresh() }
    }
  }
}

function description(): string {
  if (isBuildPhase()) return 'Настройка доступна в приложении Windows.'
  if (writing.value) return 'Ожидается результат изменения и повторного чтения.'
  if (reading.value) return 'Считываем состояние автозапуска.'
  if (value.value === null) return 'Состояние неизвестно. Обновите его перед изменением.'
  return value.value ? 'Включён для команды текущего приложения.' : 'Сообщён выключенным.'
}
</script>

<template>
  <div class="autostart-setting">
    <div class="setting-row" :aria-busy="reading || writing">
      <div class="setting-copy">
        <h3>Автозапуск при входе в Windows</h3>
        <p class="source-note">{{ description() }}</p>
        <p v-if="receivedAt" class="source-note">Получено экраном: {{ fmtDateTime(receivedAt) }}</p>
      </div>
      <div class="actions-row autostart-actions">
        <button type="button" class="btn" :aria-disabled="reading || writing || isBuildPhase()" @click="refreshFromButton">Обновить состояние автозапуска</button>
        <button type="button" class="btn" :aria-disabled="reading || writing || value === null || isBuildPhase()" @click="toggle">{{ writing ? 'Применение…' : value === null ? 'Изменить автозапуск' : value ? 'Выключить автозапуск' : 'Включить автозапуск' }}</button>
      </div>
    </div>
    <p class="source-note outcome" :class="{ error: problem }" role="status" aria-atomic="true">{{ outcome }}</p>
    <p class="source-note">Автозапуск задаёт команду запуска приложения с запросом подключения. Запись команды не подтверждает её исполнение при следующем входе; готовность VPN проверяется отдельно.</p>
  </div>
</template>

<style scoped>
.autostart-setting { min-width: 0; padding: 18px 0; border-bottom: 1px solid var(--border); }
.setting-row { display: flex; flex-wrap: wrap; align-items: center; gap: 14px 20px; }
.setting-copy { min-width: 0; flex: 1 1 320px; }
.autostart-actions { min-width: 0; flex: 0 1 auto; }
h3 { margin: 0 0 8px; font-size: 14px; }
.source-note { margin: 6px 0 0; color: var(--muted); font-size: 12.5px; line-height: 1.65; overflow-wrap: anywhere; }
.outcome { min-height: 1.65em; margin-top: 8px; }
.error { color: var(--yellow); }
.btn { min-height: 40px; white-space: normal; }
.btn[aria-disabled='true'] { color: var(--muted); cursor: not-allowed; }
</style>
