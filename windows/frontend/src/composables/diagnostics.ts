/** Production GUI diagnostics. Metadata polling never executes a diagnostic. */
import { computed, effectScope, onScopeDispose, ref, watch, type EffectScope } from 'vue'
import type { DiagnosticContext, TestResult } from '@/api/contract'
import { api } from '@/api/backend'
import { diagnosticContextKey, diagnosticResultCurrent, knownManualDiagnostic } from '@/api/labels'
import { refreshState, useBackendState } from './backendState'
import { createPoller } from './polling'
import { toast } from './toast'
import { uiLog } from './uiLog'

const root = useBackendState()
const context = computed<Readonly<DiagnosticContext> | null>(() => root.state.value?.diagnosticContext ?? null)
const blocked = computed(() => !root.stateLoaded.value || root.isBusy.value || root.switching.value || root.tunBusy.value || !!root.state.value?.probeRunning)
const tests = ref<TestResult[] | null>(null)
const runningId = ref<string | null>(null)
const batchRunning = ref(false)
const batchStopRequested = ref(false)
const busy = computed(() => batchRunning.value || runningId.value !== null)
const loading = ref(false)
const loaded = ref(false)
const loadError = ref('')
const lastOutcome = ref('')
const receivedAt = ref<string | null>(null) // renderer receipt, not measurement/expiry time
const validated = ref<Record<string, string>>({})
const contextAvailable = computed(() => !!context.value?.intentCurrent && !context.value.closing)
const canRun = computed(() => users > 0 && contextAvailable.value && !blocked.value && !busy.value && !!tests.value?.length)
type Consumer = { active: boolean }
type Owner = { consumer: Consumer; generation: number; contextKey: string }
type Batch = Owner & { continue: boolean }
type MetadataSnapshot = { result: TestResult[] | null; contextKey: string; owner: number; failed: boolean }
let users = 0
let generation = 0
let viewScope: EffectScope | null = null
let pendingAction: Promise<boolean> | null = null
let batch: Batch | null = null
let invalidating: Promise<void> | null = null
let refreshWanted = false

const poller = createPoller<MetadataSnapshot>(async () => {
  const owner = generation
  try {
    await refreshState() // same root-owned state request; a newer lifecycle event wins
    const contextKey = diagnosticContextKey(context.value)
    const result = await api.listTests()
    if (result && (result.length > 64 || result.some(item => typeof item.id !== 'string' || item.id.length > 128) ||
      new Set(result.map(item => item.id)).size !== result.length)) throw new Error('Unsupported diagnostic list')
    return { result, contextKey, owner, failed: false }
  } catch {
    return { result: null, contextKey: diagnosticContextKey(context.value), owner, failed: true }
  }
}, ({ result, contextKey, owner, failed }) => {
  if (!users || busy.value || owner !== generation || contextKey !== diagnosticContextKey(context.value)) return
  if (failed) {
    validated.value = {}; receivedAt.value = null; loaded.value = true
    loadError.value = 'Не удалось перечитать сведения. Сохранённые наблюдения показаны без подтверждения текущего контекста.'
    return
  }
  if (result !== null) tests.value = result
  loaded.value = true
  receivedAt.value = result === null ? null : new Date().toISOString()
  validated.value = Object.fromEntries((result ?? []).filter(item => diagnosticResultCurrent(item, context.value)).map(item => [item.id, contextKey]))
  loadError.value = result === null ? 'Список недоступен. Сохранённые наблюдения не подтверждены текущим ответом приложения.' :
    !contextAvailable.value ? 'Приложение не передало поддерживаемый контекст проверок. Актуальность результатов не установлена.' : ''
}, 5000, () => {}, pending => { if (users) loading.value = pending })

/** Coalesce invalidation waiters as well as actual reads while native metadata
 * is pending. An event storm keeps one continuation and one latest-needed bit. */
function invalidateMetadata(): Promise<void> {
  refreshWanted = true
  if (invalidating) return invalidating
  if (!users || busy.value) return Promise.resolve()
  refreshWanted = false
  const request: Promise<void> = poller.invalidate().finally(() => {
    if (invalidating === request) invalidating = null
    if (refreshWanted && users && !busy.value) void invalidateMetadata()
  })
  invalidating = request
  return request
}

function resultContext(id: string): Readonly<DiagnosticContext> | null {
  return !blocked.value && validated.value[id] === diagnosticContextKey(context.value) ? context.value : null
}

function owns(owner: Owner): boolean {
  return users > 0 && owner.consumer.active && owner.generation === generation && !blocked.value &&
    contextAvailable.value && owner.contextKey === diagnosticContextKey(context.value)
}

function reserveOwner(consumer: Consumer): Owner {
  return { consumer, generation, contextKey: diagnosticContextKey(context.value) }
}

async function refreshTests(consumer: Consumer): Promise<void> {
  if (!consumer.active || !users || busy.value) return
  const owner = reserveOwner(consumer)
  await invalidateMetadata()
  if (owns(owner) && receivedAt.value !== null && loadError.value === '') {
    lastOutcome.value = 'Сведения перечитаны. Измерения не запускались.'
    toast('info', lastOutcome.value)
  }
}

