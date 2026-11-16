/**
 * Реактивный стор состояния backend. Единственный источник фактов для UI.
 * Build phase: unavailable-состояния и честные ошибки (AGENTS.md §3.6).
 * Никаких выдуманных значений: null = источник недоступен.
 */
import { computed, readonly, ref, shallowRef } from 'vue'
import type { AppState, ChannelDescriptor, DiagnosticContext, ProcessModeFacts, UILifecycleReceipt } from '@/api/contract'
import { api, isBuildPhase } from '@/api/backend'
import { uiFailure } from '../api/failureMessages'
import { backendLog, toast } from './toast'
import { uiLog } from './uiLog'
import { diagnosticContextKey } from '../api/labels'
import { createPoller } from './polling'

const state = ref<AppState | null>(null)
const stateLoaded = ref(false)
const channels = ref<ChannelDescriptor[] | null>(null) // null = источник недоступен
const channelsLoading = ref(false)
const channelsError = ref('')
const channelsReceivedAt = ref<string | null>(null)
type PositiveRequest = { token: symbol; kind: 'start' | 'mode' | 'update' }
  | { token: symbol; kind: 'switch'; target: string }
const positiveRequest = shallowRef<PositiveRequest | null>(null)
const stopRequest = shallowRef<symbol | null>(null)
const commandMessage = ref('')
const stateSync = shallowRef<{ action: symbol; root: symbol; demand: symbol; intent: string | null; message: string } | null>(null)
const modeHold = shallowRef<{ message: string } | null>(null) // negative obligation survives root disposal; never a native receipt
const updateHold = shallowRef<{ message: string } | null>(null)
const updateNote = ref('')
const busy = computed(() => positiveRequest.value !== null || stopRequest.value !== null || stateSync.value !== null || modeHold.value !== null || updateHold.value !== null)
const switching = computed(() => positiveRequest.value?.kind === 'switch')
const busyLabel = computed(() => stopRequest.value ? 'Остановка…' : modeHold.value || updateHold.value ? 'Нужен новый процесс…' : stateSync.value ? 'Обновление состояния…' : switching.value ? 'Переключение…' : positiveRequest.value?.kind === 'update' ? 'Применение обновления…' : positiveRequest.value?.kind === 'mode' ? 'Смена режима…' : positiveRequest.value ? 'Запуск…' : '')

let unsubscribe: (() => void) | null = null
let initialization: Promise<void> | null = null
let generation = Symbol()
let eventRevision = Symbol()
let currentAction = Symbol()
let stateDemand = Symbol()
let contextFloor: DiagnosticContext | null = null
let channelsActive = false
let channelsRevision = Symbol()
let channelsPending: Promise<boolean> | null = null
let statePending: Promise<boolean> | null = null

function decimalOrder(a: string, b: string): number { return a.length === b.length ? (a === b ? 0 : a < b ? -1 : 1) : a.length < b.length ? -1 : 1 }

function publishState(snapshot: AppState): boolean {
  const context = snapshot.diagnosticContext
  if (context && contextFloor) {
    const order = decimalOrder(context.revision, contextFloor.revision)
    if (order < 0) return false
    if (order === 0 && diagnosticContextKey(context) !== diagnosticContextKey(contextFloor)) { state.value = null; return false }
  }
  if (context) contextFloor = context
  const selectorChanged = !state.value || state.value.state !== snapshot.state || state.value.activeChannelId !== snapshot.activeChannelId || state.value.proxyMode !== snapshot.proxyMode
  state.value = snapshot
  stateLoaded.value = true
  if (selectorChanged) {
    channelsRevision = Symbol()
    channels.value = null
    channelsReceivedAt.value = null
    void refreshChannels()
  }
  return true
}

function reconcileCommand(demand: symbol): void {
  const sync = stateSync.value, context = state.value?.diagnosticContext
  if (!sync || sync.demand !== demand || sync.root !== generation || sync.action !== currentAction) return
  if (!context || sync.intent !== null && decimalOrder(context.intent, sync.intent) < 0) return
  stateSync.value = null
  commandMessage.value = sync.message + ' Показаны обновлённые сведения приложения; завершение команды не является доказательством защищённого пути.'
}

