import { onMounted, onUnmounted, ref } from 'vue'

/**
 * One request at a time, next tick after completion. Wails calls cannot be
 * aborted, so publication is revoked when the owner stops. Restart waits for
 * the old request, then loads fresh data; it never adopts that old response.
 */
export function createPoller<T>(
  load: () => Promise<T>,
  publish: (value: T) => void,
  interval: number,
  onError: (error: unknown) => void = () => {},
) {
  let active = false
  let generation = 0
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
    pending = Promise.resolve().then(load).then(
      value => { if (active && generation === current) publish(value) },
      error => { if (active && generation === current) onError(error) },
    ).finally(() => {
      pending = undefined
      if (active) timer = setTimeout(() => void refresh(), generation === current ? interval : 0)
    })
    return pending
  }
  return {
    refresh,
    start() {
      if (active) return
      active = true
      generation++
      void refresh()
    },
    stop() { active = false; generation++; clearTimer() },
  }
}

/** Component owns the timer. null is unavailable, never a synthetic zero. */
export function usePolling<T>(load: () => Promise<T>, interval: number) {
  const data = ref<T | null>(null)
  const loading = ref(true)
  const error = ref('')
  const poller = createPoller(load, value => {
    data.value = value
    error.value = ''
    loading.value = false
  }, interval, reason => {
    data.value = null
    error.value = reason instanceof Error ? reason.message : String(reason)
    loading.value = false
  })
  onMounted(poller.start)
  onUnmounted(poller.stop)
  return { data, loading, error, refresh: poller.refresh }
}
