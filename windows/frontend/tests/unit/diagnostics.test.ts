import { beforeEach, describe, expect, it, vi } from 'vitest'
import { deferred } from '../helpers'
const api = vi.hoisted(() => ({ listTests: vi.fn(), runTest: vi.fn() }))
const toast = vi.hoisted(() => vi.fn())
vi.mock('../../src/api/backend', () => ({ api }))
vi.mock('../../src/composables/toast', () => ({ toast }))
const result = (id: string) => ({ id, ok: null, detail: '', checkedAt: null })
describe('diagnostic execution', () => {
  beforeEach(() => {
    vi.resetModules()
    api.listTests.mockReset().mockResolvedValue([result('a'), result('b')])
    api.runTest.mockReset().mockResolvedValue({ ...result('a'), ok: true })
    toast.mockReset()
  })
  it('keeps the whole batch exclusive, including the result refresh', async () => {
    const module = await import('../../src/composables/diagnostics')
    await module.refreshTests()
    const refresh = deferred<ReturnType<typeof result>[]>()
    api.listTests.mockReturnValueOnce(refresh.promise)
    const all = module.runAll()
    await Promise.resolve()
    await Promise.resolve()
    await module.runTest('outside')
    await module.runAll()
    expect(api.runTest.mock.calls).toEqual([['a']])
    expect(module.useDiagnostics().busy.value).toBe(true)
    refresh.resolve([result('a'), result('b')])
    await all
    expect(api.runTest.mock.calls).toEqual([['a'], ['b']])
    expect(module.useDiagnostics().busy.value).toBe(false)
  })
  it('stale list cannot replace a newer refresh', async () => {
    const module = await import('../../src/composables/diagnostics')
    const old = deferred<ReturnType<typeof result>[]>()
    api.listTests.mockReturnValueOnce(old.promise).mockResolvedValueOnce([result('fresh')])
    const pending = module.refreshTests()
    await module.refreshTests()
    old.resolve([result('obsolete')])
    await pending
    expect(module.useDiagnostics().tests.value?.[0].id).toBe('fresh')
  })
  it('backend failure unlocks execution without inventing a passing result', async () => {
    const module = await import('../../src/composables/diagnostics')
    api.runTest.mockRejectedValueOnce(new Error('probe unavailable'))
    await module.runTest('a')
    expect(module.useDiagnostics().busy.value).toBe(false)
    expect(module.useDiagnostics().tests.value?.[0].ok).toBeNull()
    await module.runTest('a')
    expect(api.runTest).toHaveBeenCalledTimes(2)
  })
  it('skipped checks do not create a success notification', async () => {
    const module = await import('../../src/composables/diagnostics')
    api.runTest.mockResolvedValueOnce({ ...result('a'), ok: true,
      steps: [{ name: 'not applicable', status: 'skipped', detail: '' }] })
    await module.runTest('a')
    expect(toast).toHaveBeenCalledWith('info', expect.stringContaining('пропущен'))
    expect(toast).not.toHaveBeenCalledWith('ok', expect.anything())
  })
})
