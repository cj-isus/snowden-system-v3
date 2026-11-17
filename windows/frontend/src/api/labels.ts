/**
 * Словарь фактических состояний (RU). Никаких синонимов «подключено/соединено»:
 * слова должны совпадать со states контракта 1:1 (AGENTS.md §3.6).
 */
import type { BlockedReason, DiagnosticContext, LifecycleActionBlock, LifecycleState, ProbeComparison, ProbeStep, ProbeStepStatus, SecretKind, SecretBackend, ValidationStatus, TestResult, NetGuardLoopbackObservation, NetworkEgressObservation } from './contract'

function comparisonReference(step: Pick<ProbeStep, 'name'>): ProbeComparison['reference'] | null {
  if (step.name === 'Expected egress') return 'profile_expected'
  if (step.name === 'Egress consistency') return 'protected_responses'
  if (step.name === 'Direct leak check') return 'control_vs_protected'
  return null
}

export function isEgressComparisonStep(step: Pick<ProbeStep, 'name'>): boolean {
  return comparisonReference(step) !== null
}

function hasSupportedComparison(step: ProbeStep): boolean {
  const c = step.comparison
  return !!c && comparisonReference(step) !== null && c.reference === comparisonReference(step) &&
    c.schema === 1 && (c.outcome === 'match' || c.outcome === 'mismatch' || c.outcome === 'unknown') &&
    c.observationPoint === 'remote_https_response' && c.topology === 'unknown' &&
    c.pathScope === 'unattested' && c.localSafety === 'unknown' && c.violationBasis === 'none'
}

export function hasUnknownEgressComparison(step: ProbeStep): boolean {
  const reference = comparisonReference(step)
  return (reference !== null || !!step.comparison) && !hasSupportedComparison(step) && step.status !== 'pending' && step.status !== 'running' && step.status !== 'skipped'
}

export function probeStepName(step: ProbeStep): string {
  if (step.name === 'Native TUN flow') return 'Поток через системный туннель'
  if (step.name === 'TUN admission') return 'Действующий допуск туннеля'
  if (step.name === 'TUN activation') return 'Общий допуск туннеля'
  if (step.name === 'Direct leak check') return 'Контрольный выход ОС'
  if (step.name === 'Expected egress') return 'Адрес выхода и профиль'
  if (step.name === 'Egress consistency') return 'Адреса в ответах HTTPS-целей'
  return step.name
}

/** Value-only display contract, never a grant. Unknown/malformed roles stay required. */
export function isInformationalProbeStep(step: ProbeStep): boolean {
  return step.requirement === 'informational' && step.name === 'Direct leak check' &&
    hasSupportedComparison(step) && step.status === 'unavailable'
}

export function probeStepStatus(step: ProbeStep): ProbeStepStatus {
  if (step.requirement !== undefined && step.requirement !== 'required' && step.requirement !== 'informational') return 'unavailable'
  if (step.status === 'pending' || step.status === 'running' || step.status === 'skipped') return step.status
  if (step.requirement === 'informational' && !isInformationalProbeStep(step)) return 'unavailable'
  if (hasUnknownEgressComparison(step)) return 'unavailable'
  if (isEgressComparisonStep(step)) {
    if (step.comparison?.outcome === 'unknown' || step.status === 'leak') return 'unavailable'
    if (step.comparison?.outcome === 'mismatch' && step.comparison.reference !== 'control_vs_protected') return 'fail'
  }
  return step.status
}

export function probeStepDetail(step: ProbeStep): string {
  if (step.requirement !== undefined && step.requirement !== 'required' && step.requirement !== 'informational') {
    return 'Роль шага диагностики не поддерживается; обязательная проверка не подтверждена.'
  }
  if (step.requirement === 'informational' && !isInformationalProbeStep(step)) {
    return 'Информационная роль не соответствует шагу. Обязательная проверка не подтверждена.'
  }
  if (hasUnknownEgressComparison(step)) {
    return 'Формат сравнения IP отсутствует или не соответствует шагу. Локальный маршрут и отсутствие утечек этим результатом не подтверждаются.'
  }
  if (isEgressComparisonStep(step) && step.status === 'leak') {
    return 'IP-сопоставление не подтверждает локальную утечку; требуется отдельное наблюдение.'
  }
  if (isEgressComparisonStep(step) && step.status === 'pass' && probeStepStatus(step) !== 'pass') {
    return 'Результат сравнения IP не подтверждает успешный статус. Локальная безопасность не измерена.'
  }
  return step.detail
}

