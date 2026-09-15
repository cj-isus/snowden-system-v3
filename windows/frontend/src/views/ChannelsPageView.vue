<script setup lang="ts">
/**
 * Каналы (страница, concept): поиск, фильтр статуса, сортировка по имени,
 * сводка по статусам и таблица каналов. Данные — только из backend (FR-002):
 * никаких выдуманных серверов/стран. Переключение — через selectChannel.
 */
import { computed, onMounted, ref } from 'vue'
import Icon from '../components/Icon.vue'
import HintBox from '../components/HintBox.vue'
import { useBackendState, initBackendState, selectChannel, switching } from '../composables/backendState'
import { validationStatusLabels } from '../api/labels'
import type { ValidationStatus } from '../api/contract'

const { channels, state, isRunning, isBusy } = useBackendState()

onMounted(() => {
  void initBackendState()
})

const search = ref('')
const statusFilter = ref<'all' | ValidationStatus>('all')
const sortAsc = ref(true)

const filters: { id: 'all' | ValidationStatus; label: string }[] = [
  { id: 'all', label: 'Все статусы' },
  { id: 'live-verified', label: 'проверен live' },
  { id: 'locally-tested', label: 'проверен локально' },
  { id: 'configured', label: 'настроен' },
  { id: 'degraded', label: 'деградирован' },
  { id: 'blocked', label: 'заблокирован' },
  { id: 'retired', label: 'выведен' },
]

const list = computed(() => {
  const arr = channels.value ?? []
  const q = search.value.trim().toLowerCase()
  const filtered = arr.filter((c) => {
    if (q && !(c.id.toLowerCase().includes(q) || c.server.toLowerCase().includes(q) || c.transport.toLowerCase().includes(q))) return false
    if (statusFilter.value !== 'all' && c.validation !== statusFilter.value) return false
    return true
  })
  return [...filtered].sort((a, b) => (sortAsc.value ? a.id.localeCompare(b.id) : b.id.localeCompare(a.id)))
})

const statusCounts = computed(() => {
  const arr = channels.value ?? []
  const m = new Map<ValidationStatus, number>()
  for (const c of arr) m.set(c.validation, (m.get(c.validation) ?? 0) + 1)
  return m
})

/** Клик по чипу = применить/снять фильтр статуса (UI-план v3). */
function toggleFilter(v: ValidationStatus): void {
  statusFilter.value = statusFilter.value === v ? 'all' : v
}

const canSwitch = computed(() => isRunning.value && !isBusy.value && !switching.value)

/** Честная причина недоступности переключения (UI-план v3: не «просто серая»). */
const switchBlockReason = computed(() => {
  if (switching.value) return 'идёт переключение…'
  if (isBusy.value) return 'идёт старт/остановка VPN'
  if (!isRunning.value) return 'VPN остановлен — переключение это операция работающего туннеля. Включите защиту на Главной.'
  return ''
})

const blockedLine = computed(() => {
  const r = state.value?.blockedReason
  return r || null
})

const blockedHint = computed(() => {
  const r = blockedLine.value
  if (r === 'no_validated_channel')
    return 'В селекторе нет ни одного канала со статусом «проверен live». Каналы «настроен» доказательств не дают: сначала live-проверка.'
  if (r === 'probe_failed') return 'Последний protected-probe не пройден: путь до сервера жив, но не доказан. Повторите запуск.'
  if (r === 'config_invalid') return 'Собранный конфиг не прошёл строгую валидацию ядра — это дефект конфигурации, запуск закрыт.'
  if (r === 'all_channels_failed') return 'Авто-переключение исчерпало кандидатов: все каналы недоступны. Запуск закрыт, чтобы трафик не пошёл напрямую.'
  return null
})

function statusClass(v: ValidationStatus): string {
  if (v === 'live-verified') return 'live'
  if (v === 'degraded') return 'degraded'
  if (v === 'blocked') return 'blocked'
  if (v === 'retired' || v === 'planned') return 'neutral'
  return 'configured'
}

async function onSwitch(id: string): Promise<void> {
  await selectChannel(id)
}
</script>

