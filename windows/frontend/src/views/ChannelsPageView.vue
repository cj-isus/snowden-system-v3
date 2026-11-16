<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import PageHeading from '../components/PageHeading.vue'
import ActionButton from '../components/ActionButton.vue'
import HintBox from '../components/HintBox.vue'
import { useBackendState, initBackendState } from '../composables/backendState'
import { usePolling } from '../composables/polling'
import { useNav } from '../composables/nav'
import { useRemovedControlFocus } from '../composables/removedControlFocus'
import { api } from '../api/backend'
import { validationStatusLabels, fmtDateTime } from '../api/labels'
import type { ValidationStatus } from '../api/contract'
import { uiFailure } from '../api/failureMessages'

const { channels, channelsLoading, channelsError, channelsReceivedAt, refreshChannels, state, isRunning, isBusy, switching, pendingSwitchTarget, stopPending, canStop, commandMessage, selectChannel, stopVpn } = useBackendState()
const { data: health, error: healthError, loading: healthLoading } = usePolling(api.getChannelHealth, 10000, { pauseWhenHidden: true })
const { navTo } = useNav()
onMounted(() => { void initBackendState(); void refreshChannels() })
const page = ref<HTMLElement | null>(null)
const { focusBeforeRemoval } = useRemovedControlFocus(page, '#channel-action-heading')
const query = ref('')
const statusFilter = ref<'all' | ValidationStatus>('all')
const sortAsc = ref(true)
const statuses: ValidationStatus[] = ['planned', 'configured', 'locally-tested', 'live-verified', 'degraded', 'blocked', 'retired']
const list = computed(() => {
  const search = query.value.trim().toLowerCase()
  return (channels.value ?? []).filter(channel =>
    (statusFilter.value === 'all' || channel.validation === statusFilter.value) &&
    (!search || [channel.id, channel.server, channel.transport].some(value => value.toLowerCase().includes(search))),
  ).sort((a, b) => sortAsc.value ? a.id.localeCompare(b.id) : b.id.localeCompare(a.id))
})
const selectorCount = computed(() => channels.value?.filter(channel => channel.enabled).length ?? null)
const canSwitch = computed(() => api.hasOrderedLifecycle() && isRunning.value && !isBusy.value && !switching.value && !channelsLoading.value)
const switchReason = computed(() => !api.hasOrderedLifecycle() ? uiFailure.lifecycleUnavailable : switching.value ? 'Переключение ожидает результата.' : isBusy.value ? 'Выполняется операция подключения.' : !isRunning.value ? 'Переключение применяется к работающему соединению. Запуск доступен на главной.' : channelsLoading.value ? 'Список селектора обновляется.' : '')
function healthEntry(id: string) { return health.value?.channels.find(channel => channel.id === id) }
function reachability(id: string): string {
  const entry = healthEntry(id)
  return entry?.verdict === 'healthy' ? 'Последний вердикт положительный' : entry?.verdict === 'failing' ? 'Последний вердикт неуспешный' : 'Актуальный вердикт не передан'
}
function measuredAt(id: string): string {
  const entry = healthEntry(id)
  if (!entry) return '—'
  if (entry.verdict === 'healthy') return fmtDateTime(entry.lastOkAt)
  if (entry.verdict === 'failing') return fmtDateTime(entry.lastFailAt)
  const times = [entry.lastOkAt, entry.lastFailAt].filter(value => Number.isFinite(Date.parse(value)))
  return fmtDateTime(times.sort((a, b) => Date.parse(b) - Date.parse(a))[0])
}
function onSwitch(id: string): void {
  if (!canSwitch.value || id === state.value?.activeChannelId || !channels.value?.some(channel => channel.id === id && channel.enabled)) return
  void selectChannel(id)
}
function onRefresh(): void { if (!channelsLoading.value) void refreshChannels() }
function onStop(): void {
  if (!canStop.value) return
  focusBeforeRemoval()
  void stopVpn()
}
</script>