/** Negative or incomplete steps restrict an aggregate; they never promote it. */
export function probeReportOutcome(ok: boolean | null | undefined, steps: readonly ProbeStep[]): boolean | null {
  const required = steps.filter(step => !isInformationalProbeStep(step))
  const statuses = required.map(probeStepStatus)
  if (ok === false || statuses.some(status => status === 'fail' || status === 'leak')) return false
  if (steps.length > 0 && required.length === 0) return null
  if (statuses.some(status => status === 'pending' || status === 'running' || status === 'unavailable')) return null
  return ok ?? null
}

const manualDiagnosticIds = new Set(['udp-availability', 'hy2-reachability', 'vless-config-present', 'elevated-process', 'freeze-check', 'core-wired'])

export function knownManualDiagnostic(id: string): boolean { return manualDiagnosticIds.has(id) }

/** Stable primitive key: not reference equality, time, health or an admission grant. */
export function diagnosticContextKey(value: Readonly<DiagnosticContext> | null | undefined): string {
  return value ? JSON.stringify([value.schema, value.revision, value.intent, value.path, value.intentCurrent, value.pathCurrent, value.closing,
    value.channel, value.mode, value.lifecycle, value.ready]) : ''
}

export function diagnosticResultCurrent(test: Pick<TestResult, 'id' | 'context' | 'contextCurrent'>, current: Readonly<DiagnosticContext> | null): boolean {
  const observed = test.context
  if (test.contextCurrent !== true || !knownManualDiagnostic(test.id) || !observed || !current || !current.intentCurrent || current.closing ||
    !observed.intentCurrent || observed.closing || diagnosticContextKey(observed) !== diagnosticContextKey(current)) return false
  const network = test.id === 'udp-availability' || test.id === 'hy2-reachability' || test.id === 'freeze-check'
  return !network || observed.path !== '0' && observed.pathCurrent
}

/** One manual-result aggregate for mapping, card and action feedback. Protected
 * probe roles remain unchanged; OS-route comparisons are visible observations. */
export function manualDiagnosticOutcome(test: Pick<TestResult, 'id' | 'ok' | 'steps'>): boolean | null {
  const steps = test.steps ?? []
  if (test.id !== 'freeze-check') return probeReportOutcome(test.ok, steps)
  const tunnelNames = [8, 16, 32, 64].map(size => `туннель ${size} КБ`)
  const comparisonNames = [8, 16, 32, 64].map(size => `direct ${size} КБ`)
  const required = steps.filter(step => !comparisonNames.includes(step.name))
  const completePlan = required.length === tunnelNames.length && tunnelNames.every(name => required.some(step => step.name === name)) &&
    new Set(steps.map(step => step.name)).size === steps.length
  const measured = required.every(step => ['pass', 'fail', 'leak'].includes(probeStepStatus(step)))
  return completePlan && measured ? probeReportOutcome(test.ok, required) : null
}

export function probeComparisonLabel(step: ProbeStep): string {
  if (!step.comparison || !hasSupportedComparison(step)) return ''
  return ({ match: 'IP совпадают', mismatch: 'IP различаются', unknown: 'Сравнение IP недоступно' })[step.comparison.outcome]
}

/** An inapplicable batch item may have ok=true without proving the property. */
export function isSkippedTest(test: TestResult): boolean {
  return test.ok !== false && !!test.steps?.length &&
    test.steps.every(step => step.status === 'skipped')
}

export const lifecycleLabels: Record<LifecycleState, string> = {
  stopped: 'Остановлен',
  starting: 'Запускается…',
  running: 'Работает',
  stopping: 'Останавливается…',
  reloading: 'Переключение канала…',
  error: 'Ошибка',
}

