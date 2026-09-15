/**
 * Dev-мок backend для визуального тестирования UI (UI-план v3).
 *
 * Включается ТОЛЬКО параметром ?mock=1 в dev-превью (npm run dev). В Wails-
 * сборке window.go уже существует, и этот файл ничего не делает. Мок
 * подставляет window.go/window.runtime до первого обращения фасада:
 * вызовы App.* идут в мок так же, как в реальный биндинг.
 *
 * Честность: мок — инструмент разработки, не данные продукта. При активном
 * моке App.vue показывает заметную плашку «DEV-MOCK», чтобы мок-данные нельзя
 * было перепутать с фактическими. Значения — правдоподобная фиксация реального
 * состояния (4 канала: vless/hy2 live-verified, reality/shadowtls configured).
 */

type MockState = {
  state: string
  blockedReason: string | null
  activeId: string | null
  active: {
    id: string
    transport: string
    server: string
    port: number
    validation: string
    enabled: boolean
  } | null
  proxyMode: string | null
  probe: { ok: boolean; steps: { name: string; status: string; detail: string }[] } | null
  probeRunning: boolean
  probeLastAt: string | null
  error: string
  coreReady: boolean
  coreBlockMsg: string
  nextRetryAt: string
  retryAttempt: number
}

const now = (): string => new Date().toISOString()

const channels = [
  { id: 'channel-a-vless', transport: 'vless+ws', server: 'vpn.example.com', port: 443, validation: 'live-verified', enabled: true },
  { id: 'channel-a-hy2', transport: 'hysteria2', server: '203.0.113.10', port: 8444, validation: 'live-verified', enabled: true },
  { id: 'channel-a-reality', transport: 'vless+tcp-reality', server: 'www.samsung.com', port: 8443, validation: 'live-verified', enabled: true },
  { id: 'channel-a-shadowtls', transport: 'vless+shadowtls', server: 'www.samsung.com', port: 8445, validation: 'configured', enabled: true },
]

const probeOk = (): MockState['probe'] => ({
  ok: true,
  steps: [
    { name: 'https://vpn.example.com/ip', status: 'pass', detail: `HTTP 200 за 92ms; egress IP: 203.0.113.10` },
    { name: 'https://api.ipify.org', status: 'pass', detail: 'HTTP 200 за 104ms; egress IP: 203.0.113.10' },
    { name: 'Egress consistency', status: 'pass', detail: 'обе цели видят один egress' },
    { name: 'Expected egress', status: 'pass', detail: 'egress 203.0.113.10 соответствует каналу' },
    { name: 'Direct leak check', status: 'pass', detail: 'прямой egress отличается — трафик идёт через туннель' },
    { name: 'DNS через туннель', status: 'pass', detail: 'example.com → 93.184.216.34 (DoH через туннель)' },
  ],
})

const tunSim = sessionStorage.getItem('mock-tun') === '1'

const state: MockState = {
  state: 'running',
  blockedReason: null,
  activeId: 'channel-a-vless',
  active: channels[0],
  proxyMode: tunSim ? 'tun' : 'socks',
  probe: probeOk(),
  probeRunning: false,
  probeLastAt: now(),
  error: '',
  coreReady: true,
  coreBlockMsg: '',
  nextRetryAt: '',
  retryAttempt: 0,
}

function setState(patch: Partial<MockState>): void {
  Object.assign(state, patch)
  emit('state', JSON.parse(JSON.stringify(state)))
}

type Listener = (payload: unknown) => void
const listeners = new Map<string, Listener[]>()

function emit(event: string, payload: unknown): void {
  for (const cb of listeners.get(event) ?? []) cb(payload)
}

const delay = <T>(ms: number, value: T): Promise<T> => new Promise((r) => setTimeout(() => r(value), ms))

const meta = (i: number) => ({
  id: `secret-${i}`,
  kind: 'custom',
  title: ['UUID VLESS', 'Пароль HY2', 'Пароль obfs (HY2)', 'REALITY: публичный ключ'][i],
  hint: '',
  backend: 'dpapi',
  storedValue: 'dpapi',
  fingerprint: 'a3f1c9d2e8b7',
  createdAt: '2026-09-09T12:00:00Z',
  updatedAt: '2026-09-12T15:30:00Z',
  lastVerifiedAt: now(),
  verifyStatus: 'ok',
  verifyError: '',
  reveals: i,
})

