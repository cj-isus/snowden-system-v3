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

export type ProxyRecoveryPreview = {
  schema: 1
  requestId: string
  intent: string
  preview: {
    token: string
    scope: 'restore_recorded_proxy_values'
    expiresAt: string
    recordedValues: number
    priorProcess: 'unobserved' | 'observed_exited' | 'pid_changed' | 'retained_owner'
    resume: boolean
    resumeReason: '' | 'unchanged_write' | 'notification_failed' | 'cancelled_before_intent'
  }
}
export type ProxyRecoveryResult = {
  schema: 1
  requestId: string
  intent: string
  result: { outcome: 'restored' | 'not_written' | 'foreign_preserved'; restoredValues: number }
}
export type ProxyRecoveryCancellation = {
  schema: 1
  requestId: string
  ownerRequestId: string
  outcome: 'released' | 'transaction_held' | 'blocked'
}

/** Фактические lifecycle-состояния backend. `reloading` — идёт
 *  переключение канала (Reload+probe): ручного (SelectChannel) или
 *  автоматического (failover-сторож, A4). */
export type LifecycleState = 'stopped' | 'starting' | 'running' | 'stopping' | 'reloading' | 'error'

export type UILifecycleCommand = 'start' | 'stop' | 'switch' | 'mode_proxy' | 'mode_tun' | 'update'
export type UILifecycleRequest = { schema: 1; session: string; sequence: string; action: UILifecycleCommand; channelId: string; updateToken: string }
/** Request registration/ordinary return only; never native or applied authority. */
export type UILifecycleReceipt = {
  schema: 1
  session: string
  sequence: string
  action: UILifecycleCommand
  intent: string
  outcome: 'registered' | 'returned' | 'rejected' | 'failed' | 'superseded' | 'unconfirmed'
  /** Optional on older protocol builds; never treat missing as false. */
  handlerEntered: boolean | null
  /** Original install/staging-body entry only, never native completion. */
  updateInstallEntered: boolean | null
}

export const BLOCKED_REASONS = [
  'no_validated_channel', 'probe_failed', 'config_invalid', 'all_channels_failed',
  'protection_retained', 'proxy_conflict', 'cleanup_unconfirmed',
  'profile_recovery_required', 'profile_update_pending', 'profile_reconciliation_required', 'profile_storage_recovery_required',
  'update_recovery_required', 'installed_image_changed', 'restart_failed',
  'clock_uncertain', 'profile_expired', 'trust_not_yet_valid', 'pin_policy_pending', 'policy_channels_unavailable',
  'trust_checkpoint_unavailable', 'trust_authority_unavailable', 'certificate_expired', 'trust_unavailable', 'unrecognized',
] as const
export type BlockedReason = typeof BLOCKED_REASONS[number]

export const LIFECYCLE_ACTION_BLOCKS = [
  'explicit_start_required', 'restart_committed', 'protection_release_committed', 'update_restart_required',
  'startup_unavailable', 'cleanup_unconfirmed', 'profile_reconciliation_required', 'lifecycle_busy', 'closing', 'app_unavailable', 'state_unavailable',
] as const
export type LifecycleActionBlock = typeof LIFECYCLE_ACTION_BLOCKS[number]

/** Local command registration only. Never native readiness, cleanup or traffic authority. */
export type LifecycleActions = {
  schema: 1
  intent: string
  lifecycle: LifecycleState
  startAllowed: boolean
  startBlockedBy: LifecycleActionBlock | null
  stopAllowed: boolean
  stopBlockedBy: LifecycleActionBlock | null
  explicitStartRequired: boolean
}

export type ProbeStepStatus = 'pending' | 'running' | 'pass' | 'fail' | 'skipped' | 'unavailable' | 'leak'

/** A reported-address relation never certifies the local route or leak absence. */
export type ProbeComparison = {
  schema: 1
  reference: 'profile_expected' | 'protected_responses' | 'control_vs_protected'
  outcome: 'match' | 'mismatch' | 'unknown'
  observationPoint: 'remote_https_response'
  topology: 'unknown'
  pathScope: 'unattested'
  localSafety: 'unknown'
  violationBasis: 'none'
}

export type ProbeStep = {
  name: string
  status: ProbeStepStatus
  detail: string
  comparison?: ProbeComparison
  /** Missing means required. Only the same-image OS comparison may be informational. */
  requirement?: 'required' | 'informational'
}

