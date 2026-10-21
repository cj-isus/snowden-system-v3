import { afterEach, describe, expect, it, vi } from 'vitest'
import { api, isBuildPhase } from '../../src/api/backend'
vi.mock('../../src/composables/toast', () => ({ backendLog: vi.fn() }))

afterEach(() => { Reflect.deleteProperty(window, 'go'); Reflect.deleteProperty(window, 'runtime') })
function bindings(methods: Record<string, unknown>) {
  Object.assign(window, { go: { main: { App: methods } } })
}
describe('Go to UI boundary', () => {
  it('browser without Wails exposes unavailable sources and never synthetic success', async () => {
    expect(isBuildPhase()).toBe(true)
    const state = await api.getState()
    expect(state.state).toBe('stopped')
    expect(state.probe).toBeNull()
    expect(state.coreReady).toBe(false)
    expect(state.error).toContain('Wails')
    expect(await api.listChannels()).toBeNull()
    expect(await api.listTests()).toBeNull()
  })
  it('preserves false, missing and passed test outcomes as three different states', async () => {
    bindings({ ListTests: () => Promise.resolve([
      { id: 'unknown' }, { id: 'failed', ok: false }, { id: 'passed', ok: true },
    ]) })
    expect((await api.listTests())?.map(test => test.ok)).toEqual([null, false, true])
  })
  it('maps the actual channel and protection reason, preserving an unpassed probe', async () => {
    bindings({ GetState: () => Promise.resolve({
      state: 'error', activeId: 'fixture-channel', active: { id: 'fixture-channel', transport: 'hysteria2', server: 'example.invalid', port: 443, validation: 'configured', enabled: true },
      blockedReason: 'protection_retained', probe: { ok: false, steps: [] }, probeRunning: false,
    }) })
    const state = await api.getState()
    expect(state.activeChannelId).toBe('fixture-channel')
    expect(state.activeChannel?.server).toBe('example.invalid')
    expect(state.blockedReason).toBe('protection_retained')
    expect(state.probe?.ok).toBe(false)
    expect(state.probeLastAt).toBeNull()
  })
  it('unsubscribe releases both Wails event handlers', () => {
    bindings({})
    const off = vi.fn()
    Object.assign(window, { runtime: { EventsOnMultiple: vi.fn(() => off) } })
    api.subscribe(vi.fn(), vi.fn())()
    expect(off).toHaveBeenCalledTimes(2)
  })
  it('Go nil slices are empty collections, not a crashed load', async () => {
    bindings({
      ListChannels: () => Promise.resolve(null), ListTests: () => Promise.resolve(null),
      ListSecrets: () => Promise.resolve(null),
      GetState: () => Promise.resolve({ state: 'error', probe: { ok: false, steps: null } }),
    })
    expect(await api.listChannels()).toEqual([])
    expect(await api.listTests()).toEqual([])
    expect(await api.listSecrets()).toEqual([])
    expect((await api.getState()).probe).toEqual({ ok: false, steps: [] })
  })
})
