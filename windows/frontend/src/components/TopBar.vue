<script setup lang="ts">
/**
 * Верхняя панель: бренд, поиск по настройкам (Ctrl K), уведомления.
 * Кнопок окна (свернуть/развернуть/закрыть) здесь НЕТ сознательно: окно не
 * frameless — системная рамка Windows уже даёт эти кнопки, дубликат рядом
 * выглядел как «бесовщина» (V2-051). Если появится frameless-режим — вернуть
 * контролы сюда, а не держать два набора.
 * Версия — из факта сборки (wails.json), не выдумывается.
 */
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { APP_VERSION } from '../api/contract'
import { lifecycleLabels } from '../api/labels'
import { useBackendState } from '../composables/backendState'
import { uiLogLines } from '../composables/uiLog'
import Icon from './Icon.vue'
import type { IconName } from './Icon.vue'

const APP_VERSION_LABEL = `v${APP_VERSION}`

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
        <div class="brand-version">{{ APP_VERSION_LABEL }}</div>
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
  height: 58px;
  flex: 0 0 58px;
  display: flex;
  align-items: center;
  border-bottom: 1px solid #1b3449;
  background: linear-gradient(180deg, rgba(6, 18, 30, 0.96), rgba(6, 18, 30, 0.72));
  position: relative;
  z-index: 20;
  -webkit-user-select: none;
  user-select: none;
}

.brand {
  display: flex;
  align-items: center;
  padding: 0 28px;
  height: 100%;
}
.brand-logo {
  width: 25px;
  height: 34px;
  margin-right: 15px;
}
.brand-logo svg {
  width: 100%;
  height: 100%;
}
.brand-logo svg path:first-child {
  fill: #168cff;
}
.brand-logo svg path:last-child {
  fill: #46adff;
}
.brand-info {
  display: flex;
  flex-direction: column;
  margin-top: -1px;
}
.brand-name {
  font-size: 18px;
  line-height: 20px;
  font-weight: 650;
  letter-spacing: -0.45px;
  color: #f1f5f8;
}
.brand-version {
  margin-top: 2px;
  color: #8299ad;
  font-size: 10px;
}

.topbar-center {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-left: 10px;
}
.lifecycle {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 24px;
  padding: 0 10px;
  border-radius: 12px;
  font-size: 10.5px;
  border: 1px solid var(--border);
  background: #091826;
  color: #8ea4b7;
}
.lifecycle.ok {
  color: #42e8ad;
  border-color: var(--green-border);
  background: var(--green-soft);
}
.lifecycle.err {
  color: #ff6870;
  border-color: var(--red-border);
  background: var(--red-soft);
}
.lifecycle.busy {
  color: #51b8ff;
  border-color: var(--blue-border);
  background: var(--blue-soft);
}
.clock {
  color: #6d8497;
  font-size: 11px;
}

.header-actions {
  margin-left: auto;
  display: flex;
  align-items: center;
  padding-right: 26px;
  height: 100%;
}

.settings-search {
  width: 240px;
  height: 31px;
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 0 9px;
  color: #91a5b8;
  background: #091826;
  border: 1px solid #1d3850;
  border-radius: 7px;
  font-size: 11px;
}
.settings-search:hover {
  border-color: #2c4f6a;
  background: #0b1c2c;
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
  font-size: 9px;
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
  background: var(--blue-bright);
  border: 1.5px solid #0a1c2c;
  box-shadow: 0 0 7px rgba(66, 179, 255, 0.8);
}
</style>
