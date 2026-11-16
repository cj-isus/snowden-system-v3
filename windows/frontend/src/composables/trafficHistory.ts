import type { MetricsView, NetStats } from '../api/contract'
import { diagnosticContextKey } from '../api/labels'

export type TrafficPoint = { from: number; at: number; bytes: number }
type CounterSnapshot = { alias: string; identity: string; inOctets: number; outOctets: number }

/** Adapter counters, never VPN counters. Times are renderer receipts. */
export function createTrafficHistory(limit = 24) {
  const capacity = Number.isSafeInteger(limit) && limit > 0 && limit <= 512 ? limit : 24
  let baseline: CounterSnapshot | null = null
  let previous: CounterSnapshot | null = null
  let startedAt = 0
  let previousAt = 0
  let points: TrafficPoint[] = []
  function reset(): void {
    baseline = null
    previous = null
    startedAt = 0
    previousAt = 0
    points = []
  }
  return {
    reset,
    observe(stats: NetStats, at: number) {
      const { inOctets, outOctets, alias } = stats
      if (inOctets === null || outOctets === null || !Number.isSafeInteger(inOctets) ||
          !Number.isSafeInteger(outOctets) || inOctets < 0 || outOctets < 0 || !alias || !Number.isFinite(at)) {
        reset()
        return null
      }
      const identity = stats.adapterId ? `native:${stats.adapterId}` : `legacy:${alias}`
      const current = { alias, identity, inOctets, outOctets }
      if (!baseline || !previous || identity !== previous.identity || alias !== previous.alias || at <= previousAt ||
          inOctets < previous.inOctets || outOctets < previous.outOctets) {
        baseline = current
        previous = current
        startedAt = previousAt = at
        points = []
        // The first snapshot establishes a baseline, not a zero-duration rate.
        return { deltaIn: null, deltaOut: null, from: at, to: at, points }
      }
      points = [...points, { from: previousAt, at, bytes: inOctets - previous.inOctets }].slice(-capacity)
      previous = current
      previousAt = at
      return { deltaIn: inOctets - baseline.inOctets, deltaOut: outOctets - baseline.outOctets, from: startedAt, to: at, points }
    },
  }
}

export type CoreTrafficInterval = { from: number; to: number; down: number; up: number }

/** Two actual samples from the same collector and context, never a forecast. */
export function createCoreTrafficHistory() {
  let previous: { collector: string; context: string; at: number; down: number; up: number } | null = null
  function reset(): void { previous = null }
  return {
    reset,
    observe(value: Readonly<MetricsView> | null): CoreTrafficInterval | null {
      const collector = value?.collector
      const context = diagnosticContextKey(value?.context)
      const at = Date.parse(value?.at ?? '')
      const down = value?.sessionDown, up = value?.sessionUp
      if (!value?.available || !collector || !context || !Number.isFinite(at) || at > Date.now() ||
          down == null || up == null || !Number.isSafeInteger(down) || !Number.isSafeInteger(up) || down < 0 || up < 0) {
        reset()
        return null
      }
      const before = previous
      previous = { collector, context, at, down, up }
      if (!before || collector !== before.collector || context !== before.context || at <= before.at || down < before.down || up < before.up) return null
      const seconds = (at - before.at) / 1000
      return { from: before.at, to: at, down: (down - before.down) / seconds, up: (up - before.up) / seconds }
    },
  }
}
