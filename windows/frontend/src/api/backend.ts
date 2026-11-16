/**
 * Типизированный фасад Wails-биндингов (wailsjs/go/main/App).
 *
 * В Wails-сборке вызовы идут в Go (window.go.*). В браузерном предпросмотре
 * (npm run dev / static preview) биндингов нет — фасад честно сообщает об этом,
 * UI показывает «нет данных», ничего не выдумывая (AGENTS.md §3.6).
 */
import * as App from '../../wailsjs/go/main/App'
import { EventsEmit, EventsOn } from '../../wailsjs/runtime/runtime'
import { MOCK_ACTIVE } from './preview'
import type {
  AdaptiveStatus,
  MetricsView,
  OnboardingPreview,
  PublicProfileSourceRequest,
  PublicProfileSourcePreview,
  PublicProfileRoute,
  PinPolicyPreview,
  PinPolicySourceRequest,
  PinPolicySourcePreview,
  DeliveredExportResult,
  ProtectedProfileTransferRequest,
  UpdatePreview,
  AppState,
  DiagnosticContext,
  LifecycleActions,
  LifecycleActionBlock,
  BlockedReason,
  UILifecycleCommand,
  UILifecycleRequest,
  UILifecycleReceipt,
  ChannelDescriptor,
  ChannelHealth,
  ChannelHealthEntry,
  ChannelIndependenceInfo,
  DeliveryProfile,
  IncidentCluster,
  FailoverStatus,
  NetGuardStatus,
  NetGuardLogObservation,
  NetGuardLoopbackObservation,
  NetGuardTaskState,
  NetStats,
  NetworkFacts,
  NetworkEgressFacts,
  NetworkEgressObservation,
  RuntimeInfo,
  ProcessModeFacts,
  SecretMeta,
  SecretTestReport,
  SiteStatus,
  TestResult,
  ProbeComparison,
  ProbeStep,
  ProbeStepStatus,
  ProbeScope,
  ProxyRecoveryPreview,
  ProxyRecoveryResult,
  ProxyRecoveryCancellation,
} from './contract'
import { BLOCKED_REASONS, LIFECYCLE_ACTION_BLOCKS } from './contract'
import { isInformationalProbeStep, manualDiagnosticOutcome, probeReportOutcome, probeStepDetail, probeStepStatus } from './labels'
import { backendLog } from '../composables/toast'
import { profileActionNotSent, uiFailure } from './failureMessages'

// Формы из сгенерированных моделей (структурно идентичны контракту).
type GoAppState = import('../../wailsjs/go/models').main.AppState
type GoMeta = import('../../wailsjs/go/models').secretvault.Meta
type GoTest = import('../../wailsjs/go/models').main.TestResultView

type UILifecycleBindings = {
  OpenUILifecycle?: () => Promise<unknown>
  RequestUILifecycle?: (request: UILifecycleRequest) => Promise<unknown>
}
const uiLifecycleBindings: UILifecycleBindings = {
  get OpenUILifecycle() { return Reflect.get(App, 'OpenUILifecycle') as UILifecycleBindings['OpenUILifecycle'] },
  get RequestUILifecycle() { return Reflect.get(App, 'RequestUILifecycle') as UILifecycleBindings['RequestUILifecycle'] },
}
type UIRequestInterest = { current: () => boolean; dispatched?: () => void }
type UIRequestSession = { token: string; sequence: string }
let uiRequestSession: UIRequestSession | null = null
let uiSessionPending: Promise<UIRequestSession> | null = null
let uiSessionInterest = Symbol()

function retireUIRequestInterest(): void {
  uiSessionInterest = Symbol()
  uiRequestSession = null // the original pending Open remains retained until its actual return
}

function nextUISequence(value: string): string {
  if (!nativeIdentity(value) || value === '18446744073709551615') throw new Error(uiFailure.lifecycleUnavailable)
  const digits = value.split('')
  for (let i = digits.length - 1; i >= 0; i--) {
    if (digits[i] !== '9') { digits[i] = String.fromCharCode(digits[i].charCodeAt(0) + 1); return digits.join('') }
    digits[i] = '0'
  }
  return '1' + digits.join('') // exact decimal order, no Number conversion or wrapping
}

function uiSession(): Promise<UIRequestSession> {
  if (uiRequestSession) return Promise.resolve(uiRequestSession)
  if (uiSessionPending) return uiSessionPending
  const interest = uiSessionInterest, open = uiLifecycleBindings.OpenUILifecycle
  if (isBuildPhase() || typeof open !== 'function' || typeof uiLifecycleBindings.RequestUILifecycle !== 'function') return Promise.reject(new Error(uiFailure.lifecycleUnavailable))
  uiSessionPending = Promise.resolve().then(async () => {
    if (interest !== uiSessionInterest) throw new Error(uiFailure.lifecycleUnavailable)
    const raw = nativeRecord(await open())
    if (interest !== uiSessionInterest || raw.schema !== 1 || typeof raw.session !== 'string' || !/^[0-9a-f]{64}$/.test(raw.session)) throw new Error(uiFailure.lifecycleUnavailable)
    const session = { token: raw.session, sequence: '0' }
    uiRequestSession = session
    return session
  }).finally(() => { uiSessionPending = null })
  return uiSessionPending
}

async function requestUILifecycle(action: UILifecycleCommand, channelId: string, interest: UIRequestInterest, updateToken = ''): Promise<UILifecycleReceipt> {
  if (!interest.current()) throw new Error(uiFailure.lifecycleUnavailable)
  const owner = uiSessionInterest, session = await uiSession()
  const call = uiLifecycleBindings.RequestUILifecycle
  if (!interest.current() || owner !== uiSessionInterest || uiRequestSession !== session || typeof call !== 'function') throw new Error(uiFailure.lifecycleUnavailable)
  try { session.sequence = nextUISequence(session.sequence) }
  catch { retireUIRequestInterest(); throw new Error(uiFailure.lifecycleUnavailable) }
  const request: UILifecycleRequest = { schema: 1, session: session.token, sequence: session.sequence, action, channelId, updateToken }
  interest.dispatched?.() // original client call boundary only; not a native handler/side-effect witness
  const raw = nativeRecord(await call(request))
  if (raw.schema !== 1 || raw.session !== request.session || raw.sequence !== request.sequence || raw.action !== action || !nativeIdentity(raw.intent) ||
    !['registered', 'returned', 'rejected', 'failed', 'superseded', 'unconfirmed'].includes(raw.outcome as string) ||
    raw.outcome === 'registered' && action !== 'start' || raw.outcome === 'returned' && action === 'start' ||
    (raw.outcome === 'registered' || raw.outcome === 'returned' && action !== 'mode_proxy' && action !== 'mode_tun' && action !== 'update') && raw.intent === '0' ||
    raw.handlerEntered !== undefined && typeof raw.handlerEntered !== 'boolean' ||
    raw.updateInstallEntered !== undefined && typeof raw.updateInstallEntered !== 'boolean') throw new Error(uiFailure.lifecycleUnavailable)
  if (raw.outcome === 'superseded' && uiRequestSession === session) retireUIRequestInterest()
  return { schema: 1, session: request.session, sequence: request.sequence, action, intent: raw.intent, outcome: raw.outcome as UILifecycleReceipt['outcome'],
    handlerEntered: typeof raw.handlerEntered === 'boolean' ? raw.handlerEntered : null,
    updateInstallEntered: typeof raw.updateInstallEntered === 'boolean' ? raw.updateInstallEntered : null }
}

/** True, если Wails-биндинги недоступны (браузерный предпросмотр). */
export function isBuildPhase(): boolean {
  return typeof window === 'undefined' || (window as never as { go?: unknown }).go === undefined
}

export const buildPhaseError =
  'Wails-биндинги недоступны: приложение запущено вне Wails (браузерный предпросмотр). Запустите exe из windows/build/bin.'

// Optional exports are resolved only from the generated namespace. No raw
// window.go bypass and no manual edits to generated files; older builds expose
// the feature as unavailable until the normal binding generation is allowed.
type PublicSourceBindings = {
  PreviewPinPolicySource?: (request: PinPolicySourceRequest) => Promise<unknown>
  PreviewPinPolicyFile?: (requestId: string) => Promise<unknown>
  ApplyPinPolicySource?: (token: string, confirm: boolean) => Promise<unknown>
  PreviewPublicProfileSource?: (request: PublicProfileSourceRequest) => Promise<unknown>
  ApplyPublicProfileSource?: (token: string, target: string, confirm: boolean) => Promise<unknown>
  CancelPublicProfileSource?: (requestId: string) => Promise<boolean>
  ExportPublicProfileSource?: (token: string, passphrase: string) => Promise<unknown>
  SaveProtectedProfileTransfer?: (transport: string) => Promise<string>
  PreviewProtectedProfileTransfer?: (request: ProtectedProfileTransferRequest) => Promise<unknown>
  ApplyProtectedProfileTransfer?: (request: ProtectedProfileTransferRequest, confirm: boolean) => Promise<unknown>
}
const publicSource: PublicSourceBindings = {
  get PreviewPinPolicySource() { return Reflect.get(App, 'PreviewPinPolicySource') as PublicSourceBindings['PreviewPinPolicySource'] },
  get PreviewPinPolicyFile() { return Reflect.get(App, 'PreviewPinPolicyFile') as PublicSourceBindings['PreviewPinPolicyFile'] },
  get ApplyPinPolicySource() { return Reflect.get(App, 'ApplyPinPolicySource') as PublicSourceBindings['ApplyPinPolicySource'] },
  get PreviewPublicProfileSource() { return Reflect.get(App, 'PreviewPublicProfileSource') as PublicSourceBindings['PreviewPublicProfileSource'] },
  get ApplyPublicProfileSource() { return Reflect.get(App, 'ApplyPublicProfileSource') as PublicSourceBindings['ApplyPublicProfileSource'] },
  get CancelPublicProfileSource() { return Reflect.get(App, 'CancelPublicProfileSource') as PublicSourceBindings['CancelPublicProfileSource'] },
  get ExportPublicProfileSource() { return Reflect.get(App, 'ExportPublicProfileSource') as PublicSourceBindings['ExportPublicProfileSource'] },
  get SaveProtectedProfileTransfer() { return Reflect.get(App, 'SaveProtectedProfileTransfer') as PublicSourceBindings['SaveProtectedProfileTransfer'] },
  get PreviewProtectedProfileTransfer() { return Reflect.get(App, 'PreviewProtectedProfileTransfer') as PublicSourceBindings['PreviewProtectedProfileTransfer'] },
  get ApplyProtectedProfileTransfer() { return Reflect.get(App, 'ApplyProtectedProfileTransfer') as PublicSourceBindings['ApplyProtectedProfileTransfer'] },
}
export function hasPublicProfileSource(): boolean {
  return !isBuildPhase() && typeof publicSource.PreviewPublicProfileSource === 'function' && typeof publicSource.ApplyPublicProfileSource === 'function' && typeof publicSource.CancelPublicProfileSource === 'function'
}
export function hasDeliveredExport(): boolean {
  return hasPublicProfileSource() && typeof publicSource.ExportPublicProfileSource === 'function' && typeof publicSource.SaveProtectedProfileTransfer === 'function'
}
export function hasProtectedProfileTransfer(): boolean {
  return !isBuildPhase() && typeof publicSource.PreviewProtectedProfileTransfer === 'function' && typeof publicSource.ApplyProtectedProfileTransfer === 'function'
}
export function hasPinPolicySource(): boolean {
  return !isBuildPhase() && typeof publicSource.PreviewPinPolicySource === 'function' && typeof publicSource.PreviewPinPolicyFile === 'function' && typeof publicSource.ApplyPinPolicySource === 'function' && typeof publicSource.CancelPublicProfileSource === 'function'
}

