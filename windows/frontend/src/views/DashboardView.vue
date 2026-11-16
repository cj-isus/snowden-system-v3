<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watchEffect } from 'vue'
import Icon from '../components/Icon.vue'
import ActionButton from '../components/ActionButton.vue'
import ProbeReportView from '../components/ProbeReportView.vue'
import ProcessModeSetting from '../components/ProcessModeSetting.vue'
import { useBackendState, initBackendState, refreshState } from '../composables/backendState'
import { useLifecycle } from '../composables/lifecycle'
import { useInfoFacts } from '../composables/infoFacts'
import { useNav } from '../composables/nav'
import { usePolling } from '../composables/polling'
import { useRemovedControlFocus } from '../composables/removedControlFocus'
import { api, netGuardStatus } from '../api/backend'
import { uiFailure } from '../api/failureMessages'
import { validationStatusLabels, lifecycleLabels, blockedReasonLabels, blockedReasonNextStep, lifecycleActionHelp, fmtDateTime, fmtObservationAge, probeStepStatus, isInformationalProbeStep, fmtEgressObservation, fmtEgressObservationTime } from '../api/labels'

const { state, stateLoaded, isRunning, isBusy, switching, primaryAction, stopPending, canStop, commandMessage } = useBackendState()
const { start, stop, runProbe } = useLifecycle()
const { facts } = useInfoFacts({ facts: true })
const egressObservation = computed(() => facts.value?.egress === null ? null : isRunning.value ? facts.value?.egress?.socks : facts.value?.egress?.osControl)
const egressIP = computed(() => isRunning.value ? facts.value?.tunnelIP : facts.value?.publicIP)
const { navTo } = useNav()
const { data: netGuard } = usePolling(netGuardStatus, 10000, { pauseWhenHidden: true })
onMounted(() => { void initBackendState() })

const page = ref<HTMLElement | null>(null)
useRemovedControlFocus(page, '#dashboard-primary-action')
const primaryControl = ref<InstanceType<typeof ActionButton> | null>(null)
function onPrimary(): void {
  if (primaryAction.value.disabled) return
  if (primaryAction.value.kind === 'start') void start()
  else void stop()
}
function onDisconnect(): void {
  if (!canStop.value) return
  primaryControl.value?.focus({ preventScroll: true })
  void stop()
}

