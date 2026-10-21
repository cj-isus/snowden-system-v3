import type { NetStats } from '../api/contract'

export type TrafficPoint = { at: number; bytes: number }
/**
 * Adapter counters are not VPN traffic. A reset or adapter change starts a new
 * observation window; missing observations never add zero-valued samples.
 */
export function createTrafficHistory(limit = 24) {
  let baseline: NetStats | null = null
  let previous: NetStats | null = null
  let points: TrafficPoint[] = []
  return {
    observe(stats: NetStats, at: number) {
      if (!baseline || !previous || stats.alias !== previous.alias ||
          stats.inOctets < previous.inOctets || stats.outOctets < previous.outOctets) {
        baseline = { ...stats }
        points = []
      } else {
        points = [...points, { at, bytes: stats.inOctets - previous.inOctets }].slice(-limit)
      }
      previous = { ...stats }
      return { deltaIn: stats.inOctets - baseline.inOctets, deltaOut: stats.outOctets - baseline.outOctets, points }
    },
  }
}