function refreshAfterCommand(action: symbol, root: symbol, receipt: UILifecycleReceipt | null, message: string): void {
  stateDemand = Symbol()
  stateSync.value = { action, root, demand: stateDemand, intent: receipt?.intent ?? null, message }
  commandMessage.value = message + ' ' + uiFailure.lifecyclePendingState
  void refreshState() // one latest demand, no per-command waiter list or overlapping GetState
}

/** The root owns one state read; a newer event wins over its delayed reply. */
export function refreshState(): Promise<boolean> {
  if (!channelsActive) return Promise.resolve(false)
  if (statePending) return statePending
  const owner = generation, revision = eventRevision, demand = stateDemand
  statePending = Promise.resolve().then(() => channelsActive && owner === generation && demand === stateDemand ? api.getState() : null).then(snapshot => {
    if (!snapshot || !channelsActive || owner !== generation || demand !== stateDemand) return false
    const newerNative = snapshot.diagnosticContext && contextFloor && decimalOrder(snapshot.diagnosticContext.revision, contextFloor.revision) > 0
    if (revision === eventRevision || newerNative) publishState(snapshot)
    stateLoaded.value = true
    reconcileCommand(demand)
    return state.value !== null
  }, () => {
    if (!channelsActive || owner !== generation || demand !== stateDemand) return false
    if (revision === eventRevision) state.value = null
    stateLoaded.value = true
    return state.value !== null
  }).finally(() => {
    statePending = null
    if (channelsActive && (owner !== generation || demand !== stateDemand)) void refreshState()
  })
  return statePending
}

/** The root owns this read; lifecycle edges retire an older selector view. */
export function refreshChannels(): Promise<boolean> {
  if (!channelsActive) return Promise.resolve(false)
  if (channelsPending) return channelsPending
  const owner = generation, revision = channelsRevision
  channelsLoading.value = true
  channelsError.value = ''
  channelsPending = Promise.resolve().then(() => channelsActive && owner === generation && revision === channelsRevision ? api.listChannels() : null).then(result => {
    if (!channelsActive || owner !== generation || revision !== channelsRevision) return false
    channels.value = result
    channelsReceivedAt.value = result === null ? null : new Date().toISOString()
    if (result === null) channelsError.value = 'Список каналов недоступен. Откройте приложение Windows.'
    return result !== null
  }, () => {
    if (!channelsActive || owner !== generation || revision !== channelsRevision) return false
    channels.value = null
    channelsReceivedAt.value = null
    channelsError.value = 'Не удалось получить список каналов. Повторите загрузку.'
    return false
  }).finally(() => {
    channelsPending = null
    if (channelsActive && (owner !== generation || revision !== channelsRevision)) void refreshChannels()
    else channelsLoading.value = false
  })
  return channelsPending
}

