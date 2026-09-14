/**
 * Типизированный фасад Wails-биндингов (wailsjs/go/main/App).
 *
 * В Wails-сборке вызовы идут в Go (window.go.*). В браузерном предпросмотре
 * (npm run dev / static preview) биндингов нет — фасад честно сообщает об этом,
 * UI показывает «нет данных», ничего не выдумывая (AGENTS.md §3.6).
 */
import * as App from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import type {
  AdaptiveStatus,
  MetricsView,
  AppState,
  ChannelDescriptor,
  DeliveryProfile,
  FailoverStatus,
  NetGuardStatus,
  NetGuardTaskState,
  NetStats,
  NetworkFacts,
  SecretMeta,
  SecretTestReport,
  SiteStatus,
  TestResult,
} from './contract'
import { backendLog } from '../composables/toast'

// Формы из сгенерированных моделей (структурно идентичны контракту).
type GoAppState = import('../../wailsjs/go/models').main.AppState
type GoMeta = import('../../wailsjs/go/models').secretvault.Meta
type GoTest = import('../../wailsjs/go/models').main.TestResultView

/** True, если Wails-биндинги недоступны (браузерный предпросмотр). */
export function isBuildPhase(): boolean {
  return typeof window === 'undefined' || (window as never as { go?: unknown }).go === undefined
}

export const buildPhaseError =
  'Wails-биндинги недоступны: приложение запущено вне Wails (браузерный предпросмотр). Запустите exe из windows/build/bin.'

function buildPhaseState(): AppState & { coreReady: boolean; coreBlockMsg: string } {
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
  }
}

// ---------- мапперы Go → контракт UI ----------

function mapState(s: GoAppState): AppState {
  return {
    state: (s.state ?? 'stopped') as AppState['state'],
    blockedReason: (s.blockedReason || null) as AppState['blockedReason'],
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
        { ok: s.probe.ok, steps: s.probe.steps.map((st) => ({ name: st.name, status: st.status as 'pending', detail: st.detail })) }
      : null,
    probeRunning: s.probeRunning,
    probeLastAt: s.probeLastAt || null,
    error: s.error ?? '',
    coreReady: s.coreReady,
    coreBlockMsg: s.coreBlockMsg ?? '',
  } as AppState & { coreReady: boolean; coreBlockMsg: string }
}

function mapMeta(m: GoMeta): SecretMeta {
  return {
    id: m.id,
    kind: m.kind as SecretMeta['kind'],
    title: m.title,
    hint: m.hint,
    backend: 'dpapi' as const,
    storedValue: (m.storedValue || null) as SecretMeta['storedValue'],
    fingerprint: m.fingerprint,
    createdAt: m.createdAt,
    updatedAt: m.updatedAt,
    lastVerifiedAt: m.lastVerifiedAt || null,
    verifyStatus: (m.verifyStatus || 'unverified') as SecretMeta['verifyStatus'],
    verifyError: m.verifyError ?? '',
    reveals: m.reveals ?? 0,
  }
}

function mapTest(t: GoTest): TestResult {
  return {
    id: t.id,
    ok: t.ok === null || t.ok === undefined ? null : t.ok,
    detail: t.detail,
    checkedAt: t.checkedAt || null,
    steps: t.steps?.map((st) => ({ name: st.name, status: st.status as 'pending', detail: st.detail })),
  }
}

// ---------- фасад ----------

