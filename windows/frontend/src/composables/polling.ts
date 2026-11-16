import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { uiFailure } from '../api/failureMessages'

/** Publication/continuation interest only; it cannot abort the Wails call. */
export type PollingRequest = { current: () => boolean }

/**
 * One request at a time, next tick after completion. Wails calls cannot be
 * aborted, so publication is revoked when the owner stops. Restart waits for
 * the old request, then loads fresh data; it never adopts that old response.
 * A null interval keeps the same ownership for manual reads without a timer.
 */
export function createPoller<T>(
  load: (request: PollingRequest) => Promise<T>,
  publish: (value: T) => void,
  interval: number | null,
  onError: (error: unknown) => void = () => {},
  onLoading: (loading: boolean) => void = () => {},
) {
  let active = false
  let generation = Symbol() // opaque owner; numeric counter exhaustion cannot revive an old request
  let timer: ReturnType<typeof setTimeout> | undefined
  let pending: Promise<void> | undefined
  function clearTimer() {
    clearTimeout(timer)
    timer = undefined
  }
  function refresh(): Promise<void> {
    if (!active) return Promise.resolve()
    if (pending) return pending
    clearTimer()
    const current = generation
    onLoading(true)
    pending = Promise.resolve().then(async (): Promise<{ value: T } | null> => {
      if (!active || generation !== current) return null // queued work never entered load
      return { value: await load({ current: () => active && generation === current }) } // null differs from skipped work
    }).then(
      result => { if (result !== null && active && generation === current) publish(result.value) },
      error => { if (active && generation === current) onError(error) },
    ).finally(() => {
      pending = undefined
      if (active && generation === current) onLoading(false)
      if (active && interval !== null) {
        timer = setTimeout(() => void refresh(), generation === current ? interval : 0)
      } else if (active && generation !== current) {
        void refresh() // manual owner invalidated/restarted; original load already settled
      }
    })
    return pending
  }
  return {
    refresh,
    /** Retire an older read, then share one fresh request without overlap. */
    invalidate(): Promise<void> {
      if (!active) return Promise.resolve()
      generation = Symbol()
      const revision = generation
      clearTimer()
      const resume = () => active && revision === generation ? refresh() : Promise.resolve()
      return pending ? pending.then(resume, resume) : refresh()
    },
    start() {
      if (active) return
      active = true
      generation = Symbol()
      void refresh()
    },
    stop() { active = false; generation = Symbol(); clearTimer() },
  }
}

type PollingOptions = {
  /** A changed key clears the view and retires publication; null pauses reads. */
  scope?: () => string | null
  pauseWhenHidden?: boolean
}

/** Current renderer interest only; not initial visibility history or OS focus. */
export function pageObservationVisible(): boolean {
  return typeof document === 'undefined' || document.visibilityState !== 'hidden'
}

/** Reuse the same adapter for component and shared ref-counted observations. */
export function observePageVisibility(change: () => void): () => void {
  if (typeof document === 'undefined') return () => {}
  const original = document
  original.addEventListener('visibilitychange', change)
  let listening = true
  return () => {
    if (!listening) return
    listening = false
    original.removeEventListener('visibilitychange', change)
  }
}

/** Component owns the timer. null is unavailable, never a synthetic zero. */
export function usePolling<T>(load: (request: PollingRequest) => Promise<T>, interval: number, options: PollingOptions = {}) {
  const data = ref<T | null>(null)
  const loading = ref(true)
  const error = ref('')
  // Renderer receipt time, never an OS measurement time or validity deadline.
  const receivedAt = ref<string | null>(null)
  let mounted = false
  let stopVisibility = () => {}
  function clearView(): void {
    receivedAt.value = null
    error.value = ''
    data.value = null
  }
  function enabled(): boolean {
    return mounted && (!options.scope || options.scope() !== null) &&
      (!options.pauseWhenHidden || pageObservationVisible())
  }
  const poller = createPoller(request => {
    if (!request.current() || !enabled()) throw new Error(uiFailure.load)
    return load({ current: () => request.current() && enabled() })
  }, value => {
    if (!enabled()) { clearView(); loading.value = false; return }
    // Synchronous data watchers must see this value's receipt, not the old one.
    receivedAt.value = value == null ? null : new Date().toISOString()
    error.value = ''
    data.value = value
    loading.value = false
  }, interval, () => {
    if (!enabled()) { clearView(); loading.value = false; return }
    receivedAt.value = null
    error.value = uiFailure.load
    data.value = null
    loading.value = false
  }, pending => { loading.value = pending })
  function synchronize(): void {
    poller.stop() // revoke the original response before clearing/restarting
    clearView()
    loading.value = enabled()
    if (enabled()) poller.start() // existing pending request settles before a fresh read
  }
  // Sync is required for A -> B -> A in one render turn: a batched watcher
  // could miss both edges and admit the original A response after a restart.
  const stopScope = options.scope ? watch(options.scope, synchronize, { flush: 'sync' }) : () => {}
  onMounted(() => {
    mounted = true
    if (options.pauseWhenHidden) stopVisibility = observePageVisibility(synchronize)
    synchronize() // current visibility only; no claim about initial page history
  })
  onBeforeUnmount(() => {
    mounted = false
    poller.stop()
    stopScope()
    stopVisibility()
    clearView()
    loading.value = false
  })
  return {
    data, loading, error, receivedAt,
    hasInterest: enabled, // UI scheduling/publication only; never backend retirement
    refresh: () => enabled() ? poller.refresh() : Promise.resolve(),
    invalidate: () => {
      clearView()
      if (!enabled()) { poller.stop(); loading.value = false; return Promise.resolve() }
      loading.value = true
      return poller.invalidate()
    },
  }
}
