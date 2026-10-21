<script setup lang="ts">
/**
 * Верхняя панель: бренд, поиск по настройкам (Ctrl K), уведомления.
 * Кнопок окна (свернуть/развернуть/закрыть) здесь НЕТ сознательно: окно не
 * frameless — системная рамка Windows уже даёт эти кнопки, дубликат рядом
 * выглядел как «бесовщина» (V2-051). Если появится frameless-режим — вернуть
 * контролы сюда, а не держать два набора.
 * Версия читается из запущенного backend (production ldflags).
 */
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { api } from '../api/backend'
import type { RuntimeInfo } from '../api/contract'
import { MOCK_ACTIVE } from '../api/preview'
import { lifecycleLabels } from '../api/labels'
import { useBackendState } from '../composables/backendState'
import { uiLogLines } from '../composables/uiLog'
import Icon from './Icon.vue'
import type { IconName } from './Icon.vue'

const runtimeInfo = ref<RuntimeInfo | null>(null)
const versionLabel = computed(() => runtimeInfo.value?.version ? `v${runtimeInfo.value.version}` : MOCK_ACTIVE ? 'dev-mock' : 'версия недоступна')

const { lifecycle } = useBackendState()

/* Точка-индикатор — факт: есть непрочитанные события журнала UI. Пропадает,
   когда журнал пуст, и возвращается при новых событиях (не «вечный зелёный»). */
const logLines = uiLogLines()
const unreadEvents = ref(false)
let seenCount = 0
let logWatchTimer: number | undefined

function syncUnread(): void {
  unreadEvents.value = logLines.value.length > seenCount
}

function markEventsSeen(): void {
  seenCount = logLines.value.length
  syncUnread()
}

const emit = defineEmits<{
  openSearch: []
  openNotifications: []
}>()

const clock = ref('')
let clockTimer: number | undefined

onMounted(() => {
  void api.getRuntimeInfo().then((v) => (runtimeInfo.value = v)).catch(() => (runtimeInfo.value = null))
  const tick = (): void => {
    clock.value = new Date().toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  }
  tick()
  clockTimer = window.setInterval(tick, 1000)
  // uiLogLines — общий ref, реактивность отслеживаем лёгким таймером.
  syncUnread()
  logWatchTimer = window.setInterval(syncUnread, 1000)
})
onUnmounted(() => {
  window.clearInterval(clockTimer)
  window.clearInterval(logWatchTimer)
})

function onNotifications(): void {
  markEventsSeen()
  emit('openNotifications')
}

const stateLabel = computed(() => {
  const s = lifecycle.value
  return s ? lifecycleLabels[s] : 'Нет данных'
})

const stateClass = computed(() => {
  const s = lifecycle.value
  if (s === 'running') return 'ok'
  if (s === 'error') return 'err'
  if (s === 'starting' || s === 'stopping' || s === 'reloading') return 'busy'
  return 'idle'
})

const statusIcon = computed<IconName>(() => {
  const s = lifecycle.value
  if (s === 'running') return 'shield'
  if (s === 'error') return 'error'
  if (s === 'starting' || s === 'stopping' || s === 'reloading') return 'refresh'
  return 'shield-warn'
})
</script>

<template>
  <header class="topbar">
    <div class="brand">
      <div class="brand-logo" aria-hidden="true">
        <svg viewBox="0 0 40 52">
          <path d="M20 1L38 12V28L20 39L2 28V12L20 1Z" />
          <path d="M20 8L32 15V25L20 32L8 25V15L20 8Z" />
        </svg>
      </div>
      <div class="brand-info">
        <div class="brand-name">snowden.system</div>
        <div class="brand-version">{{ versionLabel }}</div>
      </div>
    </div>

    <div class="topbar-center">
      <span class="lifecycle" :class="stateClass" :title="`Фактическое состояние: ${stateLabel}`">
        <Icon :name="statusIcon" :size="13" />
        {{ stateLabel }}
      </span>
      <span class="clock mono">{{ clock }}</span>
    </div>

    <div class="header-actions">
      <button class="settings-search" title="Поиск по настройкам (Ctrl K)" @click="emit('openSearch')">
        <span class="search-icon"><Icon name="search" :size="13" /></span>
        <span>Поиск по настройкам…</span>
        <kbd>Ctrl K</kbd>
      </button>

      <button class="notification-button" aria-label="Уведомления" title="События журнала" @click="onNotifications">
        <Icon name="bell" :size="17" />
        <span v-if="unreadEvents" class="notification-dot"></span>
      </button>
    </div>
  </header>
