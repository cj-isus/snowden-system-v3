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
let initialization: Promise<void> | null = null
let generation = 0
let eventRevision = 0

/** Инициализация: единственная подписка на backend-события + первый getState. */
function initBackendState(): Promise<void> {
  if (initialization) return initialization
  const owner = generation
  const revision = eventRevision
  if (!unsubscribe) unsubscribe = api.subscribe(
    (s) => {
      if (owner !== generation) return
      eventRevision++
      state.value = s
      stateLoaded.value = true
    },
    (l) => {
      if (owner !== generation) return
      // Backend-событие лога: пробрасываем в журнал UI. Go LogLine = {t, level,
      // text} — читаем именно эти поля (дефект V2-045: читали l.line, его нет,
      // и каждая строка падала в JSON.stringify — в журнале был сырой JSON).
      const raw = l as { t?: string; level?: string; text?: string }
      const level = raw.level ?? 'info'
      const text = raw.text ?? JSON.stringify(l)
      uiLog(level as 'debug' | 'info' | 'warn' | 'error', `[backend] ${text}`, raw.t)
    },
  )

  initialization = (async () => {
    const results = await Promise.allSettled([api.getState(), api.listChannels(), api.listTests(), api.isTUNMode()])
    if (owner !== generation) return
    const [snapshot, channelList, testList, mode] = results
    // An event delivered after getState began always wins over that snapshot.
    if (revision === eventRevision) state.value = snapshot.status === 'fulfilled' ? snapshot.value : null
    stateLoaded.value = true
    channels.value = channelList.status === 'fulfilled' ? channelList.value : null
    tests.value = testList.status === 'fulfilled' ? testList.value : null
    tunMode.value = mode.status === 'fulfilled' ? mode.value : false
    if (isBuildPhase()) backendLog('warn', 'Предпросмотр: VPN backend недоступен')
    if (results.some(result => result.status === 'rejected')) {
      backendLog('warn', 'Часть данных backend недоступна. Повторите загрузку.')
      // A failed auxiliary snapshot must not disconnect lifecycle events.
      // A retry reuses this subscription; only the root owner disposes it.
      initialization = null
    }
  })()
  return initialization
}

/** App root owns the subscription. Retire pending responses before cleanup. */
export function disposeBackendState(): void {
  generation++
  unsubscribe?.()
  unsubscribe = null
  initialization = null
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
  if (isBuildPhase()) return { label: 'Откройте приложение', kind: 'start', disabled: true }
  const s = lifecycle.value
  if (!stateLoaded.value || !s) return { label: '…', kind: 'start', disabled: true }
  if (busy.value) return { label: busyLabel.value || '…', kind: s === 'running' ? 'stop' : 'start', disabled: true }
  if (s === 'running') return { label: 'Остановить', kind: 'stop', disabled: false }
  if (s === 'starting' || s === 'stopping' || s === 'reloading') return { label: 'Подождите…', kind: 'start', disabled: true }
  return { label: 'Подключить', kind: 'start', disabled: false }
})

async function startVpn(): Promise<void> {
  if (isBusy.value || switching.value || isRunning.value) return
  const action = ++actionGeneration
  busy.value = true
  busyLabel.value = 'Запуск…'
  try {
    await api.start()
  } catch (e) {
    if (action === actionGeneration) toast('error', errText(e, 'Запуск не выполнен'))
  } finally {
    if (action === actionGeneration) { busy.value = false; busyLabel.value = '' }
  }
}

let actionGeneration = 0
let stopping = false
async function stopVpn(): Promise<void> {
  // Stop must remain available while a slow Start or Switch is pending.
  if (stopping) return
  stopping = true
  const action = ++actionGeneration
  busy.value = true
  busyLabel.value = 'Остановка…'
  try {
    await api.stop()
  } catch (e) {
    toast('error', errText(e, 'Остановка не выполнена'))
  } finally {
    stopping = false
    if (action === actionGeneration) { busy.value = false; busyLabel.value = '' }
  }
}

const switching = ref(false)

/**
 * Переключение активного канала (A1.4): Reload + обязательный probe.
 * Ошибка = fail-closed backend'а; UI показывает честный текст.
 */
async function selectChannel(id: string): Promise<void> {
  if (switching.value || isBusy.value || !isRunning.value) return
  const action = ++actionGeneration
  switching.value = true
  try {
    await api.selectChannel(id)
    if (action === actionGeneration) toast('info', 'Канал переключён: ' + id)
  } catch (e) {
    if (action === actionGeneration) toast('error', errText(e, 'Переключение не выполнено'))
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
