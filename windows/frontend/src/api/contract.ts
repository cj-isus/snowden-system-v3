/**
 * Контракт UI↔backend (v1). Единственный источник типов для UI.
 * Соответствует windows/app.go (генерируется в wailsjs). Если в сгенерированном
 * bindings появился тип/поле не из этого файла — это дефект генерации.
 *
 * Правила (AGENTS.md §3.6):
 *  - UI показывает только фактические состояния и каналы;
 *  - отсутствие данных — явный «unavailable», а не ноль/фальшка;
 *  - никакой fake-индикации.
 */

export const APP_BINDING_VERSION = 1

/** Версия приложения — единственный источник факта версии.
 *  Должна совпадать с info.productVersion в windows/wails.json (проверяет package.ps1).
 *  UI не выдумывает версию: эта константа — та же, что кладёт установщик. */
export const APP_VERSION = '2.0.0'

/** Фактические lifecycle-состояния backend. `reloading` — идёт
 *  переключение канала (Reload+probe): ручного (SelectChannel) или
 *  автоматического (failover-сторож, A4). */
export type LifecycleState = 'stopped' | 'starting' | 'running' | 'stopping' | 'reloading' | 'error'

export type BlockedReason = 'no_validated_channel' | 'probe_failed' | 'config_invalid' | 'all_channels_failed'

export type ProbeStepStatus = 'pending' | 'running' | 'pass' | 'fail' | 'skipped'

export type ProbeStep = {
  name: string
  status: ProbeStepStatus
  detail: string
}

export type ProbeReportView = {
  ok: boolean
  steps: ProbeStep[]
}

export type ValidationStatus =
  | 'planned'
  | 'configured'
  | 'locally-tested'
  | 'live-verified'
  | 'degraded'
  | 'blocked'
  | 'retired'

export type ChannelDescriptor = {
  id: string
  transport: string
  server: string
  port: number
  validation: ValidationStatus
  enabled: boolean
}

export type AppState = {
  state: LifecycleState
  blockedReason: BlockedReason | null
  activeChannelId: string | null
  activeChannel: ChannelDescriptor | null
  proxyMode: 'socks' | 'tun' | null
  probe: ProbeReportView | null
  probeRunning: boolean
  probeLastAt: string | null
  error: string
}

export type LogLine = {
  t: string
  source: 'app' | 'engine' | 'probe'
  level: 'debug' | 'info' | 'warn' | 'error'
  line: string
}

export type SecretKind =
  | 'vless-uuid'
  | 'hy2-password'
  | 'hy2-obfs-password'
  | 'vps-ssh-key'
  | 'cf-api-token'
  | 'custom'

export type SecretBackend = 'file' | 'dpapi' | 'keyring'

/** Машинный статус плановой задачи (не локализованная строка планировщика):
 *  ready/running/disabled/queued — COM State; no-access — задача есть, но
 *  скрыта правами (SYSTEM-задачи не видны не-elevated процессу — это факт,
 *  не ошибка); not-found — задачи нет; unknown:<n> — нераспознанный код. */
export type NetGuardTaskState =
  | 'ready'
  | 'running'
  | 'disabled'
  | 'queued'
  | 'no-access'
  | 'not-found'
  | 'unknown'
  | `unknown:${number}`
  | `unknown:0x${string}`

export type NetGuardTaskInfo = {
  name: string
  status: NetGuardTaskState
  lastRun: string
  nextRun: string
  lastCode: string
}

export type NetGuardProxyInfo = {
  enabled: boolean
  server: string
  port: number
  listenerAlive: boolean
  stale: boolean
}

export type NetGuardEventInfo = {
  time: string
  kind: 'HEALED' | 'HEALING' | 'PROBLEM' | 'WEEKLY'
  text: string
}

export type NetGuardStatus = {
  tasks: NetGuardTaskInfo[]
  proxy: NetGuardProxyInfo
  socksAlive: boolean
  events: NetGuardEventInfo[]
  lastRun: string
  dohDisabled: boolean
}