function stopBatch(consumer: Consumer): void {
  if (!consumer.active || !batch) return
  batch.continue = false
  batchStopRequested.value = true
  lastOutcome.value = 'Следующие проверки не будут запущены. Ожидаем завершения текущего вызова.'
}

async function execute(id: string, owner: Owner): Promise<boolean> {
  if (pendingAction || !owns(owner) || !knownManualDiagnostic(id)) return false
  runningId.value = id
  const remaining = { ...validated.value }
  delete remaining[id]
  validated.value = remaining
  poller.stop() // retain any original metadata request, revoke only its publication
  const request: Promise<boolean> = Promise.resolve().then(() => owns(owner) && !('continue' in owner && owner.continue === false) ? api.runTest(id) : null).then(result => {
    if (!owns(owner) || !result || result.id !== id || !diagnosticResultCurrent(result, context.value)) return false
    tests.value = tests.value?.map(item => item.id === id ? result : item) ?? null
    validated.value = { ...validated.value, [id]: owner.contextKey }
    loadError.value = ''
    uiLog('info', 'Диагностика: получен отчёт приложения.') // no raw details/errors/IDs in telemetry
    return true
  }).catch(() => {
    if (owns(owner)) {
      const remaining = { ...validated.value }; delete remaining[id]; validated.value = remaining
      loadError.value = 'Новый отчёт не получен. Причина сетевого или серверного отказа не установлена.'
    }
    return false
  }).finally(() => {
    if (pendingAction === request) {
      pendingAction = null
      runningId.value = null // actual Promise settlement, not unmount/timeout
    }
    if (users && !batchRunning.value) { refreshWanted = false; poller.start() }
  })
  pendingAction = request
  return request
}

async function runTest(id: string, consumer: Consumer): Promise<void> {
  if (!consumer.active || !canRun.value || !tests.value?.some(item => item.id === id)) return
  const owner = reserveOwner(consumer)
  const received = await execute(id, owner)
  if (!owns(owner)) return
  lastOutcome.value = received ? 'Ответ приложения получен. Результат и область показаны в карточке.' : 'Новый отчёт не получен; результат не установлен.'
  toast(received ? 'info' : 'warn', lastOutcome.value)
}

async function runAll(consumer: Consumer): Promise<void> {
  if (!consumer.active || !canRun.value) return
  const ids = tests.value!.map(item => item.id).filter(knownManualDiagnostic)
  if (!ids.length) return
  const owner: Batch = { ...reserveOwner(consumer), continue: true }
  batch = owner
  batchRunning.value = true
  batchStopRequested.value = false
  let received = 0
  try {
    for (const id of ids) {
      if (!owns(owner) || !owner.continue) break
      if (!await execute(id, owner)) break
      received++
      if (owns(owner)) lastOutcome.value = `Получено ответов приложения: ${received} из ${ids.length}. Это не число успешных сетевых проверок.`
    }
    if (owns(owner)) {
      lastOutcome.value = `${owner.continue ? 'Пакет завершён' : 'Пакет остановлен'}: получено ответов ${received} из ${ids.length}. Незавершённые проверки не считаются выполненными.`
      toast('info', lastOutcome.value)
    }
  } finally {
    if (batch === owner) { batch = null; batchRunning.value = false; batchStopRequested.value = false }
    if (users) { refreshWanted = false; poller.start() }
  }
}

/** One explicitly owned watcher/poller for all mounted consumers. Each action
 * also retains its initiating consumer; a late reply cannot toast in another. */
export function useDiagnostics() {
  const consumer: Consumer = { active: true }
  users++
  if (users === 1) {
    generation++
    viewScope = effectScope(true)
    viewScope.run(() => watch(() => `${diagnosticContextKey(context.value)}:${blocked.value}`, (_next, previous) => {
      generation++
      validated.value = {}
      receivedAt.value = null
      if (batch) { batch.continue = false; batchStopRequested.value = true }
      if (busy.value) lastOutcome.value = previous === undefined ? 'Ожидается ответ ранее отправленного вызова. Следующие запуски остановлены.' :
        'Контекст изменился. Старый ответ не будет показан для нового подключения; следующие запуски остановлены.'
      refreshWanted = true
      if (!busy.value) void invalidateMetadata()
    }, { immediate: true, flush: 'sync' }))
    if (!busy.value) poller.start()
  }
  onScopeDispose(() => {
    consumer.active = false
    if (batch?.consumer === consumer) { batch.continue = false; batchStopRequested.value = true }
    if (--users === 0) {
      generation++
      refreshWanted = false
      viewScope?.stop(); viewScope = null
      poller.stop()
      tests.value = null; validated.value = {}; receivedAt.value = null
      loaded.value = false; loading.value = false; loadError.value = ''; lastOutcome.value = ''
      // runningId/pendingAction/batch remain owned until their original return.
    }
  })
  return { tests, runningId, busy, batchRunning, batchStopRequested, loading, loaded, loadError, lastOutcome, receivedAt, canRun, resultContext,
    runTest: (id: string) => runTest(id, consumer), runAll: () => runAll(consumer),
    refreshTests: () => refreshTests(consumer), stopBatch: () => stopBatch(consumer) }
}