const heading = computed(() => {
  const s = state.value
  if (stopPending.value) return 'Остановка ожидает результата'
  if (switching.value) return 'Смена канала ожидает результата'
  if (isBusy.value && s?.state === 'stopped') return 'Запуск ожидает результата'
  if (!stateLoaded.value) return 'Получаем состояние…'
  if (!s) return 'Состояние неизвестно'
  if (s.blockedReason === 'proxy_conflict' || !s.blockedReason && s.lifecycleActions?.explicitStartRequired) return 'Настройки прокси изменились'
  if (s.blockedReason) return s.state === 'stopped' ? 'Подключение остановлено, требуется действие' : 'Подключение требует внимания'
  if ((s.state === 'stopped' || s.state === 'error') && !s.lifecycleActions) return 'Доступность запуска неизвестна'
  if (s.state === 'stopped' && s.lifecycleActions?.startAllowed === false) return 'Запуск сейчас недоступен'
  return s.state === 'running' ? 'Соединение запущено' : lifecycleLabels[s.state]
})
const tone = computed(() => {
  const s = state.value
  if (s?.blockedReason || s?.state === 'error') return 'danger'
  if (isBusy.value || switching.value) return 'pending'
  return isRunning.value ? 'active' : 'idle'
})
const reason = computed(() => {
  const s = state.value
  if (stopPending.value) return 'Команда остановки ожидает завершения. Предыдущий статус не подтверждает её результат.'
  if (primaryAction.value.kind === 'stop' && s?.lifecycleActions?.stopBlockedBy) return lifecycleActionHelp[s.lifecycleActions.stopBlockedBy].reason
  if (switching.value) return 'Команда смены канала ожидает результата. Можно остановить операцию.'
  if (isBusy.value && s?.state === 'stopped') return 'Команда запуска ожидает результата. Можно остановить операцию до ответа приложения.'
  if (!stateLoaded.value) return 'Ожидается ответ приложения Windows.'
  if (!s) return 'Нет ответа приложения. Состояние подключения и защиты не подтверждено.'
  if (primaryAction.value.kind === 'start' && !api.hasOrderedLifecycle()) return uiFailure.lifecycleUnavailable
  if (s.blockedReason) return blockedReasonLabels[s.blockedReason]
  if ((s.state === 'stopped' || s.state === 'error') && s.lifecycleActions?.startBlockedBy) return lifecycleActionHelp[s.lifecycleActions.startBlockedBy].reason
  if (s.error) return s.error
  if ((s.state === 'stopped' || s.state === 'error') && !s.lifecycleActions) return 'Приложение не передало поддерживаемые сведения о команде запуска. Отсутствие запущенного ядра само по себе не является запретом запуска.'
  if (s.state === 'starting') return 'Идёт запуск и проверка канала. Можно остановить ожидающую операцию.'
  if (s.state === 'reloading') return 'Идёт смена канала. Результат предыдущей проверки не подтверждает новый путь.'
  if (s.state === 'stopping') return 'Ожидается результат остановки. Отправка команды ещё не означает завершение.'
  if (s.state === 'running') return 'Проверка ниже относится к указанным целям. Защита всей системы этим статусом не подтверждается.'
  return 'Подключение остановлено. Выберите канал и подключитесь, когда приложение будет готово.'
})
const nextStep = computed(() => {
  const s = state.value
  if (stopPending.value) return 'Дождитесь подтверждения остановки. Повторная команда сейчас не нужна.'
  if (primaryAction.value.kind === 'stop' && s?.lifecycleActions?.stopBlockedBy) return lifecycleActionHelp[s.lifecycleActions.stopBlockedBy].next
  if (!s) return 'Откройте приложение Windows и повторите загрузку состояния.'
  if (primaryAction.value.kind === 'start' && !api.hasOrderedLifecycle()) return 'Используйте совместимый комплект интерфейса и приложения. Устаревшая команда запуска не используется как обход.'
  if (isBusy.value || switching.value) return 'Дождитесь результата или остановите операцию.'
  if (s.blockedReason) return blockedReasonNextStep(s.blockedReason)
  if (s.lifecycleActions?.startBlockedBy && !isRunning.value) return lifecycleActionHelp[s.lifecycleActions.startBlockedBy].next
  if (!s.lifecycleActions && !isRunning.value) return 'Обновите состояние. Если сведения о команде не появятся, используйте совместимый комплект приложения Windows и интерфейса.'
  if (s.state === 'error') return 'Изучите причину и последний отчёт. Новый запуск — новая попытка, которая должна пройти обязательную проверку.'
  return isRunning.value ? 'Проверьте время и область последнего результата перед использованием соединения.' : 'Можно запустить новую попытку подключения. Результат будет подтверждаться обязательной проверкой пути.'
})
const showDisconnect = computed(() => stateLoaded.value && primaryAction.value.kind === 'start' &&
  (!state.value || state.value.state === 'error' || !!state.value.blockedReason || !!state.value.nextRetryAt))
const showRefresh = computed(() => stateLoaded.value && (stopPending.value || isBusy.value || switching.value || !state.value?.lifecycleActions ||
  !state.value.lifecycleActions.startAllowed && !state.value.lifecycleActions.stopAllowed))
const activeChannel = computed(() => state.value?.activeChannel ?? null)
const routingMode = computed(() => state.value?.proxyMode === 'tun' ? 'TUN · режим процесса' : state.value?.proxyMode === 'socks' ? 'Системный прокси · SOCKS/HTTP' : 'Не сообщён')

// One display clock, owned by this page. It does not refresh or validate facts.
const now = ref(Date.now())
let clock: ReturnType<typeof setInterval> | null = null
watchEffect(() => {
  const needed = !!state.value?.probeLastAt || !!state.value?.nextRetryAt
  if (needed && clock === null) {
    now.value = Date.now()
    clock = setInterval(() => { now.value = Date.now() }, 1000)
  } else if (!needed && clock !== null) {
    clearInterval(clock)
    clock = null
  }
})
onUnmounted(() => { if (clock !== null) clearInterval(clock) })
const probeAge = computed(() => fmtObservationAge(state.value?.probeLastAt, now.value))
const retry = computed(() => {
  const iso = state.value?.nextRetryAt
  if (!iso) return null
  const due = Date.parse(iso)
  if (!Number.isFinite(due)) return { time: 'время неизвестно', left: 'срок не распознан' }
  const seconds = Math.max(0, Math.ceil((due - now.value) / 1000))
  return { time: fmtDateTime(iso), left: seconds ? `через ${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, '0')}` : 'назначенное время наступило; результат ожидается' }
})
const probeSummary = computed(() => {
  const report = state.value?.probe
  if (state.value?.probeRunning) return { label: 'Проверка выполняется', cls: 'pending' }
  if (!report) return { label: 'Результата нет', cls: 'unknown' }
  if (report.scope?.historical) return { label: 'Исторический отчёт', cls: 'unknown' }
  const steps = report.steps.filter(step => !isInformationalProbeStep(step))
  if (!steps.length) return { label: 'Нет обязательных измерений', cls: 'unknown' }
  const statuses = steps.map(probeStepStatus)
  if (statuses.some(status => status === 'fail' || status === 'leak')) return { label: 'Есть неуспешные шаги', cls: 'danger' }
  if (report.ok && statuses.every(status => status === 'pass')) return { label: 'Обязательные шаги пройдены', cls: 'passed' }
  return { label: 'Результат неполный', cls: 'unknown' }
})
const checkDisabled = computed(() => !stateLoaded.value || !state.value || !isRunning.value || isBusy.value || switching.value || state.value.probeRunning)
</script>