<template>
  <div class="view">
    <header class="head">
      <div class="section-title-group">
        <h2>Каналы</h2>
        <span class="count-badge">{{ channels ? channels.length : '—' }}</span>
      </div>
      <p class="sub">
        Только каналы активного конфига с их фактическим статусом валидации. В protected
        selector попадают исключительно проверенные каналы (fail-closed).
      </p>
    </header>

    <section class="controls card">
      <div class="channel-search">
        <Icon name="search" :size="13" />
        <input v-model="search" placeholder="Поиск каналов…" spellcheck="false" />
      </div>

      <select v-model="statusFilter" class="status-filter">
        <option v-for="f in filters" :key="f.id" :value="f.id">{{ f.label }}</option>
      </select>

      <button class="sort-button" :title="sortAsc ? 'Сортировка: А→Я' : 'Сортировка: Я→А'" @click="sortAsc = !sortAsc">
        <span>Имя</span>
        <span>{{ sortAsc ? '↑' : '↓' }}</span>
      </button>

      <div class="spacer"></div>

      <span v-if="statusCounts.size" class="status-summary">
        <button
          v-for="[k, n] in statusCounts"
          :key="k"
          class="channel-status as-filter"
          :class="[statusClass(k), { selected: statusFilter === k }]"
          :title="statusFilter === k ? 'Снять фильтр' : `Показать только: ${validationStatusLabels[k] ?? k}`"
          @click="toggleFilter(k)"
        >
          <span class="status-dot"></span>{{ validationStatusLabels[k] ?? k }}: {{ n }}
        </button>
      </span>
    </section>

    <div v-if="blockedLine" class="blocked-banner card">
      <Icon name="warn" :size="16" />
      <div>
        <strong>Запуск заблокирован (BLOCKED — {{ blockedLine }})</strong>
        <p>{{ blockedHint }}</p>
      </div>
    </div>

    <div v-if="channels === null" class="empty-block card">
      <Icon name="warn" :size="18" />
      <p>Список каналов недоступен: backend bindings не отвечают. Вымышленные серверы и страны не показываются.</p>
    </div>

    <div v-else-if="channels.length === 0" class="empty-block card">
      <Icon name="info" :size="18" />
      <p>В активном конфиге нет каналов. Protected selector пуст — запуск останется заблокированным (BLOCKED).</p>
    </div>

    <div v-else class="table-card card">
      <div class="table-wrapper">
        <table class="channels-table">
          <thead>
            <tr>
              <th></th>
              <th>Имя</th>
              <th>Статус</th>
              <th>Транспорт</th>
              <th>Сервер</th>
              <th>Порт</th>
              <th>Действия</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="c in list" :key="c.id" :class="{ 'selected-channel': c.id === state?.activeChannelId }">
              <td>
                <span class="favorite" :class="{ active: c.id === state?.activeChannelId }">{{ c.id === state?.activeChannelId ? '★' : '☆' }}</span>
              </td>
              <td><strong>{{ c.id }}</strong></td>
              <td>
                <span class="channel-status" :class="statusClass(c.validation)">
                  <span class="status-dot"></span>
                  {{ validationStatusLabels[c.validation] ?? c.validation }}
                </span>
              </td>
              <td>{{ c.transport }}</td>
              <td>{{ c.server }}</td>
              <td>{{ c.port }}</td>
              <td>
                <button v-if="c.id === state?.activeChannelId" class="table-action" disabled>Текущий</button>
                <button
                  v-else-if="c.enabled"
                  class="table-action"
                  :disabled="!canSwitch"
                  :title="canSwitch
                    ? 'Переключиться: новый конфиг пройдёт защищённый probe до активации'
                    : switchBlockReason || 'Переключение доступно при запущенном VPN'"
                  @click="onSwitch(c.id)"
                >
                  {{ switching ? 'Переключение…' : 'Переключиться' }}
                </button>
                <button v-else class="table-action disabled" disabled title="Канал выключен в дескрипторе конфига (enabled: false) — он не участвует ни в селекторе, ни в авто-переключении">Выключен</button>
              </td>
            </tr>
            <tr v-if="list.length === 0">
              <td colspan="7" class="no-match">Под фильтр ничего не подошло.</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <HintBox kind="info" title="Откуда берётся этот список">
      Дескрипторы каналов описаны в конфиге и подписанном metadata-envelope (FR-008) и попадают
      сюда через backend. Страны, серверы и outbound-теги не зашиваются в интерфейс — только
      данные активного конфига.
    </HintBox>
    <HintBox kind="warn" title="Статусы — это уровни доказательства">
      «настроен» ≠ «проверен live». Канал включается в protected selector только после
      live-проверки (probe + egress). Переключение проходит через Reload + обязательный probe
      (fail-closed).
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
  max-width: 680px;
}

