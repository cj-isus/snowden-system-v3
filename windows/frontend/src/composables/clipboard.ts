import { computed, onBeforeUnmount, ref, shallowRef } from 'vue'
import { ClipboardSetText } from '../../wailsjs/runtime/runtime'

type Purpose = 'support' | 'secret'
type Phase = 'preparing' | 'writing' | 'unconfirmed'
type Flight = Readonly<{ token: symbol; owner: symbol; purpose: Purpose; phase: Phase }>
type WriteResult = 'confirmed' | 'unconfirmed' | 'retired' | 'unavailable'
type RunResult = 'finished' | 'failed' | 'busy' | 'retired' | 'unavailable'
type WriteLease = { current: () => boolean; writeText: (text: string) => Promise<WriteResult> }
type Provider = { current: () => boolean; write: (text: string) => unknown; confirms: (result: unknown) => boolean }

// One renderer's metadata only. Never retain text, secret IDs, callbacks or
// promises in reactive state. Other applications/renderers can still write.
const flight = shallowRef<Flight | null>(null)
const active = ref(false)
let root = Symbol()

export function activateClipboard(): void { active.value = true }
export function disposeClipboard(): void { active.value = false; root = Symbol() }

function record(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}

// Choose once, before a secret is requested. A failed native write never falls
// through to another provider. Identity checks precede dispatch without await.
function selectProvider(): Provider | null {
  try {
    if (typeof window === 'undefined') return null
    const runtime: unknown = Reflect.get(window, 'runtime')
    if (runtime !== undefined && runtime !== null) {
      if (!record(runtime) || typeof runtime.ClipboardSetText !== 'function') return null
      const method = runtime.ClipboardSetText
      return {
        current: () => Reflect.get(window, 'runtime') === runtime && runtime.ClipboardSetText === method,
        write: text => ClipboardSetText(text),
        confirms: result => result === true,
      }
    }
    if (typeof navigator === 'undefined') return null
    const clipboard = navigator.clipboard
    if (!clipboard || typeof clipboard.writeText !== 'function') return null
    const method = clipboard.writeText
    return {
      current: () => Reflect.get(window, 'runtime') == null && navigator.clipboard === clipboard && clipboard.writeText === method,
      write: text => method.call(clipboard, text),
      confirms: result => result === undefined,
    }
  } catch { return null }
}

const busy = computed(() => flight.value !== null)
const pending = computed(() => flight.value !== null && flight.value.phase !== 'unconfirmed')
const pendingLabel = computed(() => {
  const original = flight.value
  switch (original?.phase) {
    case 'preparing': return original.purpose === 'secret' ? 'Ожидается выдача значения для копирования…' : 'Подготавливается копирование сводки…'
    case 'writing': return 'Ожидается ответ службы буфера обмена. Запись может завершиться после ухода со страницы.'
    case 'unconfirmed': return 'Ответ записи не получен. Новое копирование заблокировано; отложенная запись может перезаписать содержимое буфера.'
    default: return ''
  }
})

export function useClipboardAction() {
  const owner = Symbol(), scope = root
  let mounted = true
  const current = () => mounted && active.value && scope === root
  const available = computed(() => active.value && selectProvider() !== null)
  const canStart = computed(() => current() && !busy.value && available.value)
  const ownsPending = computed(() => flight.value?.owner === owner)
  onBeforeUnmount(() => { mounted = false })

  async function run(purpose: Purpose, interested: () => boolean, work: (lease: WriteLease) => Promise<void>): Promise<RunResult> {
    const interestedNow = () => { try { return current() && interested() } catch { return false } }
    if (!interestedNow()) return 'retired'
    if (flight.value !== null) return 'busy'
    const provider = selectProvider()
    if (!provider) return 'unavailable'
    if (!interestedNow()) return 'retired'
    if (flight.value !== null) return 'busy'
    const original: Flight = { token: Symbol(), owner, purpose, phase: 'preparing' }
    flight.value = original // reserve before work's first await (including vault reveal)
    const isCurrent = () => flight.value?.token === original.token && interestedNow()
    const phase = (value: Phase) => {
      if (flight.value?.token === original.token) flight.value = { ...original, phase: value }
    }
    let closed = false, used = false, uncertain = false
    let native: Promise<WriteResult> | null = null
    const writeText = (text: string): Promise<WriteResult> => {
      if (closed || used || !isCurrent()) return Promise.resolve('retired')
      used = true
      if (typeof text !== 'string' || text.length === 0) return Promise.resolve('unavailable')
      phase('writing')
      // Publishing reactive phase metadata may synchronously retire a view.
      // Check again after that publication, immediately before dispatch.
      if (!isCurrent()) return Promise.resolve('retired')
      try { if (!provider.current()) return Promise.resolve('unavailable') }
      catch { return Promise.resolve('unavailable') }
      try {
        const response = provider.write(text)
        if ((!record(response) && typeof response !== 'function') || typeof Reflect.get(response, 'then') !== 'function') {
          // Dispatch happened but there is no original completion handle.
          uncertain = true
          phase('unconfirmed')
          return Promise.resolve('unconfirmed')
        }
        native = Promise.resolve(response).then<WriteResult, WriteResult>(
          value => provider.confirms(value) ? 'confirmed' : 'unconfirmed',
          () => 'unconfirmed',
        )
        return native
      } catch {
        // A synchronous throw after entering a provider is not an absent-write
        // receipt. Preserve negative admission even after view/root disposal.
        uncertain = true
        phase('unconfirmed')
        return Promise.resolve('unconfirmed')
      }
    }
    let result: RunResult = 'finished'
    try { await work({ current: isCurrent, writeText }) }
    catch { result = 'failed' }
    finally {
      closed = true
      // The owner, rather than a caller's finally, awaits the original handle;
      // even a callback that forgets to await writeText cannot release it early.
      if (native !== null) await native
      if (!uncertain && flight.value?.token === original.token) flight.value = null
    }
    return result
  }

  return { run, current, canStart, available, busy, pending, ownsPending, pendingLabel }
}
