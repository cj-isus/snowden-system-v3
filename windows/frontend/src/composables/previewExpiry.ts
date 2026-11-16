import { onBeforeUnmount, readonly, ref } from 'vue'

// One timer belongs to one preview generation. This is UI affordance only;
// the backend repeats its own clock/authority/deadline checks at commit.
export function usePreviewExpiry(onExpire: () => void) {
  const expired = ref(false)
  let timer: ReturnType<typeof setTimeout> | undefined
  let deadline: number | undefined
  let generation = 0
  let disposed = false

  function clear(): void {
    generation++
    clearTimeout(timer)
    timer = undefined
    deadline = undefined
    expired.value = false
  }
  function expire(): void {
    if (disposed || expired.value) return
    clearTimeout(timer)
    timer = undefined
    expired.value = true
    onExpire()
  }
  function check(): boolean {
    if (disposed) return true
    if (deadline !== undefined && (!Number.isFinite(deadline) || Date.now() >= deadline)) expire()
    return expired.value
  }
  function arm(iso: string): void {
    if (disposed) return
    clear()
    const parsed = Date.parse(iso)
    deadline = parsed
    if (check()) return
    const current = generation
    timer = setTimeout(() => { if (current === generation) expire() }, Math.min(parsed - Date.now(), 120_000))
  }

  onBeforeUnmount(() => { disposed = true; clear() })
  return { expired: readonly(expired), arm, clear, check }
}