function policyCounter(value: unknown): string {
  const text = nativeText(value)
  if (!/^[1-9][0-9]{0,19}$/.test(text) || text.length === 20 && text > '18446744073709551615') throw new Error('Версия списка отзывов недоступна.')
  return text
}
function policyHash(value: unknown): string {
  const text = nativeText(value)
  if (!/^[a-f0-9]{64}$/.test(text)) throw new Error('Отпечаток списка отзывов недоступен.')
  return text
}
function policyTime(value: unknown): string {
  const text = nativeText(value)
  if (!text || !Number.isFinite(Date.parse(text))) throw new Error('Срок действия списка отзывов недоступен.')
  return text
}
function mapPinPolicy(value: unknown): PinPolicyPreview {
  const p = nativeRecord(value), approval = nativeRecord(p.approval)
  for (const key of ['will_retire_session', 'committed', 'requires_fresh_start']) {
    if (typeof p[key] !== 'boolean') throw new Error('Результат применения списка отзывов неполон.')
  }
  const appliedVersion = p.applied_version === undefined ? undefined : policyCounter(p.applied_version)
  const appliedSHA256 = p.applied_sha256 === undefined ? undefined : policyHash(p.applied_sha256)
  if (Boolean(appliedVersion) !== Boolean(appliedSHA256) || !nativeText(p.profile_identity)) throw new Error('Текущее состояние списка отзывов недоступно.')
  return {
    approval: { sourceTrustSHA256: policyHash(approval.source_trust_sha256), sourceCheckpointSHA256: policyHash(approval.source_checkpoint_sha256), capsuleSHA256: policyHash(approval.capsule_sha256), policySHA256: policyHash(approval.policy_sha256) },
    profileIdentity: nativeText(p.profile_identity), policyVersion: policyCounter(p.policy_version), rootVersion: policyCounter(p.root_version),
    appliedVersion, appliedSHA256, validUntil: policyTime(p.valid_until), willRetireSession: p.will_retire_session === true,
    committed: p.committed === true, requiresFreshStart: p.requires_fresh_start === true,
  }
}
function mapPinPolicySource(value: unknown, requestId: string, route: PinPolicySourcePreview['route']): PinPolicySourcePreview {
  const p = nativeRecord(value), policy = mapPinPolicy(p.policy)
  const token = policyHash(p.token), packageSHA256 = policyHash(p.package_sha256), releaseId = nativeText(p.release_id)
  if (nativeText(p.request_id) !== requestId || nativeText(p.route) !== route || packageSHA256 !== policy.approval.capsuleSHA256 || policy.committed || (route === 'file' ? releaseId !== '' : !/^[a-f0-9]{64}$/.test(releaseId))) throw new Error('Превью списка отзывов не соответствует запросу.')
  const expiresAt = policyTime(p.expires_at)
  if (Date.parse(expiresAt) > Date.parse(policy.validUntil)) throw new Error('Срок превью превышает срок действия списка отзывов.')
  return { token, requestId, route, releaseId: releaseId || undefined, packageSHA256, expiresAt, policy }
}

function nativeRecord(value: unknown): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('Приложение вернуло неполные данные профиля.')
  return value as Record<string, unknown>
}

// Empty means a cancelled dialog only when it is an actual empty string.
// A missing/malformed native reply never becomes a harmless cancellation.
function selectedFilePath(value: unknown): string {
  if (typeof value !== 'string' || value.length > 32767 || value.includes('\0')) throw new Error(uiFailure.profileSave)
  return value // preserve the selected path exactly; never trim or reconstruct it
}
function protectedTransferText(value: unknown): string {
  if (typeof value !== 'string' || !/^SNOB[12]\./.test(value) || value.length <= 6 || value.length > (value.startsWith('SNOB2.') ? 2 * 1024 * 1024 : 256 * 1024)) throw new Error(uiFailure.profileExport)
  return value // shape/budget only; trust and crypto acceptance remain in Go
}
function nativeText(value: unknown): string { return typeof value === 'string' ? value : '' }

