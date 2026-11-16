import type { UiLogLine } from '../api/contract'

export type UiLogSource = 'backend' | 'engine' | 'probe' | 'ui'
const LOG_TIME = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?(?:Z|[+-]\d{2}:\d{2})$/
export function normalizeLogTime(value: unknown): string {
  if (typeof value !== 'string' || value.length > 64 || !LOG_TIME.test(value) || !Number.isFinite(Date.parse(value))) return ''
  return new Date(value).toISOString()
}
export function uiLogSource(text: string): UiLogSource {
  if (text.startsWith('[backend] движок ') || text.startsWith('[backend] движок:')) return 'engine'
  if (text.startsWith('[backend]')) return 'backend'
  if (text.startsWith('[probe]')) return 'probe'
  return 'ui'
}

const MAX_RECORDS = 200
const MAX_CHARS = 32_000
const LEVELS: readonly string[] = ['debug', 'info', 'warn', 'error']
// Only these literal codes can leave the raw-text buffer. A match is a mention,
// not an authenticated origin, measured cause or current protection outcome.
const REASON_CODES = [
  'cleanup_unconfirmed', 'protection_retained', 'no_validated_channel', 'probe_failed',
  'config_invalid', 'all_channels_failed', 'observation_unavailable', 'request_timeout',
  'tls_verification_failed', 'tls_handshake_failed', 'connection_failed', 'response_incomplete',
  'protected_channel_unavailable', 'dns_path_failure', 'measurement_endpoint_failure',
] as const
const RULES = REASON_CODES.map(code => ({ code, pattern: new RegExp(`\\b${code}\\b`, 'i') }))

export type SupportLogSnapshot = Readonly<{
  text: string
  records: number
  omitted: number
  generatedAt: string
}>

/** Closed-vocabulary summary: never copies any raw message or filter input. */
export function createSupportLogSnapshot(lines: readonly UiLogLine[]): SupportLogSnapshot {
  const generatedAt = new Date().toISOString()
  const heading = [
    'Snowden System: local UI event summary v1',
    `Prepared: ${generatedAt}`,
    'Scope: filtered local UI buffer, not native logs or a complete support bundle.',
    'Fields: normalized time, reported level, source inferred from prefix, mentioned code.',
    'Raw messages, filter queries, addresses, paths and channel names are omitted.',
    'Unknown means no recognized code. A code mention does not establish a cause.',
    '',
  ].join('\n')
  const rows: string[] = []
  let chars = heading.length
  for (const line of lines.slice(-MAX_RECORDS)) {
    const input = typeof line.text === 'string' ? line.text.slice(0, 4096) : ''
    const code = RULES.find(rule => rule.pattern.test(input))?.code ?? 'unknown'
    const level = LEVELS.includes(line.level) ? line.level : 'unknown'
    const time = normalizeLogTime(line.t) || 'unknown'
    const row = `${time} [${level}] ${uiLogSource(input)} code-mentioned=${code}`
    if (chars + row.length + 1 > MAX_CHARS - 100) break
    rows.push(row)
    chars += row.length + 1
  }
  const omitted = lines.length - rows.length
  const footer = `\nIncluded: ${rows.length}; omitted: ${omitted}; limits: ${MAX_RECORDS} records / ${MAX_CHARS} characters.\n`
  return Object.freeze({ text: heading + rows.join('\n') + footer, records: rows.length, omitted, generatedAt })
}
