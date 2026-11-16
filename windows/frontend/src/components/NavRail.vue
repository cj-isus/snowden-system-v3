<script setup lang="ts">
/**
 * Разделы единого реестра. Их подпункты принадлежат ContextTabs.
 */
import { nextTick, ref, watch } from 'vue'
import Icon from './Icon.vue'

import { sections } from '../composables/navigation'
import type { PageId, SectionId } from '../composables/navigation'
export type NavId = PageId

const props = defineProps<{
  active: SectionId
}>()

const emit = defineEmits<{ navigate: [id: SectionId] }>()

const mainNav = sections

const navigation = ref<HTMLElement | null>(null)
watch(() => props.active, async id => {
  await nextTick()
  if (props.active === id) navigation.value?.querySelector<HTMLElement>('[aria-current="location"]')?.scrollIntoView({ block: 'nearest', inline: 'nearest', behavior: 'auto' })
}, { immediate: true })

function onClick(id: SectionId): void {
  emit('navigate', id)
}
</script>

<template>
  <aside class="sidebar">
    <nav ref="navigation" class="main-navigation" aria-label="Разделы приложения">
      <button
        v-for="item in mainNav"
        :key="item.id"
        type="button"
        class="navigation-item"
        :class="{ active: props.active === item.id }"
        :title="item.label + ' · ' + item.hint"
        :aria-label="item.label"
        :aria-current="active === item.id ? 'location' : undefined"
        @click="onClick(item.id)"
      >
        <span class="navigation-icon"><Icon :name="item.icon" :size="16" /></span>
        <span class="navigation-label">{{ item.label }}</span>
      </button>
    </nav>

  </aside>
</template>

<style scoped>
.sidebar {
  width: var(--navigation-width, 210px);
  flex: 0 0 var(--navigation-width, 210px);
  min-height: 0;
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
  overscroll-behavior: contain;
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
.navigation-item:focus-visible {
  clip-path: none;
  outline: 2px solid var(--accent);
  outline-offset: -4px;
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

@media (max-width: 900px) {
  .navigation-item { width: 52px; margin-inline: 10px; padding: 0; justify-content: center; }
  .navigation-icon { width: auto; }
  .navigation-label { display: none; }
}

@media (max-width: 700px) {
  .sidebar {
    width: 100%;
    flex: 0 0 auto;
    border-right: 0;
    border-top: 1px solid rgba(255, 98, 38, 0.24);
    order: 2;
  }
  .sidebar::after { display: none; }
  .main-navigation {
    display: flex;
    padding: 7px 8px;
    overflow-x: auto;
    overflow-y: hidden;
    justify-content: flex-start;
  }
  .navigation-item {
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
@media (forced-colors: active) {
  .navigation-item.active { border-inline-start: 3px solid Highlight; }
  .navigation-item:focus-visible { outline-color: Highlight; }
}
</style>
