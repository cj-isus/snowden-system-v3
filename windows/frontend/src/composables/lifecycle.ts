/**
 * Обёртка lifecycle-действий для UI (старт/стоп/probe/логи/диагностика).
 * Ошибки backend показываются как есть (без секретов), состояние — только факты.
 */
import { api } from '@/api/backend'
import { toast } from './toast'
import { startVpn, stopVpn } from './backendState'

async function start(): Promise<void> {
  await startVpn()
}

async function stop(): Promise<void> {
  await stopVpn()
}

async function runProbe(): Promise<void> {
  try {
    await api.runProbe()
  } catch (e) {
    toast('error', e instanceof Error ? e.message : String(e))
  }
}

async function openLogs(): Promise<void> {
  try {
    await api.openLogsDir()
  } catch (e) {
    toast('error', e instanceof Error ? e.message : String(e))
  }
}

export function useLifecycle() {
  return { start, stop, runProbe, openLogs }
}
