<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import PageHeading from '../components/PageHeading.vue'
import ActionButton from '../components/ActionButton.vue'
import HintBox from '../components/HintBox.vue'
import { api, netGuardStatus } from '../api/backend'
import { fmtDateTime, fmtLoopbackObservation, fmtEgressObservation, fmtEgressObservationTime } from '../api/labels'
import { usePolling } from '../composables/polling'
import { useBackendState, initBackendState } from '../composables/backendState'
import { useInfoFacts } from '../composables/infoFacts'
import { useNav } from '../composables/nav'

const { state, isRunning, isBusy, switching } = useBackendState()
const { facts, factsLoading, factsError, factsReceivedAt, refreshFacts } = useInfoFacts({ facts: true })
const { data: netGuard, loading: guardLoading, error: guardError, refresh: refreshGuard } = usePolling(netGuardStatus, 30000, { pauseWhenHidden: true })
const { data: splitData, loading: splitLoading, error: splitError, refresh: refreshSplit } = usePolling(api.getSplitDirect, 30000, { pauseWhenHidden: true })
const { data: adaptive, loading: sitesLoading, error: sitesError, refresh: refreshSites, invalidate: invalidateSites, hasInterest: hasSitesInterest } = usePolling(api.getAdaptiveStatus, 30000, { pauseWhenHidden: true })
const { navTo } = useNav()
const refreshBusy = ref(false)
const actionBusy = ref(false)
const actionError = ref('')
const actionNote = ref('')
const refreshError = ref('')
let owner = true
onMounted(() => { void initBackendState() })
onUnmounted(() => { owner = false })
const loading = computed(() => refreshBusy.value || factsLoading.value || guardLoading.value || splitLoading.value || sitesLoading.value)
const observationSources = {
  facts: { loading: factsLoading, refresh: refreshFacts },
  guard: { loading: guardLoading, refresh: refreshGuard },
  split: { loading: splitLoading, refresh: refreshSplit },
  sites: { loading: sitesLoading, refresh: refreshSites },
}
function refreshSource(source: keyof typeof observationSources): void {
  const selected = observationSources[source]
  if (owner && !selected.loading.value) void selected.refresh()
}
const canCheckSites = computed(() => isRunning.value && !isBusy.value && !switching.value && !actionBusy.value)
async function refresh(): Promise<void> {
  if (refreshBusy.value) return
  refreshBusy.value = true
  refreshError.value = ''
  try { await Promise.all([refreshFacts(), refreshGuard(), refreshSplit(), refreshSites()]) }
  catch { if (owner) refreshError.value = 'Не удалось обновить все сведения. Смотрите состояние каждого источника ниже.' }
  finally { if (owner) refreshBusy.value = false }
}
async function runSites(): Promise<void> {
  if (!canCheckSites.value) return
  actionBusy.value = true
  actionError.value = ''
  actionNote.value = ''
  try {
    const result = await api.runAdaptiveCheck()
    if (!owner) return
    if (result === null) { actionError.value = 'Результат проверки сайтов недоступен.'; return }
    if (!result.sites.length) {
      actionError.value = result.note || 'Новая выборка не получена. Сохранённые ответы относятся к прошлым наблюдениям.'
      return
    }
    actionNote.value = result.note
    // Retire any GetAdaptiveStatus begun before completion; never assign its ref directly.
    await invalidateSites()
    if (owner && hasSitesInterest() && !sitesLoading.value && adaptive.value === null) actionError.value = 'Команда завершилась, но обновлённый отчёт не получен. Повторите загрузку.'
  } catch { if (owner) actionError.value = 'Проверка сайтов не завершена. Состояние сервера и причина ошибки не установлены.' }
  finally { if (owner) actionBusy.value = false }
}
const classLabels: Record<string, string> = { mobile: 'Мобильный интерфейс', wifi: 'Wi-Fi', ethernet: 'Ethernet' }
const controlObservation = computed(() => facts.value?.egress === null ? null : facts.value?.egress?.osControl)
const socksObservation = computed(() => facts.value?.egress === null ? null : facts.value?.egress?.socks)
const rows = computed(() => {
  const f = facts.value
  return [
    { label: 'Подключение', value: f?.connectionType, source: 'Снимок интерфейса Windows' },
    { label: 'Класс интерфейса', value: f?.netClass ? classLabels[f.netClass] ?? f.netClass : '', source: 'Тип адаптера; не оценка транспорта или CGNAT' },
    { label: 'Название Wi-Fi', value: f?.ssid, source: 'Локальное наблюдение Windows' },
    { label: 'Локальный IP', value: f?.localIP, source: f?.localRouteTarget ? `Локальный выбор маршрута ОС к ${f.localRouteTarget}` : 'Привязка маршрута не передана' },
    { label: 'IP контрольного HTTPS-ответа', value: f?.publicIP || fmtEgressObservation(controlObservation.value), source: `${f?.publicIP ? `${fmtEgressObservation(controlObservation.value)}. ` : ''}${fmtEgressObservationTime(controlObservation.value)}. Используется маршрут ОС; внешний VPN не исключён.` },
    { label: 'Организация для этого IP', value: f?.isp, source: 'Ответ внешнего сервиса; не идентификация провайдера Wi-Fi' },
    { label: 'Страна для этого IP', value: f?.country, source: f?.countryOrigin || 'Источник страны не передан' },
    { label: 'IP HTTPS-ответа через прокси', value: f?.tunnelIP || fmtEgressObservation(socksObservation.value), source: `${f?.tunnelIP ? `${fmtEgressObservation(socksObservation.value)}. ` : ''}${fmtEgressObservationTime(socksObservation.value)}.` },
  ]
})
</script>