<template>
  <div ref="page" class="dashboard">
    <section class="connection-overview" :class="tone">
      <div class="connection-heading" role="status" aria-live="polite" aria-atomic="true">
        <p class="eyebrow">Подключение</p>
        <h2 class="connection-title">{{ heading }}</h2>
        <p class="reason">{{ reason }}</p>
      </div>
      <div class="connection-actions">
        <ActionButton id="dashboard-primary-action" ref="primaryControl" class="primary" :disabled="primaryAction.disabled" :busy="stopPending" aria-describedby="connection-next-step" @click="onPrimary">
          <Icon :name="primaryAction.kind === 'start' ? 'play' : 'cross'" :size="17" />
          {{ primaryAction.label }}
        </ActionButton>
        <ActionButton v-if="showDisconnect" :disabled="!canStop" aria-describedby="disconnect-effect" @click="onDisconnect">Отключить VPN и отменить повторы</ActionButton>
        <ActionButton v-if="showRefresh" @click="refreshState">Обновить состояние</ActionButton>
        <button class="btn" @click="navTo('channels')">Каналы</button>
      </div>
      <p id="connection-next-step" class="next-step">{{ nextStep }}</p>
      <p v-if="commandMessage" class="next-step" role="status" aria-atomic="true">{{ commandMessage }}</p>
      <p v-if="showDisconnect" id="disconnect-effect" class="next-step">Ручное отключение запрашивает снятие защиты после очистки. Отправка команды не подтверждает её результат.</p>
      <p v-if="retry" class="retry-note">Автоповтор: {{ retry.left }} · {{ retry.time }}. Расписание не подтверждает успешный запуск.</p>
      <dl class="connection-facts">
        <div><dt>Режим</dt><dd>{{ routingMode }}</dd></div>
        <div><dt>{{ isRunning ? 'Сообщённый активный канал' : 'Выбранный канал' }}</dt><dd>{{ activeChannel?.id ?? 'Не сообщён' }}</dd></div>
        <div><dt>Транспорт</dt><dd>{{ activeChannel?.transport || 'Не сообщён' }}</dd></div>
      </dl>
      <p v-if="activeChannel" class="channel-note">
        {{ activeChannel.server }}:{{ activeChannel.port }} · Реестр: {{ validationStatusLabels[activeChannel.validation] ?? activeChannel.validation }}.
        Статус реестра не подтверждает текущую доступность.
      </p>
    </section>

    <section class="verification">
      <div class="section-title">
        <h2>Последняя проверка пути</h2>
        <span class="result-label" :class="probeSummary.cls" role="status">{{ probeSummary.label }}</span>
      </div>
      <p class="observation-time">
        <time v-if="state?.probeLastAt && fmtDateTime(state.probeLastAt) !== '—'" :datetime="state.probeLastAt">{{ fmtDateTime(state.probeLastAt) }}</time>
        <span v-else>Время не сообщено</span> · {{ probeAge }}
      </p>
      <p class="scope-note">HTTPS-цели и DoH через выбранный канал. Системный DNS, IPv6, UDP и отсутствие утечек других приложений этим отчётом не измеряются.</p>
      <p class="scope-note">Срок применимости не передан. Возраст отчёта сам по себе не подтверждает его актуальность для текущей сети и канала.</p>
      <details class="probe-details" :open="probeSummary.cls === 'danger'">
        <summary>Шаги и границы измерения</summary>
        <ProbeReportView :report="state?.probe ?? null" :running="state?.probeRunning ?? false" />
      </details>
      <div class="actions-row">
        <ActionButton :disabled="checkDisabled" :busy="state?.probeRunning ?? false" @click="runProbe">{{ state?.probeRunning ? 'Проверка выполняется…' : 'Проверить путь' }}</ActionButton>
        <button class="btn" @click="navTo('tests')">Все проверки</button>
      </div>
      <p v-if="!isRunning" class="scope-note">Для проверки действующего пути сначала нужно подключение.</p>
    </section>

    <section class="environment">
      <h2>Локальные наблюдения</h2>
      <dl class="environment-facts">
        <div><dt>Системный прокси</dt><dd>{{ netGuard ? (netGuard.proxy.enabled ? netGuard.proxy.server || 'Включён, адрес не сообщён' : 'Выключен') : 'Нет данных' }}</dd></div>
        <div><dt>Локальный адрес</dt><dd>{{ facts?.localIP || 'Нет данных' }}<small>{{ facts?.connectionType || 'Тип сети не сообщён' }}</small></dd></div>
        <div><dt>{{ isRunning ? 'Внешний IP в ответе через прокси' : 'Внешний IP контрольного запроса' }}</dt><dd>{{ egressIP || fmtEgressObservation(egressObservation) }}<small>{{ egressIP ? `${fmtEgressObservation(egressObservation)}. ` : '' }}{{ fmtEgressObservationTime(egressObservation) }}. IP не устанавливает локальный маршрут.</small></dd></div>
      </dl>
      <div class="actions-row">
        <button class="btn" @click="navTo('netguard')">Локальное восстановление</button>
        <button class="btn" @click="navTo('logs')">Открыть журнал</button>
      </div>
      <ProcessModeSetting id="dashboard-process-mode" />
      <p class="scope-note">Прокси охватывает приложения, которые его используют. Флаг режима процесса не подтверждает общий допуск трафика TUN.</p>
    </section>
  </div>