.controls {
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 10px 11px;
}
.channel-search {
  width: 220px;
  height: 30px;
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 0 10px;
  border: 1px solid #1e3b52;
  border-radius: 6px;
  color: #8ea4b7;
  background: #091a29;
}
.channel-search input {
  flex: 1;
  min-width: 0;
  border: 0;
  background: transparent;
  color: var(--text);
  font-size: 11px;
  outline: none;
}
.channel-search input::placeholder {
  color: #5d788c;
}
.status-summary .as-filter { cursor: pointer; }
.status-summary .as-filter:hover { filter: brightness(1.15); }
.status-summary .as-filter.selected { outline: 1.5px solid var(--blue-bright); outline-offset: 1px; }

.blocked-banner {
  display: flex;
  gap: 10px;
  align-items: flex-start;
  padding: 11px 13px;
  border-color: var(--red-border);
  background: var(--red-soft);
}
.blocked-banner strong { color: var(--red); font-size: 12px; }
.blocked-banner p { margin: 3px 0 0; font-size: 11.5px; color: var(--text-soft); }
.blocked-banner svg { color: var(--red); flex: 0 0 auto; margin-top: 2px; }

.status-filter {
  width: 170px;
  height: 30px;
  padding: 0 9px;
  border: 1px solid #1e3b52;
  border-radius: 6px;
  color: #dce8ef;
  background: #091a29;
  font-size: 10.5px;
  outline: none;
}
.sort-button {
  height: 30px;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 0 10px;
  border: 1px solid #1e3b52;
  border-radius: 6px;
  color: #8ea4b7;
  background: #091a29;
  font-size: 10.5px;
}
.sort-button:hover {
  border-color: #2c4f6a;
}
.spacer {
  flex: 1;
}
.status-summary {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.empty-block {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 14px 16px;
  color: var(--muted);
  font-size: 12px;
}

.table-card {
  padding: 11px;
}
.table-wrapper {
  overflow: auto;
  border-radius: 5px;
  max-height: 480px;
}

.channels-table {
  width: 100%;
  border-collapse: collapse;
  table-layout: fixed;
  font-size: 11px;
}
.channels-table th {
  height: 30px;
  padding: 0 9px;
  text-align: left;
  color: #8298ab;
  font-weight: 500;
  background: #0c2031;
  border-bottom: 1px solid #1a344a;
  position: sticky;
  top: 0;
  z-index: 2;
}
.channels-table td {
  height: 36px;
  padding: 0 9px;
  color: #c6d3dd;
  border-bottom: 1px solid #152d40;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.channels-table tr:hover {
  background: rgba(20, 64, 94, 0.16);
}
.channels-table tr.selected-channel {
  background: linear-gradient(90deg, rgba(18, 99, 150, 0.13), rgba(15, 53, 80, 0.08));
}
.channels-table td strong {
  color: #e3ebf1;
  font-weight: 600;
}
.no-match {
  text-align: center;
  color: #6d8497;
}
.favorite {
  color: #8196aa;
  font-size: 15px;
}
.favorite.active {
  color: #ffc43c;
  text-shadow: 0 0 5px rgba(255, 196, 60, 0.25);
}

.channel-status {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  width: fit-content;
  border-radius: 5px;
  padding: 4px 7px;
  font-size: 9px;
  line-height: 1;
  white-space: nowrap;
}
.channel-status .status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
}
.channel-status.live {
  color: #42e8ad;
  border: 1px solid var(--green-border);
  background: var(--green-soft);
}
.channel-status.live .status-dot {
  background: var(--green);
}
.channel-status.configured {
  color: #51b8ff;
  border: 1px solid var(--blue-border);
  background: var(--blue-soft);
}
.channel-status.configured .status-dot {
  background: var(--blue);
}
.channel-status.degraded {
  color: #ffc643;
  border: 1px solid var(--yellow-border);
  background: var(--yellow-soft);
}
.channel-status.degraded .status-dot {
  background: var(--yellow);
}
.channel-status.blocked {
  color: #ff6870;
  border: 1px solid var(--red-border);
  background: var(--red-soft);
}
.channel-status.blocked .status-dot {
  background: var(--red);
}
.channel-status.neutral {
  color: #a1b2c0;
  border: 1px solid var(--gray-border);
  background: var(--gray-soft);
}
.channel-status.neutral .status-dot {
  background: #71869a;
}

.table-action {
  min-width: 110px;
  height: 28px;
  padding: 0 10px;
  border: 1px solid #27435b;
  border-radius: 5px;
  color: #dce7ee;
  background: #0b2031;
  font-size: 9.5px;
}
.table-action:hover:not(:disabled) {
  background: #102a3f;
}
.table-action.disabled,
.table-action:disabled {
  color: #61778a;
  border-color: #1a3042;
  background: #091724;
  cursor: default;
}
</style>