<template>
  <div class="view">
    <PageHeading title="Контекст сети" description="Наблюдения Windows и ответы отдельных сервисов. Тип интерфейса не определяет лучший транспорт.">
      <ActionButton :disabled="loading" :busy="refreshBusy" @click="refresh">{{ loading ? 'Обновление…' : 'Обновить наблюдения' }}</ActionButton>
    </PageHeading>
    <p v-if="refreshError" class="error" role="status">{{ refreshError }}</p>

    <section class="network-observations">
      <div class="section-heading"><h2>Снимок подключения</h2><ActionButton :disabled="factsLoading" :busy="factsLoading" @click="refreshSource('facts')">{{ factsLoading ? 'Ожидается ответ…' : 'Обновить снимок' }}</ActionButton></div>
      <p v-if="!facts" class="state-note" role="status">{{ factsLoading ? 'Получаем сетевые сведения…' : factsError || 'Сетевой снимок недоступен.' }}</p>
      <p class="source-time">Начало сбора: {{ fmtDateTime(facts?.checkedAt) }} · Получено экраном: {{ fmtDateTime(factsReceivedAt) }}. Возраст отдельных попыток указан на момент передачи; срок применимости и сетевая эпоха не переданы.</p>
      <dl class="facts-list">
        <div v-for="row in rows" :key="row.label"><dt>{{ row.label }}</dt><dd>{{ row.value || 'Нет данных' }}<small>{{ row.source }}</small></dd></div>
      </dl>
      <p class="state-note">Исходы относятся к отдельным HTTPS-попыткам. Их длительность не является RTT или скоростью VPN. Системный DNS, UDP, IPv6 и защита остальных приложений этими ответами не подтверждаются.</p>
    </section>

    <div class="context-grid">
      <section class="local-context">
        <div class="section-heading"><h2>Локальные настройки</h2><ActionButton :disabled="guardLoading" :busy="guardLoading" @click="refreshSource('guard')">Обновить локальные сведения</ActionButton></div>
        <p v-if="!netGuard" class="state-note">{{ guardLoading ? 'Получаем наблюдения прокси…' : guardError || 'Наблюдения прокси недоступны.' }}</p>
        <dl v-else class="facts-list">
          <div><dt>Системный прокси</dt><dd>{{ netGuard.proxy.enabled ? netGuard.proxy.server || 'Включён, адрес не передан' : 'Выключен' }}</dd></div>
          <div><dt>Порт из настройки</dt><dd>{{ netGuard.proxy.port > 0 ? fmtLoopbackObservation(netGuard.loopbacks, netGuard.proxy.port) : 'Порт не определён' }}</dd></div>
          <div><dt>Порт 127.0.0.1:1080</dt><dd>{{ fmtLoopbackObservation(netGuard.loopbacks, 1080) }}</dd></div>
        </dl>
        <p class="state-note">Источник — наблюдение локального IPv4/TCP. Неизвестный исход не означает отсутствие ответа. Порт не устанавливает владельца процесса, работоспособность протокола или защиту трафика.</p>
        <button class="btn" @click="navTo('netguard')">Локальное восстановление</button>
      </section>
      <section class="split-context">
        <div class="section-heading"><h2>Имена процессов в исключениях</h2><ActionButton :disabled="splitLoading" :busy="splitLoading" @click="refreshSource('split')">Обновить список</ActionButton></div>
        <p v-if="splitData === null" class="state-note">{{ splitLoading ? 'Получаем список…' : splitError || 'Список недоступен.' }}</p>
        <ul v-else-if="splitData.length" class="process-list"><li v-for="name in splitData" :key="name" class="mono">{{ name }}</li></ul>
        <p v-else class="state-note">Получен пустой список. Этот API не различает отсутствие правил и ошибку чтения профиля.</p>
        <p class="state-note">Имена не подтверждают фактический охват процессов. Список может описывать применённую или загруженную конфигурацию; идентификатор сессии и effective policy не переданы.</p>
        <p class="state-note">Режим процесса: {{ state?.proxyMode ?? 'не сообщён' }}. В режиме прокси перехват всех приложений не обещается.</p>
      </section>
    </div>

    <section class="sites-section">
      <div class="section-heading"><h2>Последние ответы сайтов</h2><div class="section-actions">
        <ActionButton :disabled="sitesLoading" :busy="sitesLoading" @click="refreshSource('sites')">Обновить сведения об отчёте</ActionButton>
        <ActionButton :disabled="!canCheckSites" :busy="actionBusy" @click="runSites">{{ actionBusy ? 'Проверка выполняется…' : 'Проверить через канал' }}</ActionButton>
      </div></div>
      <p class="state-note">Это отдельные HTTPS-запросы через прокси. Привязка к текущим сети, каналу и срок действия не переданы. Таймаут или HTTP 403 не устанавливают DPI и не подтверждают необходимость другого выхода.</p>
      <p v-if="!isRunning" class="state-note">Для новой проверки подключите VPN на главной странице.</p>
      <p v-if="actionError" class="error" role="status">{{ actionError }}</p>
      <p v-if="actionNote" class="state-note" role="status">{{ actionNote }}</p>
      <p v-if="adaptive === null" class="state-note">{{ sitesLoading ? 'Получаем сохранённые ответы…' : sitesError || 'Сохранённые ответы недоступны.' }}</p>
      <ul v-else-if="adaptive.sites.length" class="site-list" :aria-busy="actionBusy || sitesLoading">
        <li v-for="site in adaptive.sites" :key="site.domain">
          <div class="site-title"><strong>{{ site.domain }}</strong><span>{{ site.code === null ? 'Код не передан' : site.code === 0 ? 'HTTP-ответ не получен' : `HTTP ${site.code}` }}</span></div>
          <p>{{ site.detail || 'Подробности не переданы.' }}</p>
          <small>{{ fmtDateTime(site.checkedAt) }}<template v-if="site.challenge"> · Сообщён маркер challenge; причина ограничения не установлена</template></small>
        </li>
      </ul>
      <p v-else class="state-note">Список ответов пуст. Измеренные результаты отсутствуют.</p>
      <details v-if="adaptive" class="declared-routing">
        <summary>Заявленная конфигурация дополнительного выхода</summary>
        <p class="state-note">Список WARP-доменов возвращает приложение. Соответствие текущему серверу и применение этих правил не измерены.</p>
        <p class="mono">{{ adaptive.warpDomains.length ? adaptive.warpDomains.join(', ') : 'Домены не перечислены' }}</p>
        <p v-if="adaptive.note" class="state-note">Сообщение приложения: {{ adaptive.note }}</p>
      </details>
    </section>
    <HintBox kind="info">Контрольный запрос без системного прокси всё равно использует маршрут ОС. Совпадение или различие внешних IP не подтверждает обход внешнего VPN или отсутствие локальных утечек.</HintBox>
  </div>