export const blockedReasonLabels: Record<BlockedReason, string> = {
  no_validated_channel: 'нет проверенного канала',
  probe_failed: 'probe не пройден',
  config_invalid: 'конфиг невалиден',
  all_channels_failed: 'все каналы недоступны (failover исчерпан)',
  protection_retained: 'сохранён запрос системной защиты; состояние требует проверки',
  proxy_conflict: 'настройки прокси изменены извне; нужен явный запуск после проверки',
  cleanup_unconfirmed: 'возврат ресурсов не подтверждён',
  profile_recovery_required: 'локальный профиль требует восстановления',
  profile_update_pending: 'смена профиля ещё не завершена',
  profile_reconciliation_required: 'результат смены профиля требует согласования',
  profile_storage_recovery_required: 'хранилище профиля требует восстановления',
  update_recovery_required: 'целостность установки требует восстановления',
  installed_image_changed: 'файлы приложения изменились; нужен новый запуск после очистки',
  restart_failed: 'перезапуск приложения не подтверждён',
  clock_uncertain: 'время системы не позволяет подтвердить доверие',
  profile_expired: 'срок действия профиля истёк',
  trust_not_yet_valid: 'срок действия доверия ещё не наступил',
  pin_policy_pending: 'новая политика сертификатов ожидает безопасного применения',
  policy_channels_unavailable: 'нет канала с действующим разрешённым сертификатом',
  trust_checkpoint_unavailable: 'сохранение состояния доверия не подтверждено',
  trust_authority_unavailable: 'полномочия источника доверия не подтверждены',
  certificate_expired: 'нет действующего сертификата для подключения',
  trust_unavailable: 'доверие для подключения не подтверждено',
  unrecognized: 'причина блокировки не распознана этой версией интерфейса',
}

export function blockedReasonNextStep(reason: BlockedReason): string {
  switch (reason) {
    case 'proxy_conflict':
      return 'Проверьте системный прокси и работу других VPN. Затем явно запустите новую попытку; чужие настройки автоматически не восстанавливаются.'
    case 'cleanup_unconfirmed':
      return 'Явно отключите VPN и дождитесь результата очистки. Если она не подтвердится, откройте журнал и локальное восстановление; не запускайте автоматические повторы.'
    case 'profile_update_pending':
      return 'Дождитесь результата смены профиля или явно отключите VPN. Отправка команды не подтверждает сохранение профиля.'
    case 'profile_recovery_required':
    case 'profile_reconciliation_required':
    case 'profile_storage_recovery_required':
      return 'Откройте настройки профиля и перечитайте его состояние. Повторите подтверждённый импорт или восстановление; прежнее подключение автоматически не возобновляется.'
    case 'installed_image_changed':
    case 'update_recovery_required':
    case 'restart_failed':
      return 'Завершите очистку и проверьте установку приложения в настройках. При неподтверждённом обновлении восстановите полный комплект приложения.'
    case 'clock_uncertain':
    case 'trust_not_yet_valid':
      return 'Проверьте системное время и срок действия профиля. Не отключайте проверку сертификатов ради подключения.'
    case 'profile_expired':
    case 'certificate_expired':
    case 'policy_channels_unavailable':
      return 'Получите актуальный профиль из доверенного источника. Если указан запланированный сертификат, дождитесь его срока и новой обязательной проверки.'
    case 'pin_policy_pending':
    case 'trust_checkpoint_unavailable':
    case 'trust_authority_unavailable':
    case 'trust_unavailable':
      return 'Перечитайте состояние профиля и доверия в настройках. Примените подтверждённое восстановление; проверку TLS отключать нельзя.'
    case 'protection_retained':
      return 'Для новой попытки используйте явный запуск. Для отмены повторов и запроса снятия защиты выберите отключение; его результат подтвердится после очистки.'
    case 'no_validated_channel':
    case 'config_invalid':
      return 'Проверьте действующий профиль и список доступных каналов. Наличие записи в списке не подтверждает доступность сервера.'
    case 'probe_failed':
    case 'all_channels_failed':
      return 'Проверьте область и время последнего отчёта. Можно дождаться указанного автоповтора или явно отключить VPN и отменить повторы.'
    default:
      return 'Обновите сведения. Если причина останется неизвестной, откройте журнал и локальное восстановление; успешное подключение не подтверждено.'
  }
}

