import { computed, readonly, ref, shallowRef, triggerRef } from 'vue'
import { cancelProxyRecovery, confirmProxyRecovery, hasProxyRecovery, prepareProxyRecovery } from '../api/backend'
import type { ProxyRecoveryPreview, ProxyRecoveryResult } from '../api/contract'

type Phase = 'idle' | 'preparing' | 'awaiting' | 'confirming' | 'cancelling' | 'complete' | 'held' | 'blocked'
type Owner = { requestId: string; previousRequestId: string; scopeKnown: boolean; intent: string; pending: boolean; cancelling: boolean; closed: boolean; confirmed: boolean; finished: boolean; failureMessage: string; timer: ReturnType<typeof setInterval> | null }

// One original callback owner survives route disposal. Closing only withdraws
// UI interest; its pending promise cannot be replaced by another preparation.
const owner = shallowRef<Owner | null>(null)
const phase = ref<Phase>('idle')
const visible = ref(false)
const preview = shallowRef<ProxyRecoveryPreview | null>(null)
const result = shallowRef<ProxyRecoveryResult | null>(null)
const message = ref('')
const remaining = ref(0)
const holding = computed(() => owner.value !== null)
const canCheck = computed(() => owner.value !== null && !owner.value.pending && !owner.value.cancelling)
const canResume = computed(() => owner.value !== null && owner.value.scopeKnown && !owner.value.finished &&
  !owner.value.pending && !owner.value.cancelling && (phase.value === 'blocked' || phase.value === 'held'))

