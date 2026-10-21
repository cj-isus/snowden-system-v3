import { describe, expect, it } from 'vitest'
import { createTrafficHistory } from '../../src/composables/trafficHistory'
const sample = (inOctets: number, outOctets = 10, alias = 'Wi-Fi') => ({ inOctets, outOctets, alias, speedBps: 1000 })
describe('traffic observations', () => {
  it('waits for two observations and uses actual timestamps', () => {
    const history = createTrafficHistory()
    expect(history.observe(sample(100), 1000)).toEqual({ deltaIn: 0, deltaOut: 0, points: [] })
    expect(history.observe(sample(350, 60), 7800)).toEqual({
      deltaIn: 250, deltaOut: 50, points: [{ at: 7800, bytes: 250 }],
    })
  })
  it.each([
    ['adapter change', sample(2000, 30, 'Ethernet')],
    ['receive reset', sample(1)],
    ['send reset', sample(200, 0)],
  ])('%s starts a new baseline instead of a fake spike', (_, next) => {
    const history = createTrafficHistory()
    history.observe(sample(100), 0)
    history.observe(sample(200), 1)
    expect(history.observe(next, 2)).toEqual({ deltaIn: 0, deltaOut: 0, points: [] })
  })
  it('retains measured zero and bounds history without truncating totals', () => {
    const history = createTrafficHistory(2)
    history.observe(sample(100), 0)
    history.observe(sample(100), 1)
    expect(history.observe(sample(100), 2).points[0].bytes).toBe(0)
    const result = history.observe(sample(200), 3)
    expect(result.points).toEqual([{ at: 2, bytes: 0 }, { at: 3, bytes: 100 }])
    expect(result.deltaIn).toBe(100)
  })
})