function mapNetworkEgress(value: unknown): NetworkEgressFacts | null {
  const outcomes: readonly string[] = ['ok', 'skipped', 'busy', 'budget', 'canceled', 'timeout', 'unavailable', 'invalid_response', 'http_status', 'cleanup_unconfirmed']
  const reasons: readonly string[] = ['', 'not_requested', 'policy', 'no_session', 'no_intent']
  const stages: readonly string[] = ['', 'dns', 'connect', 'proxy', 'tls', 'request', 'headers', 'body', 'complete']
  const timestamp = (raw: unknown): raw is string => typeof raw === 'string' && raw.length <= 64 && /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?Z$/.test(raw) && Number.isFinite(Date.parse(raw))
  const read = (raw: unknown, route: NetworkEgressObservation['route']): NetworkEgressObservation => {
    const r = nativeRecord(raw), outcome = nativeText(r.outcome), reason = r.reason === undefined ? '' : r.reason, stage = nativeText(r.stage)
    const elapsedMicros = observedCounter(r.elapsedMicros), ageMicros = observedCounter(r.ageMicros)
    const httpStatus = r.httpStatus === undefined || r.httpStatus === null ? null : observedCounter(r.httpStatus)
    if (r.elapsedMicros !== undefined && r.elapsedMicros !== null && elapsedMicros === null || r.ageMicros !== undefined && r.ageMicros !== null && ageMicros === null) throw new Error('Invalid egress timing')
    if (r.route !== route || !outcomes.includes(outcome) || typeof reason !== 'string' || !reasons.includes(reason) || !stages.includes(stage) || typeof r.stage !== 'string' || typeof r.attempted !== 'boolean') throw new Error('Unsupported egress observation')
    if (r.httpStatus !== undefined && r.httpStatus !== null && (httpStatus === null || httpStatus < 100 || httpStatus > 599)) throw new Error('Invalid HTTP observation')
    if (httpStatus !== null && !['headers', 'body', 'complete'].includes(stage)) throw new Error('HTTP status precedes response')
    if (outcome === 'skipped' ? r.attempted || !reason : reason !== '') throw new Error('Invalid egress admission')
    if (r.attempted) {
      if (!timestamp(r.startedAt) || !timestamp(r.completedAt) || elapsedMicros === null || stage === '' || outcome === 'busy' || outcome === 'budget') throw new Error('Incomplete egress attempt')
      if (r.ageMicros !== null && r.ageMicros !== undefined && ageMicros === null) throw new Error('Invalid egress age')
    } else if (r.startedAt !== '' || r.completedAt !== '' || elapsedMicros !== null || ageMicros !== null || stage !== '' || httpStatus !== null || ['ok', 'http_status', 'invalid_response'].includes(outcome)) throw new Error('Unstarted egress has measurement fields')
    if (outcome === 'ok' && (stage !== 'complete' || httpStatus !== 200) || outcome === 'http_status' && (stage !== 'headers' || httpStatus === null || httpStatus === 200)) throw new Error('Inconsistent egress outcome')
    return {
      route,
      outcome: outcome as NetworkEgressObservation['outcome'],
      reason: reason as NetworkEgressObservation['reason'],
      attempted: r.attempted,
      stage: stage as NetworkEgressObservation['stage'],
      startedAt: nativeText(r.startedAt), completedAt: nativeText(r.completedAt),
      elapsedMicros, ageMicros, httpStatus,
    }
  }
  try {
    const root = nativeRecord(value)
    if (root.schema !== 1) return null
    return { schema: 1, osControl: read(root.osControl, 'os_control'), socks: read(root.socks, 'tunnel_socks') }
  } catch { return null }
}
function mapDeliveryProfile(value: unknown): DeliveryProfile {
  const p = nativeRecord(value)
  const unknown: DeliveryProfile = { present: false, version: null, keyId: '', expiresAt: '', trustedKeys: null, channelsApplied: null, state: 'unknown', error: nativeText(p.error) || 'Эта версия приложения не предоставляет проверенную сводку профиля.' }
  if (p.factsSchema !== 1) return unknown // old raw counters/signature claims are not effective facts
  const states = ['embedded', 'accepted', 'legacy_unanchored', 'restricted', 'policy_pending', 'unavailable']
  const state = nativeText(p.state), pinPolicyState = nativeText(p.pinPolicyState)
  let storageState: DeliveryProfile['storageState'] = 'unknown'
  let storageWarning = ''
  if (p.storageFactsSchema === 1) {
    const observed = nativeText(p.storageState)
    if (!['not_observed', 'cleanup_pending', 'recovery_required'].includes(observed) || typeof p.storageWarning !== 'string' || p.storageWarning.length > 2000) throw new Error('Сведения о ресурсах хранилища неполны.')
    storageWarning = p.storageWarning
    if ((observed === 'not_observed') !== (storageWarning === '') || observed !== 'not_observed' && storageWarning.trim() === '' || observed === 'recovery_required' && state !== 'unavailable') throw new Error('Состояние хранилища не соответствует сводке профиля.')
    storageState = observed as DeliveryProfile['storageState']
  }
  if (!states.includes(state) || !['not_observed', 'applied', 'pending', 'unknown'].includes(pinPolicyState) || typeof p.present !== 'boolean') throw new Error('Проверенное состояние профиля недоступно.')
  for (const field of ['profileIdentity', 'retainedIdentity', 'keyId', 'expiresAt', 'checkedAt', 'pinPolicyVersion', 'pinPolicySHA256', 'error']) {
    if (typeof p[field] !== 'string') throw new Error('Проверенная сводка профиля неполна.')
  }
  if ((state === 'policy_pending') !== (pinPolicyState === 'pending')) throw new Error('Состояние применения отзывов не подтверждено.')
  const count = (field: string): number | null => {
    const result = p[field]
    if (result === null) return null
    if (typeof result !== 'number' || !Number.isSafeInteger(result) || result < 0) throw new Error('Состав профиля не подтверждён.')
    return result
  }
  const counter = (value: unknown): number | null => typeof value === 'number' && Number.isSafeInteger(value) && value >= 0 ? value : null
  const tuple = (field: string): string => {
    const text = nativeText(p[field])
    if (!text) return ''
    const parts = text.split(':')
    if (parts.length !== 2 || parts[1] === '0' || parts.some(part => !/^(0|[1-9][0-9]{0,19})$/.test(part) || part.length === 20 && part > '18446744073709551615')) throw new Error('Идентификатор профиля недоступен.')
    return text
  }
  const profileIdentity = tuple('profileIdentity'), retainedIdentity = tuple('retainedIdentity')
  const channelsDeclared = count('channelsDeclared'), channelsEligible = count('channelsEligible'), channelsRestricted = count('channelsRestricted')
  const knownEligibility = state === 'accepted' || state === 'restricted'
  const applied = knownEligibility ? count('channelsApplied') : null
  if (knownEligibility) {
    if (!p.present || !profileIdentity || channelsDeclared === null || channelsEligible === null || channelsRestricted === null || applied === null) throw new Error('Разрешённый состав каналов не подтверждён.')
    if (channelsEligible > channelsDeclared || channelsRestricted > channelsDeclared || applied > channelsDeclared || applied < channelsEligible || channelsRestricted < channelsDeclared - applied || channelsRestricted > channelsDeclared - channelsEligible || state === 'accepted' && channelsEligible === 0 || state === 'restricted' && channelsEligible !== 0) throw new Error('Количество каналов не соответствует принятому профилю.')
  }
  if (!knownEligibility && (channelsDeclared !== null || channelsEligible !== null || channelsRestricted !== null)) throw new Error('Состав каналов не относится к принятому профилю.')
  const policyVersion = nativeText(p.pinPolicyVersion), policySHA = nativeText(p.pinPolicySHA256)
  if (Boolean(policyVersion) !== Boolean(policySHA) || pinPolicyState === 'applied' && !policyVersion) throw new Error('Сведения о применённом списке отзывов неполны.')
  if (policyVersion) { policyCounter(policyVersion); policyHash(policySHA) }
  const checkedAt = nativeText(p.checkedAt), expiresAt = nativeText(p.expiresAt)
  if (checkedAt && !Number.isFinite(Date.parse(checkedAt)) || expiresAt && !Number.isFinite(Date.parse(expiresAt))) throw new Error('Время проверки профиля недоступно.')
  if (p.present !== (knownEligibility || state === 'legacy_unanchored') || p.present && (!profileIdentity || !expiresAt || !/^[a-f0-9]{16}$/.test(nativeText(p.keyId)))) throw new Error('Происхождение профиля не подтверждено.')
  return { present: p.present, version: counter(p.version), keyId: nativeText(p.keyId), expiresAt, trustedKeys: counter(p.trustedKeys), channelsApplied: applied, error: nativeText(p.error), factsSchema: 1, state: state as DeliveryProfile['state'], profileIdentity, retainedIdentity, checkedAt,
    activeProfileKeys: count('activeProfileKeys'), channelsDeclared, channelsEligible, channelsRestricted, certificatesRevoked: count('certificatesRevoked'), channelsRetired: count('channelsRetired'), pinPolicyState: pinPolicyState as DeliveryProfile['pinPolicyState'], pinPolicyVersion: policyVersion, pinPolicySHA256: policySHA,
    storageFactsSchema: p.storageFactsSchema === 1 ? 1 : undefined, storageState, storageWarning }
}
function mapOnboarding(value: unknown): OnboardingPreview {
  const p = nativeRecord(value)
  if (typeof p.would_downgrade !== 'boolean') throw new Error('Результат проверки версии недоступен.')
  const validUntil = p.valid_until === undefined ? undefined : policyTime(p.valid_until)
  const policyVersion = nativeText(p.pin_policy_version)
  const policySHA = nativeText(p.pin_policy_sha256), currentPolicySHA = nativeText(p.current_pin_policy_sha256), bootstrapPolicySHA = nativeText(p.bootstrap_policy_sha256)
  const policyError = () => new Error('Сведения об отзыве сертификатов неполны; получите новое превью.')
  for (const name of ['pin_policy_version', 'pin_policy_sha256', 'current_pin_policy_sha256', 'pin_policy_valid_until', 'bootstrap_policy_sha256']) {
    if (p[name] !== undefined && typeof p[name] !== 'string') throw policyError()
  }
  if (Boolean(policyVersion) !== Boolean(policySHA)) throw policyError()
  if (policyVersion && (!/^[1-9][0-9]{0,19}$/.test(policyVersion) || policyVersion.length === 20 && policyVersion > '18446744073709551615')) throw policyError()
  for (const hash of [policySHA, currentPolicySHA, bootstrapPolicySHA]) {
    if (hash && !/^[a-f0-9]{64}$/.test(hash)) throw policyError()
  }
  if (p.pin_policy_required !== undefined && typeof p.pin_policy_required !== 'boolean') throw policyError()
  if (p.pin_policy_required === true && (!policyVersion || !policySHA)) throw policyError()
  if (bootstrapPolicySHA && bootstrapPolicySHA !== policySHA) throw policyError()
  const count = (value: unknown): number => {
    if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < 0) throw new Error('Сведения о составе профиля недоступны.')
    return value
  }
  const counter = (value: unknown): number => typeof value === 'number' && Number.isSafeInteger(value) ? value : Number.NaN
  return {
    createdAt: nativeText(p.created_at), deviceName: nativeText(p.device_name),
    channels: count(p.channels), secrets: count(p.secrets), splitDirect: count(p.split_direct),
    keys: (Array.isArray(p.keys) ? p.keys : []).map((value) => { const k = nativeRecord(value); return { keyId: nativeText(k.key_id), fingerprint: nativeText(k.fingerprint), comment: nativeText(k.comment) } }),
    bundleVersion: counter(p.bundle_version), currentVersion: counter(p.current_version), wouldDowngrade: p.would_downgrade === true,
    bundleIdentity: nativeText(p.bundle_identity) || undefined, currentIdentity: nativeText(p.current_identity) || undefined,
    authorityMode: nativeText(p.authority_mode) || undefined, signerFingerprint: nativeText(p.signer_fingerprint) || undefined,
    targetSHA256: nativeText(p.target_sha256) || undefined, deliveryAnchorSHA256: nativeText(p.delivery_anchor_sha256) || undefined,
    deliveryValidUntil: nativeText(p.delivery_valid_until) || undefined,
    validUntil,
    pinPolicyVersion: policyVersion || undefined, pinPolicySHA256: policySHA || undefined,
    currentPinPolicySHA256: currentPolicySHA || undefined,
    pinPolicyValidUntil: nativeText(p.pin_policy_valid_until) || undefined,
    pinPolicyRequired: typeof p.pin_policy_required === 'boolean' ? p.pin_policy_required : undefined,
    bootstrapPolicySHA256: bootstrapPolicySHA || undefined,
  }
}

// Older native handlers may ignore new request fields. Require the enforced
// baseline echo before exposing a first-enrollment confirmation in this UI.
function requireBootstrapPolicyApproval(profile: OnboardingPreview, request: { mode: string; approved_root_sha256?: string; approved_pin_policy_sha256?: string }): OnboardingPreview {
  if (request.mode === 'enroll' || request.mode === 'epoch_enroll') {
    const root = request.approved_root_sha256, policy = request.approved_pin_policy_sha256
    if (!root || !policy || !/^[a-f0-9]{64}$/.test(root) || !/^[a-f0-9]{64}$/.test(policy) || profile.deliveryAnchorSHA256 !== root || profile.bootstrapPolicySHA256 !== policy || profile.pinPolicySHA256 !== policy) {
      throw new Error('Независимое подтверждение первого списка отзывов не поддержано или не совпало. Обновите клиент и проверьте оба отпечатка.')
    }
  }
  return profile
}

function buildPhaseState(): AppState {
  return {
    state: 'stopped',
    blockedReason: null,
    activeChannelId: null,
    activeChannel: null,
    proxyMode: null,
    probe: null,
    probeRunning: false,
    probeLastAt: null,
    error: buildPhaseError,
    coreReady: false,
    coreBlockMsg: 'Приложение запущено вне Wails (предпросмотр). Запустите exe.',
    nextRetryAt: '',
    retryAttempt: 0,
  }
}

