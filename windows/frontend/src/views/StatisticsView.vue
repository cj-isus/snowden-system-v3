<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import PageHeading from '../components/PageHeading.vue'
import HintBox from '../components/HintBox.vue'
import { api } from '../api/backend'
import { diagnosticContextKey, fmtDateTime, fmtTime } from '../api/labels'
import { usePolling } from '../composables/polling'
import { createTrafficHistory } from '../composables/trafficHistory'
import type { TrafficPoint } from '../composables/trafficHistory'
import { useBackendState, initBackendState } from '../composables/backendState'
import { useCoreMetrics } from '../composables/coreMetrics'

const { state } = useBackendState()
const adapterScope = () => diagnosticContextKey(state.value?.diagnosticContext) ||
  JSON.stringify([state.value?.state, state.value?.activeChannelId, state.value?.proxyMode])
const { data: stats, loading: statsLoading, error: statsError, receivedAt: statsReceivedAt, refresh: refreshStats } =
  usePolling(api.getNetStats, 3000, { scope: adapterScope, pauseWhenHidden: true })
const { data: metrics, loading: metricsLoading, note: metricsNote, receivedAt: metricsReceivedAt, refresh: refreshMetrics, interval: coreInterval } = useCoreMetrics()
const loading = computed(() => statsLoading.value || metricsLoading.value)
async function refresh(): Promise<void> { await Promise.all([refreshStats(), refreshMetrics()]) }
onMounted(() => { void initBackendState() })

const history = createTrafficHistory()
const points = ref<TrafficPoint[]>([])
const deltaIn = ref<number | null>(null)
const deltaOut = ref<number | null>(null)
const windowFrom = ref<number | null>(null)
const windowTo = ref<number | null>(null)
watch(stats, value => {
  const result = value ? history.observe(value, Date.parse(statsReceivedAt.value ?? '')) : null
  if (!result) history.reset()
  deltaIn.value = result?.deltaIn ?? null
  deltaOut.value = result?.deltaOut ?? null
  points.value = result?.points ?? []
  windowFrom.value = result?.from ?? null
  windowTo.value = result?.to ?? null
}, { flush: 'sync' })

const coreDown = computed(() => coreInterval.value?.down ?? null)
const coreUp = computed(() => coreInterval.value?.up ?? null)
const coreFrom = computed(() => coreInterval.value?.from ?? null)
const coreTo = computed(() => coreInterval.value?.to ?? null)

function formatAmount(value: number | null | undefined, units: readonly string[]): string {
  if (value == null || !Number.isFinite(value) || value < 0) return 'Нет данных'
  let amount = value, index = 0
  while (amount >= 1024 && index < units.length - 1) { amount /= 1024; index++ }
  return `${amount.toLocaleString('ru-RU', { maximumFractionDigits: 1 })} ${units[index]}`
}
function fmtBytes(value: number | null | undefined): string { return formatAmount(value, ['Б', 'КиБ', 'МиБ', 'ГиБ', 'ТиБ']) }
function fmtRate(value: number | null): string { return formatAmount(value, ['Б/с', 'КиБ/с', 'МиБ/с', 'ГиБ/с']) }
function timeAt(at: number | null): string { return at === null ? 'Не установлено' : fmtTime(new Date(at).toISOString()) }
function intervalSeconds(from: number, to: number): string { return ((to - from) / 1000).toLocaleString('ru-RU', { maximumFractionDigits: 1 }) }
const linkSpeed = computed(() => {
  const speed = stats.value?.speedBps
  return speed == null || speed === 0 ? 'Не подтверждена' : `${(speed / 1e9).toLocaleString('ru-RU', { maximumFractionDigits: 3 })} Гбит/с`
})
const maxBarBytes = computed(() => Math.max(1, ...points.value.map(point => point.bytes)))
const graphFrom = computed(() => points.value[0]?.from ?? null)
const graphTo = computed(() => points.value.at(-1)?.at ?? null)
</script>

