<script setup lang="ts">
import { computed, onMounted } from 'vue'
import PageHeading from '../components/PageHeading.vue'
import HintBox from '../components/HintBox.vue'
import { useBackendState, initBackendState } from '../composables/backendState'
import { useInfoFacts } from '../composables/infoFacts'
import { usePolling } from '../composables/polling'
import { useNav } from '../composables/nav'
import { uiLogLines } from '../composables/uiLog'
import { api } from '../api/backend'
import { fmtDateTime, fmtObservationAge } from '../api/labels'
import type { ChannelHealthEntry } from '../api/contract'

const { state } = useBackendState()
const { failover: status, failoverLoading: statusLoading, failoverError, failoverReceivedAt, refreshFailover: refreshStatus } = useInfoFacts({ failover: true })
const { data: health, loading: healthLoading, error: healthError, receivedAt: healthReceivedAt, refresh: refreshHealth } = usePolling(api.getChannelHealth, 10000, { pauseWhenHidden: true })
const { data: dependencies, loading: dependenciesLoading, error: dependenciesError, refresh: refreshDependencies } = usePolling(api.getChannelIndependence, 10000, { pauseWhenHidden: true })
const { data: clusters, loading: clustersLoading, error: clustersError, refresh: refreshClusters } = usePolling(api.getIncidents, 10000, { pauseWhenHidden: true })
const { navTo } = useNav()
const logLines = uiLogLines()
const loading = computed(() => statusLoading.value || healthLoading.value || dependenciesLoading.value || clustersLoading.value)
async function refresh(): Promise<void> { await Promise.all([refreshStatus(), refreshHealth(), refreshDependencies(), refreshClusters()]) }
onMounted(() => { void initBackendState() })
const policyLabels: Record<string, string> = { closed: 'Наблюдение активно', open: 'Пауза после сбоя', 'half-open': 'Повторная попытка' }
const stateLabels: Record<string, string> = { HEALTHY: 'Положительный вердикт', SUSPECTED: 'Нужно подтверждение', CLASSIFYING: 'Анализ наблюдений', DEGRADED: 'Деградация', SWITCHING: 'Переключение', BLOCKED: 'Запуск ограничен', RECOVERING: 'Повторное подключение' }
const recoveryLabels: Record<string, string> = { HEALTHY: 'Положительный вердикт', PROBATION: 'Под наблюдением', RECOVERING: 'Восстановление', QUARANTINED: 'Карантин' }
const failureLabels: Record<string, string> = {
  reset_injected: 'RST · исторический ярлык', port_blackholed: 'Порт · историческая гипотеза', silent_freeze: 'Обрыв · причина неизвестна',
  tls_policing_443: 'TLS · историческая гипотеза', whitelist_mode: 'Allowlist · не подтверждён', protected_channel_unavailable: 'Путь не подтверждён',
  dns_path_failure: 'DoH-ответ не подтверждён', egress_divergence: 'Выходной IP отличается от ожидаемого', direct_leak_confirmed: 'Сообщён код локального нарушения',
  server_unreachable: 'Соединение отклонено', degraded: 'Разные исходы целей', observation_unavailable: 'Измерение не выполнено',
  measurement_endpoint_failure: 'Некорректный ответ измерителя', tls_verification_failed: 'Ошибка сертификата HTTPS-цели', tls_handshake_failed: 'TLS-рукопожатие не завершено',
  connection_failed: 'Ошибка соединения', request_timeout: 'Таймаут запроса', response_incomplete: 'Неполный ответ', unknown: 'Причина неизвестна',
}
function failureLabel(code: string): string { return failureLabels[code] ?? code }
const title = computed(() => !status.value ? (statusLoading.value ? 'Получаем состояние…' : 'Состояние неизвестно') : !status.value.enabled ? 'Сторож не запущен' : policyLabels[status.value.state] ?? 'Состояние политики не распознано')
const cooldown = computed(() => {
  const s = status.value
  if (!s || s.nextCheckIn === null) return 'Не сообщена'
  if (!s.enabled) return 'Сторож не запущен; время попытки не установлено'
  return s.nextCheckIn === 0 ? '0 в снимке: срок не задан или не применяется' : `${s.nextCheckIn} с в последнем снимке`
})
const epoch = computed(() => health.value?.epoch == null ? 'Не сообщена' : health.value.epoch === 0 ? 'Ещё не открыта' : String(health.value.epoch))
function verdictLabel(channel: ChannelHealthEntry): string {
  return channel.verdict === 'healthy' ? 'Положительный вердикт' : channel.verdict === 'failing' ? 'Неуспешный вердикт' : 'Актуального вердикта нет'
}
function observationAge(channel: ChannelHealthEntry): string {
  const times = [channel.lastOkAt, channel.lastFailAt].filter(iso => Number.isFinite(Date.parse(iso)))
  const last = times.sort((a, b) => Date.parse(b) - Date.parse(a))[0]
  return fmtObservationAge(last, Date.parse(health.value?.now ?? ''))
}
const relatedLogs = computed(() => logLines.value.filter(line => /failover|переключ|probe|BLOCKED|heal/i.test(line.text)).slice(-12).reverse())
</script>