// ---------- мапперы Go → контракт UI ----------

/** Missing, malformed or imprecise counts are unknown; known zero survives. */
function observedCounter(value: unknown): number | null {
  return typeof value === 'number' && Number.isSafeInteger(value) && value >= 0 ? value : null
}

function observedRate(value: unknown): number | null {
  return typeof value === 'number' && Number.isFinite(value) && value >= 0 ? value : null
}

function mapProbeComparison(value: unknown): ProbeComparison | undefined {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return undefined
  const p = nativeRecord(value)
  if (p.schema !== 1 || p.observation_point !== 'remote_https_response' || p.topology !== 'unknown' || p.path_scope !== 'unattested' || p.local_safety !== 'unknown' || p.violation_basis !== 'none') return undefined
  if (p.reference !== 'profile_expected' && p.reference !== 'protected_responses' && p.reference !== 'control_vs_protected') return undefined
  if (p.outcome !== 'match' && p.outcome !== 'mismatch' && p.outcome !== 'unknown') return undefined
  return { schema: 1, reference: p.reference, outcome: p.outcome, observationPoint: 'remote_https_response', topology: 'unknown', pathScope: 'unattested', localSafety: 'unknown', violationBasis: 'none' }
}

function mapProbeStep(value: unknown): ProbeStep {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return { name: 'Шаг диагностики', status: 'unavailable', detail: 'Приложение вернуло неполный шаг диагностики.' }
  }
  const p = nativeRecord(value)
  if (!nativeText(p.name)) {
    return { name: 'Шаг диагностики', status: 'unavailable', detail: 'Приложение вернуло шаг без имени проверки.' }
  }
  const allowed: readonly string[] = ['pending', 'running', 'pass', 'fail', 'skipped', 'unavailable', 'leak']
  const rawStatus = typeof p.status === 'string' && allowed.includes(p.status) ? p.status as ProbeStepStatus : 'unavailable'
  const requirement = p.requirement === undefined || p.requirement === '' || p.requirement === 'required' ? 'required' : p.requirement === 'informational' ? 'informational' : undefined
  if (!requirement) {
    return { name: nativeText(p.name), status: 'unavailable', detail: 'Приложение вернуло неподдерживаемую роль шага диагностики.' }
  }
  const step: ProbeStep = { name: nativeText(p.name), status: rawStatus, detail: nativeText(p.detail), comparison: mapProbeComparison(p.comparison), requirement }
  if (requirement === 'informational' && !isInformationalProbeStep(step)) {
    // Burn the rejected role too: a second normalization must not reinterpret
    // the newly unavailable status as a supported informational result.
    return { ...step, requirement: 'required', status: 'unavailable', detail: 'Информационная роль не соответствует шагу. Обязательная проверка не подтверждена.' }
  }
  if (p.comparison !== undefined && p.comparison !== null && !step.comparison && rawStatus !== 'pending' && rawStatus !== 'running' && rawStatus !== 'skipped') {
    return { ...step, status: 'unavailable', detail: 'Приложение вернуло неподдерживаемую схему сравнения IP.' }
  }
  return { ...step, status: probeStepStatus(step), detail: probeStepDetail(step) }
}

function mapProbeSteps(value: unknown): ProbeStep[] {
  if (value === null || value === undefined) return []
  if (!Array.isArray(value)) {
    return [{ name: 'Отчёт диагностики', status: 'unavailable', detail: 'Приложение вернуло некорректный список шагов.' }]
  }
  return value.map(mapProbeStep)
}

function mapProbeScope(value: unknown): ProbeScope | undefined {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return undefined
  const scope = nativeRecord(value)
  const applicationStatus = (value: unknown): value is ProbeScope['https'] =>
    value === 'observed' || value === 'incomplete' || value === 'unmeasured'
  if (scope.schema !== 1 || !applicationStatus(scope.https) || !applicationStatus(scope.doh) || typeof scope.historical !== 'boolean') return undefined
  if (scope.ipv4_tcp !== 'unmeasured' || scope.ipv4_udp !== 'unmeasured' || scope.ipv6_tcp !== 'unmeasured' || scope.ipv6_udp !== 'unmeasured' || scope.local_safety !== 'unmeasured') return undefined
  if (scope.historical && (scope.https === 'observed' || scope.doh === 'observed')) return undefined
  return {
    schema: 1, https: scope.https, doh: scope.doh, historical: scope.historical,
    ipv4TCP: 'unmeasured', ipv4UDP: 'unmeasured', ipv6TCP: 'unmeasured', ipv6UDP: 'unmeasured', localSafety: 'unmeasured',
  }
}

function nativeIdentity(part: unknown): part is string {
  return typeof part === 'string' && /^(0|[1-9][0-9]{0,19})$/.test(part) &&
    (part.length < 20 || part <= '18446744073709551615')
}

function mapDiagnosticContext(value: unknown): DiagnosticContext | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null
  const raw = nativeRecord(value)
  if (raw.schema !== 1 || !nativeIdentity(raw.revision) || raw.revision === '0' || !nativeIdentity(raw.intent) || !nativeIdentity(raw.path) || typeof raw.intentCurrent !== 'boolean' ||
    typeof raw.pathCurrent !== 'boolean' || typeof raw.closing !== 'boolean' || raw.path === '0' && raw.pathCurrent ||
    typeof raw.channel !== 'string' || raw.channel.length > 128 || !['', 'socks', 'tun'].includes(raw.mode as string) ||
    !['stopped', 'starting', 'running', 'stopping', 'reloading', 'error'].includes(raw.lifecycle as string) || typeof raw.ready !== 'boolean') return null
  return { schema: 1, revision: raw.revision, intent: raw.intent, path: raw.path, intentCurrent: raw.intentCurrent, pathCurrent: raw.pathCurrent, closing: raw.closing,
    channel: raw.channel, mode: raw.mode as DiagnosticContext['mode'], lifecycle: raw.lifecycle as DiagnosticContext['lifecycle'], ready: raw.ready }
}

function mapLifecycleActions(value: unknown, context: DiagnosticContext | null, lifecycle: unknown): LifecycleActions | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null
  const raw = nativeRecord(value)
  if (!context || raw.schema !== 1 || raw.intent !== context.intent || raw.lifecycle !== lifecycle || raw.lifecycle !== context.lifecycle ||
    typeof raw.startAllowed !== 'boolean' || typeof raw.stopAllowed !== 'boolean' || typeof raw.explicitStartRequired !== 'boolean') return null
  const block = (value: unknown): value is LifecycleActionBlock => typeof value === 'string' && LIFECYCLE_ACTION_BLOCKS.some(code => code === value)
  if ((raw.startAllowed ? raw.startBlockedBy !== '' : !block(raw.startBlockedBy)) ||
    (raw.stopAllowed ? raw.stopBlockedBy !== '' : !['restart_committed', 'protection_release_committed', 'closing'].includes(raw.stopBlockedBy as string)) ||
    raw.startAllowed && lifecycle !== 'stopped' && lifecycle !== 'error' || context.closing && (raw.startAllowed || raw.stopAllowed)) return null
  return {
    schema: 1, intent: context.intent, lifecycle: context.lifecycle,
    startAllowed: raw.startAllowed, startBlockedBy: raw.startAllowed ? null : raw.startBlockedBy as LifecycleActionBlock,
    stopAllowed: raw.stopAllowed, stopBlockedBy: raw.stopAllowed ? null : raw.stopBlockedBy as LifecycleActionBlock,
    explicitStartRequired: raw.explicitStartRequired,
  }
}

function mapBlockedReason(value: unknown): BlockedReason | null {
  if (value === '' || value === undefined || value === null) return null
  return BLOCKED_REASONS.find(code => code === value) ?? 'unrecognized'
}

function mapProcessMode(value: unknown, pid: unknown): ProcessModeFacts | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null
  const raw = nativeRecord(value)
  if (raw.schema !== 1 || typeof pid !== 'number' || !Number.isSafeInteger(pid) || pid <= 0 || pid > 0xffffffff || raw.pid !== pid ||
    raw.mode !== 'socks' && raw.mode !== 'tun' || typeof raw.tunAdmission !== 'boolean' || typeof raw.elevationAdmission !== 'boolean' ||
    raw.elevationAdmission && !raw.tunAdmission) return null
  return { schema: 1, pid, mode: raw.mode, tunAdmission: raw.tunAdmission, elevationAdmission: raw.elevationAdmission }
}

function mapState(s: GoAppState): AppState {
  const steps = mapProbeSteps(s.probe?.steps)
  const requiredSteps = steps.filter(step => !isInformationalProbeStep(step))
  const raw = nativeRecord(s), context = mapDiagnosticContext(raw.diagnosticContext)
  const rawProbe = s.probe && typeof s.probe === 'object' && !Array.isArray(s.probe) ? nativeRecord(s.probe) : undefined
  const scope = mapProbeScope(rawProbe?.scope)
  return {
    diagnosticContext: context,
    lifecycleActions: mapLifecycleActions(raw.lifecycleActions, context, raw.state),
    state: (s.state ?? 'stopped') as AppState['state'],
    blockedReason: mapBlockedReason(raw.blockedReason),
    activeChannelId: s.activeId || null,
    activeChannel:
      s.active ?
        {
          id: s.active.id,
          transport: s.active.transport,
          server: s.active.server,
          port: s.active.port,
          validation: s.active.validation as ChannelDescriptor['validation'],
          enabled: s.active.enabled,
        } as ChannelDescriptor
      : null,
    proxyMode: (s.proxyMode || null) as AppState['proxyMode'],
    probe:
      s.probe ?
        {
          ok: s.probe.ok === true && requiredSteps.length > 0 && requiredSteps.every(step => probeStepStatus(step) === 'pass'),
          steps,
          scope,
        }
      : null,
    probeRunning: s.probeRunning,
    probeLastAt: s.probeLastAt || null,
    error: s.error ?? '',
    coreReady: s.coreReady ?? false,
    coreBlockMsg: s.coreBlockMsg ?? '',
    nextRetryAt: s.nextRetryAt ?? '',
    retryAttempt: s.retryAttempt ?? 0,
  }
}

