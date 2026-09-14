<script setup lang="ts">
/**
 * Боковой слайдер меню (sidebar concept): навигация по разделам.
 * Счётчики — факты из стора (число секретов), не выдумка.
 */
import { computed } from 'vue'
import Icon from './Icon.vue'
import type { IconName } from './Icon.vue'

export type NavId =
  | 'dashboard'
  | 'channels'
  | 'autoprotect'
  | 'network'
  | 'statistics'
  | 'logs'
  | 'settings'
  | 'tests'
  | 'secrets'
  | 'netguard'

const props = defineProps<{
  active: NavId
  secretsCount: number | null
}>()

const emit = defineEmits<{ navigate: [id: NavId] }>()

type Item = { id: NavId; label: string; icon: IconName; hint: string; main?: boolean }

const mainNav: Item[] = [
  { id: 'dashboard', label: 'Главная', icon: 'dashboard', hint: 'Power-кнопка, состояние защиты, текущий канал', main: true },
  { id: 'channels', label: 'Каналы', icon: 'server', hint: 'Каналы активного конфига и их статусы', main: true },
  { id: 'autoprotect', label: 'Автозащита', icon: 'shield', hint: 'Сторож защищённого пути и авто-переключение', main: true },
  { id: 'network', label: 'Сеть', icon: 'pulse', hint: 'Сетевая информация и NetGuard', main: true },
  { id: 'statistics', label: 'Статистика', icon: 'flask', hint: 'Трафик, счётчики и тестирование', main: true },
  { id: 'logs', label: 'Логи', icon: 'logs', hint: 'Журнал приложения и ядра', main: true },
  { id: 'settings', label: 'Настройки', icon: 'key', hint: 'Секреты, профиль доставки, о защите', main: true },
]

const toolNav: Item[] = [
  { id: 'tests', label: 'Тестирование', icon: 'flask', hint: 'Проверка egress, DNS, каналов и HY2/UDP' },
  { id: 'secrets', label: 'Секреты', icon: 'key', hint: 'Локальное хранилище значений с хешами и проверкой' },
  { id: 'netguard', label: 'NetGuard', icon: 'pulse', hint: 'Авто-починка сети: прокси, DNS-DoH, задачи' },
]

const activeLabel = computed(() => {
  const all = [...mainNav, ...toolNav]
  return all.find((i) => i.id === props.active)?.label ?? ''
})

function onClick(id: NavId): void {
  emit('navigate', id)
}
</script>

<template>
  <aside class="sidebar">
    <nav class="main-navigation" aria-label="Основная навигация">
      <button
        v-for="item in mainNav"
        :key="item.id"
        class="navigation-item"
        :class="{ active: props.active === item.id }"
        :title="item.hint"
        @click="onClick(item.id)"
      >
        <span class="navigation-icon"><Icon :name="item.icon" :size="16" /></span>
        <span class="navigation-label">{{ item.label }}</span>
      </button>

      <div class="nav-divider" role="separator">
        <span>инструменты</span>
      </div>

      <button
        v-for="item in toolNav"
        :key="item.id"
        class="navigation-item sub"
        :class="{ active: props.active === item.id }"
        :title="item.hint"
        @click="onClick(item.id)"
      >
        <span class="navigation-icon"><Icon :name="item.icon" :size="15" /></span>
        <span class="navigation-label">{{ item.label }}</span>
        <span v-if="item.id === 'secrets' && props.secretsCount !== null" class="nav-count">{{ props.secretsCount }}</span>
      </button>
    </nav>

    <div class="sidebar-bottom">
      <div class="active-hint" :title="activeLabel">{{ activeLabel }}</div>
    </div>
  </aside>
</template>

<style scoped>
.sidebar {
  width: 178px;
  flex: 0 0 178px;
  display: flex;
  flex-direction: column;
  border-right: 1px solid #193247;
  background: linear-gradient(180deg, rgba(7, 21, 34, 0.97), rgba(5, 17, 29, 0.97));
  z-index: 15;
}


.main-navigation {
  padding-top: 20px;
  flex: 1;
  overflow-y: auto;
}

.navigation-item {
  position: relative;
  height: 38px;
  display: flex;
  align-items: center;
  margin: 0 12px 2px;
  padding: 0 12px;
  border: 0;
  border-radius: 8px;
  color: #9db0c1;
  background: transparent;
  font-size: 13px;
  text-align: left;
  width: calc(100% - 24px);
  transition: background 0.15s, color 0.15s;
}
.navigation-item:hover {
  color: #e9f2f7;
  background: rgba(20, 58, 88, 0.45);
}
.navigation-item.active {
  color: #f1f6fa;
  background: linear-gradient(90deg, #103b66, #0d2b49);
  box-shadow: inset 3px 0 #168cff, 0 5px 15px rgba(0, 0, 0, 0.12);
}
.navigation-item.active .navigation-icon {
  color: #46adff;
}
.navigation-item.sub {
  font-size: 12px;
  color: #8ba0b3;
}
.navigation-item.sub.active {
  color: #f1f6fa;
}

.navigation-icon {
  width: 26px;
  display: inline-flex;
  color: #c8d8e4;
}
.navigation-label {
  flex: 1;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.nav-count {
  font-family: var(--mono);
  font-size: 9.5px;
  color: #b8c8d4;
  background: #172d41;
  border-radius: 999px;
  padding: 1px 7px;
}

.nav-divider {
  margin: 12px 12px 6px;
  border-top: 1px solid #14293c;
  padding-top: 7px;
}
.nav-divider span {
  font-size: 8.5px;
  letter-spacing: 1.2px;
  text-transform: uppercase;
  color: #55708a;
}

.sidebar-bottom {
  padding: 0 18px 16px 18px;
}

.active-hint {
  margin-top: 14px;
  margin-left: 6px;
  color: #55708a;
  font-size: 8.5px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
</style>