<template>
  <div class="view">
    <PageHeading title="Адаптация" description="Решения и наблюдения приложения. Состояние политики не равно доступности канала или защите всей системы.">
      <button class="btn" :disabled="loading" @click="refresh">{{ loading ? 'Обновление…' : 'Обновить снимки' }}</button>
    </PageHeading>

    <section class="policy-overview">
      <div role="status" aria-live="polite" aria-atomic="true"><h2>{{ title }}</h2><p v-if="!status" class="state-note">{{ statusLoading ? 'Ожидается ответ приложения.' : failoverError || 'Снимок недоступен.' }}</p></div>
      <p class="source-time">Состояние сторожа · Получено экраном: {{ fmtDateTime(failoverReceivedAt) }}. Время исходного измерения и срок применимости не переданы.</p>
      <dl class="policy-facts">
        <div><dt>Активный канал по lifecycle</dt><dd>{{ state?.activeChannelId || 'Не сообщён' }}</dd></div>
        <div><dt>Причина выбора этого канала</dt><dd>Точное решение и набор кандидатов этим API не переданы.</dd></div>
        <div><dt>Состояние политики</dt><dd>{{ status?.state || 'Не сообщено' }}</dd></div>
        <div><dt>Адаптивный цикл</dt><dd>{{ status ? (status.smState ? stateLabels[status.smState] ?? status.smState : 'Не активен по снимку') : 'Нет данных' }}</dd></div>
        <div><dt>С какого момента</dt><dd>{{ fmtDateTime(status?.smSince) }}</dd></div>
        <div><dt>Пауза / следующая попытка</dt><dd>{{ cooldown }}</dd></div>
        <div><dt>Переключений за инцидент</dt><dd>{{ status?.switches ?? 'Не сообщено' }}</dd></div>
        <div><dt>Оставшийся бюджет попыток</dt><dd>Не передан. Число прошлых переключений не устанавливает доступный бюджет.</dd></div>
        <div><dt>Последняя причина</dt><dd>{{ status?.lastError || 'Не сообщена' }}<small v-if="status?.smClass">Код: {{ failureLabel(status.smClass) }}</small></dd></div>
      </dl>
      <div class="actions-row"><button class="btn" @click="navTo('dashboard')">Управление подключением</button><button class="btn" @click="navTo('tests')">Проверки пути</button></div>
    </section>

    <section class="health-section">
      <div class="section-heading"><h2>Оценки каналов</h2><span class="muted">Сетевая эпоха: {{ epoch }}</span></div>
      <p class="state-note">Вердикты относятся к сообщённой эпохе. Дедлайн каждого результата и причина выбора активного канала не переданы; успешная проверка не обещает будущую доступность.</p>
      <p class="source-time">Снимок оценок: {{ fmtDateTime(health?.now) }} · Начало эпохи: {{ fmtDateTime(health?.started) }} · Получено экраном: {{ fmtDateTime(healthReceivedAt) }}</p>
      <p v-if="!health" class="state-note">{{ healthLoading ? 'Получаем оценки…' : healthError || 'Оценки недоступны.' }}</p>
      <ul v-else-if="health.channels.length" class="channel-list">
        <li v-for="channel in health.channels" :key="channel.id">
          <div class="channel-title"><strong>{{ channel.id }}</strong><span :class="{ failed: channel.verdict === 'failing' }">{{ verdictLabel(channel) }}</span></div>
          <dl class="channel-facts">
            <div><dt>Транспорт</dt><dd>{{ channel.transport || 'Не сообщён' }}</dd></div>
            <div><dt>Наблюдения эпохи</dt><dd>Успешные {{ channel.ok ?? '—' }} · Неуспешные {{ channel.fail ?? '—' }}</dd></div>
            <div><dt>Последнее наблюдение</dt><dd>{{ observationAge(channel) }} на момент снимка</dd></div>
            <div><dt>Последний код</dt><dd>{{ channel.lastClass ? failureLabel(channel.lastClass) : 'Не сообщён' }}</dd></div>
            <div><dt>Восстановление</dt><dd>{{ channel.d2State ? recoveryLabels[channel.d2State] ?? channel.d2State : 'Не сообщено' }}<small v-if="channel.d2Note">{{ channel.d2Note }}</small></dd></div>
          </dl>
        </li>
      </ul>
      <p v-else class="state-note">В полученном снимке нет оценок каналов. Доступность не установлена.</p>
    </section>

    <details class="details-section">
      <summary>Зависимости каналов</summary>
      <p class="state-note">Сопоставление заявленных origin, ASN, exit, resolver, аккаунтов и доставки профиля. Индекс модели не является вероятностью, измеренной физической независимостью или допуском маршрута.</p>
      <p v-if="dependencies === null" class="state-note">{{ dependenciesLoading ? 'Получаем сравнения…' : dependenciesError || 'Сравнения недоступны.' }}</p>
      <ul v-else-if="dependencies.length" class="dependency-list">
        <li v-for="pair in dependencies" :key="pair.a + '|' + pair.b"><strong>{{ pair.a }} ↔ {{ pair.b }}</strong><span>Индекс: {{ pair.score === null ? 'не сообщён' : pair.score.toFixed(2) }}</span><p>{{ pair.sharedFdoms || 'Описание общих и неизвестных зависимостей не передано.' }}</p><small>{{ pair.belowLimit ? 'Приложение сообщает индекс ниже порога модели.' : 'Порог модели не подтверждает независимость или безопасное переключение.' }}</small></li>
      </ul>
      <p v-else class="state-note">Получен пустой список. Отсутствие пар и ошибка построения модели этим API не различаются.</p>
    </details>

    <details class="details-section">
      <summary>Сводка отказов</summary>
      <p class="state-note">Приложение группирует события по времени внутри эпохи. Совпадение по времени не устанавливает общую причину, DPI или согласованную блокировку.</p>
      <p v-if="clusters === null" class="state-note">{{ clustersLoading ? 'Получаем сводку…' : clustersError || 'Сводка недоступна. Причина отсутствия данных не передана.' }}</p>
      <ol v-else-if="clusters.length" class="incident-list">
        <li v-for="cluster in clusters" :key="cluster.clusterId">
          <strong>{{ cluster.channels.join(' · ') || 'Каналы не перечислены' }}</strong>
          <p>{{ cluster.isWave ? 'Временная группа' : 'Отдельное событие' }} · {{ cluster.count ?? 'Неизвестное число' }} событий · {{ fmtDateTime(cluster.since) }} — {{ fmtDateTime(cluster.until) }}</p>
          <p v-if="cluster.classes.length">{{ cluster.classes.map(failureLabel).join(' · ') }}</p>
          <small v-if="cluster.netClasses.length || cluster.isps.length">Сообщённый контекст: {{ [...cluster.netClasses, ...cluster.isps].join(' · ') }}</small>
        </li>
      </ol>
      <p v-else class="state-note">Приложение не вернуло событий для этой сводки. Неполнота наблюдений не исключает сбои.</p>
      <h2 class="related-title">Связанные строки журнала</h2>
      <ul v-if="relatedLogs.length" class="log-list"><li v-for="(line, index) in relatedLogs" :key="index"><time>{{ fmtDateTime(line.t) }}</time><p>{{ line.text }}</p></li></ul>
      <p v-else class="state-note">Совпадений по ключевым словам нет. Это не отсутствие инцидентов.</p>
      <button class="btn" @click="navTo('logs')">Полный журнал</button>
    </details>

    <details class="details-section">
      <summary>Границы автоматического выбора</summary>
      <ul class="policy-notes">
        <li>HTTPS/DoH-ответы измеряют конкретные цели; native DNS, IPv6, UDP и отсутствие утечек других приложений требуют отдельных результатов.</li>
        <li>Один таймаут не устанавливает DPI. Код старой гипотезы в журнале не превращается в измеренную причину.</li>
        <li>Параметры серии проверок, оставшиеся бюджеты и полный набор кандидатов не переданы экрану; UI не рассчитывает их по константам.</li>
        <li>Защищённый трафик не получает разрешение на прямой fallback из-за ошибки канала. Общий охват TUN и native приёмка остаются отдельными условиями.</li>
      </ul>
    </details>
    <HintBox kind="info">Снимки политики, оценок каналов и журнала получаются отдельно. Общего атомарного снимка выбора этот экран не получает; сравнивайте их время и эпоху.</HintBox>
  </div>