function mapMeta(m: GoMeta): SecretMeta {
  return {
    id: m.id,
    kind: m.kind as SecretMeta['kind'],
    title: m.title,
    hint: m.hint,
    backend: m.backend === 'file' || m.backend === 'dpapi' || m.backend === 'keyring' ? m.backend : null,
    storedValue: m.storedValue === '' ? null : m.storedValue === 'file' || m.storedValue === 'dpapi' || m.storedValue === 'keyring' ? m.storedValue : 'unknown',
    fingerprint: typeof m.fingerprint === 'string' && /^[a-f0-9]{16}$/i.test(m.fingerprint) ? m.fingerprint : '',
    createdAt: m.createdAt,
    updatedAt: m.updatedAt,
    lastVerifiedAt: m.lastVerifiedAt || null,
    verifyStatus: m.verifyStatus === 'ok' || m.verifyStatus === 'failed' ? m.verifyStatus : 'unverified',
    verifyError: m.verifyStatus === 'failed' ? 'Локальная проверка не подтвердила формат. Сверьте требования к типу значения.' : '',
    reveals: observedCounter(m.reveals),
  }
}

function mapTest(t: GoTest): TestResult {
  const steps = t.steps === null || t.steps === undefined ? undefined : mapProbeSteps(t.steps)
  return {
    id: t.id,
    ok: manualDiagnosticOutcome({ id: t.id, ok: t.ok, steps }),
    context: mapDiagnosticContext(nativeRecord(t).context),
    contextCurrent: typeof nativeRecord(t).contextCurrent === 'boolean' ? nativeRecord(t).contextCurrent as boolean : null,
    detail: t.detail,
    checkedAt: t.checkedAt || null,
    steps,
  }
}

// ---------- фасад ----------

