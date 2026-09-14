/**
 * Реактивный стор состояния backend. Единственный источник фактов для UI.
 * Build phase: unavailable-состояния и честные ошибки (AGENTS.md §3.6).
 * Никаких выдуманных значений: null = источник недоступен.
 */
import { computed, readonly, ref } from 'vue'
import type { AppState, ChannelDescriptor, TestResult } from '@/api/contract'
import { api, isBuildPhase } from '@/api/backend'
import { backendLog, toast } from './toast'
import { uiLog } from './uiLog'

const state = ref<AppState | null>(null)
const stateLoaded = ref(false)
const channels = ref<ChannelDescriptor[] | null>(null) // null = источник недоступен
const tests = ref<TestResult[] | null>(null) // null = источник недоступен
const busy = ref(false) // идёт Start/Stop/Reload — кнопки блокируются
const busyLabel = ref('')

let unsubscribe: (() => void) | null = null

/** Инициализация: единственная подписка на backend-события + первый getState. */
async function initBackendState(): Promise<void> {
  if (unsubscribe) return
  unsubscribe = api.subscribe(
    (s) => {
      state.value = s
      stateLoaded.value = true
    },
    (l) => {
      // Backend-событие лога: пробрасываем в журнал UI. Go LogLine = {t, level,
      // text} — читаем именно эти поля (дефект V2-045: читали l.line, его нет,
      // и каждая строка падала в JSON.stringify — в журнале был сырой JSON).
      const raw = l as { t?: string; level?: string; text?: string }
      const level = raw.level ?? 'info'
      const text = raw.text ?? JSON.stringify(l)
      uiLog(level as 'debug' | 'info' | 'warn' | 'error', `[backend] ${text}`, raw.t)
    },
  )

  state.value = await api.getState()
  stateLoaded.value = true
  channels.value = await api.listChannels()
  tests.value = await api.listTests()

  if (isBuildPhase()) {
    backendLog('warn', 'UI работает в build phase: backend ещё не собран; статус недоступен, это честно показано')
  }
}

const lifecycle = computed(() => state.value?.state ?? null)
const isRunning = computed(() => lifecycle.value === 'running')
const isBusy = computed(
  () =>
    busy.value ||
    lifecycle.value === 'starting' ||
    lifecycle.value === 'stopping' ||
    lifecycle.value === 'reloading',
)
const hasError = computed(() => lifecycle.value === 'error' || !!state.value?.error)

/** Глагол действия на главной кнопке — только фактические состояния. */
const primaryAction = computed<{ label: string; kind: 'start' | 'stop'; disabled: boolean }>(() => {
  const s = lifecycle.value
  if (!stateLoaded.value || !s) return { label: '…', kind: 'start', disabled: true }
  if (busy.value) return { label: busyLabel.value || '…', kind: s === 'running' ? 'stop' : 'start', disabled: true }
  if (s === 'running') return { label: 'Остановить', kind: 'stop', disabled: false }
  if (s === 'starting' || s === 'stopping' || s === 'reloading') return { label: 'Подождите…', kind: 'start', disabled: true }
  return { label: 'Подключить', kind: 'start', disabled: false }
})

async function startVpn(): Promise<void> {
  busy.value = true
  busyLabel.value = 'Запуск…'
  try {
    await api.start()
  } catch (e) {
    toast('error', errText(e, 'Запуск не выполнен'))
  } finally {
    busy.value = false
    busyLabel.value = ''
  }
}

async function stopVpn(): Promise<void> {
  busy.value = true
  busyLabel.value = 'Остановка…'
  try {
    await api.stop()
  } catch (e) {
    toast('error', errText(e, 'Остановка не выполнена'))
  } finally {
    busy.value = false
    busyLabel.value = ''
  }
}

const switching = ref(false)

/**
 * Переключение активного канала (A1.4): Reload + обязательный probe.
 * Ошибка = fail-closed backend'а; UI показывает честный текст.
 */
async function selectChannel(id: string): Promise<void> {
  switching.value = true
  try {
    await api.selectChannel(id)
    toast('info', 'Канал переключён: ' + id)
  } catch (e) {
    toast('error', errText(e, 'Переключение не выполнено'))
  } finally {
    switching.value = false
  }
}

const tunMode = ref(false) // режим ЭТОГО процесса (--tun), не желание
const tunBusy = ref(false)

async function initTUNMode(): Promise<void> {
  if (isBuildPhase()) return
  tunMode.value = await api.isTUNMode()
}

/**
 * TUN-режим (WIN-TUN-1): Wintun требует администратора. Переключение =
 * перезапуск exe с/без --tun (UAC-запрос при включении). Backend сам
 * останавливает VPN и завершает процесс; UI просто ждёт перезапуска.
 */
async function setTUNMode(enable: boolean): Promise<void> {
  if (tunBusy.value) return
  tunBusy.value = true
  try {
    if (enable) {
      await api.enableTUN()
      toast('info', 'Перезапуск в TUN-режиме (права администратора). Нажмите «Подключить» после запуска.')
    } else {
      await api.disableTUN()
      toast('info', 'Перезапуск в SOCKS-режиме.')
    }
  } catch (e) {
    toast('error', errText(e, enable ? 'TUN не включён' : 'TUN не выключен'))
    tunBusy.value = false
  }
  // При успехе процесс завершится; tunBusy сбрасывать не нужно.
}

/** Единая точка превращения ошибки в безопасный текст (без секретов). */
function errText(e: unknown, prefix: string): string {
  const raw = e instanceof Error ? e.message : String(e)
  return `${prefix}: ${raw}`
}

export {
  state,
  stateLoaded,
  channels,
  tests,
  busy,
  busyLabel,
  switching,
  tunMode,
  tunBusy,
  lifecycle,
  isRunning,
  isBusy,
  hasError,
  primaryAction,
  initBackendState,
  initTUNMode,
  startVpn,
  stopVpn,
  selectChannel,
  setTUNMode,
}

/** Composable-доступ для компонентов. */
export function useBackendState() {
  return {
    state: readonly(state),
    stateLoaded,
    channels,
    tests,
    lifecycle,
    isRunning,
    isBusy,
    hasError,
    switching,
    tunMode,
    tunBusy,
    primaryAction,
    startVpn,
    stopVpn,
    selectChannel,
    setTUNMode,
  }
}
