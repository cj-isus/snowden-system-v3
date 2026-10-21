<script setup lang="ts">
import { computed } from 'vue'
import Icon from './Icon.vue'
import { sections } from '../composables/navigation'
import type { PageId, SectionId } from '../composables/navigation'
const props = defineProps<{ active: PageId; section: SectionId }>()
const emit = defineEmits<{ navigate: [id: PageId] }>()
const tabs = computed(() => sections.find(section => section.id === props.section)!.pages)
function onKey(event: KeyboardEvent, index: number) {
  const keys = ['ArrowLeft', 'ArrowRight', 'Home', 'End']
  if (!keys.includes(event.key)) return
  event.preventDefault()
  const length = tabs.value.length
  const next = event.key === 'Home' ? 0 : event.key === 'End' ? length - 1 : (index + (event.key === 'ArrowRight' ? 1 : length - 1)) % length
  const target = tabs.value[next]
  emit('navigate', target.id)
  document.getElementById('tab-' + target.id)?.focus()
}
</script>
<template>
  <nav v-if="tabs.length > 1" class="context-tabs" role="tablist" aria-label="Подразделы">
    <button v-for="(tab, index) in tabs" :id="'tab-' + tab.id" :key="tab.id" class="context-tab" role="tab"
      :class="{ active: tab.id === active }" :aria-selected="tab.id === active" :aria-controls="'page-' + tab.id"
      :aria-label="tab.label" :tabindex="tab.id === active ? 0 : -1" @keydown="onKey($event, index)" @click="emit('navigate', tab.id)">
      <Icon :name="tab.icon" :size="18" />
      <span class="tab-copy"><strong>{{ tab.label }}</strong><small>{{ tab.caption }}</small></span>
    </button>
  </nav>
  <div v-else class="tabs-spacer" aria-hidden="true"></div>
</template>
<style scoped>
.context-tabs {
  width: min(1280px, 100%);
  margin: -1px auto 18px;
  min-height: 66px;
  display: flex;
  border: 1px solid #1b2a36;
  border-top-color: rgba(255, 98, 38, .35);
  background: rgba(5, 12, 18, .86);
  clip-path: polygon(0 8px, 8px 0, 100% 0, 100% calc(100% - 8px), calc(100% - 8px) 100%, 0 100%);
  overflow-x: auto;
}
.tabs-spacer { height: 18px; }

.context-tab {
  min-width: 185px;
  flex: 1 0 185px;
  min-height: 64px;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 18px;
  border: 0;
  border-right: 1px solid #172631;
  background: transparent;
  color: #9aabba;
  text-align: left;
  position: relative;
}
.context-tab:last-child { border-right: 0; }
.context-tab:hover { color: #fff; background: rgba(255, 98, 38, .055); }
.context-tab.active {
  color: #fff7f2;
  background: linear-gradient(180deg, rgba(255, 98, 38, .16), rgba(255, 98, 38, .045));
  box-shadow: inset 0 -2px #ff6226, inset 0 0 30px rgba(255, 98, 38, .06);
}
.context-tab.active > :first-child { color: #ff7540; filter: drop-shadow(0 0 6px rgba(255, 98, 38, .45)); }
.tab-copy { display: flex; flex-direction: column; min-width: 0; }
.tab-copy strong { font-size: 12.5px; font-weight: 600; white-space: nowrap; }
.tab-copy small { margin-top: 2px; color: #66798b; font-size: 12px; white-space: nowrap; }
.context-tab.active small { color: #9d7b6c; }

@media (max-width: 900px) {
  .context-tabs { min-height: 50px; margin-bottom: 12px; }
  .context-tab { min-width: 54px; flex-basis: 54px; min-height: 50px; padding: 0 15px; justify-content: center; }
  .tab-copy small { display: none; }
  .tab-copy strong { font-size: 11px; }
}

@media (max-width: 620px) {
  .tab-copy { display: flex; }
  .context-tab { flex: 0 0 auto; min-width: 120px; padding: 0 12px; }
}
</style>