</template>

<style scoped>
.view { display: flex; flex-direction: column; gap: 20px; min-width: 0; overflow-wrap: anywhere; }
h2 { margin: 0; font-size: 17px; }
.network-observations, .sites-section { padding: 20px; border: 1px solid var(--border); border-radius: var(--radius); background: var(--panel); min-width: 0; }
.section-heading { display: flex; flex-wrap: wrap; justify-content: space-between; align-items: baseline; gap: 12px; }
.source-time, .state-note, .muted { color: var(--muted); font-size: 12.5px; line-height: 1.65; }
.source-time { margin: 12px 0; }
.facts-list { margin: 12px 0; }
.facts-list > div { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 2fr); gap: 12px 20px; border-bottom: 1px solid var(--border); padding: 12px 0; }
dt { color: var(--muted); font-size: 12.5px; }
dd { color: var(--text); margin: 0; font-size: 13px; }
dd small { display: block; color: var(--muted); font-size: 11.5px; line-height: 1.6; margin-top: 5px; }
.context-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 24px; }
.context-grid > section { min-width: 0; }
.process-list { list-style: none; margin: 12px 0; padding: 0; display: flex; flex-wrap: wrap; gap: 8px; }
.process-list li { background: var(--panel-2); padding: 5px 10px; border-radius: 4px; }
.site-list { list-style: none; padding: 0; margin: 12px 0; }
.site-list li { border-bottom: 1px solid var(--border); padding: 14px 0; }
.site-title { display: flex; flex-wrap: wrap; justify-content: space-between; gap: 8px 16px; font-size: 13px; }
.site-title span, .site-list small { color: var(--muted); font-size: 12px; }
.site-list p { font-size: 13px; line-height: 1.6; color: var(--text-soft); margin: 8px 0; }
.declared-routing { margin-top: 16px; }
summary { padding: 8px 0; cursor: pointer; color: var(--text-soft); }
.btn { min-height: 40px; white-space: normal; }
.section-heading { flex-wrap: wrap; }
.section-actions { display: flex; flex-wrap: wrap; gap: 8px; }
.error { color: var(--red); font-size: 13px; line-height: 1.6; }
@media (max-width: 1180px) { .context-grid { grid-template-columns: minmax(0, 1fr); gap: 20px; } }
@media (max-width: 760px) { .facts-list > div { grid-template-columns: minmax(0, 1fr); gap: 6px; } .network-observations, .sites-section { padding: 16px; } }
</style>