export type SecretMeta = {
  id: string
  kind: SecretKind
  title: string
  hint: string
  backend: SecretBackend
  storedValue: 'file' | 'dpapi' | 'keyring' | null
  fingerprint: string
  createdAt: string
  updatedAt: string
  lastVerifiedAt: string | null
  verifyStatus: 'unverified' | 'ok' | 'failed'
  verifyError: string
  reveals: number
}

export type SecretVerifyRequest = {
  id: string
}

export type SecretVerifyResult = {
  status: 'unverified' | 'ok' | 'failed'
  checkedAt: string
  error: string
}

export type SecretRevealResult = {
  value: string | null
  error: string
}

/** Живой тест значения секрета (TestSecret): шаги с фактами, значения секретов
 * не возвращаются никогда. */
export type SecretTestReport = {
  ok: boolean
  steps: ProbeStep[]
}

export type TestResult = {
  id: string
  ok: boolean | null
  detail: string
  checkedAt: string | null
  steps?: ProbeStep[]
}

export type UiLogLine = {
  t: string
  level: 'debug' | 'info' | 'warn' | 'error'
  text: string
}

// ---------- информационные факты (infobindings.go, только чтение) ----------

/** Факты о текущем сетевом подключении хоста. Пустая строка = факт недоступен. */
export type NetworkFacts = {
  connectionType: string // Wi-Fi | Ethernet | ''
  ssid: string
  localIP: string
  isp: string
  publicIP: string
  country: string
  countryOrigin: string
  dnsViaTunnel: boolean
  checkedAt: string
}

/** Счётчики адаптера маршрута по умолчанию (факт ОС). */
export type NetStats = {
  inOctets: number
  outOctets: number
  speedBps: number
  alias: string
}

/** Факты доставленного подписанного metadata-envelope (FR-008). */
export type DeliveryProfile = {
  present: boolean
  version: number
  keyId: string
  expiresAt: string
  trustedKeys: number
  channelsApplied: number
  error: string
}

/** Снимок сторожа автозащиты (failoverPolicy). */
export type FailoverStatus = {
  enabled: boolean
  state: string // closed | open | half-open
  switches: number
  lastError: string
  nextCheckIn: number
}

/** Класс ответа одного сайта через живой туннель (адаптивный слой, V2-041). */
export type SiteStatus = {
  domain: string
  code: number // 0 = ответа не было (нет туннеля/сети) — честный факт
  ok: boolean
  challenge: boolean
  detail: string
  checkedAt: string
}

/** Статус адаптивного обхода IP-блоков (V2-041). */
export type AdaptiveStatus = {
  sites: SiteStatus[]
  warpDomains: string[]
  note: string
}
/** Одно живое соединение ядра (V2-048, clash_api read-only). */
export type ConnView = {
  host: string
  network: string // tcp | udp
  chain: string   // outbound-цепочка ядра (имя канала — ближний к цели)
  process: string // процесс-инициатор (TUN-режим; в SOCKS-режиме пусто)
  up: number
  down: number
  since: string
}

/** Снимок метрик ядра (F12). available=false — честная недоступность с причиной. */
export type MetricsView = {
  available: boolean
  note: string
  rateUp: number   // байт/сек
  rateDown: number
  sessionUp: number   // от первого замера сессии ядра (подписано в UI)
  sessionDown: number
  at: string
  connections: ConnView[]
}

/** Отпечаток доверенного ключа из onboarding-бандла (F8). */
export type OnboardingKeyPreview = {
  keyId: string
  fingerprint: string
  comment: string
}

/** Превью содержимого onboarding-бандла до применения (F8). */
export type OnboardingPreview = {
  createdAt: string
  deviceName: string
  channels: number
  secrets: number
  splitDirect: number
  keys: OnboardingKeyPreview[]
  bundleVersion: number
  currentVersion: number
  wouldDowngrade: boolean
}

/** Превью проверки обновления (F15). ok=false — честный отказ с причиной. */
export type UpdatePreview = {
  currentVersion: string
  version: string
  notes: string
  releasedAt: string
  size: number
  sha256: string
  keyId: string
  floor: string
  ok: boolean
  reason: string
}