export const lifecycleActionHelp: Record<LifecycleActionBlock, { reason: string; next: string }> = {
  explicit_start_required: { reason: 'Для продолжения нужен явный запуск.', next: blockedReasonNextStep('proxy_conflict') },
  restart_committed: { reason: 'Приложение уже передаёт управление новому процессу.', next: 'Дождитесь результата перезапуска и откройте состояние нового приложения.' },
  protection_release_committed: { reason: 'Запрос снятия защиты ещё не завершён.', next: 'Дождитесь результата отключения. Новая команда сейчас не нужна.' },
  update_restart_required: { reason: blockedReasonLabels.installed_image_changed, next: blockedReasonNextStep('installed_image_changed') },
  startup_unavailable: { reason: 'Подготовка приложения прервана.', next: 'Проверьте завершение остановки VPN и перезапустите приложение.' },
  cleanup_unconfirmed: { reason: blockedReasonLabels.cleanup_unconfirmed, next: blockedReasonNextStep('cleanup_unconfirmed') },
  profile_reconciliation_required: { reason: blockedReasonLabels.profile_reconciliation_required, next: blockedReasonNextStep('profile_reconciliation_required') },
  lifecycle_busy: { reason: 'Другая операция приложения ещё выполняется.', next: 'Дождитесь её завершения.' },
  closing: { reason: 'Приложение закрывается.', next: 'Дождитесь завершения закрытия. Повторные команды этому процессу недоступны.' },
  app_unavailable: { reason: 'Действующий сеанс приложения не подтверждён.', next: 'Откройте приложение Windows и перечитайте его состояние.' },
  state_unavailable: { reason: 'Текущее состояние приложения не распознано.', next: 'Обновите сведения и используйте совместимый комплект приложения и интерфейса. Неизвестное состояние не разрешает новый запуск.' },
}

export const probeStepStatusLabels: Record<ProbeStepStatus, string> = {
  pending: 'ожидает',
  running: 'выполняется',
  pass: 'пройдено',
  fail: 'провалено',
  skipped: 'пропущено',
  unavailable: 'нет доказательства',
  leak: 'обнаружена утечка',
}

export const validationStatusLabels: Record<ValidationStatus, string> = {
  planned: 'запланирован',
  configured: 'настроен',
  'locally-tested': 'проверен локально',
  'live-verified': 'проверен live',
  degraded: 'деградирован',
  blocked: 'заблокирован',
  retired: 'выведен',
}

export const secretKindLabels: Record<SecretKind, string> = {
  'vless-uuid': 'UUID VLESS',
  'hy2-password': 'Пароль HY2',
  'hy2-obfs-password': 'Пароль obfs (HY2)',
  'vps-ssh-key': 'SSH-ключ VPS',
  'cf-api-token': 'API-токен Cloudflare',
  custom: 'Своё значение',
}

export const secretKindHints: Record<SecretKind, string> = {
  'vless-uuid':
    'Идентификатор пользователя VLESS на сервере. Значение знает только сервер и клиент; в чат и скриншоты не вставлять.',
  'hy2-password':
    'Пароль аутентификации Hysteria2 (UDP-канал). Сверка с сервером — по SHA256-хешу, не по значению.',
  'hy2-obfs-password':
    'Пароль обфускации salamander для HY2. Должен совпадать с серверным; иначе UDP-пакеты не распознаются.',
  'vps-ssh-key':
    'Приватный SSH-ключ доступа к VPS. Значение никогда не попадает в логи и diagnostics.',
  'cf-api-token':
    'API-токен Cloudflare. Утечка в git-историю = компрометация: токен нужно отозвать и перевыпустить.',
  custom: 'Произвольный секрет для тестов. Хранится локально, в git не попадает.',
}

export const secretBackendLabels: Record<SecretBackend, string> = {
  file: 'Файл (secrets/)',
  dpapi: 'DPAPI (шифрование Windows)',
  keyring: 'Системное хранилище',
}

export const proxyModeLabels: Record<'socks' | 'tun', string> = {
  socks: 'SOCKS :1080',
  tun: 'TUN (нужен admin)',
}

/** Дата/время для журналов. Возвращает '—' вместо пустой строки (unavailable как unavailable). */
export function fmtTime(iso: string | null | undefined): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return d.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

