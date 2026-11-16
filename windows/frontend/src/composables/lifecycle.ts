/**
 * Обёртка lifecycle-действий для UI (старт/стоп/probe/логи/диагностика).
 * Отказ команды показывает локальный текст действия, без raw backend ошибки.
 */
import { api } from '@/api/backend'
import { uiFailure } from '../api/failureMessages'
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
  } catch {
    toast('error', uiFailure.probe)
  }
}

async function openLogs(): Promise<void> {
  try {
    await api.openLogsDir()
  } catch {
    toast('error', uiFailure.openLogs)
  }
}

export function useLifecycle() {
  return { start, stop, runProbe, openLogs }
}
