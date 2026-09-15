<script setup lang="ts">
/**
 * Статистика (concept): трафик из фактов ОС (GetNetStats: счётчики адаптера
 * маршрута по умолчанию) + сводка lifecycle. Счётчики абсолютные — дельта
 * считается от момента открытия раздела (честно подписано). Гистограмма —
 * живые замеры дельты, а не выдуманная история. Никаких «ГБ за месяц».
 */
import { computed, onMounted, onUnmounted, ref } from 'vue'
import HintBox from '../components/HintBox.vue'
import ProbeReportView from '../components/ProbeReportView.vue'
import { api, isBuildPhase } from '../api/backend'
import type { MetricsView, NetStats } from '../api/contract'
import { useBackendState, initBackendState } from '../composables/backendState'
import { lifecycleLabels, fmtDateTime } from '../api/labels'

const { state, channels } = useBackendState()
const probe = computed(() => state.value?.probe ?? null)

const stats = ref<NetStats | null>(null)
const loadError = ref('')
let timer: number | undefined

// Базовая точка для дельты (счётчики ОС монотонно растут).
const baseline = ref<{ inB: number; outB: number; at: number } | null>(null)

async function refresh(): Promise<void> {
  if (isBuildPhase()) {
    stats.value = null
    loadError.value = 'Биндинги недоступны: приложение запущено вне Wails (предпросмотр).'
    return
  }
  try {
    const s = await api.getNetStats()
    stats.value = s
    loadError.value = ''
    if (s && !baseline.value) baseline.value = { inB: s.inOctets, outB: s.outOctets, at: Date.now() }
  } catch (e) {
    loadError.value = e instanceof Error ? e.message : String(e)
  }
}

function fmtBytes(n: number | null | undefined): string {
  if (n === null || n === undefined) return '—'
  const units = ['Б', 'КБ', 'МБ', 'ГБ', 'ТБ']
  let v = n
  let u = 0
  while (v >= 1024 && u < units.length - 1) {
    v /= 1024
    u++
  }
  return `${v >= 100 ? Math.round(v) : v.toFixed(1)} ${units[u]}`
}

const deltaIn = computed(() => {
  if (!stats.value || !baseline.value) return null
  return Math.max(0, stats.value.inOctets - baseline.value.inB)
})
const deltaOut = computed(() => {
  if (!stats.value || !baseline.value) return null
  return Math.max(0, stats.value.outOctets - baseline.value.outB)
})

const speed = computed(() => {
  const s = stats.value
  if (!s?.speedBps) return null
  return `${(s.speedBps / 1e9).toFixed(1)} Гбит/с`
})

// Гистограмма трафика (последние N измерений дельты, живые данные, не выдумка).
// Таймер живёт только пока открыт раздел: фоновый замер выключен вместе со страницей.
const barCount = 24
const bars = ref<number[]>([])
let measureTimer: number | undefined
let lastMeasure = 0
function measure(): void {
  const dIn = deltaIn.value
  if (dIn === null) return
  if (lastMeasure === 0) {
    lastMeasure = dIn
    return
  }
  const step = Math.max(0, dIn - lastMeasure)
  lastMeasure = dIn
  bars.value.push(step)
  if (bars.value.length > barCount) bars.value.shift()
}

const maxBar = computed(() => Math.max(1, ...bars.value))
const barHeights = computed(() => {
  const filled = bars.value.map((b) => Math.max(4, Math.round((b / maxBar.value) * 100)))
  while (filled.length < barCount) filled.unshift(0)
  return filled
})

// Ось времени — реальное окно наблюдения: одна точка на 6 замеров (12 с).
const axisLabels = computed(() => {
  if (bars.value.length === 0) return []
  const now = new Date()
  return [3, 2, 1, 0]
    .map((i) => {
      const d = new Date(now.getTime() - i * 6 * 12 * 1000)
      return d.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
    })
    .filter((_, idx) => idx === 0 || bars.value.length >= 18 || idx !== 1)
})
onMounted(() => {
  void initBackendState()
  void refresh()
  void refreshMetrics()
  timer = window.setInterval(() => void refresh(), 3000)
  metricsTimer = window.setInterval(() => void refreshMetrics(), 2000)
  measureTimer = window.setInterval(measure, 2000)
})
onUnmounted(() => {
  window.clearInterval(timer)
  window.clearInterval(metricsTimer)
  window.clearInterval(measureTimer)
})