const loopbackLabels: Record<NetGuardLoopbackObservation['outcome'], string> = {
  reachable: 'TCP-соединение установлено', refused: 'TCP-соединение отклонено',
  timeout: 'Время наблюдения истекло', canceled: 'Наблюдение отменено',
  busy: 'Предыдущее наблюдение ещё выполняется', unavailable: 'Состояние порта неизвестно',
}

/** Same finite observation on both pages; missing legacy booleans are not facts. */
export function fmtLoopbackObservation(facts: readonly NetGuardLoopbackObservation[] | null | undefined, port: number): string {
  const fact = facts?.find(entry => entry.port === port)
  return fact ? loopbackLabels[fact.outcome] ?? 'Состояние порта неизвестно' : 'Нет наблюдения с известным исходом'
}

export function fmtDateTime(iso: string | null | undefined): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return d.toLocaleString('ru-RU', {
    day: '2-digit',
    month: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

/** Display-only optional request outcome, never a VPN health or DNS verdict. */
export function fmtEgressObservation(value: NetworkEgressObservation | null | undefined): string {
  if (value === undefined) return 'Подробности запроса не переданы'
  if (value === null) return 'Формат наблюдения не поддерживается'
  const stage = { '': '', dns: 'получение адреса', connect: 'соединение', proxy: 'прокси', tls: 'TLS', request: 'отправка запроса', headers: 'заголовки', body: 'чтение ответа', complete: 'ответ' }[value.stage]
  switch (value.outcome) {
    case 'ok': return 'HTTPS-ответ с IP принят'
    case 'skipped': return { '': 'Измерение не запрошено', not_requested: 'Измерение не запрошено', policy: 'Запрос отключён режимом защиты', no_session: 'Нет подходящей сессии для запроса', no_intent: 'Запрос не запущен в текущем состоянии' }[value.reason]
    case 'busy': return 'Источник занят предыдущим запросом'
    case 'budget': return 'Достигнут лимит необязательных запросов'
    case 'canceled': return 'Результат отменён'
    case 'timeout': return value.attempted ? 'Результат не применён: истёк срок наблюдения' : 'Запрос не начат: истёк общий срок'
    case 'http_status': return `Сервис ответил HTTP ${value.httpStatus}`
    case 'invalid_response': return `Ответ не принят${value.httpStatus === null ? '' : ` · HTTP ${value.httpStatus}`}`
    case 'cleanup_unconfirmed': return 'Завершение ресурсов запроса не подтверждено'
    default: return stage ? `Исход не установлен: ${stage}` : 'Исход не установлен'
  }
}

/** Backend monotonic durations; no client clock comparison, timer or freshness grant. */
export function fmtEgressObservationTime(value: NetworkEgressObservation | null | undefined): string {
  if (value === undefined) return 'Время и длительность запроса не переданы'
  if (value === null) return 'Время наблюдения недоступно'
  if (!value.attempted) return ['skipped', 'busy', 'budget'].includes(value.outcome) ? 'Новая попытка не начата' : 'Время попытки не передано'
  const duration = (micros: number | null) => micros === null ? 'неизвестно' : micros < 1000 ? 'менее 1 мс' : micros < 1_000_000 ? `${Math.ceil(micros / 1000)} мс` : `${(micros / 1_000_000).toLocaleString('ru-RU', { maximumFractionDigits: 1 })} с`
  const at = new Date(value.startedAt).toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  return `Начало ${at} · попытка ${duration(value.elapsedMicros)} · возраст исхода при передаче ${duration(value.ageMicros)}`
}

/** Display elapsed time only; it never assigns a freshness or validity budget. */
export function fmtObservationAge(iso: string | null | undefined, now: number): string {
  const at = iso ? Date.parse(iso) : NaN
  if (!Number.isFinite(at) || !Number.isFinite(now)) return 'возраст неизвестен'
  const elapsed = now - at
  if (elapsed < 0) return 'время в будущем — проверьте часы'
  const minutes = Math.floor(elapsed / 60_000)
  if (minutes < 1) return 'менее минуты назад'
  if (minutes < 60) return `${minutes} мин назад`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours} ч ${minutes % 60} мин назад`
  return `${Math.floor(hours / 24)} д ${hours % 24} ч назад`
}
