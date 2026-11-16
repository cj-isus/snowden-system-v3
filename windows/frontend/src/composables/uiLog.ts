/**
 * Local bounded UI buffer. Backend text is not an export-safe structured fact.
 * Секреты сюда не логировать никогда (AGENTS.md §2.4).
 */
import { ref } from 'vue'
import type { UiLogLine } from '@/api/contract'
import { normalizeLogTime } from './supportLog'

const MAX_LINES = 1000
const MAX_TEXT_UNITS = 4096
const TEXT_OMITTED = ' [сообщение усечено]'
const lines = ref<UiLogLine[]>([])

export type UiLevel = UiLogLine['level']

/** Visible single-record framing, not secret redaction; marker stays inside cap. */
function projectLocalLogText(text: string): string {
  const pieces: string[] = []
  let used = 0, safeCount = 0
  for (const symbol of text) {
    const point = symbol.codePointAt(0) ?? 0xfffd
    let piece = symbol
    if (point === 10) piece = '\\n'
    else if (point === 13) piece = '\\r'
    else if (point === 9) piece = '\\t'
    else if (point < 32 || point >= 0x7f && point <= 0x9f || point === 0xad || point === 0x61c || point === 0x180e ||
      point >= 0x200b && point <= 0x200f || point >= 0x2028 && point <= 0x202e ||
      point >= 0x2060 && point <= 0x206f || point === 0xfeff || point >= 0xfff9 && point <= 0xfffb ||
      point >= 0xd800 && point <= 0xdfff) {
      piece = point <= 0xff ? `\\x${point.toString(16).padStart(2, '0')}` : `\\u${point.toString(16).padStart(4, '0')}`
    }
    if (used + piece.length > MAX_TEXT_UNITS) {
      pieces.length = safeCount
      pieces.push(TEXT_OMITTED)
      return pieces.join('')
    }
    pieces.push(piece)
    used += piece.length
    if (used <= MAX_TEXT_UNITS - TEXT_OMITTED.length) safeCount = pieces.length
  }
  return pieces.join('')
}

/** t — опциональная метка времени источника (backend отдаёт свою RFC3339). */
export function uiLog(level: UiLevel, text: string, t?: string): void {
  const stamp = t === undefined ? new Date().toISOString() : normalizeLogTime(t)
  const knownLevel = level === 'debug' || level === 'info' || level === 'warn' || level === 'error'
  const boundedText = !knownLevel ? 'Сообщение с неподдерживаемым уровнем журнала; исходный текст не принят.' :
    typeof text === 'string' ? projectLocalLogText(text) : 'Текст сообщения не передан в поддерживаемом формате.'
  lines.value.push({ t: stamp, level: knownLevel ? level : 'warn', text: boundedText })
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