</template>

<style scoped>
.dashboard { display: grid; grid-template-columns: minmax(0, 1.15fr) minmax(0, 1fr); gap: 20px; min-width: 0; }
.connection-overview, .verification, .environment { min-width: 0; padding: 24px; border: 1px solid var(--border); border-radius: var(--radius); background: var(--panel); overflow-wrap: anywhere; }
.connection-overview { grid-column: 1 / -1; border-top: 2px solid var(--accent); background: linear-gradient(110deg, var(--panel-2), var(--panel)); }
.eyebrow { margin: 0 0 8px; color: var(--accent); font-size: 12px; letter-spacing: .08em; text-transform: uppercase; }
.connection-title { margin: 0; font-size: clamp(24px, 2rem, 36px); line-height: 1.2; }
h2 { margin: 0; font-size: 17px; line-height: 1.4; }
.reason { margin: 14px 0 0; max-width: 82ch; color: var(--text-soft); line-height: 1.65; }
.connection-actions, .actions-row { display: flex; flex-wrap: wrap; gap: 10px; margin-top: 18px; }
.btn { min-height: 40px; white-space: normal; text-align: left; }
.connection-actions .primary { min-width: 180px; justify-content: center; }
.next-step, .retry-note { margin: 12px 0 0; color: var(--muted); font-size: 13px; line-height: 1.6; }
.retry-note { color: var(--yellow); }
.connection-facts { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 16px 24px; border-top: 1px solid var(--border); margin: 24px 0 0; padding-top: 18px; }
dt { color: var(--muted); font-size: 12px; }
dd { margin: 5px 0 0; color: var(--text); font-size: 14px; }
.channel-note { margin: 12px 0 0; color: var(--muted); font-size: 12px; }
.section-title { display: flex; flex-wrap: wrap; align-items: baseline; gap: 10px; }
.result-label { font-size: 12px; color: var(--muted); }
.result-label.passed { color: var(--green-bright); }
.result-label.pending { color: var(--accent); }
.result-label.danger, .danger .connection-title { color: var(--red); }
.observation-time, .scope-note { margin: 12px 0 0; color: var(--muted); font-size: 12.5px; line-height: 1.6; }
.observation-time { color: var(--text-soft); font-variant-numeric: tabular-nums; }
.probe-details { margin-top: 18px; border-block: 1px solid var(--border); padding-block: 10px; }
.probe-details summary { cursor: pointer; padding-block: 5px; color: var(--text-soft); }
.probe-details :deep(.report), .probe-details :deep(.empty), .probe-details :deep(.running) { margin-top: 12px; }
.environment-facts { margin: 12px 0 0; }
.environment-facts > div { padding: 12px 0; border-bottom: 1px solid var(--border); }
.environment-facts small { display: block; color: var(--muted); margin-top: 5px; font-size: 12px; }
@media (max-width: 1180px) { .dashboard { grid-template-columns: minmax(0, 1fr); gap: 14px; } }
@media (max-width: 760px) { .connection-overview, .verification, .environment { padding: 18px; } .connection-facts { grid-template-columns: minmax(0, 1fr); gap: 14px; } }
</style>