/** Инициализация: единственная подписка на backend-события + первый getState. */
function initBackendState(): Promise<void> {
  if (initialization) return initialization
  channelsActive = true
  const owner = generation
  if (!unsubscribe) unsubscribe = api.subscribe(
    (s) => {
      if (owner !== generation) return
      if (publishState(s)) eventRevision = Symbol()
    },
    (l) => {
      if (owner !== generation) return
      // Consume only Go's known fields; never serialize an unknown event body.
      const raw = l && typeof l === 'object' && !Array.isArray(l) ? l as { t?: unknown; level?: unknown; text?: unknown } : {}
      const stamp = typeof raw.t === 'string' ? raw.t : ''
      const level = raw.level
      if (level !== 'debug' && level !== 'info' && level !== 'warn' && level !== 'error') {
        uiLog('warn', '[backend] Сообщение с неподдерживаемым уровнем; подробности не приняты.', stamp)
        return
      }
      const text = typeof raw.text === 'string' ? raw.text : 'Текст сообщения backend не передан в поддерживаемом формате.'
      uiLog(level, `[backend] ${text}`, stamp)
    },
  )

  initialization = (async () => {
    const results = await Promise.allSettled([refreshState(), refreshChannels(), initTUNMode()])
    if (owner !== generation) return
    const [snapshot, channelList, mode] = results
    stateLoaded.value = true
    if (isBuildPhase()) backendLog('warn', 'Предпросмотр: VPN backend недоступен')
    if (results.some(result => result.status === 'rejected') || snapshot.status === 'fulfilled' && !snapshot.value || mode.status === 'fulfilled' && !mode.value || channelList.status === 'fulfilled' && !channelList.value && channelsError.value !== '') {
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
  generation = Symbol()
  currentAction = Symbol()
  stateDemand = Symbol()
  stateSync.value = null
  commandMessage.value = ''
  api.retireLifecycleRequests()
  modeReader.stop()
  processModeFacts.value = null
  processModeReceivedAt.value = null
  processModeError.value = ''
  modeNote.value = ''
  updateNote.value = '' // keep the original call and any unknown install obligation
  state.value = null
  stateLoaded.value = false
  channelsActive = false
  channelsRevision = Symbol()
  channels.value = null
  channelsReceivedAt.value = null
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
const stopPending = computed(() => stopRequest.value !== null)
// Missing positive metadata cannot remove the negative Stop escape. The Go
// handler still checks its original registration and release boundaries.
const canStop = computed(() => !isBuildPhase() && !stopPending.value && state.value?.lifecycleActions?.stopAllowed !== false)
// Presentation of the latest request only. Stop/root retirement revokes this
// label without clearing the original call or granting selected/applied status.
const pendingSwitchTarget = computed(() => {
  const request = positiveRequest.value
  return channelsActive && stateLoaded.value && stopRequest.value === null && request?.kind === 'switch' && request.token === currentAction ? request.target : null
})

/** Глагол действия на главной кнопке — только фактические состояния. */
const primaryAction = computed<{ label: string; kind: 'start' | 'stop'; disabled: boolean }>(() => {
  if (isBuildPhase()) return { label: 'Откройте приложение', kind: 'start', disabled: true }
  const s = lifecycle.value
  if (stopPending.value) return { label: 'Остановка…', kind: 'stop', disabled: true }
  if (busy.value || switching.value || s === 'starting' || s === 'reloading' || s === 'stopping') {
    return { label: 'Остановить и отменить', kind: 'stop', disabled: !canStop.value }
  }
  if (!stateLoaded.value || !s) return { label: '…', kind: 'start', disabled: true }
  if (s === 'running') return { label: 'Остановить', kind: 'stop', disabled: !canStop.value }
  if (!api.hasOrderedLifecycle()) return { label: 'Нужно обновить приложение', kind: 'start', disabled: true }
  const actions = state.value?.lifecycleActions
  if (!actions) return { label: 'Доступность запуска неизвестна', kind: 'start', disabled: true }
  if (!actions.startAllowed) return { label: 'Запуск сейчас недоступен', kind: 'start', disabled: true }
  return { label: actions.explicitStartRequired ? 'Проверить и подключить' : 'Подключить', kind: 'start', disabled: false }
})

async function startVpn(): Promise<void> {
  if (isBusy.value || switching.value || isRunning.value || primaryAction.value.kind !== 'start' || primaryAction.value.disabled) return
  const action = Symbol(), root = generation
  currentAction = action
  positiveRequest.value = { token: action, kind: 'start' }
  const interest = { current: () => channelsActive && root === generation && action === currentAction }
  commandMessage.value = 'Команда запуска ожидает ответа. Соединение ещё не подтверждено.'
  try {
    const receipt = await api.start(interest)
    if (interest.current()) {
      const failed = receipt.outcome !== 'registered'
      if (failed) toast('error', uiFailure.start)
      refreshAfterCommand(action, root, receipt, failed ? uiFailure.start : 'Запрос запуска зарегистрирован.')
    }
  } catch {
    if (interest.current()) { toast('error', uiFailure.start); refreshAfterCommand(action, root, null, uiFailure.start) }
  } finally {
    if (positiveRequest.value?.token === action) positiveRequest.value = null
  }
}

async function stopVpn(): Promise<void> {
  // Stop must remain available while a slow Start or Switch is pending.
  if (!canStop.value) return
  const action = Symbol(), root = generation
  currentAction = action
  stateSync.value = null
  stopRequest.value = action
  const interest = { current: () => channelsActive && root === generation && action === currentAction }
  commandMessage.value = 'Команда отключения ожидает ответа. Очистка и снятие защиты ещё не подтверждены.'
  try {
    const receipt = await api.stop(interest)
    if (interest.current()) {
      const failed = receipt !== null && receipt.outcome !== 'returned'
      if (failed) toast('error', uiFailure.stop)
      refreshAfterCommand(action, root, receipt, failed ? uiFailure.stop : 'Получен ответ на запрос отключения.')
    }
  } catch {
    if (interest.current()) { toast('error', uiFailure.stop); refreshAfterCommand(action, root, null, uiFailure.stop) }
  } finally {
    if (stopRequest.value === action) stopRequest.value = null
  }
}

/**
 * Переключение активного канала (A1.4): Reload + обязательный probe.
 * Ошибка = fail-closed backend'а; UI показывает честный текст.
 */
async function selectChannel(id: string): Promise<void> {
  if (switching.value || isBusy.value || !isRunning.value || !api.hasOrderedLifecycle()) return
  const action = Symbol(), root = generation
  currentAction = action
  positiveRequest.value = { token: action, kind: 'switch', target: id }
  const interest = { current: () => channelsActive && root === generation && action === currentAction }
  commandMessage.value = 'Команда переключения ожидает ответа. Новый канал ещё не подтверждён.'
  try {
    const receipt = await api.selectChannel(id, interest)
    if (interest.current()) {
      const failed = receipt.outcome !== 'returned'
      if (failed) toast('error', uiFailure.switchChannel)
      refreshAfterCommand(action, root, receipt, failed ? uiFailure.switchChannel : 'Получен ответ на запрос переключения.')
    }
  } catch {
    if (interest.current()) { toast('error', uiFailure.switchChannel); refreshAfterCommand(action, root, null, uiFailure.switchChannel) }
  } finally {
    if (positiveRequest.value?.token === action) positiveRequest.value = null
  }
}

const processModeFacts = shallowRef<ProcessModeFacts | null>(null)
const processModeLoading = ref(false)
const processModeError = ref('')
const processModeReceivedAt = ref<string | null>(null) // renderer receipt time, not a mode-change or validity receipt
const modeNote = ref('')
let modeAnchor: { pid: number; mode: ProcessModeFacts['mode'] } | null = null
const modeReader = createPoller(api.getRuntimeInfo, runtime => {
  const facts = runtime?.processMode ?? null
  if (facts && modeAnchor && (facts.pid !== modeAnchor.pid || facts.mode !== modeAnchor.mode)) {
    processModeFacts.value = null
    processModeError.value = 'Источник или режим процесса изменился без нового сеанса интерфейса. Смена режима не подтверждена.'
    return
  }
  if (facts && !modeAnchor) modeAnchor = { pid: facts.pid, mode: facts.mode }
  processModeFacts.value = facts
  processModeReceivedAt.value = facts ? new Date().toISOString() : null
  processModeError.value = facts ? '' : 'Приложение не передало поддерживаемые сведения о режиме процесса. Обновите сведения или комплект приложения.'
}, null, () => {
  processModeFacts.value = null
  processModeReceivedAt.value = null
  processModeError.value = 'Сведения о режиме процесса не получены. Значение по умолчанию не подставляется.'
}, pending => {
  processModeLoading.value = pending
  if (pending) { processModeFacts.value = null; processModeReceivedAt.value = null; processModeError.value = '' }
})
const tunMode = computed<boolean | null>(() => processModeFacts.value ? processModeFacts.value.mode === 'tun' : null)
const modeRequestPending = computed(() => positiveRequest.value?.kind === 'mode')
const tunBusy = computed(() => modeRequestPending.value || modeHold.value !== null)
const processModeLabel = computed(() => processModeFacts.value?.mode === 'tun' ? 'TUN · режим процесса' : processModeFacts.value?.mode === 'socks' ? 'Прокси · режим процесса' : 'Режим процесса неизвестен')
const modeMessage = computed(() => modeHold.value?.message || modeNote.value)
const modeChangeReason = computed(() => {
  const facts = processModeFacts.value, context = state.value?.diagnosticContext
  if (isBuildPhase()) return 'Смена режима доступна в приложении Windows.'
  if (modeHold.value) return modeHold.value.message
  if (processModeLoading.value) return 'Ожидаются сведения о текущем процессе.'
  if (!facts) return 'Сначала обновите сведения о режиме процесса.'
  if (!api.hasOrderedLifecycle()) return uiFailure.lifecycleUnavailable
  if (isBusy.value || switching.value) return 'Дождитесь завершения операции подключения или явно отключите VPN.'
  if (!context || !context.intentCurrent || context.closing) return 'Действующее намерение приложения не подтверждено. Обновите состояние перед сменой режима.'
  if (facts.mode === 'socks' && (!facts.tunAdmission || !facts.elevationAdmission)) return 'В этой сборке системный допуск TUN или передача новому процессу ещё не приняты.'
  return ''
})
const canChangeMode = computed(() => modeChangeReason.value === '')

async function initTUNMode(): Promise<boolean> {
  if (!channelsActive || isBuildPhase()) return false
  modeReader.start()
  await modeReader.refresh()
  return channelsActive && processModeFacts.value !== null
}

/** Target choice never changes the observed process flag. The actual ordered
 * handler captures the restart source and rechecks all native handoff gates. */
async function setTUNMode(enable: boolean): Promise<void> {
  const facts = processModeFacts.value
  if (!canChangeMode.value || !facts || typeof enable !== 'boolean' || enable === (facts.mode === 'tun')) return
  const action = Symbol(), root = generation
  currentAction = action
  positiveRequest.value = { token: action, kind: 'mode' }
  let dispatched = false
  const interest = { current: () => channelsActive && root === generation && action === currentAction, dispatched: () => { dispatched = true } }
  modeNote.value = 'Команда смены режима ожидает ответа. Текущий процесс продолжает иметь прежний режим.'
  const failure = enable ? uiFailure.enableTUN : uiFailure.disableTUN
  try {
    const receipt = enable ? await api.enableTUN(interest) : await api.disableTUN(interest)
    const rejectedBeforeBody = receipt.handlerEntered === false && (receipt.outcome === 'rejected' || receipt.outcome === 'superseded')
    if (!rejectedBeforeBody) {
      modeHold.value = { message: receipt.outcome === 'returned'
        ? 'Ответ на смену режима получен. Ожидается новый процесс; его запуск, режим и защищённое подключение здесь ещё не подтверждены.'
        : 'Результат смены режима не подтверждён. Повторные команды этому процессу остановлены; сначала завершите отключение и восстановление приложения.' }
    }
    if (interest.current()) {
      modeNote.value = rejectedBeforeBody ? 'Команда смены режима отклонена. Обновите сведения перед новой попыткой.' : ''
      if (receipt.outcome !== 'returned') toast('error', failure)
      refreshAfterCommand(action, root, receipt, receipt.outcome === 'returned' ? 'Получен ответ на запрос смены режима.' : failure)
    }
  } catch {
    if (dispatched) modeHold.value = { message: 'Ответ на отправленную смену режима не подтверждён. Прежний процесс не считается новым; повторный запуск смены режима требует восстановления приложения.' }
    if (interest.current()) { modeNote.value = failure; toast('error', failure); refreshAfterCommand(action, root, null, failure) }
  } finally {
    if (positiveRequest.value?.token === action) positiveRequest.value = null
    if (interest.current()) void initTUNMode()
  }
}

export const updateActionReason = computed(() => {
  if (isBuildPhase()) return 'Обновление доступно в приложении Windows.'
  const loaded = stateLoaded.value, snapshot = state.value // reactive even before the root activates
  if (!channelsActive || !api.hasOrderedLifecycle()) return uiFailure.lifecycleUnavailable
  if (updateHold.value) return updateHold.value.message
  if (modeHold.value) return modeHold.value.message
  if (isBusy.value) return 'Дождитесь ответа текущей операции или явно отключите VPN.'
  if (!loaded || !snapshot) return 'Обновите сведения о состоянии приложения.'
  return ''
})
export const updateMessage = computed(() => updateHold.value?.message || updateNote.value)
export const updateCommandPending = computed(() => positiveRequest.value?.kind === 'update')

/** The caller consumes its visible approval first. This original slot is shared
 * with Start/Switch/mode; Stop still supersedes it without dropping ownership. */
export type UpdateApplyOutcome = 'not_sent' | 'before_install' | 'next_process_unconfirmed' | 'unconfirmed'
export async function applyUpdatePackage(token: string, viewCurrent: () => boolean): Promise<UpdateApplyOutcome> {
  if (updateActionReason.value || !viewCurrent()) return 'not_sent'
  const action = Symbol(), root = generation
  currentAction = action
  positiveRequest.value = { token: action, kind: 'update' }
  let dispatched = false
  let outcome: UpdateApplyOutcome = 'unconfirmed'
  const interest = { current: () => channelsActive && root === generation && action === currentAction && viewCurrent(), dispatched: () => { dispatched = true } }
  updateNote.value = 'Применение ожидает ответа. Новая версия и перезапуск ещё не подтверждены.'
  const failure = 'Обновление не завершено. Подтверждение пакета использовано; для повторной попытки нужна новая проверка.'
  try {
    const receipt = await api.applyUpdate(token, interest)
    const beforeInstall = receipt.updateInstallEntered === false && ['rejected', 'superseded', 'failed'].includes(receipt.outcome)
    outcome = beforeInstall ? 'before_install' : receipt.outcome === 'returned' ? 'next_process_unconfirmed' : 'unconfirmed'
    if (!beforeInstall) {
      updateHold.value = { message: receipt.outcome === 'returned'
        ? 'Получен ответ на применение. Ожидается новый процесс: проверьте его версию. Установка, запуск и подключение здесь ещё не подтверждены.'
        : 'Исход установки не подтверждён. Повторное применение в этом процессе остановлено; завершите отключение и восстановление приложения.' }
    }
    if (interest.current()) {
      updateNote.value = beforeInstall ? failure + ' Проверенная история доставки уже могла измениться.' : ''
      refreshAfterCommand(action, root, receipt, beforeInstall ? failure : 'Получен ответ на запрос обновления приложения.')
    }
  } catch {
    outcome = dispatched ? 'unconfirmed' : 'not_sent'
    if (dispatched) updateHold.value = { message: 'Ответ на отправленное применение не подтверждён. Требуется восстановление приложения; повторная установка в этом процессе недоступна.' }
    if (interest.current()) {
      updateNote.value = dispatched ? failure : 'Запрос установки не передан приложению. Для новой попытки нужна новая проверка пакета.'
      refreshAfterCommand(action, root, null, updateNote.value)
    }
  } finally {
    if (positiveRequest.value?.token === action) positiveRequest.value = null
    if (!interest.current() && !updateHold.value) updateNote.value = ''
  }
  return outcome
}

export {
  state,
  stateLoaded,
  channels,
  busy,
  busyLabel,
  switching,
  pendingSwitchTarget,
  tunMode,
  tunBusy,
  modeRequestPending,
  processModeFacts,
  processModeLoading,
  processModeError,
  processModeReceivedAt,
  processModeLabel,
  modeMessage,
  modeChangeReason,
  canChangeMode,
  lifecycle,
  isRunning,
  isBusy,
  hasError,
  primaryAction,
  stopPending,
  canStop,
  commandMessage,
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
    channelsLoading,
    channelsError,
    channelsReceivedAt,
    refreshChannels,
    lifecycle,
    isRunning,
    isBusy,
    hasError,
    switching,
    pendingSwitchTarget,
    tunMode,
    tunBusy,
    modeRequestPending,
    processModeFacts: readonly(processModeFacts),
    processModeLoading,
    processModeError,
    processModeReceivedAt,
    processModeLabel,
    modeMessage,
    modeChangeReason,
    canChangeMode,
    initTUNMode,
    primaryAction,
    stopPending,
    canStop,
    commandMessage: readonly(commandMessage),
    startVpn,
    stopVpn,
    selectChannel,
    setTUNMode,
  }
}