</template>

<style scoped>
.topbar {
  height: 76px;
  flex: 0 0 76px;
  display: flex;
  align-items: center;
  border-bottom: 1px solid rgba(255, 98, 38, 0.3);
  background:
    linear-gradient(90deg, rgba(5, 12, 18, 0.98), rgba(9, 17, 24, 0.92)),
    repeating-linear-gradient(135deg, transparent 0 32px, rgba(255, 98, 38, 0.025) 33px 34px);
  position: relative;
  z-index: 20;
  -webkit-user-select: none;
  user-select: none;
}

.brand {
  display: flex;
  align-items: center;
  width: 210px;
  padding: 0 24px;
  height: 100%;
}
.brand-logo {
  width: 28px;
  height: 38px;
  margin-right: 13px;
}
.brand-logo svg {
  width: 100%;
  height: 100%;
}
.brand-logo svg path:first-child {
  fill: transparent;
  stroke: #f4f7fa;
  stroke-width: 2;
}
.brand-logo svg path:last-child {
  fill: transparent;
  stroke: #ff6226;
  stroke-width: 1.5;
}
.brand-info {
  display: flex;
  flex-direction: column;
  margin-top: -1px;
}
.brand-name {
  font-size: 16px;
  line-height: 20px;
  font-weight: 650;
  letter-spacing: 0.02em;
  color: #f1f5f8;
}
.brand-version {
  margin-top: 2px;
  color: #6d7d8e;
  font-size: 12px;
  letter-spacing: .14em;
}

.topbar-center {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-left: 30px;
}
.lifecycle {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 34px;
  padding: 0 13px;
  border-radius: 2px;
  font-size: 11px;
  letter-spacing: .07em;
  text-transform: uppercase;
  border: 1px solid var(--border);
  background: #091826;
  color: #8ea4b7;
}
.lifecycle.ok {
  color: #ff9b72;
  border-color: var(--blue-border);
  background: var(--blue-soft);
  box-shadow: inset 2px 0 #ff6226, 0 0 22px rgba(255, 98, 38, .08);
}
.lifecycle.err {
  color: #ff6870;
  border-color: var(--red-border);
  background: var(--red-soft);
}
.lifecycle.busy {
  color: #ff9b72;
  border-color: var(--blue-border);
  background: var(--blue-soft);
}
.clock {
  color: #667789;
  font-size: 12px;
  letter-spacing: .08em;
}

.header-actions {
  margin-left: auto;
  display: flex;
  align-items: center;
  padding-right: 22px;
  height: 100%;
}

.settings-search {
  width: 260px;
  height: 38px;
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 0 9px;
  color: #91a5b8;
  background: rgba(7, 14, 21, 0.74);
  border: 1px solid #223340;
  border-radius: 4px;
  font-size: 11.5px;
}
.settings-search:hover {
  border-color: rgba(255, 98, 38, .48);
  background: rgba(255, 98, 38, .05);
}
.search-icon {
  color: #c0d2e0;
  display: inline-flex;
}
.settings-search kbd {
  margin-left: auto;
  padding: 2px 5px;
  border: 1px solid #294359;
  border-radius: 4px;
  color: #8299ad;
  font-size: 12px;
  background: #0c1d2c;
}

.notification-button {
  position: relative;
  width: 42px;
  height: 42px;
  margin-left: 13px;
  border: 0;
  background: transparent;
  color: #cbd8e2;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.notification-button:hover {
  color: #fff;
}
.notification-dot {
  position: absolute;
  top: 9px;
  right: 9px;
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #ff6226;
  border: 1.5px solid #0a1c2c;
  box-shadow: 0 0 9px rgba(255, 98, 38, 0.85);
}

@media (max-width: 1180px) {
  .brand { width: 72px; padding: 0 20px; }
  .brand-info { display: none; }
  .topbar-center { margin-left: 14px; }
  .settings-search { width: 42px; justify-content: center; }
  .settings-search > span:not(.search-icon), .settings-search kbd { display: none; }
}

@media (max-width: 820px) {
  .topbar { height: 62px; flex-basis: 62px; }
  .brand { width: 58px; padding: 0 14px; }
  .topbar-center { margin-left: 5px; }
  .clock { display: none; }
  .header-actions { padding-right: 8px; }
  .notification-button { margin-left: 2px; }
  .lifecycle { height: 30px; padding: 0 9px; font-size: 12px; }
}
</style>