function clearTimer(current: Owner): void {
  if (current.timer !== null) clearInterval(current.timer)
  current.timer = null
}
function settle(current: Owner): void {
  if (owner.value === current && current.finished && !current.pending && !current.cancelling) {
    clearTimer(current)
    owner.value = null
  }
}
async function cancel(current: Owner): Promise<void> {
  if (owner.value !== current || current.cancelling || current.finished) { settle(current); return }
  clearTimer(current)
  current.closed = true
  current.cancelling = true
  phase.value = 'cancelling'
  try {
    const response = await cancelProxyRecovery(current.requestId, current.previousRequestId)
    // Bounded proposal/previous-ID receipt: a refused proposal may never have
    // registered. Only this ordinary App reply selects the original request.
    current.requestId = response.ownerRequestId
    current.previousRequestId = ''
    current.scopeKnown = true
    current.finished = response.outcome === 'released'
    if (owner.value === current && result.value === null) {
      phase.value = response.outcome === 'released' ? 'idle' : response.outcome === 'transaction_held' ? 'held' : 'blocked'
      message.value = response.outcome === 'transaction_held' ? 'Разрешение отозвано. Исходная транзакция восстановления ещё удерживается; можно подготовить новое подтверждение продолжения.' :
        response.outcome === 'blocked' ? 'Исходная операция ещё удерживается. Можно проверить возможность продолжения; неизвестный результат системного вызова повторяться не будет.' :
          current.failureMessage || (current.confirmed ? 'Операция освобождена. Закрытие окна не отменяет уже выполненное восстановление.' : 'Подготовка освобождена. Восстановление системных значений не запрашивалось.')
    }
  } catch {
    if (owner.value === current) {
      phase.value = 'blocked'
      current.scopeKnown = false // do not lose either ID or invent an accepted successor on an unknown reply
      message.value = 'Завершение запроса не подтверждено. Проверьте исходную операцию; до ответа продолжение недоступно.'
    }
  } finally {
    current.cancelling = false
    if (owner.value === current) triggerRef(owner)
    settle(current)
  }
}
function close(): void {
  visible.value = false
  const current = owner.value
  if (current) { current.closed = true; clearTimer(current); void cancel(current) }
}
function invalidate(intent: string | null, lifecycle: string | undefined): void {
  const current = owner.value
  if (current && (intent !== current.intent || lifecycle !== 'stopped' && lifecycle !== 'error')) close()
}
async function open(intent: string, continuation = false): Promise<void> {
  const previous = owner.value
  if (!hasProxyRecovery() || (continuation ? !canResume.value || previous === null : previous !== null)) return
  const raw = new Uint8Array(16)
  try { globalThis.crypto.getRandomValues(raw) } catch {
    message.value = 'Не удалось подготовить идентификатор операции.'
    phase.value = 'blocked'
    return
  }
  if (previous) { previous.closed = true; clearTimer(previous) }
  const current: Owner = { requestId: [...raw].map(value => value.toString(16).padStart(2, '0')).join(''),
    previousRequestId: continuation && previous ? previous.requestId : '', scopeKnown: !continuation, intent,
    pending: true, cancelling: false, closed: false, confirmed: false, finished: false, failureMessage: '', timer: null }
  owner.value = current
  preview.value = null
  result.value = null
  message.value = ''
  remaining.value = 0
  visible.value = true
  phase.value = 'preparing'
  try {
    const value = await prepareProxyRecovery(current.requestId, intent, current.previousRequestId)
    if (owner.value !== current) return
    current.scopeKnown = true
    current.previousRequestId = '' // actual accepted preview binds this fresh App request
    if (current.closed) {
      // An early cancellation can return before a delayed Prepare dispatch.
      // Reconfirm negative retirement only after that original callback returns.
      current.finished = false
      void cancel(current)
      return
    }
    preview.value = value
    phase.value = 'awaiting'
    const tick = () => {
      if (owner.value !== current || current.closed || preview.value !== value) { clearTimer(current); return }
      remaining.value = Math.max(0, Math.ceil((Date.parse(value.preview.expiresAt) - Date.now()) / 1000))
      if (remaining.value === 0) {
        message.value = 'Срок подтверждения истёк. Подготовка отменяется.'
        visible.value = false
        void cancel(current)
      }
    }
    tick()
    if (!current.closed) current.timer = setInterval(tick, 1000)
  } catch {
    if (owner.value === current) {
      if (current.closed) {
        if (!current.finished) void cancel(current)
        return
      }
      phase.value = 'blocked'
      message.value = continuation ? 'Продолжение не подготовлено. Для новой попытки потребуется отдельная подготовка.' :
        'Подготовка восстановления недоступна. Нужен сохранённый журнал Snowden и завершённые операции прокси. При необходимости сначала остановите VPN.'
      current.failureMessage = message.value
      void cancel(current)
    }
  } finally {
    current.pending = false
    if (owner.value === current) triggerRef(owner)
    settle(current)
  }
}
async function confirm(): Promise<void> {
  const current = owner.value, value = preview.value
  if (!current || current.closed || current.pending || current.cancelling || phase.value !== 'awaiting' || !value || remaining.value <= 0) return
  clearTimer(current)
  current.pending = true
  current.confirmed = true
  phase.value = 'confirming'
  try {
    const response = await confirmProxyRecovery(value)
    current.finished = true // actual component result, never a VPN-health or whole-cleanup receipt
    if (owner.value === current && !current.closed) {
      result.value = response
      phase.value = 'complete'
      message.value = response.result.outcome === 'foreign_preserved' ? 'Сохранены настройки другой программы. Автоподключение не выполнялось.' :
        response.result.outcome === 'not_written' ? 'Незавершённых изменений прокси не осталось. Новые значения не записывались.' :
          `Восстановление завершено. Изменено параметров в этой операции: ${response.result.restoredValues}.`
    }
  } catch {
    if (owner.value === current) {
      phase.value = 'blocked'
      message.value = 'Восстановление не завершено. Можно подготовить продолжение на исходной операции; новое подтверждение будет доступно только при известном результате предыдущих вызовов.'
    }
  } finally {
    current.pending = false
    if (owner.value === current) triggerRef(owner)
    settle(current)
  }
}
function retryCancel(): void {
  const current = owner.value
  if (current && !current.pending && !current.cancelling) void cancel(current)
}
export function useProxyRecovery() {
  return { available: hasProxyRecovery(), phase: readonly(phase), holding, canCheck, canResume, visible: readonly(visible), preview: readonly(preview), result: readonly(result),
    message: readonly(message), remaining: readonly(remaining), open, confirm, close, invalidate, retryCancel }
}