const mockApp = {
  // ---------- lifecycle ----------
  GetState: (): MockState => JSON.parse(JSON.stringify(state)),
  Start: () => {
    // Ручной запуск отменяет расписание авто-повтора (как реальный Start, V2-053).
    setState({ state: 'starting', probeRunning: false, nextRetryAt: '', retryAttempt: 0 })
    setTimeout(() => {
      setState({ state: 'running', activeId: 'channel-a-vless', active: channels[0], probe: probeOk(), probeLastAt: now(), error: '', blockedReason: null })
    }, 2500)
  },
  Stop: () => {
    setState({ state: 'stopping', nextRetryAt: '', retryAttempt: 0 })
    setTimeout(() => setState({ state: 'stopped', activeId: null, active: null, probe: null, probeLastAt: null }), 1200)
  },
  RunProbe: () => {
    setState({ probeRunning: true })
    setTimeout(() => setState({ probeRunning: false, probe: probeOk(), probeLastAt: now() }), 2200)
  },
  SelectChannel: (id: string) => {
    const ch = channels.find((c) => c.id === id)
    if (!ch) return Promise.reject(new Error(`канал ${id} не найден`))
    setState({ state: 'reloading' })
    if (id === 'channel-a-shadowtls') {
      // Честный отказ канала D из фильтрующей сети (V2-051): probe fail, откат.
      return new Promise((_res, rej) =>
        setTimeout(() => {
          setState({ state: 'running' })
          rej(new Error('channel switch failed probe: rolled back to previous channel: 3 of 4 steps failed; first: HTTPS https://api.ipify.org: context deadline exceeded (откат на "channel-a-vless" подтверждён probe)'))
        }, 2600),
      )
    }
    setTimeout(() => setState({ state: 'running', activeId: id, active: ch }), 2400)
    return delay(2400, undefined)
  },
  IsTUNMode: () => Promise.resolve(state.proxyMode === 'tun'),
  EnableTUN: () => {
    // Верная симуляция реального потока (WIN-TUN-1): backend останавливает VPN
    // и завершает процесс (UAC) — UI видит stopped, затем начинается «новый
    // процесс». В превью это перезагрузка страницы с флагом-режимом.
    setState({ state: 'stopping' })
    setTimeout(() => {
      setState({ state: 'stopped', proxyMode: null, activeId: null, active: null, probe: null, probeLastAt: null })
      sessionStorage.setItem('mock-tun', '1')
      setTimeout(() => location.reload(), 700)
    }, 1000)
    return delay(1000, undefined)
  },
  DisableTUN: () => {
    setState({ state: 'stopping' })
    setTimeout(() => {
      setState({ state: 'stopped', proxyMode: null, activeId: null, active: null, probe: null, probeLastAt: null })
      sessionStorage.removeItem('mock-tun')
      setTimeout(() => location.reload(), 700)
    }, 1000)
    return delay(1000, undefined)
  },
  OpenLogsDir: () => Promise.resolve(),
  // Dev-хук для превью (только мок, не контракт): включить фактическую форму
  // fail-closed после исчерпания failover-on-start (failCore: state=error +
  // «fail-closed» в тексте — баннер на главной цепляется именно к этому).
  SetFailClosed: (on: boolean) => {
    if (on) {
      // Как реальный failCore (V2-053): error + запланированная авто-попытка
      // с первой паузой backoff (30с).
      setState({
        state: 'error',
        error: 'запуск: все каналы не прошли probe (channel-a-vless и [channel-a-hy2 channel-a-reality]), включая повторную серию — fail-closed (FR-001)',
        activeId: null,
        active: null,
        probe: null,
        nextRetryAt: new Date(Date.now() + 30_000).toISOString(),
        retryAttempt: 1,
      })
    } else {
      setState({ state: 'stopped', error: '', nextRetryAt: '', retryAttempt: 0 })
    }
    return Promise.resolve()
  },
  // Dev-хук: вторая фактическая форма fail-closed — watchdog исчерпал попытки
  // (failWatchdog: state=stopped + blockedReason='all_channels_failed').
  SetWatchdogExhausted: (on: boolean) => {
    if (on) {
      setState({
        state: 'stopped',
        blockedReason: 'all_channels_failed',
        error: 'адаптивный watchdog: все каналы не прошли probe, автопереключение исчерпано — туннель остановлен (fail-closed)',
        activeId: null,
        active: null,
        probe: null,
        nextRetryAt: new Date(Date.now() + 60_000).toISOString(),
        retryAttempt: 2,
      })
    } else {
      setState({ state: 'stopped', blockedReason: null, error: '', nextRetryAt: '', retryAttempt: 0 })
    }
    return Promise.resolve()
  },

  // ---------- данные ----------
  ListChannels: () => Promise.resolve(JSON.parse(JSON.stringify(channels))),
  ListTests: () =>
    Promise.resolve([
      { id: 'udp-availability', ok: false, detail: 'UDP отфильтрован: ответа нет (типично для сети агента). HY2 проверять из сети без UDP-фильтра', checkedAt: now(), steps: [{ name: 'QUIC Initial → 8.8.8.8:443', status: 'fail', detail: 'ответа нет за 3 с' }] },
      { id: 'hy2-reachability', ok: true, detail: 'пропущено как нерелевантное: сейчас активен channel-a-vless, туннель идёт не через HY2', checkedAt: now(), steps: [{ name: 'UDP → канал HY2', status: 'skipped', detail: 'активен channel-a-vless (не HY2)' }] },
      { id: 'vless-config-present', ok: true, detail: 'Слоты секретов заполнены (8 шт.), формат значений валиден', checkedAt: now(), steps: [] },
      { id: 'elevated-process', ok: false, detail: 'Процесс запущен без прав администратора — штатно для SOCKS-режима (TUN потребует UAC)', checkedAt: now(), steps: [] },
      { id: 'core-wired', ok: true, detail: 'Ядро подключено, egress подтверждён', checkedAt: now(), steps: [] },
    ]),
  RunTest: (id: string) => delay(1500, { id, ok: true, detail: `мок: тест ${id} пройден`, checkedAt: now(), steps: [] }),
  ListSecrets: () => Promise.resolve([0, 1, 2, 3].map(meta)),
  SaveSecret: (id: string) => delay(300, meta(Number(id.split('-')[1] ?? 0))),
  AddSecret: () => delay(300, meta(0)),
  DeleteSecret: () => Promise.resolve(),
  VerifySecret: (id: string) => delay(900, { ...meta(Number(id.split('-')[1] ?? 0)), verifyStatus: 'ok' }),
  TestSecret: () =>
    delay(1400, { ok: true, steps: [{ name: 'формат значения', status: 'pass', detail: 'формат корректен' }, { name: 'живой тест', status: 'pass', detail: 'мок: сервер подтвердил значение' }] }),
  RevealSecret: () => delay(200, 'mock-revealed-value'),

  // ---------- факты ----------
  GetNetworkFacts: () =>
    Promise.resolve({ connectionType: 'Wi-Fi', ssid: 'HomeNet-5G', localIP: '192.168.1.42', isp: 'МГТС', publicIP: '91.92.42.245', country: '—', countryOrigin: 'RU', dnsViaTunnel: true, netClass: 'wifi', checkedAt: now() }),
  GetNetStats: () =>
    Promise.resolve({ inOctets: 7_412_336_128 + Math.floor(Math.random() * 3_000_000), outOctets: 1_236_802_048 + Math.floor(Math.random() * 900_000), speedBps: 450_000_000, alias: 'Wi-Fi (wlan0)' }),
  GetAutostart: () => Promise.resolve(false),
  SetAutostart: () => Promise.resolve(),

  ExportOnboardingQR: () => delay(800, { transport: 'SNOB1.mock', dataUrl: '' }),
  SaveOnboardingFile: () => Promise.resolve('C:\\mock\\device.snob'),
  LoadOnboardingFile: () => Promise.resolve('SNOB1.mock'),
  PreviewOnboardingString: () =>
    delay(600, {
      created_at: now(),
      device_name: '',
      channels: 4,
      secrets: 10,
      split_direct: 2,
      keys: [{ key_id: 'a1b2c3d4e5f60718', fingerprint: '9f2d8c1a4b3e5f60718293a4b5c6d7e8', comment: 'profile-signing' }],
      bundle_version: 3,
      current_version: 3,
      would_downgrade: false,
    }),
  ApplyOnboardingString: () =>
    delay(900, { created_at: now(), device_name: '', channels: 4, secrets: 10, split_direct: 2, keys: [], bundle_version: 3, current_version: 3, would_downgrade: false }),

  PickUpdateDir: () => Promise.resolve('C:\\mock\\updates'),
  CheckUpdate: () => delay(600, { current_version: '2.0.0', version: '2.1.0', notes: 'мок: улучшена живучесть каналов', released_at: now(), size: 36_912_128, sha256: '0fd4628ba1c2d3e4f5061728394a5b6c', key_id: 'a1b2c3d4e5f60718', floor: '2.0.0', ok: true, reason: '' }),
  ApplyUpdate: () => delay(1000, ''),

  GetSplitDirect: () => Promise.resolve(['steam.exe', 'epicgameslauncher.exe']),
  GetMetrics: () =>
    Promise.resolve({
      available: state.state === 'running',
      note: state.state === 'running' ? '' : 'ядро не запущено',
      rateUp: Math.floor(Math.random() * 180_000),
      rateDown: Math.floor(Math.random() * 2_400_000),
      sessionUp: 48_213_504,
      sessionDown: 712_930_816,
      at: now(),
      connections: [
        { host: 'cdn.jsdelivr.net', network: 'tcp', chain: 'channel-a-vless', process: '', up: 18_432, down: 1_204_608, since: '09:41:02' },
        { host: 'api.github.com', network: 'tcp', chain: 'channel-a-vless', process: '', up: 5_120, down: 92_160, since: '09:43:18' },
        { host: 'chatgpt.com', network: 'tcp', chain: 'warp-ep', process: '', up: 92_672, down: 4_812_032, since: '09:40:55' },
        { host: 'steamcontent.com', network: 'udp', chain: 'direct (split)', process: 'steam.exe', up: 3_041_280, down: 12_884_901_376, since: '09:12:40' },
      ],
    }),
  GetDeliveryProfile: () =>
    Promise.resolve({ present: false, version: 0, keyId: '', expiresAt: '', trustedKeys: 2, channelsApplied: 0, error: '' }),
  GetFailoverStatus: () =>
    Promise.resolve({ enabled: state.state === 'running', state: 'closed', switches: 1, lastError: '', nextCheckIn: 18 }),
  RunAdaptiveCheck: () =>
    delay(4000, {
      sites: [
        { domain: 'chatgpt.com', code: 200, ok: true, challenge: false, detail: '200 OK (WARP egress)', checkedAt: now() },
        { domain: 'claude.ai', code: 403, ok: false, challenge: true, detail: '403 challenge: IP деградировал', checkedAt: now() },
        { domain: 'gemini.google.com', code: 200, ok: true, challenge: false, detail: '200 OK', checkedAt: now() },
      ],
      warpDomains: ['chatgpt.com', 'claude.ai', 'perplexity.ai'],
      note: 'мок: классификация через живой туннель',
    }),
  GetAdaptiveStatus: () => Promise.resolve(null),
  NetGuardStatus: () =>
    Promise.resolve({
      tasks: [
        { name: 'NetGuard-AutoHeal', status: 'no-access', lastRun: '2026-09-14T08:07:37Z', nextRun: '2026-09-14T14:00:00Z', lastCode: '0' },
        { name: 'NetGuard-AutoHeal-User', status: 'ready', lastRun: '2026-09-14T09:12:00Z', nextRun: '2026-09-14T15:00:00Z', lastCode: '0' },
        { name: 'NetGuard-AutoHeal-NetEvent', status: 'ready', lastRun: '', nextRun: '', lastCode: '0' },
      ],
      proxy: { enabled: false, server: '127.0.0.1', port: 1080, listenerAlive: true, stale: false },
      socksAlive: state.state === 'running',
      events: [
        { time: '08:07:37', kind: 'HEALED', text: 'зависший системный прокси отключён (ProxyEnable 1 → 0)' },
        { time: '07:55:02', kind: 'PROBLEM', text: 'обнаружен stale-прокси на мёртвый порт 1080' },
      ],
      lastRun: now(),
      dohDisabled: false,
    }),
  RunProxyHeal: () => delay(1200, false),
}

/** Включить мок (только ?mock=1 и только вне Wails). Возвращает true, если применён. */
export function installMockIfRequested(): boolean {
  if (typeof window === 'undefined') return false
  const w = window as unknown as Record<string, unknown>
  if (w.go) return false // реальный backend — мок не нужен и запрещён
  if (!new URLSearchParams(window.location.search).has('mock')) return false

  w.go = { main: { App: mockApp } }
  w.runtime = {
    EventsOnMultiple: (event: string, cb: Listener): (() => void) => {
      const arr = listeners.get(event) ?? []
      arr.push(cb)
      listeners.set(event, arr)
      return () => {
        const i = (listeners.get(event) ?? []).indexOf(cb)
        if (i >= 0) (listeners.get(event) ?? []).splice(i, 1)
      }
    },
    EventsOn: (event: string, cb: Listener): (() => void) =>
      (w.runtime as Record<string, (e: string, c: Listener) => () => void>).EventsOnMultiple(event, cb),
    WindowMinimise: () => {},
    WindowToggleMaximise: () => {},
    WindowIsMaximised: () => Promise.resolve(false),
    Quit: () => {},
  }
  return true
}

export const MOCK_ACTIVE = installMockIfRequested()
