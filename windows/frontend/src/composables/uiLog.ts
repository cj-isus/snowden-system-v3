/**
 * Локальный журнал UI (диагностика, без секретов). Это НЕ backend-логи:
 * у строк нет source backend — в LogsView они помечены как 'ui'.
 * Секреты сюда не логировать никогда (AGENTS.md §2.4).
 */
import { ref } from 'vue'
import type { UiLogLine } from '@/api/contract'

const MAX_LINES = 1000
const lines = ref<UiLogLine[]>([])

export type UiLevel = UiLogLine['level']

/** t — опциональная метка времени источника (backend отдаёт свою RFC3339). */
export function uiLog(level: UiLevel, text: string, t?: string): void {
  lines.value.push({ t: t ?? new Date().toISOString(), level, text })
  if (lines.value.length > MAX_LINES) {
    // Переполнение кольца: тихо обрезаем. Отдельный лог не нужен — иначе рекурсия.
    lines.value.splice(0, lines.value.length - MAX_LINES)
  }
}

export function uiLogLines(): typeof lines {
  return lines
}

export function clearUiLog(): void {
  lines.value.length = 0
  uiLog('info', 'Журнал UI очищен')
}