<template>
  <div ref="page" class="view">
    <PageHeading id="channel-action-heading" tabindex="-1" title="Реестр маршрутов" description="Состав профиля, допуск селектора и последние наблюдения показаны отдельно." :count="channels?.length">
      <ActionButton :disabled="channelsLoading" :busy="channelsLoading" @click="onRefresh">{{ channelsLoading ? 'Обновление…' : 'Обновить список' }}</ActionButton>
      <ActionButton v-if="isRunning || isBusy || switching" :disabled="!canStop" :busy="stopPending" @click="onStop">{{ stopPending ? 'Остановка ожидает ответа…' : 'Остановить' }}</ActionButton>
    </PageHeading>
    <p v-if="commandMessage" class="command-message" role="status" aria-atomic="true"><strong v-if="pendingSwitchTarget">Цель запроса: {{ pendingSwitchTarget }}. </strong>{{ commandMessage }}</p>

    <section class="channel-context">
      <h2>Что известно о выборе</h2>
      <dl class="context-facts">
        <div><dt>Объявлено в ответе реестра</dt><dd>{{ channels?.length ?? 'Нет данных' }}</dd></div>
        <div><dt>Допущено текущим селектором</dt><dd>{{ selectorCount ?? 'Нет данных' }}</dd></div>
        <div><dt>Активный ID по lifecycle</dt><dd>{{ state?.activeChannelId || 'Не сообщён' }}</dd></div>
        <div><dt>Подтверждение применения</dt><dd>Отдельная квитанция применения канала не передана.</dd></div>
      </dl>
      <p class="source-note">Получено экраном: {{ fmtDateTime(channelsReceivedAt) }}. Реестр, lifecycle и оценки получены отдельно; общего атомарного снимка нет. Это не срок действия профиля или текущей доступности. Метка lifecycle не является избранным маршрутом.</p>
      <p v-if="state?.blockedReason" class="blocked-note" role="status">Приложение ограничило запуск · код {{ state.blockedReason }}. Изучите результат проверок перед новой попыткой.</p>
      <p v-if="switchReason" class="source-note">{{ switchReason }}</p>
      <div class="actions-row"><button class="btn" @click="navTo('dashboard')">Управление подключением</button><button class="btn" @click="navTo('tests')">Проверки пути</button></div>
    </section>

    <div class="filters">
      <label class="search-field">Поиск<input v-model="query" type="search" maxlength="256" placeholder="Имя, сервер или транспорт" spellcheck="false" /></label>
      <label>Статус реестра<select v-model="statusFilter"><option value="all">Все статусы</option><option v-for="status in statuses" :key="status" :value="status">{{ validationStatusLabels[status] }}</option></select></label>
      <button class="btn" :aria-label="`Изменить сортировку. Сейчас ${sortAsc ? 'А — Я' : 'Я — А'}`" @click="sortAsc = !sortAsc">Имя: {{ sortAsc ? 'А → Я' : 'Я → А' }}</button>
    </div>
    <p v-if="channels === null" class="empty" role="status">{{ channelsLoading ? 'Получаем список каналов…' : channelsError || 'Список недоступен. Повторите загрузку.' }}</p>
    <p v-else-if="!channels.length" class="empty" role="status">Приложение вернуло пустой реестр. Состав пригодных маршрутов этим не подтверждён.</p>
    <template v-else>
      <p class="source-note">Показано {{ list.length }} из {{ channels.length }}. Источник оценок: {{ fmtDateTime(health?.now) }} · Эпоха {{ health?.epoch ?? 'не сообщена' }}. {{ healthLoading ? 'Оценки обновляются.' : healthError ? 'Оценки недоступны.' : '' }}</p>
      <div class="table-scroll" tabindex="0" role="region" aria-label="Реестр и наблюдения каналов" :aria-busy="channelsLoading">
        <table>
          <caption>Статус реестра, допуск и наблюдение не являются одним подтверждением пути</caption>
          <thead><tr><th scope="col">Канал</th><th scope="col">Реестр</th><th scope="col">Допуск в селекторе</th><th scope="col">Метка lifecycle</th><th scope="col">Последнее наблюдение</th><th scope="col">Действие</th></tr></thead>
          <tbody>
            <tr v-for="channel in list" :key="channel.id">
              <th scope="row"><strong>{{ channel.id }}</strong><small>{{ channel.transport }} · {{ channel.server }}:{{ channel.port }}</small></th>
              <td>{{ validationStatusLabels[channel.validation] ?? channel.validation }}</td>
              <td>{{ channel.enabled ? 'Попытка переключения разрешена селектором' : 'Допуск не подтверждён' }}</td>
              <td>{{ channel.id === state?.activeChannelId ? 'Сообщён активным' : state ? 'Не отмечен активным' : 'Нет данных lifecycle' }}</td>
              <td>{{ reachability(channel.id) }}<small>{{ measuredAt(channel.id) }}</small></td>
              <td>
                <ActionButton :disabled="!canSwitch || !channel.enabled || channel.id === state?.activeChannelId"
                  :tabindex="!canSwitch && pendingSwitchTarget !== channel.id ? -1 : 0" :busy="pendingSwitchTarget === channel.id"
                  :aria-label="`Переключиться на ${channel.id}`"
                  :title="pendingSwitchTarget === channel.id ? 'Это цель текущего запроса. Ответ и применение нового канала ещё не подтверждены.' : switchReason || (!channel.enabled ? 'Причина недопуска не передана; после запуска список может измениться.' : 'Окончательное решение и проверку выполняет приложение.')"
                  @click="onSwitch(channel.id)">{{ pendingSwitchTarget === channel.id ? 'Запрос переключения…' : channel.id === state?.activeChannelId ? 'По lifecycle' : 'Переключиться' }}</ActionButton>
              </td>
            </tr>
            <tr v-if="!list.length"><td colspan="6">По этим фильтрам каналов нет. Измените поиск или статус.</td></tr>
          </tbody>
        </table>
      </div>
    </template>
    <HintBox kind="info">Допуск относится к текущему селектору, а не только к enabled в профиле. До запуска отсутствие допуска не означает, что все маршруты выключены. Исторический live-статус, положительный вердикт и работающий lifecycle не подтверждают применение конкретной сессии или охват всей системы.</HintBox>
  </div>