export const api = {
  async getState(): Promise<AppState & { coreReady?: boolean; coreBlockMsg?: string }> {
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

  async start(): Promise<void> {
    await App.Start()
  },
  async selectChannel(id: string): Promise<void> {
    await App.SelectChannel(id)
  },
  async stop(): Promise<void> {
    await App.Stop()
  },
  async enableTUN(): Promise<void> {
    await App.EnableTUN()
  },
  async disableTUN(): Promise<void> {
    await App.DisableTUN()
  },
  async isTUNMode(): Promise<boolean> {
    return await App.IsTUNMode()
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
    return list.map((c) => ({
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
    return list.map(mapTest)
  },
  async runTest(id: string): Promise<TestResult> {
    return mapTest(await App.RunTest(id))
  },

  async listSecrets(): Promise<SecretMeta[] | null> {
    if (isBuildPhase()) return null
    const list = await App.ListSecrets()
    return list.map(mapMeta)
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
    return { status: (m.verifyStatus || 'unverified') as 'unverified' | 'ok' | 'failed', checkedAt: m.lastVerifiedAt ?? '', error: m.verifyError ?? '' }
  },
  async testSecret(id: string): Promise<SecretTestReport> {
    const r = await App.TestSecret(id)
    return { ok: r.ok, steps: r.steps.map((st) => ({ name: st.name, status: st.status as 'pending', detail: st.detail })) }
  },
  async revealSecret(id: string): Promise<{ value: string | null; error: string }> {
    const v = await App.RevealSecret(id)
    return { value: v, error: v === null ? 'значение не задано' : '' }
  },

  async getNetworkFacts(): Promise<NetworkFacts | null> {
    if (isBuildPhase()) return null
    const f = await App.GetNetworkFacts()
    return {
      connectionType: f.connectionType ?? '',
      ssid: f.ssid ?? '',
      localIP: f.localIP ?? '',
      isp: f.isp ?? '',
      publicIP: f.publicIP ?? '',
      country: f.country ?? '',
      countryOrigin: f.countryOrigin ?? '',
      dnsViaTunnel: !!f.dnsViaTunnel,
      checkedAt: f.checkedAt ?? '',
    }
  },

  async getNetStats(): Promise<NetStats | null> {
    if (isBuildPhase()) return null
    const s = await App.GetNetStats()
    return {
      inOctets: s.inOctets ?? 0,
      outOctets: s.outOctets ?? 0,
      speedBps: s.speedBps ?? 0,
      alias: s.alias ?? '',
    }
  },

  async getAutostart(): Promise<boolean> {
    if (isBuildPhase()) return false
    return await App.GetAutostart()
  },

  async setAutostart(on: boolean): Promise<void> {
    if (isBuildPhase()) return
    await App.SetAutostart(on)
  },

  async getSplitDirect(): Promise<string[]> {
    if (isBuildPhase()) return []
    const list = await App.GetSplitDirect()
    return (list ?? []).map((x) => String(x))
  },

  async getMetrics(): Promise<MetricsView | null> {
    if (isBuildPhase()) return null
    const m = await App.GetMetrics()
    return {
      available: !!m.available,
      note: m.note ?? '',
      rateUp: m.rateUp ?? 0,
      rateDown: m.rateDown ?? 0,
      sessionUp: m.sessionUp ?? 0,
      sessionDown: m.sessionDown ?? 0,
      at: m.at ?? '',
      connections: (m.connections ?? []).map((c) => ({
        host: c.host ?? '',
        network: c.network ?? '',
        chain: c.chain ?? '',
        process: c.process ?? '',
        up: c.up ?? 0,
        down: c.down ?? 0,
        since: c.since ?? '',
      })),
    }
  },

  async getDeliveryProfile(): Promise<DeliveryProfile | null> {
    if (isBuildPhase()) return null
    const p = await App.GetDeliveryProfile()
    return {
      present: !!p.present,
      version: p.version ?? 0,
      keyId: p.keyId ?? '',
      expiresAt: p.expiresAt ?? '',
      trustedKeys: p.trustedKeys ?? 0,
      channelsApplied: p.channelsApplied ?? 0,
      error: p.error ?? '',
    }
  },

  async getFailoverStatus(): Promise<FailoverStatus | null> {
    if (isBuildPhase()) return null
    const f = await App.GetFailoverStatus()
    return {
      enabled: !!f.enabled,
      state: f.state ?? '',
      switches: f.switches ?? 0,
      lastError: f.lastError ?? '',
      nextCheckIn: f.nextCheckIn ?? 0,
    }
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
        code: s.code ?? 0,
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

/** NetGuard: фактическое состояние авто-починки (null = биндинг недоступен). */
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
    events: (s.events ?? []).map((e) => ({
      time: e.time ?? '',
      kind: (e.kind ?? 'PROBLEM') as NetGuardStatus['events'][number]['kind'],
      text: e.text ?? '',
    })),
    lastRun: s.lastRun ?? '',
    dohDisabled: s.dohDisabled ?? false,
  }
}

/** Ручной heal зависшего системного прокси (тот же код, что startup self-heal).
 * true = был исправлен; false = чинить нечего (факт, не ошибка). */
export async function runProxyHeal(): Promise<boolean | null> {
  if (isBuildPhase()) return null
  return await App.RunProxyHeal()
}