<template>
  <div class="view">
    <PageHeading title="Телеметрия" description="Счётчики адаптера и ядра разделены. История экрана начинается с полученных здесь снимков.">
      <button class="btn" :disabled="loading" @click="refresh">{{ loading ? 'Обновление…' : 'Обновить снимки' }}</button>
    </PageHeading>

    <section class="adapter-section">
      <div class="section-heading"><h2>Адаптер Windows</h2><span class="muted">{{ stats?.alias || 'Интерфейс не установлен' }}</span></div>
      <p v-if="!stats" class="state-note" role="status">{{ statsLoading ? 'Получаем счётчики адаптера…' : statsError || 'Снимок адаптера недоступен.' }}</p>
      <p class="source-time">Получено экраном: {{ fmtDateTime(statsReceivedAt) }}. Это время ответа интерфейсу, не точное время измерения ОС.</p>
      <p v-if="stats?.source === 'native_route_adapter'" class="source-time">Источник: счётчики интерфейса, выбранного Windows для маршрута к {{ stats.routeTarget }}. Это чтение не выполняет HTTP-пробы. Смена переданного идентификатора адаптера начинает новое окно.</p>
      <p v-else class="source-time">Привязка маршрута и идентификатор адаптера не переданы. Имя само по себе не устанавливает непрерывность счётчиков.</p>
      <dl class="metrics-list">
        <div><dt>Получено за окно наблюдения</dt><dd>{{ fmtBytes(deltaIn) }}</dd></div>
        <div><dt>Отправлено за окно наблюдения</dt><dd>{{ fmtBytes(deltaOut) }}</dd></div>
        <div><dt>Всего получено адаптером</dt><dd>{{ fmtBytes(stats?.inOctets) }}</dd></div>
        <div><dt>Всего отправлено адаптером</dt><dd>{{ fmtBytes(stats?.outOctets) }}</dd></div>
      </dl>
      <p class="state-note">Окно накопленной дельты: {{ timeAt(windowFrom) }} — {{ timeAt(windowTo) }}. Первый снимок задаёт базу; нужны два ответа. Ошибка, смена имени или идентификатора, откат времени или счётчиков начинают новое окно.</p>
      <p class="state-note">Скорость линии из снимка: {{ linkSpeed }}. Это не текущая пропускная способность VPN. Два снимка могут содержать одинаковые значения счётчиков.</p>
      <figure v-if="points.length" class="traffic-figure">
        <figcaption>Приращения входящего счётчика между ответами · {{ timeAt(graphFrom) }} — {{ timeAt(graphTo) }}</figcaption>
        <div class="traffic-bars" aria-hidden="true"><span v-for="(point, index) in points" :key="index" :style="{ height: point.bytes / maxBarBytes * 100 + '%' }"></span></div>
        <p class="state-note">Высота — число байтов за интервал ответа; длительность интервалов может различаться. Таблица содержит те же значения.</p>
        <details class="interval-details"><summary>Интервалы и значения</summary>
          <div class="table-scroll" tabindex="0" role="region" aria-label="Интервалы счётчиков адаптера"><table>
            <caption>Время получения ответов интерфейсом, не время измерения ОС</caption>
            <thead><tr><th scope="col">От</th><th scope="col">До</th><th scope="col">Интервал</th><th scope="col">Получено</th></tr></thead>
            <tbody><tr v-for="(point, index) in points" :key="index"><td>{{ timeAt(point.from) }}</td><td>{{ timeAt(point.at) }}</td><td>{{ intervalSeconds(point.from, point.at) }} с</td><td>{{ fmtBytes(point.bytes) }}</td></tr></tbody>
          </table></div>
        </details>
      </figure>
      <p v-else class="state-note">Приращений для графика нет. Отсутствие интервала не означает нулевой трафик.</p>
    </section>

    <section class="core-section" :aria-busy="metricsLoading">
      <h2>Счётчики ядра</h2>
      <p v-if="!metrics" class="state-note" role="status">{{ metricsLoading ? 'Получаем снимок ядра…' : metricsNote }}</p>
      <p v-else-if="!metrics.available" class="state-note">Метрики недоступны: {{ metrics.note || 'Причина не передана.' }}</p>
      <template v-else>
        <p class="source-time">Источник: read-only счётчики ядра. Время снимка: {{ fmtDateTime(metrics.at) }} · Получено экраном: {{ fmtDateTime(metricsReceivedAt) }}.</p>
        <dl class="metrics-list">
          <div><dt>Среднее получение за интервал</dt><dd>{{ fmtRate(coreDown) }}</dd></div>
          <div><dt>Средняя отправка за интервал</dt><dd>{{ fmtRate(coreUp) }}</dd></div>
          <div><dt>Получено от базы сборщика ядра</dt><dd>{{ fmtBytes(metrics.sessionDown) }}</dd></div>
          <div><dt>Отправлено от базы сборщика ядра</dt><dd>{{ fmtBytes(metrics.sessionUp) }}</dd></div>
        </dl>
        <p class="state-note">Интервал скорости: {{ timeAt(coreFrom) }} — {{ timeAt(coreTo) }}. Нужны два снимка с возрастающим временем и счётчиками; известное отсутствие прироста даёт 0 Б/с. Числа не прогнозируют скорость канала.</p>
        <p class="state-note">Скорость рассчитывается между снимками одной сессии в одном контексте. Перезапуск, смена сети или состояния, отсутствие ответа и пауза экрана снимают базу интервала. Первый ответ после них показывает суммы; для скорости нужен следующий.</p>
        <details class="connections-details">
          <summary>Соединения в полученном ответе · {{ metrics.connections.length }}</summary>
          <div v-if="metrics.connections.length" class="table-scroll" tabindex="0" role="region" aria-label="Соединения из снимка ядра"><table>
            <caption>Возвращённая выборка; она может быть ограничена сборщиком</caption>
            <thead><tr><th scope="col">Назначение</th><th scope="col">Протокол</th><th scope="col">Канал</th><th scope="col">Получено</th><th scope="col">Отправлено</th><th scope="col">Начало</th></tr></thead>
            <tbody><tr v-for="(connection, index) in metrics.connections" :key="index"><th scope="row" :title="connection.process || undefined">{{ connection.host || 'Не сообщено' }}</th><td>{{ connection.network || '—' }}</td><td>{{ connection.chain || '—' }}</td><td>{{ fmtBytes(connection.down) }}</td><td>{{ fmtBytes(connection.up) }}</td><td>{{ connection.since || '—' }}</td></tr></tbody>
          </table></div>
          <p v-else class="state-note">В ответе нет перечисленных соединений. Это не доказывает отсутствие трафика.</p>
        </details>
      </template>
    </section>
    <HintBox kind="info">Счётчики адаптера включают трафик других приложений и не равны трафику VPN. Совпадение чисел не доказывает защищённый маршрут. Пропуск наблюдения не заполняется нулём; утраченные интервалы не восстанавливаются.</HintBox>
  </div>