export type ProbeReportView = {
  ok: boolean
  steps: ProbeStep[]
  /** Optional for older binaries/bindings. This describes the report, never live route admission. */
  scope?: ProbeScope
}

export type ProbeMeasurementStatus = 'observed' | 'incomplete' | 'unmeasured'

export type ProbeScope = {
  schema: 1
  https: ProbeMeasurementStatus
  doh: ProbeMeasurementStatus
  ipv4TCP: 'unmeasured'
  ipv4UDP: 'unmeasured'
  ipv6TCP: 'unmeasured'
  ipv6UDP: 'unmeasured'
  localSafety: 'unmeasured'
  historical: boolean
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

export type DiagnosticContext = {
  schema: 1
  revision: string
  intent: string
  path: string
  intentCurrent: boolean
  pathCurrent: boolean
  closing: boolean
  channel: string
  mode: '' | 'socks' | 'tun'
  lifecycle: LifecycleState
  ready: boolean
}

export type AppState = {
  /** Optional for old binaries. Negative display fence only, never traffic authority. */
  diagnosticContext?: DiagnosticContext | null
  /** Missing/unsupported on old binaries: positive request availability is unknown. */
  lifecycleActions?: LifecycleActions | null
  state: LifecycleState
  blockedReason: BlockedReason | null
  activeChannelId: string | null
  activeChannel: ChannelDescriptor | null
  proxyMode: 'socks' | 'tun' | null
  probe: ProbeReportView | null
  probeRunning: boolean
  probeLastAt: string | null
  error: string
  /** Reported running readiness; distinct from command registration and the path probe. */
  coreReady: boolean
  coreBlockMsg: string
  /** V2-053: RFC3339 (UTC) следующей авто-попытки запуска; '' = не запланировано */
  nextRetryAt: string
  /** V2-053: номер следующей авто-попытки (1..); 0 = не запланировано */
  retryAttempt: number
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
  kind: 'HEALED' | 'HEALING' | 'PROBLEM' | 'WEEKLY' | 'unknown'
  text: string
}

/** Read-only bounded log observation; neither repair nor traffic authority. */
export type NetGuardLogObservation = {
  schema: 1
  state: 'observed' | 'missing' | 'busy' | 'unavailable'
  bytesRead: number
  byteLimit: number
  rows: number
  truncated: boolean
  incomplete: boolean
}

export type NetGuardLoopbackObservation = {
  schema: 1
  port: number
  outcome: 'reachable' | 'refused' | 'timeout' | 'canceled' | 'busy' | 'unavailable'
}

export type NetGuardStatus = {
  tasks: NetGuardTaskInfo[]
  proxy: NetGuardProxyInfo
  socksAlive: boolean
  events: NetGuardEventInfo[]
  lastRun: string
  dohDisabled: boolean
  log?: NetGuardLogObservation | null
  loopbacks?: NetGuardLoopbackObservation[] | null
}

export type SecretMeta = {
  id: string
  kind: SecretKind
  title: string
  hint: string
  backend: SecretBackend | null
  storedValue: 'file' | 'dpapi' | 'keyring' | 'unknown' | null
  fingerprint: string
  createdAt: string
  updatedAt: string
  lastVerifiedAt: string | null
  verifyStatus: 'unverified' | 'ok' | 'failed'
  verifyError: string
  reveals: number | null
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
  context?: DiagnosticContext | null
  contextCurrent?: boolean | null
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
export type NetworkEgressObservation = {
  route: 'os_control' | 'tunnel_socks'
  outcome: 'ok' | 'skipped' | 'busy' | 'budget' | 'canceled' | 'timeout' | 'unavailable' | 'invalid_response' | 'http_status' | 'cleanup_unconfirmed'
  reason: '' | 'not_requested' | 'policy' | 'no_session' | 'no_intent'
  attempted: boolean // target frame entered; packets/server arrival are not attested
  stage: '' | 'dns' | 'connect' | 'proxy' | 'tls' | 'request' | 'headers' | 'body' | 'complete'
  startedAt: string
  completedAt: string
  elapsedMicros: number | null // whole attempt frame, never wire RTT
  ageMicros: number | null // monotonic age at backend reply, not a new measurement
  httpStatus: number | null
}
export type NetworkEgressFacts = { schema: 1; osControl: NetworkEgressObservation; socks: NetworkEgressObservation }

export type NetworkFacts = {
  connectionType: string // Wi-Fi | Ethernet | ''
  ssid: string
  localIP: string
  isp: string
  publicIP: string
  tunnelIP: string
  country: string
  countryOrigin: string
  dnsViaTunnel: boolean | null // absence is not a failed measurement
  netClass: string // wifi | ethernet | mobile | '' (V2-054)
  checkedAt: string
  localRouteTarget?: string
  egress?: NetworkEgressFacts | null // absent: legacy; null: present but unsupported/malformed
}

/** Факты о конкретном запущенном EXE; version задаётся production ldflags. */
export type ProcessModeFacts = {
  schema: 1
  pid: number
  mode: 'socks' | 'tun'
  tunAdmission: boolean
  elevationAdmission: boolean
}

export type RuntimeInfo = {
  version: string
  os: string
  arch: string
  pid: number
  coreVersion: string
  shellVersion: string
  /** Process flags and compiled negative gates, never a protected-path result. */
  processMode?: ProcessModeFacts | null
}

/** Locally route-selected whole-adapter counters, never VPN-only counters. */
export type NetStats = {
  inOctets: number | null
  outOctets: number | null
  speedBps: number | null
  alias: string
  source?: 'native_route_adapter' | 'legacy'
  adapterId?: string | null
  routeTarget?: string | null
}

/** Факты доставленного подписанного metadata-envelope (FR-008). */
export type DeliveryProfile = {
  present: boolean
  version: number | null // legacy numeric field; use the lossless string identity
  keyId: string
  expiresAt: string
  trustedKeys: number | null
  channelsApplied: number | null
  error: string
  factsSchema?: number
  state?: 'embedded' | 'accepted' | 'legacy_unanchored' | 'restricted' | 'policy_pending' | 'unavailable' | 'unknown'
  profileIdentity?: string
  retainedIdentity?: string
  checkedAt?: string
  activeProfileKeys?: number | null
  channelsDeclared?: number | null
  channelsEligible?: number | null
  channelsRestricted?: number | null
  certificatesRevoked?: number | null
  channelsRetired?: number | null
  pinPolicyState?: 'not_observed' | 'applied' | 'pending' | 'unknown'
  pinPolicyVersion?: string
  pinPolicySHA256?: string
  storageFactsSchema?: number
  storageState?: 'not_observed' | 'cleanup_pending' | 'recovery_required' | 'unknown'
  storageWarning?: string
}

/** Снимок сторожа автозащиты (failoverPolicy). */
export type FailoverStatus = {
  enabled: boolean
  state: string // closed | open | half-open
  switches: number | null
  lastError: string
  nextCheckIn: number | null // 0 can mean not applicable, not necessarily due now
  smState: string // C1 (V2-060): машина состояний адаптивного цикла; "" = цикл не активен (нет туннеля)
  smSince: string // RFC3339: с какого момента в этом состоянии ("" = нет)
  smClass: string // код THEORY последнего классифицированного отказа ("" = не было)
}

/** Здоровье канала в текущей сетевой эпохе (PLAN-V3 A0, V2-057). */
export type ChannelHealthEntry = {
  id: string
  transport: string // из дескриптора ("vless+ws", "hysteria2+quic", …), "" = не найден
  ok: number | null
  fail: number | null
  lastOkAt: string
  lastFailAt: string
  lastClass: string // класс отказа из failclass.go ("" = не было)
  verdict: 'no_data' | 'healthy' | 'failing'
  // D2 (V2-064): производное состояние восстановления — выводится из фактов
  // (защищённый probe = сильные, фон D1 = слабые), не хранится и не решает.
  d2State: 'HEALTHY' | 'PROBATION' | 'RECOVERING' | 'QUARANTINED' | ''
  d2Note: string // человекочитаемое основание ("" = нет)
}

export type ChannelHealth = {
  epoch: number | null // 0 = факта сети ещё не было; null = поле недоступно
  started: string // момент начала эпохи (RFC3339, "" = нет)
  netClass: string
  now: string
  channels: ChannelHealthEntry[]
}

/** Configured dependency comparison; public shape retained from B2.
 * `score` is a ranking hint, never a probability or proof of independence.
 * `sharedFdoms` includes shared roles, coverage and unknowns; `belowLimit`
 * is advisory only and does not establish that a fallback is safe/unsafe. */
export type ChannelIndependenceInfo = {
  a: string
  b: string
  score: number | null
  sharedFdoms: string
  belowLimit: boolean
}

/** Волна отказов D3 (V2-065): кластер подтверждённых отказов текущей эпохи. */
export type IncidentCluster = {
  since: string // момент первого отказа волны (RFC3339)
  until: string // момент последнего отказа волны (RFC3339)
  channels: string[] // уникальные id каналов (порядок первого отказа)
  count: number | null // событий в волне (может быть > числа каналов)
  classes: string[] // уникальные классы отказов (failclass)
  netClasses: string[] // snapshot типов сети событий
  isps: string[] // snapshot операторов, если collector успел получить факт
  isWave: boolean // склеено ли в волну (false = одиночный инцидент)
  clusterId: string // детерминированный id (для ссылок)
}

/** Класс ответа одного сайта через живой туннель (адаптивный слой, V2-041). */
export type SiteStatus = {
  domain: string
  code: number | null // 0 = ответа не было; null = код не передан
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
  up: number | null
  down: number | null
  since: string
}

/** Снимок метрик ядра (F12). available=false — честная недоступность с причиной. */
export type MetricsView = {
  /** Optional on older binaries; missing receipt cannot correlate core samples. */
  collector?: string
  context?: DiagnosticContext | null
  available: boolean
  note: string
  rateUp: number | null   // reported bytes/sec; validity interval is not supplied
  rateDown: number | null
  sessionUp: number | null   // от первого замера сессии ядра (подписано в UI)
  sessionDown: number | null
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
  bundleIdentity?: string
  currentIdentity?: string
  authorityMode?: string
  signerFingerprint?: string
  targetSHA256?: string
  deliveryAnchorSHA256?: string
  deliveryValidUntil?: string
  validUntil?: string // actual minimum of profile/proof/grant/policy/permit deadlines
  pinPolicyVersion?: string
  pinPolicySHA256?: string
  currentPinPolicySHA256?: string
  pinPolicyValidUntil?: string
  pinPolicyRequired?: boolean
  bootstrapPolicySHA256?: string // echoed only after enforcing independent initial approval
}

export type PublicProfileMode = 'profile' | 'enroll' | 'recover' | 'epoch_recover' | 'epoch_enroll'
export type PublicProfileRoute = 'protected' | 'bootstrap' | 'local'
export type PublicProfileSourceRequest = {
  request_id: string
  purpose?: 'import' | 'export'
  source: { repository: string; target_path: string; route: PublicProfileRoute; allow_direct: boolean }
  import: { json: string; mode: PublicProfileMode; approved_root_sha256?: string; approved_pin_policy_sha256?: string }
}
export type PublicProfileSourcePreview = {
  token: string
  requestId: string
  route: PublicProfileRoute
  releaseId: string
  packageSHA256: string
  expiresAt: string
  profile: OnboardingPreview
  purpose: 'import' | 'export'
  destinationRootSHA256?: string
  destinationPinPolicySHA256?: string
}

export type PinPolicyApproval = {
  sourceTrustSHA256: string
  sourceCheckpointSHA256: string
  capsuleSHA256: string
  policySHA256: string
}
export type PinPolicyPreview = {
  approval: PinPolicyApproval
  profileIdentity: string
  policyVersion: string
  appliedVersion?: string
  appliedSHA256?: string
  validUntil: string
  rootVersion: string
  willRetireSession: boolean
  committed: boolean
  requiresFreshStart: boolean
}
export type PinPolicySourceRequest = {
  request_id: string
  source: { repository: string; target_path: 'trust/pin-history.json'; route: PublicProfileRoute; allow_direct: boolean }
}
export type PinPolicySourcePreview = {
  token: string
  requestId: string
  route: PublicProfileRoute | 'file'
  releaseId?: string
  packageSHA256: string
  expiresAt: string
  policy: PinPolicyPreview
}

export type DeliveredExportResult = {
  destinationMode: 'enroll' | 'epoch_enroll'
  transport: string
  profileIdentity: string
  destinationRootSHA256: string
  destinationPinPolicySHA256: string
  targetSHA256: string
  secrets: number
}
export type ProtectedProfileTransferRequest = {
  transport: string
  passphrase: string
  mode: PublicProfileMode
  approved_root_sha256?: string
  approved_pin_policy_sha256?: string
  approved_target_sha256?: string
}

/** Превью проверки обновления (F15). ok=false — честный отказ с причиной. */
export type UpdatePreview = {
  currentVersion: string
  version: string
  notes: string
  releasedAt: string
  size: number | null
  sha256: string
  keyId: string
  floor: string
  ok: boolean
  reason: string
  approval: { token: string; intent: string; manifestSHA256: string; expiresAt: string } | null
}
