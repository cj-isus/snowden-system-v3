<script setup lang="ts">
/**
 * Корневая оболочка (concept): topbar + боковой слайдер меню + активный раздел.
 * Ctrl K — палитра поиска по разделам и действиям. Колокольчик — журнал событий
 * UI (факты, без секретов). Правило фактов (AGENTS.md §3.6) действует везде.
 */
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import TopBar from './components/TopBar.vue'
import NavRail from './components/NavRail.vue'
import type { NavId } from './components/NavRail.vue'
import ToastHost from './components/ToastHost.vue'
import Icon from './components/Icon.vue'
import type { IconName } from './components/Icon.vue'
import DashboardView from './views/DashboardView.vue'
import ChannelsPageView from './views/ChannelsPageView.vue'
import AutoProtectView from './views/AutoProtectView.vue'
import NetworkView from './views/NetworkView.vue'
import StatisticsView from './views/StatisticsView.vue'
import TestsView from './views/TestsView.vue'
import SecretsView from './views/SecretsView.vue'
import NetGuardView from './views/NetGuardView.vue'
import LogsView from './views/LogsView.vue'
import SettingsView from './views/SettingsView.vue'
import { useSecretsStore } from './composables/secretsStore'
import { uiLogLines } from './composables/uiLog'
import { fmtTime } from './api/labels'

const active = ref<NavId>('dashboard')
const { items } = useSecretsStore()

const secretsCount = computed(() => (items.value === null ? null : items.value.length))

function nav(id: NavId): void {
  active.value = id
}

// ---------- палитра поиска (Ctrl K) ----------

type SearchEntry = { label: string; hint: string; icon: IconName; target: NavId }

const searchEntries: SearchEntry[] = [
  { label: 'Главная — состояние защиты', hint: 'Power-кнопка, текущий канал, карта маршрута', icon: 'dashboard', target: 'dashboard' },
  { label: 'Каналы', hint: 'Таблица каналов, фильтры, переключение', icon: 'server', target: 'channels' },
  { label: 'Автозащита', hint: 'Сторож, правила авто-переключения, инциденты', icon: 'shield', target: 'autoprotect' },
  { label: 'Сеть', hint: 'Сетевая информация, NetGuard, диагностика', icon: 'pulse', target: 'network' },
  { label: 'Статистика', hint: 'Трафик по периодам, счётчики', icon: 'flask', target: 'statistics' },
  { label: 'Логи', hint: 'Журнал приложения и ядра', icon: 'logs', target: 'logs' },
  { label: 'Настройки — секреты', hint: 'Хранилище значений, fingerprint, аудит', icon: 'key', target: 'settings' },
  { label: 'Тестирование', hint: 'Проверки окружения и защищённого пути', icon: 'flask', target: 'tests' },
  { label: 'Секреты', hint: 'Карточки значений, проверка, живой тест', icon: 'key', target: 'secrets' },
  { label: 'NetGuard', hint: 'Авто-починка прокси и DNS', icon: 'pulse', target: 'netguard' },
]

const searchOpen = ref(false)
const searchQuery = ref('')
const searchIndex = ref(0)

const searchResults = computed<SearchEntry[]>(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return searchEntries
  return searchEntries.filter((e) => (e.label + ' ' + e.hint).toLowerCase().includes(q))
})

function openSearch(): void {
  searchOpen.value = true
  searchQuery.value = ''
  searchIndex.value = 0
}

function closeSearch(): void {
  searchOpen.value = false
}

function onSearchKey(e: KeyboardEvent): void {
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    searchIndex.value = Math.min(searchIndex.value + 1, searchResults.value.length - 1)
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    searchIndex.value = Math.max(searchIndex.value - 1, 0)
  } else if (e.key === 'Enter') {
    const hit = searchResults.value[searchIndex.value]
    if (hit) {
      nav(hit.target)
      closeSearch()
    }
  } else if (e.key === 'Escape') {
    closeSearch()
  }
}

function onGlobalKey(e: KeyboardEvent): void {
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
    e.preventDefault()
    if (searchOpen.value) closeSearch()
    else openSearch()
  } else if (e.key === 'Escape' && searchOpen.value) {
    closeSearch()
  }
}

onMounted(() => window.addEventListener('keydown', onGlobalKey))
onBeforeUnmount(() => window.removeEventListener('keydown', onGlobalKey))

// ---------- уведомления (последние события журнала UI) ----------

const notificationsOpen = ref(false)
const logLines = uiLogLines()
const recentNotifications = computed(() => logLines.value.slice(-8).reverse())

function toggleNotifications(): void {
  notificationsOpen.value = !notificationsOpen.value
}
function closeNotifications(): void {
  notificationsOpen.value = false
}

function levelIcon(level: string): IconName {
  if (level === 'error') return 'error'
  if (level === 'warn') return 'warn'
  if (level === 'info') return 'info'
  return 'info'
}
</script>