export const api = {
  /** Root mounted receipt only; never a VPN Ready or trust verdict. */
  acknowledgeRendererMounted(): () => void {
    if (MOCK_ACTIVE || isBuildPhase()) return () => {}
    let active = true
    const off = EventsOn('app:renderer-challenge-v1', (token: unknown) => {
      if (active && typeof token === 'string' && token.length === 64 && /^[0-9a-f]{64}$/.test(token)) {
        EventsEmit('app:renderer-mounted-v1', token)
      }
    })
    return () => { active = false; off() }
  },

  async getState(): Promise<AppState> {
    if (isBuildPhase()) return buildPhaseState()
    return mapState(await App.GetState())
  },

  subscribe(onState: (s: AppState) => void, onLog: (l: unknown) => void): () => void {
    if (isBuildPhase()) {
      backendLog('warn', 'Подписка на события backend недоступна вне Wails')
      return () => {}
    }
    // EventsOn возвращает функцию отписки (wails v2).
    const offState = EventsOn('state', (s: GoAppState) => onState(mapState(s)))
    const offLog = EventsOn('log', (l: unknown) => onLog(l))
    return () => {
      offState()
      offLog()
    }
  },

  retireLifecycleRequests(): void { retireUIRequestInterest() },
  hasOrderedLifecycle(): boolean { return !isBuildPhase() && typeof uiLifecycleBindings.OpenUILifecycle === 'function' && typeof uiLifecycleBindings.RequestUILifecycle === 'function' },

  async start(interest: UIRequestInterest = { current: () => true }): Promise<UILifecycleReceipt> {
    return requestUILifecycle('start', '', interest)
  },
  async selectChannel(id: string, interest: UIRequestInterest = { current: () => true }): Promise<UILifecycleReceipt> {
    return requestUILifecycle('switch', id, interest)
  },
  async stop(interest: UIRequestInterest = { current: () => true }): Promise<UILifecycleReceipt | null> {
    if (!interest.current()) throw new Error(uiFailure.stop)
    if (!uiRequestSession || uiRequestSession.sequence === '18446744073709551615' || typeof uiLifecycleBindings.RequestUILifecycle !== 'function') {
      retireUIRequestInterest() // revoke a queued positive before the explicit legacy negative request
      await App.Stop()
      return null // no ordered/native receipt is fabricated for the legacy escape
    }
    const receipt = await requestUILifecycle('stop', '', interest)
    if (receipt.outcome === 'superseded' && interest.current()) {
      retireUIRequestInterest()
      await App.Stop() // original ordered request returned; explicit negative fallback only
      return null
    }
    return receipt
  },
  async enableTUN(interest: UIRequestInterest = { current: () => true }): Promise<UILifecycleReceipt> {
    return requestUILifecycle('mode_tun', '', interest)
  },
  async disableTUN(interest: UIRequestInterest = { current: () => true }): Promise<UILifecycleReceipt> {
    return requestUILifecycle('mode_proxy', '', interest)
  },
  async isTUNMode(): Promise<boolean> {
    if (isBuildPhase()) throw new Error(uiFailure.load)
    const value = await App.IsTUNMode()
    if (typeof value !== 'boolean') throw new Error(uiFailure.load)
    return value
  },
  async runProbe(): Promise<void> {
    await App.RunProbe()
  },
  async openLogsDir(): Promise<void> {
    await App.OpenLogsDir()
  },

  async listChannels(): Promise<ChannelDescriptor[] | null> {
    if (isBuildPhase()) return null
    const list = await App.ListChannels()
    return (list ?? []).map((c) => ({
      id: c.id,
      transport: c.transport,
      server: c.server,
      port: c.port,
      validation: c.validation as ChannelDescriptor['validation'],
      enabled: c.enabled,
    }))
  },

  async listTests(): Promise<TestResult[] | null> {
    if (isBuildPhase()) return null
    const list = await App.ListTests()
    return (list ?? []).map(mapTest)
  },
  async runTest(id: string): Promise<TestResult> {
    return mapTest(await App.RunTest(id))
  },

  async listSecrets(): Promise<SecretMeta[] | null> {
    if (isBuildPhase()) return null
    const list = await App.ListSecrets()
    return (list ?? []).map(mapMeta)
  },
  async saveSecret(id: string, value: string): Promise<SecretMeta> {
    return mapMeta(await App.SaveSecret(id, value))
  },
  async addSecret(kind: string, title: string, value: string, hint: string): Promise<SecretMeta> {
    return mapMeta(await App.AddSecret(kind, title, value, hint))
  },
  async deleteSecret(id: string): Promise<void> {
    await App.DeleteSecret(id)
  },
  async verifySecret(id: string): Promise<{ status: 'unverified' | 'ok' | 'failed'; checkedAt: string; error: string }> {
    const m = await App.VerifySecret(id)
    const status = m.verifyStatus === 'ok' || m.verifyStatus === 'failed' ? m.verifyStatus : 'unverified'
    return { status, checkedAt: m.lastVerifiedAt ?? '', error: status === 'failed' ? 'Формат не подтверждён; сверьте требования к типу значения.' : '' }
  },
  async testSecret(id: string): Promise<SecretTestReport> {
    const r = await App.TestSecret(id)
    const steps = mapProbeSteps(r.steps)
    return { ok: probeReportOutcome(r.ok, steps) === true, steps }
  },
  async revealSecret(id: string): Promise<{ value: string | null; error: string }> {
    const v = await App.RevealSecret(id)
    return { value: typeof v === 'string' ? v : null, error: typeof v === 'string' ? '' : 'Значение не получено.' }
  },

  async getNetworkFacts(): Promise<NetworkFacts | null> {
    if (isBuildPhase()) return null
    const f = await App.GetNetworkFacts()
    const raw = nativeRecord(f)
    let egress = raw.egress === undefined ? undefined : mapNetworkEgress(raw.egress)
    const address = (value: unknown): value is string => typeof value === 'string' && value.length > 0 && value.length <= 64 && !/[\s\x00-\x1f\x7f]/.test(value)
    if (egress && (egress.osControl.outcome === 'ok' && !address(f.publicIP) || egress.socks.outcome === 'ok' && !address(f.tunnelIP))) egress = null
    const controlAccepted = egress === undefined || egress?.osControl.outcome === 'ok'
    const socksAccepted = egress === undefined || egress?.socks.outcome === 'ok'
    return {
      connectionType: f.connectionType ?? '',
      ssid: f.ssid ?? '',
      localIP: f.localIP ?? '',
      isp: controlAccepted ? f.isp ?? '' : '',
      publicIP: controlAccepted ? f.publicIP ?? '' : '',
      tunnelIP: socksAccepted ? f.tunnelIP ?? '' : '',
      country: controlAccepted ? f.country ?? '' : '',
      countryOrigin: controlAccepted ? f.countryOrigin ?? '' : '',
      dnsViaTunnel: socksAccepted && f.dnsViaTunnel === true ? true : null,
      netClass: f.netClass ?? '',
      checkedAt: f.checkedAt ?? '',
      localRouteTarget: ['9.9.9.9', '2620:fe::fe'].includes(nativeText(nativeRecord(f).localRouteTarget)) ? nativeText(nativeRecord(f).localRouteTarget) : '',
      egress,
    }
  },

  async getRuntimeInfo(): Promise<RuntimeInfo | null> {
    if (isBuildPhase()) return null
    const r = await App.GetRuntimeInfo()
    return {
      version: r.version ?? '',
      os: r.os ?? '',
      arch: r.arch ?? '',
      pid: r.pid ?? 0,
      coreVersion: r.coreVersion ?? '',
      shellVersion: r.shellVersion ?? '',
      processMode: mapProcessMode(nativeRecord(r).processMode, r.pid),
    }
  },

  async getNetStats(): Promise<NetStats | null> {
    if (isBuildPhase()) return null
    const s = await App.GetNetStats()
    if (!s || typeof s.alias !== 'string' || !s.alias || s.alias.length > 512 || s.alias.includes('\0')) return null
    const raw = nativeRecord(s)
    let source: NetStats['source'] = 'legacy', adapterId: string | null = null, routeTarget: string | null = null
    if (raw.sourceSchema !== undefined) {
      const id = nativeText(raw.adapterId), target = nativeText(raw.routeTarget)
      if (raw.sourceSchema !== 1 || raw.source !== 'native_route_adapter' || !/^[1-9a-f][0-9a-f]{0,15}:[1-9]\d{0,9}$/.test(id) || !['9.9.9.9', '2620:fe::fe'].includes(target)) return null
      if (Number(id.split(':')[1]) > 0xffffffff) return null
      source = 'native_route_adapter'; adapterId = id; routeTarget = target
    }
    return {
      inOctets: observedCounter(s.inOctets),
      outOctets: observedCounter(s.outOctets),
      speedBps: observedCounter(s.speedBps),
      alias: s.alias ?? '',
      source, adapterId, routeTarget,
    }
  },

  async getAutostart(): Promise<boolean> {
    if (isBuildPhase()) throw new Error('Состояние автозапуска недоступно в предпросмотре.')
    const value = await App.GetAutostart()
    if (typeof value !== 'boolean') throw new Error('Состояние автозапуска не получено в поддерживаемом формате.')
    return value
  },

  async setAutostart(on: boolean): Promise<void> {
    if (isBuildPhase()) return
    await App.SetAutostart(on)
  },

  // Onboarding (V2-049/F8): экспорт/импорт бандла переноса устройства.
  async exportOnboardingQr(passphrase: string): Promise<{ transport: string; dataUrl: string }> {
    if (isBuildPhase() || typeof App.ExportOnboardingQR !== 'function') throw profileActionNotSent('transfer_export')
    const r = nativeRecord(await App.ExportOnboardingQR(passphrase))
    const transport = protectedTransferText(r.transport), dataUrl = r.dataUrl
    if (typeof dataUrl !== 'string' || !dataUrl.startsWith('data:image/png;base64,') || dataUrl.length <= 22 || dataUrl.length > 2 * 1024 * 1024) throw new Error(uiFailure.profileExport)
    return { transport, dataUrl }
  },

  async saveOnboardingFile(passphrase: string): Promise<string> {
    if (isBuildPhase() || typeof App.SaveOnboardingFile !== 'function') throw profileActionNotSent('transfer_save')
    return selectedFilePath(await App.SaveOnboardingFile(passphrase))
  },

  async loadOnboardingFile(): Promise<string> {
    if (isBuildPhase() || typeof App.LoadOnboardingFile !== 'function') throw profileActionNotSent('transfer_preview')
    const result: unknown = await App.LoadOnboardingFile()
    return result === '' ? '' : protectedTransferText(result)
  },

  async previewOnboarding(transport: string, passphrase: string): Promise<OnboardingPreview> {
    if (isBuildPhase() || typeof App.PreviewOnboardingString !== 'function') throw profileActionNotSent('transfer_preview')
    const p = await App.PreviewOnboardingString(transport, passphrase)
    return mapOnboarding(p)
  },

  async applyOnboarding(transport: string, passphrase: string, confirm: boolean): Promise<OnboardingPreview> {
    if (isBuildPhase() || typeof App.ApplyOnboardingString !== 'function') throw profileActionNotSent('transfer_apply')
    const p = await App.ApplyOnboardingString(transport, passphrase, confirm)
    return mapOnboarding(p)
  },

  async previewPinPolicySource(request: PinPolicySourceRequest): Promise<PinPolicySourcePreview> {
    if (!hasPinPolicySource() || !publicSource.PreviewPinPolicySource) throw profileActionNotSent('policy_preview')
    return mapPinPolicySource(await publicSource.PreviewPinPolicySource(request), request.request_id, request.source.route)
  },
  async previewPinPolicyFile(requestId: string): Promise<PinPolicySourcePreview | null> {
    if (!hasPinPolicySource() || !publicSource.PreviewPinPolicyFile) throw profileActionNotSent('policy_preview')
    const value = await publicSource.PreviewPinPolicyFile(requestId)
    return value === null ? null : mapPinPolicySource(value, requestId, 'file')
  },
  async applyPinPolicySource(token: string): Promise<PinPolicyPreview> {
    if (!hasPinPolicySource() || !publicSource.ApplyPinPolicySource) throw profileActionNotSent('policy_apply')
    return mapPinPolicy(await publicSource.ApplyPinPolicySource(token, true))
  },
  async cancelPinPolicySource(requestId: string): Promise<boolean> {
    if (!hasPinPolicySource() || !publicSource.CancelPublicProfileSource) return false
    return publicSource.CancelPublicProfileSource(requestId)
  },
  async previewPublicProfileSource(request: PublicProfileSourceRequest): Promise<PublicProfileSourcePreview> {
    if (!hasPublicProfileSource() || !publicSource.PreviewPublicProfileSource) throw profileActionNotSent('profile_preview')
    const p = nativeRecord(await publicSource.PreviewPublicProfileSource(request))
    const token = nativeText(p.token), route = nativeText(p.route)
    if (!/^[a-f0-9]{64}$/.test(token) || route !== request.source.route || !['protected', 'bootstrap', 'local'].includes(route) || nativeText(p.request_id) !== request.request_id || !/^[a-f0-9]{64}$/.test(nativeText(p.release_id)) || !/^[a-f0-9]{64}$/.test(nativeText(p.package_sha256))) throw new Error('Превью не соответствует текущему запросу.')
    const purpose = nativeText(p.purpose) || 'import'
    if (!['import', 'export'].includes(purpose) || purpose !== (request.purpose || 'import')) throw new Error('Назначение превью не совпадает с выбранным действием.')
    const profile = mapOnboarding(p.profile), expiresAt = policyTime(p.expires_at)
    if (purpose === 'import') requireBootstrapPolicyApproval(profile, request.import)
    const destinationRootSHA256 = nativeText(p.destination_root_sha256), destinationPinPolicySHA256 = nativeText(p.destination_pin_policy_sha256)
    if (purpose === 'export' && (!/^[a-f0-9]{64}$/.test(destinationRootSHA256) || !/^[a-f0-9]{64}$/.test(destinationPinPolicySHA256) || destinationPinPolicySHA256 !== profile.pinPolicySHA256)) throw new Error('Подтверждение первого списка отзывов для переноса недоступно. Обновите клиент и пакет издателя.')
    if (!profile.validUntil || Date.parse(expiresAt) > Date.parse(profile.validUntil)) throw new Error('Общий срок подтверждения профиля недоступен в этой версии приложения.')
    return { token, requestId: request.request_id, route: route as PublicProfileRoute, releaseId: nativeText(p.release_id), packageSHA256: nativeText(p.package_sha256), expiresAt, profile, purpose: purpose as 'import' | 'export', destinationRootSHA256: destinationRootSHA256 || undefined, destinationPinPolicySHA256: destinationPinPolicySHA256 || undefined }
  },
  async applyPublicProfileSource(token: string, target: string): Promise<OnboardingPreview> {
    if (!hasPublicProfileSource() || !publicSource.ApplyPublicProfileSource) throw profileActionNotSent('profile_apply')
    return mapOnboarding(await publicSource.ApplyPublicProfileSource(token, target, true))
  },
  async cancelPublicProfileSource(requestId: string): Promise<boolean> {
    if (!hasPublicProfileSource() || !publicSource.CancelPublicProfileSource) return false
    return publicSource.CancelPublicProfileSource(requestId)
  },
  async exportPublicProfileSource(token: string, passphrase: string): Promise<DeliveredExportResult> {
    if (!hasDeliveredExport() || !publicSource.ExportPublicProfileSource) throw profileActionNotSent('transfer_export')
    const p = nativeRecord(await publicSource.ExportPublicProfileSource(token, passphrase))
    const destinationMode = nativeText(p.destination_mode)
    const transport = protectedTransferText(p.transport)
    const destinationPinPolicySHA256 = nativeText(p.destination_pin_policy_sha256)
    if (!/^[a-f0-9]{64}$/.test(nativeText(p.destination_root_sha256)) || !/^[a-f0-9]{64}$/.test(destinationPinPolicySHA256) || !/^[a-f0-9]{64}$/.test(nativeText(p.target_sha256)) || !['enroll', 'epoch_enroll'].includes(destinationMode) || typeof p.secrets !== 'number' || !Number.isSafeInteger(p.secrets) || p.secrets < 0) throw new Error('Зашифрованный перенос не получен полностью.')
    return { transport, profileIdentity: nativeText(p.profile_identity), destinationRootSHA256: nativeText(p.destination_root_sha256), destinationPinPolicySHA256, targetSHA256: nativeText(p.target_sha256), secrets: p.secrets, destinationMode: destinationMode as 'enroll' | 'epoch_enroll' }
  },
  async saveProtectedProfileTransfer(transport: string): Promise<string> {
    if (isBuildPhase() || typeof publicSource.SaveProtectedProfileTransfer !== 'function') throw profileActionNotSent('transfer_save')
    return selectedFilePath(await publicSource.SaveProtectedProfileTransfer(transport))
  },
  async previewProtectedProfileTransfer(request: ProtectedProfileTransferRequest): Promise<OnboardingPreview> {
    if (!hasProtectedProfileTransfer() || !publicSource.PreviewProtectedProfileTransfer) throw profileActionNotSent('transfer_preview')
    return requireBootstrapPolicyApproval(mapOnboarding(await publicSource.PreviewProtectedProfileTransfer(request)), request)
  },
  async applyProtectedProfileTransfer(request: ProtectedProfileTransferRequest): Promise<OnboardingPreview> {
    if (!hasProtectedProfileTransfer() || !publicSource.ApplyProtectedProfileTransfer) throw profileActionNotSent('transfer_apply')
    return requireBootstrapPolicyApproval(mapOnboarding(await publicSource.ApplyProtectedProfileTransfer(request, true)), request)
  },

  // Обновления приложения (V2-050/F15): подписанный манифест + payload.
  async pickUpdateDir(): Promise<string> {
    if (isBuildPhase() || typeof App.PickUpdateDir !== 'function') throw new Error(uiFailure.lifecycleUnavailable)
    const directory: unknown = await App.PickUpdateDir()
    if (typeof directory !== 'string' || directory.length > 32767 || directory.includes('\0')) throw new Error(uiFailure.lifecycleUnavailable)
    return directory
  },

  async checkUpdate(dir: string, interest: UIRequestInterest = { current: () => true }): Promise<UpdatePreview> {
    if (isBuildPhase()) throw new Error(uiFailure.lifecycleUnavailable)
    if (!interest.current()) throw new Error(uiFailure.lifecycleUnavailable)
    const session = await uiSession() // preview and apply belong to the same ordered renderer
    if (!interest.current() || uiRequestSession !== session) throw new Error(uiFailure.lifecycleUnavailable)
    const p = nativeRecord(await App.CheckUpdate(dir))
    if (!interest.current() || uiRequestSession !== session) throw new Error(uiFailure.lifecycleUnavailable)
    const text = (value: unknown, limit: number): string => typeof value === 'string' && value.length <= limit ? value : ''
    const hash = (value: unknown): value is string => typeof value === 'string' && /^[0-9a-f]{64}$/.test(value)
    const size = typeof p.size === 'number' && Number.isSafeInteger(p.size) && p.size > 0 && p.size <= 1024 * 1024 * 1024 ? p.size : null
    const version = text(p.version, 128), currentVersion = text(p.current_version, 128)
    const expiresAt = text(p.approval_expires_at, 64)
    const approval = p.schema === 1 && p.ok === true && hash(p.approval_token) && hash(p.manifest_sha256) && nativeIdentity(p.approval_intent) &&
      expiresAt !== '' && Number.isFinite(Date.parse(expiresAt)) && version !== '' && currentVersion !== '' && size !== null && hash(p.sha256)
      ? { token: p.approval_token, intent: p.approval_intent, manifestSHA256: p.manifest_sha256, expiresAt } : null
    const reasons: Record<string, string> = {
      invalid_directory: 'Каталог пакета не принят. Выберите каталог заново.',
      floor_unavailable: 'История принятых версий недоступна. Требуется восстановление приложения.',
      manifest_or_authority: 'Подпись, доставка или источник пакета не подтверждены.',
      version_policy: 'Версия пакета не проходит проверку обновления и защиту от отката.',
      installer_unavailable: 'Установка этого формата в текущей сборке ещё недоступна.',
      payload_mismatch: 'Содержимое пакета не соответствует подписанному манифесту.',
      source_changed: 'Состояние приложения изменилось. Обновите сведения и проверьте пакет заново.',
      approval_unavailable: 'Подтверждение конкретного пакета не получено. Нужна новая проверка.',
    }
    const reasonCode = text(p.reason_code, 64)
    const reason = Object.prototype.hasOwnProperty.call(reasons, reasonCode) ? reasons[reasonCode]
      : 'Проверка пакета не дала поддерживаемого подтверждения. Обновите комплект приложения и повторите выбор.'
    return {
      currentVersion, version, notes: text(p.notes, 16384), releasedAt: text(p.released_at, 64), size,
      sha256: hash(p.sha256) ? p.sha256 : '', keyId: text(p.key_id, 128), floor: text(p.floor, 128),
      ok: approval !== null, approval,
      reason: approval ? '' : reason,
    }
  },

  async applyUpdate(token: string, interest: UIRequestInterest): Promise<UILifecycleReceipt> {
    if (!/^[0-9a-f]{64}$/.test(token)) throw new Error(uiFailure.lifecycleUnavailable)
    return requestUILifecycle('update', '', interest, token) // no legacy positive fallback
  },

  async getSplitDirect(): Promise<string[] | null> {
    if (isBuildPhase()) return null
    const list = await App.GetSplitDirect()
    return (list ?? []).map((x) => String(x))
  },

  async getMetrics(): Promise<MetricsView | null> {
    if (isBuildPhase()) return null
    const m = await App.GetMetrics()
    if (!m) return null
    // Runtime additions cross the existing native-record boundary; generated
    // Wails sources stay untouched until generation is explicitly allowed.
    const raw = nativeRecord(m)
    const context = mapDiagnosticContext(raw.context)
    const collector = nativeIdentity(raw.collector) && raw.collector !== '0' ? raw.collector : undefined
    const available = m.available === true && !!context && !!collector
    return {
      collector, context, available,
      note: m.available === true && !available ? 'Снимок не содержит поддерживаемой привязки к сессии сборщика.' : nativeText(m.note),
      rateUp: available ? observedRate(m.rateUp) : null,
      rateDown: available ? observedRate(m.rateDown) : null,
      sessionUp: available ? observedCounter(m.sessionUp) : null,
      sessionDown: available ? observedCounter(m.sessionDown) : null,
      at: available ? nativeText(m.at) : '',
      connections: (available && Array.isArray(m.connections) ? m.connections.slice(0, 20) : []).map((c) => ({
        host: c.host ?? '',
        network: c.network ?? '',
        chain: c.chain ?? '',
        process: c.process ?? '',
        up: observedCounter(c.up),
        down: observedCounter(c.down),
        since: c.since ?? '',
      })),
    }
  },

  async getDeliveryProfile(): Promise<DeliveryProfile | null> {
    if (isBuildPhase()) return null
    return mapDeliveryProfile(await App.GetDeliveryProfile())
  },

  async getFailoverStatus(): Promise<FailoverStatus | null> {
    if (isBuildPhase()) return null
    const f = await App.GetFailoverStatus()
    if (!f || typeof f.enabled !== 'boolean') return null
    return {
      enabled: !!f.enabled,
      state: f.state ?? '',
      switches: observedCounter(f.switches),
      lastError: f.lastError ?? '',
      nextCheckIn: observedCounter(f.nextCheckIn),
      smState: f.smState ?? '',
      smSince: f.smSince ?? '',
      smClass: f.smClass ?? '',
    }
  },

  async getChannelHealth(): Promise<ChannelHealth | null> {
    if (isBuildPhase()) return null
    const h = await App.GetChannelHealth()
    return {
      epoch: observedCounter(h.epoch),
      started: h.started ?? '',
      netClass: h.netClass ?? '',
      now: h.now ?? '',
      channels: (h.channels ?? []).map(
        (e): ChannelHealthEntry => ({
          id: e.id ?? '',
          transport: e.transport ?? '',
          ok: observedCounter(e.ok),
          fail: observedCounter(e.fail),
          lastOkAt: e.lastOkAt ?? '',
          lastFailAt: e.lastFailAt ?? '',
          lastClass: e.lastClass ?? '',
          verdict: (e.verdict as ChannelHealthEntry['verdict']) ?? 'no_data',
          d2State: (e.d2State as ChannelHealthEntry['d2State']) ?? '',
          d2Note: e.d2Note ?? '',
        }),
      ),
    }
  },

  async getIncidents(): Promise<IncidentCluster[] | null> {
    if (isBuildPhase()) return null
    try {
      const r = await App.GetIncidents()
      return (r ?? []).map((c) => ({
        since: c.since ?? '',
        until: c.until ?? '',
        channels: c.channels ?? [],
        count: observedCounter(c.count),
        classes: c.classes ?? [],
        netClasses: c.netClasses ?? [],
        isps: c.isps ?? [],
        isWave: !!c.isWave,
        clusterId: c.clusterId ?? '',
      }))
    } catch {
      // Эпоха 0 (сеть не наблюдалась) — бэкенд честно отказывается: «нет данных».
      return null
    }
  },

  /** V2-074 (аудит C5/D16): матрица независимости каналов по failure domains.
   *  null = нет данных (build-phase / дескрипторы нечитаемы) — НЕ «всё
   *  независимо»: UI обязан показать честное «нет данных». */
  async getChannelIndependence(): Promise<ChannelIndependenceInfo[] | null> {
    if (isBuildPhase()) return null
    const m = await App.GetChannelIndependence()
    if (!m) return null
    return m.map((row) => ({
      a: row.a ?? '',
      b: row.b ?? '',
      score: observedRate(row.score),
      sharedFdoms: row.shared_fdoms ?? '',
      belowLimit: !!row.below_limit,
    }))
  },

  async runAdaptiveCheck(): Promise<AdaptiveStatus | null> {
    if (isBuildPhase()) return null
    const r = await App.RunAdaptiveCheck()
    return mapAdaptive(r)
  },

  async getAdaptiveStatus(): Promise<AdaptiveStatus | null> {
    if (isBuildPhase()) return null
    const r = await App.GetAdaptiveStatus()
    return mapAdaptive(r)
  },
}

