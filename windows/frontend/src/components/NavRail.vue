<script setup lang="ts">
/**
 * Боковой слайдер меню (sidebar concept): навигация по разделам.
 * Счётчики — факты из стора (число секретов), не выдумка.
 */
import { computed } from 'vue'
import Icon from './Icon.vue'

import { sections } from '../composables/navigation'
import type { PageId, SectionId } from '../composables/navigation'
export type NavId = PageId

const props = defineProps<{
  active: SectionId
}>()

const emit = defineEmits<{ navigate: [id: SectionId] }>()

const mainNav = sections

const activeLabel = computed(() => {
  return mainNav.find((i) => i.id === props.active)?.label ?? ''
})

function onClick(id: SectionId): void {
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
        :aria-label="item.label"
        :aria-current="active === item.id ? 'page' : undefined"
        @click="onClick(item.id)"
      >
        <span class="navigation-icon"><Icon :name="item.icon" :size="16" /></span>
        <span class="navigation-label">{{ item.label }}</span>
      </button>
    </nav>

    <div class="sidebar-bottom">
      <div class="active-hint" :title="activeLabel">{{ activeLabel }}</div>
    </div>
  </aside>
</template>

<style scoped>
.sidebar {
  width: 210px;
  flex: 0 0 210px;
  display: flex;
  flex-direction: column;
  border-right: 1px solid rgba(255, 98, 38, 0.28);
  background:
    linear-gradient(180deg, rgba(5, 12, 18, 0.98), rgba(4, 9, 14, 0.99)),
    repeating-linear-gradient(135deg, transparent 0 22px, rgba(255, 98, 38, 0.025) 23px 24px);
  z-index: 15;
  position: relative;
}

.sidebar::after {
  content: '';
  position: absolute;
  top: 16%;
  right: -1px;
  width: 1px;
  height: 38%;
  background: linear-gradient(transparent, #ff6226, transparent);
  box-shadow: 0 0 14px rgba(255, 98, 38, 0.58);
}

.main-navigation {
  padding: 22px 0 10px;
  flex: 1;
  overflow-y: auto;
}

.navigation-item {
  position: relative;
  min-height: 48px;
  display: flex;
  align-items: center;
  margin: 0 10px 3px;
  padding: 0 15px;
  border: 0;
  border-radius: 0;
  color: #9baabd;
  background: transparent;
  font-size: 13.5px;
  text-align: left;
  width: calc(100% - 20px);
  transition: background 0.18s, color 0.18s, box-shadow 0.18s;
  clip-path: polygon(0 0, calc(100% - 8px) 0, 100% 8px, 100% 100%, 8px 100%, 0 calc(100% - 8px));
}
.navigation-item:hover {
  color: #f6f8fa;
  background: rgba(255, 98, 38, 0.07);
}
.navigation-item.active {
  color: #fff5f0;
  background: linear-gradient(90deg, rgba(255, 98, 38, 0.22), rgba(255, 98, 38, 0.035));
  box-shadow: inset 3px 0 #ff6226, 0 0 28px rgba(255, 98, 38, 0.08);
}
.navigation-item.active .navigation-icon {
  color: #ff7b47;
  filter: drop-shadow(0 0 6px rgba(255, 98, 38, 0.55));
}
.navigation-item.sub {
  min-height: 42px;
  font-size: 12.5px;
  color: #8ba0b3;
}
.navigation-item.sub.active {
  color: #f1f6fa;
}

.navigation-icon {
  width: 31px;
  display: inline-flex;
  color: #bed0e5;
}
.navigation-label {
  flex: 1;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.nav-count {
  font-family: var(--mono);
  font-size: 12px;
  color: #b8c8d4;
  background: rgba(255, 98, 38, 0.12);
  border: 1px solid rgba(255, 98, 38, 0.2);
  border-radius: 999px;
  padding: 1px 7px;
}

.nav-divider {
  margin: 20px 18px 9px;
  border-top: 1px solid #1a2a37;
  padding-top: 10px;
}
.nav-divider span {
  font-size: 12px;
  letter-spacing: 1.7px;
  text-transform: uppercase;
  color: #55708a;
}

.sidebar-bottom {
  padding: 0 18px 16px 18px;
}

.active-hint {
  margin-top: 14px;
  margin-left: 6px;
  color: #566b7d;
  font-size: 12px;
  letter-spacing: .12em;
  text-transform: uppercase;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

@media (max-width: 1180px) {
  .sidebar { width: 72px; flex-basis: 72px; }
  .navigation-item { width: 52px; margin-inline: 10px; padding: 0; justify-content: center; }
  .navigation-icon { width: auto; }
  .navigation-label, .nav-count, .nav-divider span, .active-hint { display: none; }
  .nav-divider { margin: 16px 15px 8px; }
  .sidebar-bottom { display: none; }
}

@media (max-width: 700px) {
  .sidebar {
    width: 100%;
    flex: 0 0 62px;
    border-right: 0;
    border-top: 1px solid rgba(255, 98, 38, 0.24);
    order: 2;
  }
  .sidebar::after, .nav-divider, .sidebar-bottom { display: none; }
  .main-navigation {
    display: flex;
    padding: 7px 8px;
    overflow-x: auto;
    overflow-y: hidden;
    justify-content: space-around;
  }
  .navigation-item, .navigation-item.sub {
    min-width: 66px;
    width: auto;
    flex: 1 0 66px;
    min-height: 50px;
    margin: 0 2px;
    flex-direction: column;
    gap: 3px;
  }
  .navigation-label {
    display: block;
    width: 100%;
    font-size: 12px;
    text-align: center;
  }
}
</style>