<template>
  <div class="app">
    <TopBar @open-search="openSearch" @open-notifications="toggleNotifications" />

    <div class="app-body">
      <NavRail :active="active" :secrets-count="secretsCount" @navigate="nav" />

      <main class="main-content">
        <DashboardView v-if="active === 'dashboard'" />
        <ChannelsPageView v-else-if="active === 'channels'" />
        <AutoProtectView v-else-if="active === 'autoprotect'" />
        <NetworkView v-else-if="active === 'network'" />
        <StatisticsView v-else-if="active === 'statistics'" />
        <TestsView v-else-if="active === 'tests'" />
        <SecretsView v-else-if="active === 'secrets'" />
        <NetGuardView v-else-if="active === 'netguard'" />
        <LogsView v-else-if="active === 'logs'" />
        <SettingsView v-else />
      </main>
    </div>

    <!-- Палитра поиска -->
    <div v-if="searchOpen" class="overlay" @click.self="closeSearch">
      <div class="palette card" role="dialog" aria-label="Поиск по настройкам">
        <div class="palette-input">
          <Icon name="logs" :size="14" />
          <input
            v-model="searchQuery"
            placeholder="Раздел или действие…"
            autofocus
            @keydown="onSearchKey"
          />
          <kbd>Esc</kbd>
        </div>
        <div class="palette-list">
          <button
            v-for="(e, i) in searchResults"
            :key="e.target"
            class="palette-item"
            :class="{ sel: i === searchIndex }"
            @mouseenter="searchIndex = i"
            @click="nav(e.target); closeSearch()"
          >
            <Icon :name="e.icon" :size="14" />
            <span class="pi-label">{{ e.label }}</span>
            <span class="pi-hint">{{ e.hint }}</span>
          </button>
          <div v-if="searchResults.length === 0" class="palette-empty">Ничего не найдено</div>
        </div>
      </div>
    </div>

    <!-- Уведомления: последние строки журнала UI (факты) -->
    <div v-if="notificationsOpen" class="overlay" @click.self="closeNotifications">
      <div class="notif-popover card" role="dialog" aria-label="Уведомления">
        <div class="notif-head">
          <strong>События</strong>
          <button class="notif-close" @click="closeNotifications">×</button>
        </div>
        <div v-if="recentNotifications.length === 0" class="notif-empty">
          Событий пока нет — это норма до первых действий.
        </div>
        <div v-for="(l, i) in recentNotifications" :key="i" class="notif-row" :class="l.level">
          <Icon :name="levelIcon(l.level)" :size="13" />
          <span class="notif-time mono">{{ fmtTime(l.t) }}</span>
          <span class="notif-text">{{ l.text }}</span>
        </div>
      </div>
    </div>

    <ToastHost />
  </div>
</template>

<style scoped>
.app {
  height: 100vh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: var(--app-glow);
}

.app::before {
  content: '';
  position: absolute;
  inset: 0;
  pointer-events: none;
  opacity: 0.23;
  background: linear-gradient(120deg, transparent 0 55%, rgba(38, 97, 137, 0.1) 55.1%, transparent 70%);
}

.app-body {
  flex: 1;
  display: flex;
  min-height: 0;
}

.main-content {
  flex: 1;
  min-width: 0;
  overflow-y: auto;
  padding: 14px 16px 22px 16px;
  background: radial-gradient(ellipse 600px 300px at 42% 8%, rgba(11, 53, 82, 0.12), transparent 70%);
}

/* Палитра поиска */
.overlay {
  position: fixed;
  inset: 0;
  background: rgba(3, 9, 16, 0.6);
  z-index: 90;
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding-top: 12vh;
}

.palette {
  width: 480px;
  max-width: 92vw;
  overflow: hidden;
  background: linear-gradient(145deg, rgba(10, 26, 40, 0.99), rgba(7, 19, 31, 0.99));
}
.palette-input {
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 11px 13px;
  border-bottom: 1px solid var(--border);
  color: #c0d2e0;
}
.palette-input input {
  flex: 1;
  border: 0;
  background: transparent;
  color: var(--text);
  font-size: 13px;
  outline: none;
}
.palette-input kbd {
  padding: 2px 5px;
  border: 1px solid #294359;
  border-radius: 4px;
  color: #8299ad;
  font-size: 9px;
  background: #0c1d2c;
}
.palette-list {
  max-height: 320px;
  overflow-y: auto;
  padding: 6px;
}
.palette-item {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 8px 10px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: #c6d3dd;
  font-size: 12px;
  text-align: left;
}
.palette-item.sel {
  background: rgba(20, 58, 88, 0.55);
  color: #eef4f8;
}
.pi-label {
  flex: 0 0 auto;
}
.pi-hint {
  margin-left: auto;
  color: #6d8497;
  font-size: 10px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.palette-empty {
  padding: 14px;
  color: #6d8497;
  font-size: 12px;
  text-align: center;
}

/* Уведомления */
.notif-popover {
  position: fixed;
  top: 62px;
  right: 20px;
  width: 360px;
  max-height: 50vh;
  overflow-y: auto;
  padding: 10px;
  background: linear-gradient(145deg, rgba(10, 26, 40, 0.99), rgba(7, 19, 31, 0.99));
  z-index: 95;
}
.notif-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 12px;
  color: #eef4f8;
  padding: 2px 4px 8px;
}
.notif-close {
  border: 0;
  background: transparent;
  color: #8299ad;
  font-size: 15px;
}
.notif-close:hover {
  color: #fff;
}
.notif-empty {
  padding: 10px;
  color: #70889d;
  font-size: 11px;
}
.notif-row {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 7px 6px;
  border-top: 1px solid #14293c;
  font-size: 11px;
  color: #a4b6c5;
}
.notif-row.error {
  color: #ff8a90;
}
.notif-row.warn {
  color: #ffd479;
}
.notif-time {
  color: #6d8497;
  flex: 0 0 auto;
}
.notif-text {
  word-break: break-word;
}
</style>