</template>

<style scoped>
.view { display: flex; flex-direction: column; gap: 20px; min-width: 0; overflow-wrap: anywhere; }
h2 { margin: 0; font-size: 17px; }
.policy-overview, .health-section { padding: 20px; background: var(--panel); border: 1px solid var(--border); border-radius: var(--radius); min-width: 0; }
.policy-overview { border-top: 2px solid var(--accent); }
.source-time, .state-note, .muted { color: var(--muted); font-size: 12.5px; line-height: 1.65; }
.policy-facts { margin: 14px 0; }
.policy-facts > div { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 2fr); gap: 12px 20px; padding: 12px 0; border-bottom: 1px solid var(--border); }
dt { color: var(--muted); font-size: 12.5px; }
dd { color: var(--text-soft); margin: 0; font-size: 13px; line-height: 1.6; }
dd small, .dependency-list small, .incident-list small { display: block; color: var(--muted); margin-top: 5px; font-size: 12px; line-height: 1.6; }
.actions-row, .section-heading { display: flex; flex-wrap: wrap; gap: 10px 16px; }
.section-heading { justify-content: space-between; align-items: baseline; }
.btn { min-height: 40px; white-space: normal; }
.channel-list, .dependency-list, .incident-list, .log-list { list-style: none; padding: 0; margin: 12px 0; }
.channel-list > li, .dependency-list > li, .incident-list > li, .log-list > li { padding: 16px 0; border-bottom: 1px solid var(--border); }
.channel-title { display: flex; flex-wrap: wrap; justify-content: space-between; gap: 8px 16px; font-size: 13px; }
.channel-title span { color: var(--muted); font-size: 12px; }
.channel-title .failed { color: var(--red); }
.channel-facts { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px 20px; margin: 12px 0 0; }
.details-section { min-width: 0; border-top: 1px solid var(--border); padding-top: 10px; }
summary { cursor: pointer; color: var(--text-soft); padding: 8px 0; font-size: 14px; }
.dependency-list strong, .dependency-list span { display: block; font-size: 13px; margin-bottom: 6px; }
.dependency-list p, .incident-list p, .log-list p { color: var(--text-soft); font-size: 13px; line-height: 1.6; margin: 8px 0; }
.related-title { margin-top: 20px; }
.log-list time { color: var(--muted); font-size: 12px; }
.policy-notes { color: var(--text-soft); padding-left: 20px; font-size: 13px; line-height: 1.7; }
.policy-notes li { margin: 8px 0; }
@media (max-width: 760px) { .policy-overview, .health-section { padding: 16px; } .policy-facts > div, .channel-facts { grid-template-columns: minmax(0, 1fr); gap: 7px; } }
</style>