import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { AppState } from '../../src/api/contract'
import { appState, deferred } from '../helpers'

const mocks = vi.hoisted(() => ({
  getState: vi.fn(), listChannels: vi.fn(), listTests: vi.fn(), isTUNMode: vi.fn(),
  subscribe: vi.fn(), start: vi.fn(), stop: vi.fn(), selectChannel: vi.fn(), toast: vi.fn(),
}))
vi.mock('../../src/api/backend', () => ({ api: mocks, isBuildPhase: () => false }))
vi.mock('../../src/composables/toast', () => ({ toast: mocks.toast, backendLog: vi.fn() }))

describe('backend state and action ownership', () => {
  let onState: (value: AppState) => void
  let store: typeof import('../../src/composables/backendState')
  const unsubscribe = vi.fn()
  beforeEach(async () => {
    vi.resetModules()
    Object.values(mocks).forEach(mock => mock.mockReset())
    unsubscribe.mockReset()
    mocks.getState.mockResolvedValue(appState())
    mocks.listChannels.mockResolvedValue([])
    mocks.listTests.mockResolvedValue([])
    mocks.isTUNMode.mockResolvedValue(false)
    mocks.subscribe.mockImplementation(cb => { onState = cb; return unsubscribe })
    store = await import('../../src/composables/backendState')
  })
  it('coalesces bootstrap and never lets its older response replace an event', async () => {
    const request = deferred<AppState>()
    mocks.getState.mockReturnValue(request.promise)
    const first = store.initBackendState()
    expect(store.initBackendState()).toBe(first)
    onState(appState('running'))
    request.resolve(appState('stopped'))
    await first
    expect(store.state.value?.state).toBe('running')
    expect(mocks.subscribe).toHaveBeenCalledTimes(1)
  })
  it('dispose revokes an in-flight snapshot and late event', async () => {
    const request = deferred<AppState>()
    mocks.getState.mockReturnValue(request.promise)
    const pending = store.initBackendState()
    store.disposeBackendState()
    request.resolve(appState('running'))
    onState(appState('running'))
    await pending
    expect(store.state.value).toBeNull()
    expect(store.stateLoaded.value).toBe(false)
    expect(unsubscribe).toHaveBeenCalledTimes(1)
  })
  it('partial load failure keeps independent results and allows retry', async () => {
    mocks.listChannels.mockRejectedValueOnce(new Error('offline'))
    await store.initBackendState()
    expect(store.channels.value).toBeNull()
    expect(store.tests.value).toEqual([])
    expect(store.stateLoaded.value).toBe(true)
    expect(unsubscribe).not.toHaveBeenCalled()
    onState(appState('running'))
    expect(store.state.value?.state).toBe('running')
    await store.initBackendState()
    expect(store.channels.value).toEqual([])
    expect(mocks.getState).toHaveBeenCalledTimes(2)
    expect(mocks.subscribe).toHaveBeenCalledTimes(1)
  })
  it('double Start dispatches once; a rejection unlocks retry', async () => {
    await store.initBackendState()
    const request = deferred<void>()
    mocks.start.mockReturnValueOnce(request.promise)
    const first = store.startVpn()
    await store.startVpn()
    expect(mocks.start).toHaveBeenCalledTimes(1)
    request.reject(new Error('failed'))
    await first
    expect(store.isBusy.value).toBe(false)
    expect(mocks.toast).toHaveBeenCalledWith('error', expect.stringContaining('failed'))
    await store.startVpn()
    expect(mocks.start).toHaveBeenCalledTimes(2)
  })
  it('Stop can interrupt a slow Start and owns the busy label until it finishes', async () => {
    await store.initBackendState()
    const start = deferred<void>(), stop = deferred<void>()
    mocks.start.mockReturnValue(start.promise)
    mocks.stop.mockReturnValue(stop.promise)
    const starting = store.startVpn()
    const stopping = store.stopVpn()
    await store.stopVpn()
    expect(mocks.stop).toHaveBeenCalledTimes(1)
    start.reject(new Error('obsolete start error'))
    await starting
    expect(store.busyLabel.value).toBe('Остановка…')
    expect(store.isBusy.value).toBe(true)
    expect(mocks.toast).not.toHaveBeenCalled()
    stop.resolve()
    await stopping
    expect(store.isBusy.value).toBe(false)
  })
  it('blocks duplicate switch and suppresses success of a switch superseded by Stop', async () => {
    await store.initBackendState()
    onState(appState('running'))
    const request = deferred<void>()
    mocks.selectChannel.mockReturnValue(request.promise)
    const switching = store.selectChannel('fixture-b')
    await store.selectChannel('fixture-c')
    await store.stopVpn()
    request.resolve()
    await switching
    expect(mocks.selectChannel).toHaveBeenCalledTimes(1)
    expect(mocks.toast).not.toHaveBeenCalled()
  })
  it.each(['starting', 'stopping', 'reloading'] as const)('%s disables Start but does not reject explicit Stop', async state => {
    await store.initBackendState()
    onState(appState(state))
    await store.startVpn()
    expect(mocks.start).not.toHaveBeenCalled()
    expect(store.primaryAction.value.disabled).toBe(true)
    await store.stopVpn()
    expect(mocks.stop).toHaveBeenCalledTimes(1)
  })
})
