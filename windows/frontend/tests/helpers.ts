import type { AppState } from '../src/api/contract'
export function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (error: unknown) => void
  const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no })
  return { promise, resolve, reject }
}
export function appState(state: AppState['state'] = 'stopped'): AppState {
  return {
    state, blockedReason: null, activeChannelId: null, activeChannel: null, proxyMode: null,
    probe: null, probeRunning: false, probeLastAt: null, error: '', coreReady: false,
    coreBlockMsg: '', nextRetryAt: '', retryAttempt: 0,
  }
}