</template>

<style scoped>
.view { display: flex; flex-direction: column; gap: 18px; min-width: 0; overflow-wrap: anywhere; }
h2 { margin: 0; font-size: 17px; }
.channel-context { padding: 20px; background: var(--panel); border: 1px solid var(--border); border-radius: var(--radius); }
.context-facts { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px 24px; margin: 18px 0; }
dt { color: var(--muted); font-size: 12px; }
dd { margin: 6px 0 0; color: var(--text-soft); font-size: 13px; }
.source-note, .empty { color: var(--muted); font-size: 12.5px; line-height: 1.65; margin: 8px 0; }
.command-message { margin: 0; padding: 8px 12px; border-left: 2px solid var(--accent); color: var(--text-soft); font-size: 13px; line-height: 1.65; }
.blocked-note { color: var(--yellow); font-size: 13px; line-height: 1.6; }
.filters, .actions-row { display: flex; align-items: flex-end; flex-wrap: wrap; gap: 12px; }
label { display: flex; flex-direction: column; gap: 7px; color: var(--muted); font-size: 12px; min-width: 0; }
.search-field { flex: 1 1 220px; }
input, select { min-height: 40px; min-width: 0; max-width: 100%; padding: 8px 12px; background: var(--panel); border: 1px solid var(--border-light); border-radius: var(--radius); color: var(--text); font: inherit; }
.btn { min-height: 40px; white-space: normal; }
.table-scroll { overflow-x: auto; min-width: 0; }
table { width: 100%; min-width: 850px; border-collapse: collapse; font-size: 12px; }
caption { text-align: left; color: var(--muted); padding: 10px 0; line-height: 1.6; }
th, td { padding: 14px 12px 14px 0; border-bottom: 1px solid var(--border); vertical-align: top; text-align: left; max-width: 220px; }
th { color: var(--muted); font-weight: 500; }
td, tbody th strong { color: var(--text-soft); }
small { display: block; color: var(--muted); font-size: 11px; line-height: 1.6; margin-top: 6px; }
@media (max-width: 760px) { .channel-context { padding: 16px; } .context-facts { grid-template-columns: minmax(0, 1fr); } }
</style>