function mapAdaptive(r: import('../../wailsjs/go/models').main.AdaptiveStatus): AdaptiveStatus {
  return {
    sites: (r.sites ?? []).map(
      (s): SiteStatus => ({
        domain: s.domain ?? '',
        code: observedCounter(s.code),
        ok: !!s.ok,
        challenge: !!s.challenge,
        detail: s.detail ?? '',
        checkedAt: s.checkedAt ?? '',
      }),
    ),
    warpDomains: r.warpDomains ?? [],
    note: r.note ?? '',
  }
}

function mapNetguardLog(value: unknown): NetGuardLogObservation | null {
  const log = nativeRecord(value)
  const state = log.state
  if (log.schema !== 1 || !['observed', 'missing', 'busy', 'unavailable'].includes(nativeText(state))) return null
  if (log.byteLimit !== 65792 || typeof log.bytesRead !== 'number' || !Number.isSafeInteger(log.bytesRead) || typeof log.rows !== 'number' || !Number.isSafeInteger(log.rows)) return null
  const bytesRead = log.bytesRead, rows = log.rows
  if (bytesRead < 0 || bytesRead > 65792 || rows < 0 || rows > 256 || typeof log.truncated !== 'boolean' || typeof log.incomplete !== 'boolean') return null
  if (state !== 'observed' && (bytesRead !== 0 || rows !== 0 || log.truncated || log.incomplete)) return null
  return { schema: 1, state: state as NetGuardLogObservation['state'], bytesRead, byteLimit: 65792, rows, truncated: log.truncated, incomplete: log.incomplete }
}

