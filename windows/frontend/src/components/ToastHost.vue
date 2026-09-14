<script setup lang="ts">
/**
 * Отображение всплывающих уведомлений. Тексты уже безопасны (без секретов).
 */
import { toastsRef, dismissToast } from '../composables/toast'
import Icon from './Icon.vue'
import type { IconName } from './Icon.vue'

const toasts = toastsRef()

const ico: Record<string, IconName> = { ok: 'check', info: 'info', warn: 'warn', error: 'error' }
</script>

<template>
  <div class="toasts" aria-live="polite">
    <TransitionGroup name="toast">
      <div v-for="t in toasts" :key="t.id" class="toast" :class="t.kind">
        <Icon :name="ico[t.kind]" :size="15" />
        <span class="toast-text">{{ t.text }}</span>
        <button class="toast-x" title="Скрыть" @click="dismissToast(t.id)">
          <Icon name="cross" :size="13" />
        </button>
      </div>
    </TransitionGroup>
  </div>
</template>

<style scoped>
.toasts {
  position: fixed;
  right: 18px;
  bottom: 18px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  z-index: 100;
  max-width: 420px;
}

.toast {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 11px 12px;
  border-radius: 10px;
  background: var(--bg-panel);
  border: 1px solid var(--border);
  box-shadow: 0 12px 32px #00000066;
  font-size: 13px;
}
.toast.ok {
  border-color: #3ecf8e55;
}
.toast.ok :first-child {
  color: var(--ok);
}
.toast.info :first-child {
  color: var(--accent);
}
.toast.warn {
  border-color: #f0b42955;
}
.toast.warn :first-child {
  color: var(--warn);
}
.toast.error {
  border-color: #ff636355;
}
.toast.error :first-child {
  color: var(--danger);
}

.toast-text {
  flex: 1;
  color: var(--text);
}

.toast-x {
  border: none;
  background: transparent;
  color: var(--text-faint);
  cursor: pointer;
  padding: 2px;
  border-radius: 4px;
}
.toast-x:hover {
  color: var(--text);
  background: var(--bg-hover);
}

.toast-enter-active,
.toast-leave-active {
  transition: all 0.18s ease;
}
.toast-enter-from,
.toast-leave-to {
  opacity: 0;
  transform: translateY(8px);
}
</style>
