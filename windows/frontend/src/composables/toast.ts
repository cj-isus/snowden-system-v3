/**
 * Всплывающие уведомления. Тексты должны описывать фактическую ошибку,
 * без секретов и без гипотез о причинах (AGENTS.md §2.4, §3.3).
 */
import { ref } from 'vue'
import { uiLog } from './uiLog'

export type ToastKind = 'ok' | 'info' | 'warn' | 'error'

export type Toast = {
  id: number
  kind: ToastKind
  text: string
}

const toasts = ref<Toast[]>([])
let nextId = 1

const MAX_TOASTS = 4

export function toastsRef(): typeof toasts {
  return toasts
}

export function toast(kind: ToastKind, text: string, ttlMs = 5000): void {
  const id = nextId++
  toasts.value.push({ id, kind, text })
  if (toasts.value.length > MAX_TOASTS) toasts.value.shift()
  if (kind === 'error') {
    // Ошибки дублируем в журнал UI — для diagnostics (текст уже без секретов).
    uiLog('error', text)
  }
  window.setTimeout(() => dismissToast(id), ttlMs)
}

export function dismissToast(id: number): void {
  const i = toasts.value.findIndex((t) => t.id === id)
  if (i >= 0) toasts.value.splice(i, 1)
}

/** Уровень лога из типа тоста. */
export function backendLog(level: 'debug' | 'info' | 'warn' | 'error', text: string): void {
  uiLog(level, text)
}