function mapNetguardLoopbacks(value: unknown): NetGuardLoopbackObservation[] | null {
  if (!Array.isArray(value) || value.length < 1 || value.length > 2) return null
  const facts: NetGuardLoopbackObservation[] = []
  for (const entry of value) {
    const fact = nativeRecord(entry)
    if (fact.schema !== 1 || typeof fact.port !== 'number' || !Number.isSafeInteger(fact.port) || fact.port < 1 || fact.port > 65535) return null
    if (!['reachable', 'refused', 'timeout', 'canceled', 'busy', 'unavailable'].includes(nativeText(fact.outcome)) || facts.some(previous => previous.port === fact.port)) return null
    facts.push({ schema: 1, port: fact.port, outcome: fact.outcome as NetGuardLoopbackObservation['outcome'] })
  }
  return facts.some(fact => fact.port === 1080) ? facts : null
}

function netguardLogText(value: unknown, limit: number): string {
  return typeof value === 'string' && value.length <= limit ? value : ''
}

/** Read-only scheduler/proxy observations and optional bounded log scope. */
export async function netGuardStatus(): Promise<NetGuardStatus | null> {
  if (isBuildPhase()) return null
  const s = await App.NetGuardStatus()
  return {
    tasks: (s.tasks ?? []).map((t) => ({
      name: t.name ?? '',
      status: (t.status || 'unknown') as NetGuardTaskState,
      lastRun: t.lastRun ?? '',
      nextRun: t.nextRun ?? '',
      lastCode: t.lastCode ?? '',
    })),
    proxy: {
      enabled: s.proxy?.enabled ?? false,
      server: s.proxy?.server ?? '',
      port: s.proxy?.port ?? 0,
      listenerAlive: s.proxy?.listenerAlive ?? false,
      stale: s.proxy?.stale ?? false,
    },
    socksAlive: s.socksAlive ?? false,
    events: (s.events ?? []).slice(-12).map((e) => ({
      time: netguardLogText(e.time, 64),
      kind: (['HEALED', 'HEALING', 'PROBLEM', 'WEEKLY'].includes(e.kind) ? e.kind : 'unknown') as NetGuardStatus['events'][number]['kind'],
      text: netguardLogText(e.text, 4096) || 'Текст сообщения недоступен.',
    })),
    lastRun: netguardLogText(s.lastRun, 4096),
    dohDisabled: s.dohDisabled ?? false,
    log: mapNetguardLog(nativeRecord(s).log),
    loopbacks: mapNetguardLoopbacks(nativeRecord(s).loopbacks),
  }
}

/** Legacy API: without an original persistent receipt recovery is unavailable.
 * A false response alone is not proof that Windows needs no recovery. */
export async function runProxyHeal(): Promise<boolean | null> {
  if (isBuildPhase()) return null
  return await App.RunProxyHeal()
}

type ProxyRecoveryBindings = {
  PrepareProxyRecovery?: (request: { requestId: string; intent: string; previousRequestId: string }) => Promise<unknown>
  ConfirmProxyRecovery?: (request: { requestId: string; intent: string; token: string; scope: string }) => Promise<unknown>
  CancelProxyRecovery?: (request: { requestId: string; previousRequestId: string }) => Promise<unknown>
}
const proxyRecoveryBindings: ProxyRecoveryBindings = {
  get PrepareProxyRecovery() { return Reflect.get(App, 'PrepareProxyRecovery') as ProxyRecoveryBindings['PrepareProxyRecovery'] },
  get ConfirmProxyRecovery() { return Reflect.get(App, 'ConfirmProxyRecovery') as ProxyRecoveryBindings['ConfirmProxyRecovery'] },
  get CancelProxyRecovery() { return Reflect.get(App, 'CancelProxyRecovery') as ProxyRecoveryBindings['CancelProxyRecovery'] },
}
const proxyRecoveryUnavailable = 'Ручное восстановление недоступно в этой версии приложения.'
export function hasProxyRecovery(): boolean {
  return !MOCK_ACTIVE && !isBuildPhase() && typeof proxyRecoveryBindings.PrepareProxyRecovery === 'function' &&
    typeof proxyRecoveryBindings.ConfirmProxyRecovery === 'function' && typeof proxyRecoveryBindings.CancelProxyRecovery === 'function'
}
function proxyRecoveryEnvelope(value: unknown, requestId: string, intent: string): Record<string, unknown> {
  const item = nativeRecord(value)
  if (item.schema !== 1 || item.requestId !== requestId || item.intent !== intent || !/^[a-f0-9]{32}$/.test(requestId) ||
    !/^(0|[1-9][0-9]{0,19})$/.test(intent) || intent.length === 20 && intent > '18446744073709551615') {
    throw new Error('Ответ восстановления не соответствует текущему запросу.')
  }
  return item
}
function validateProxyRecoveryInterest(requestId: string, intent: string): void {
  if (!/^[a-f0-9]{32}$/.test(requestId) || !/^(0|[1-9][0-9]{0,19})$/.test(intent) ||
    intent.length === 20 && intent > '18446744073709551615') throw new Error('Запрос восстановления устарел.')
}
function recoveryValueCount(value: unknown): number {
  if (typeof value !== 'number' || !Number.isInteger(value) || value < 0 || value > 3) throw new Error('Результат восстановления неполон.')
  return value
}
export async function prepareProxyRecovery(requestId: string, intent: string, previousRequestId = ''): Promise<ProxyRecoveryPreview> {
  validateProxyRecoveryInterest(requestId, intent)
  if (previousRequestId !== '' && (!/^[a-f0-9]{32}$/.test(previousRequestId) || previousRequestId === requestId)) throw new Error('Исходный запрос восстановления неизвестен.')
  const method = proxyRecoveryBindings.PrepareProxyRecovery
  if (!hasProxyRecovery() || !method) throw new Error(proxyRecoveryUnavailable)
  const item = proxyRecoveryEnvelope(await method({ requestId, intent, previousRequestId }), requestId, intent), preview = nativeRecord(item.preview)
  if (typeof preview.token !== 'string' || !/^[a-f0-9]{64}$/.test(preview.token) || preview.scope !== 'restore_recorded_proxy_values' ||
    typeof preview.expiresAt !== 'string' || preview.expiresAt.length > 64 || !Number.isFinite(Date.parse(preview.expiresAt)) ||
    typeof preview.resume !== 'boolean' || typeof preview.resumeReason !== 'string' ||
    (preview.resume ? preview.priorProcess !== 'retained_owner' || !['unchanged_write', 'notification_failed', 'cancelled_before_intent'].includes(preview.resumeReason) :
      preview.resumeReason !== '' || typeof preview.priorProcess !== 'string' || !['unobserved', 'observed_exited', 'pid_changed'].includes(preview.priorProcess)) ||
    previousRequestId !== '' && !preview.resume) throw new Error('Подтверждение восстановления неполно.')
  return { schema: 1, requestId, intent, preview: { token: preview.token, scope: preview.scope, expiresAt: preview.expiresAt,
    recordedValues: recoveryValueCount(preview.recordedValues), priorProcess: preview.priorProcess as ProxyRecoveryPreview['preview']['priorProcess'],
    resume: preview.resume, resumeReason: preview.resumeReason as ProxyRecoveryPreview['preview']['resumeReason'] } }
}
export async function confirmProxyRecovery(preview: ProxyRecoveryPreview): Promise<ProxyRecoveryResult> {
  validateProxyRecoveryInterest(preview.requestId, preview.intent)
  if (!/^[a-f0-9]{64}$/.test(preview.preview.token) || preview.preview.scope !== 'restore_recorded_proxy_values') throw new Error('Подтверждение восстановления неполно.')
  const method = proxyRecoveryBindings.ConfirmProxyRecovery
  if (!hasProxyRecovery() || !method) throw new Error(proxyRecoveryUnavailable)
  const item = proxyRecoveryEnvelope(await method({ requestId: preview.requestId, intent: preview.intent, token: preview.preview.token, scope: preview.preview.scope }), preview.requestId, preview.intent)
  const result = nativeRecord(item.result)
  if (typeof result.outcome !== 'string' || !['restored', 'not_written', 'foreign_preserved'].includes(result.outcome)) throw new Error('Исход восстановления неизвестен.')
  const count = recoveryValueCount(result.restoredValues)
  if (result.outcome === 'not_written' && count !== 0) throw new Error('Результат восстановления противоречив.')
  return { schema: 1, requestId: preview.requestId, intent: preview.intent, result: { outcome: result.outcome as ProxyRecoveryResult['result']['outcome'], restoredValues: count } }
}
export async function cancelProxyRecovery(requestId: string, previousRequestId = ''): Promise<ProxyRecoveryCancellation> {
  const method = proxyRecoveryBindings.CancelProxyRecovery
  if (!hasProxyRecovery() || !method || !/^[a-f0-9]{32}$/.test(requestId)) throw new Error(proxyRecoveryUnavailable)
  if (previousRequestId !== '' && (!/^[a-f0-9]{32}$/.test(previousRequestId) || previousRequestId === requestId)) throw new Error('Исходный запрос восстановления неизвестен.')
  const item = nativeRecord(await method({ requestId, previousRequestId }))
  if (item.schema !== 1 || item.requestId !== requestId || typeof item.ownerRequestId !== 'string' || !/^[a-f0-9]{32}$/.test(item.ownerRequestId) ||
    item.ownerRequestId !== requestId && item.ownerRequestId !== previousRequestId ||
    typeof item.outcome !== 'string' || !['released', 'transaction_held', 'blocked'].includes(item.outcome)) throw new Error('Завершение запроса восстановления не подтверждено.')
  return { schema: 1, requestId, ownerRequestId: item.ownerRequestId, outcome: item.outcome as ProxyRecoveryCancellation['outcome'] }
}
