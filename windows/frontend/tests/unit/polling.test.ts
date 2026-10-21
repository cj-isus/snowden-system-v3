import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPoller } from '../../src/composables/polling'
import { usePolling } from '../../src/composables/polling'
import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { deferred } from '../helpers'

describe('polling ownership', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())
  it('coalesces refreshes and schedules from completion, not request start', async () => {
    const request = deferred<number>()
    const load = vi.fn(() => request.promise)
    const publish = vi.fn()
    const poller = createPoller(load, publish, 1000)
    poller.start()
    const first = poller.refresh()
    expect(poller.refresh()).toBe(first)
    await vi.advanceTimersByTimeAsync(10000)
    expect(load).toHaveBeenCalledTimes(1)
    request.resolve(42)
    await first
    expect(publish).toHaveBeenCalledWith(42)
    await vi.advanceTimersByTimeAsync(999)
    expect(load).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(1)
    expect(load).toHaveBeenCalledTimes(2)
    poller.stop()
  })
  it('does not publish or reschedule after unmount', async () => {
    const request = deferred<number>()
    const publish = vi.fn()
    const poller = createPoller(() => request.promise, publish, 10)
    poller.start()
    const pending = poller.refresh()
    poller.stop()
    request.resolve(1)
    await pending
    await vi.advanceTimersByTimeAsync(100)
    expect(publish).not.toHaveBeenCalled()
    expect(vi.getTimerCount()).toBe(0)
  })
  it('reacquiring cannot adopt a response from the old owner', async () => {
    const old = deferred<number>()
    const publish = vi.fn()
    const load = vi.fn().mockReturnValueOnce(old.promise).mockResolvedValue(2)
    const poller = createPoller(load, publish, 1000)
    poller.start()
    const pending = poller.refresh()
    await Promise.resolve()
    poller.stop()
    poller.start()
    old.resolve(1)
    await pending
    expect(publish).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(0)
    expect(publish).toHaveBeenCalledExactlyOnceWith(2)
    poller.stop()
  })
  it('reports rejection and recovers on the next tick without overlapping calls', async () => {
    const error = new Error('unavailable')
    const onError = vi.fn()
    const publish = vi.fn()
    const load = vi.fn().mockRejectedValueOnce(error).mockResolvedValue(9)
    const poller = createPoller(load, publish, 100, onError)
    poller.start()
    await poller.refresh()
    expect(onError).toHaveBeenCalledWith(error)
    await vi.advanceTimersByTimeAsync(100)
    expect(publish).toHaveBeenCalledWith(9)
    poller.stop()
  })
  it('component resource clears unavailable data and ignores completion after unmount', async () => {
    const late = deferred<number>()
    const load = vi.fn().mockResolvedValueOnce(7).mockRejectedValueOnce(new Error('lost')).mockReturnValue(late.promise)
    let resource!: ReturnType<typeof usePolling<number>>
    const wrapper = mount(defineComponent({
      setup() { resource = usePolling(load, 100); return () => null },
    }))
    await flushPromises()
    expect(resource.data.value).toBe(7)
    expect(resource.loading.value).toBe(false)
    await resource.refresh()
    expect(resource.data.value).toBeNull()
    expect(resource.error.value).toBe('lost')
    const pending = resource.refresh()
    wrapper.unmount()
    late.resolve(99)
    await pending
    expect(resource.data.value).toBeNull()
    expect(vi.getTimerCount()).toBe(0)
  })
})