// ---------- Живой трафик ядра (V2-048/F12, clash_api read-only) ----------
const metrics = ref<MetricsView | null>(null)
let metricsTimer: number | undefined
async function refreshMetrics(): Promise<void> {
  if (isBuildPhase()) return
  try {
    metrics.value = await api.getMetrics()
  } catch {
    /* контроллер мог умереть вместе с ядром — покажем past-value */
  }
}
const coreDown = computed(() => metrics.value?.rateDown ?? 0)
const coreUp = computed(() => metrics.value?.rateUp ?? 0)
function fmtRate(bps: number): string {
  if (!bps) return '0 Б/с'
  const units = ['Б/с', 'КБ/с', 'МБ/с']
  let v = bps
  let i = 0
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v >= 100 ? Math.round(v) : v.toFixed(1)} ${units[i]}`
}
</script>

<template>
  <div class="view">
    <header class="head">
      <div class="section-title-group">
        <h2>Статистика</h2>
      </div>
      <p class="sub">
        Счётчики сетевого адаптера маршрута по умолчанию (факт ОС). Дельта считается от момента
        открытия раздела — это наблюдаемое изменение, не обещанный «трафик за период».
      </p>
    </header>

    <section class="card summary">
      <div class="card-header">
        <h2>Трафик</h2>
        <span v-if="stats?.alias" class="adapter-pill mono">{{ stats.alias }}</span>
      </div>

      <div class="traffic-summary">
        <div class="traffic-metric">
          <div class="traffic-value"><span class="traffic-arrow down">↓</span> {{ fmtBytes(deltaIn) }}</div>
          <div class="traffic-label">получено (дельта наблюдения)</div>
        </div>
        <div class="traffic-metric">
          <div class="traffic-value"><span class="traffic-arrow up">↑</span> {{ fmtBytes(deltaOut) }}</div>
          <div class="traffic-label">отправлено (дельта наблюдения)</div>
        </div>
        <div class="traffic-metric">
          <div class="traffic-value">{{ fmtBytes(stats?.inOctets ?? null) }}</div>
          <div class="traffic-label">всего получено адаптером</div>
        </div>
        <div class="traffic-metric">
          <div class="traffic-value">{{ fmtBytes(stats?.outOctets ?? null) }}</div>
          <div class="traffic-label">всего отправлено адаптером</div>
        </div>
      </div>

      <div class="traffic-chart">
        <div class="chart-grid"></div>
        <div class="chart-bars">
          <span v-for="(h, i) in barHeights" :key="i" :style="{ height: h + '%' }"></span>
        </div>
      </div>
      <div class="chart-axis">
        <span v-for="l in axisLabels" :key="l">{{ l }}</span>
      </div>

      <p v-if="loadError" class="load-error">Нет данных: {{ loadError }}</p>
      <p v-else-if="stats?.alias" class="adapter-note mono">адаптер: {{ stats.alias }}<template v-if="speed"> · {{ speed }}</template></p>
    </section>

    <section v-if="metrics" class="card">
      <div class="card-header">
        <h2>Соединения ядра</h2>
        <span class="adapter-pill mono">через туннель, live</span>
      </div>
      <template v-if="metrics.available">
        <div class="traffic-summary">
          <div class="traffic-metric">
            <div class="traffic-value"><span class="traffic-arrow down">↓</span> {{ fmtRate(coreDown) }}</div>
            <div class="traffic-label">скорость получения (ядро)</div>
          </div>
          <div class="traffic-metric">
            <div class="traffic-value"><span class="traffic-arrow up">↑</span> {{ fmtRate(coreUp) }}</div>
            <div class="traffic-label">скорость отправки (ядро)</div>
          </div>
          <div class="traffic-metric">
            <div class="traffic-value">{{ fmtBytes(metrics.sessionDown) }}</div>
            <div class="traffic-label">получено за сессию ядра</div>
          </div>
          <div class="traffic-metric">
            <div class="traffic-value">{{ fmtBytes(metrics.sessionUp) }}</div>
            <div class="traffic-label">отправлено за сессию ядра</div>
          </div>
        </div>
        <table v-if="metrics.connections.length" class="conn-table">
          <thead>
            <tr><th>Хост</th><th>Тип</th><th>Канал</th><th>↓</th><th>↑</th><th>С</th></tr>
          </thead>
          <tbody>
            <tr v-for="(c, i) in metrics.connections" :key="i">
              <td class="mono" :title="c.process">{{ c.host }}</td>
              <td>{{ c.network }}</td>
              <td>{{ c.chain || '—' }}</td>
              <td class="mono">{{ fmtBytes(c.down) }}</td>
              <td class="mono">{{ fmtBytes(c.up) }}</td>
              <td class="mono">{{ c.since }}</td>
            </tr>
          </tbody>
        </table>
        <p v-else class="adapter-note">Соединений пока нет — откройте любой сайт.</p>
        <p class="adapter-note">
          Источник — сам ядро (read-only clash_api на loopback): счётчики от старта ядра,
          скорость — дельта между опросами. Никакой выдуманной истории.
        </p>
      </template>
      <p v-else class="adapter-note">Метрики недоступны: {{ metrics.note }}</p>
    </section>

    <div class="grid">
      <section class="card kv-card">
        <div class="card-header"><h2>Сводка состояния</h2></div>
        <div class="rows">
          <div class="row"><span>Состояние VPN</span><strong>{{ state ? (lifecycleLabels[state.state] ?? state.state) : '—' }}</strong></div>
          <div class="row"><span>Активный канал</span><strong>{{ state?.activeChannelId || '—' }}</strong></div>
          <div class="row"><span>Каналов в конфиге</span><strong>{{ channels?.length ?? '—' }}</strong></div>
          <div class="row"><span>Последняя проверка</span><strong>{{ state?.probeLastAt ? fmtDateTime(state.probeLastAt) : '—' }}</strong></div>
          <div class="row">
            <span>Последний probe</span>
            <strong :class="probe?.ok ? 'green-value' : probe ? 'red-value' : ''">
              {{ probe ? (probe.ok ? 'пройден' : 'не пройден') : 'не запускался' }}
            </strong>
          </div>
        </div>
      </section>

      <section class="card note-card">
        <div class="card-header"><h2>Отчёт о проверке пути</h2></div>
        <ProbeReportView :report="probe" :running="state?.probeRunning ?? false" />
        <p v-if="probe && !probe.ok && state?.blockedReason" class="empty">
          BLOCKED — причина: {{ state.blockedReason }}
        </p>
      </section>
    </div>

    <HintBox kind="info" title="Почему дельта, а не «за период»">
      Windows отдаёт накопленные счётчики адаптера. Привязать их задним числом к «1 часу /
      7 дням» невозможно без собственной истории — вместо выдуманной истории UI показывает
      живую дельту наблюдения и абсолютные счётчики. Столбики гистограммы — живые замеры
      раз в 2 секунды, пока открыт этот раздел.
    </HintBox>
  </div>
</template>

<style scoped>
.view {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.head h2 {
  margin: 0;
  font-size: 18px;
  font-weight: 650;
}
.sub {
  margin: 5px 0 0;
  color: var(--muted);
  font-size: 11.5px;
  max-width: 720px;
}

.summary,
.kv-card,
.note-card {
  padding: 13px;
}

.adapter-pill {
  margin-left: auto;
  padding: 3px 9px;
  border-radius: 5px;
  font-size: 9.5px;
  color: #a1b2c0;
  border: 1px solid var(--gray-border);
  background: var(--gray-soft);
}

.traffic-summary {
  display: flex;
  gap: 36px;
  margin-top: 12px;
  flex-wrap: wrap;
}
.traffic-metric {
  min-width: 120px;
}
.traffic-value {
  color: #ecf3f7;
  font-size: 16px;
  font-weight: 650;
}
.traffic-arrow {
  font-size: 19px;
}
.traffic-arrow.down {
  color: var(--blue-bright);
}
.traffic-arrow.up {
  color: var(--green);
}
.traffic-label {
  margin-top: 2px;
  color: #7e94a8;
  font-size: 9px;
}

.traffic-chart {
  position: relative;
  height: 96px;
  margin-top: 14px;
  border-bottom: 1px solid #294256;
}
.chart-grid {
  position: absolute;
  inset: 0;
  background:
    repeating-linear-gradient(to bottom, transparent 0, transparent 21px, rgba(24, 53, 73, 0.65) 22px, rgba(24, 53, 73, 0.65) 23px);
}
.chart-bars {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: flex-end;
  gap: 4px;
}
.chart-bars span {
  flex: 1;
  min-width: 3px;
  border-radius: 2px 2px 0 0;
  background: linear-gradient(to top, #11a5db, #32dfa4);
  min-height: 2px;
}
.chart-axis {
  display: flex;
  justify-content: space-between;
  color: #70879b;
  font-size: 9px;
  margin-top: 5px;
}

.load-error {
  margin: 10px 0 0;
  color: #ff8a90;
  font-size: 10.5px;
}
.adapter-note {
  margin: 8px 0 0;
  color: #55708a;
  font-size: 9px;
}

.grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}
.rows {
  margin-top: 8px;
}
.row {
  min-height: 28px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid #142d40;
  color: #8298ab;
  font-size: 10.5px;
}
.row strong {
  color: #dbe6ed;
  font-weight: 500;
}

.empty {
  margin-top: 8px;
  color: #70889d;
  font-size: 11px;
}

/* Таблица соединений ядра — палитра приложения (v3: была в чужой серой палитре) */
.conn-table {
  width: 100%;
  border-collapse: collapse;
  margin-top: 12px;
  font-size: 11px;
}
.conn-table th {
  height: 30px;
  padding: 0 9px;
  text-align: left;
  color: #8298ab;
  font-weight: 500;
  background: #0c2031;
  border-bottom: 1px solid #1a344a;
}
.conn-table td {
  height: 32px;
  padding: 0 9px;
  color: #c6d3dd;
  border-bottom: 1px solid #152d40;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 260px;
}
.conn-table tbody tr:hover {
  background: rgba(20, 64, 94, 0.16);
}
</style>