</template>

<style scoped>
.view { display: flex; flex-direction: column; gap: 20px; min-width: 0; overflow-wrap: anywhere; }
h2 { margin: 0; font-size: 17px; }
.adapter-section, .core-section { padding: 20px; border: 1px solid var(--border); border-radius: var(--radius); background: var(--panel); min-width: 0; }
.section-heading { display: flex; flex-wrap: wrap; justify-content: space-between; gap: 12px; align-items: baseline; }
.source-time, .state-note, .muted { color: var(--muted); font-size: 12.5px; line-height: 1.65; }
.metrics-list { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); margin: 20px 0; gap: 18px 24px; }
dt { color: var(--muted); font-size: 12.5px; }
dd { color: var(--text); margin: 6px 0 0; font-size: clamp(18px, 1.4rem, 26px); line-height: 1.4; font-variant-numeric: tabular-nums; }
.traffic-figure { margin: 20px 0 0; }
figcaption { color: var(--text-soft); font-size: 13px; line-height: 1.6; }
.traffic-bars { display: flex; gap: 4px; align-items: flex-end; height: 100px; margin: 16px 0; border-bottom: 1px solid var(--border-light); background: linear-gradient(to top, var(--accent-soft), transparent); }
.traffic-bars span { flex: 1; min-width: 0; background: var(--accent); max-height: 100%; }
summary { cursor: pointer; padding: 10px 0; color: var(--text-soft); }
.connections-details { margin-top: 20px; border-top: 1px solid var(--border); }
.table-scroll { overflow-x: auto; min-width: 0; }
table { border-collapse: collapse; width: 100%; font-size: 12px; }
caption { color: var(--muted); text-align: left; padding: 10px 0; line-height: 1.6; }
th, td { text-align: left; padding: 10px 12px 10px 0; vertical-align: top; border-bottom: 1px solid var(--border); }
th { font-weight: 500; color: var(--muted); }
td { color: var(--text-soft); font-variant-numeric: tabular-nums; }
.connections-details table { min-width: 620px; }
.connections-details tbody th { max-width: 240px; color: var(--text-soft); }
.btn { min-height: 40px; white-space: normal; }
@media (max-width: 760px) { .adapter-section, .core-section { padding: 16px; } .metrics-list { grid-template-columns: minmax(0, 1fr); gap: 16px; } }
</style>
