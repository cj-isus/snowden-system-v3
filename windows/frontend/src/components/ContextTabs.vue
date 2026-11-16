<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import Icon from './Icon.vue'
import { sections } from '../composables/navigation'
import type { PageId, SectionId } from '../composables/navigation'
const props = defineProps<{ active: PageId; section: SectionId }>()
const emit = defineEmits<{ navigate: [id: PageId] }>()
const currentSection = computed(() => sections.find(section => section.id === props.section)!)
const tabs = computed(() => currentSection.value.pages)
const tablist = ref<HTMLElement | null>(null)
const focused = ref<PageId>(props.active)
function tabButton(id: PageId) {
  return tablist.value?.querySelector<HTMLButtonElement>('#tab-' + id)
}
function reveal(id: PageId) {
  tabButton(id)?.scrollIntoView({ block: 'nearest', inline: 'nearest', behavior: 'auto' })
}
watch(() => [props.section, props.active] as const, async ([section, active]) => {
  focused.value = active
  await nextTick()
  if (props.section === section && props.active === active) reveal(active)
}, { immediate: true })
function onFocus(id: PageId) { focused.value = id; reveal(id) }
function onFocusOut(event: FocusEvent) {
  if (!(event.relatedTarget instanceof Node) || !tablist.value?.contains(event.relatedTarget)) focused.value = props.active
}
function onKey(event: KeyboardEvent, index: number) {
  const keys = ['ArrowLeft', 'ArrowRight', 'Home', 'End']
  if (event.defaultPrevented || event.altKey || event.ctrlKey || event.metaKey || event.shiftKey || !keys.includes(event.key)) return
  const length = tabs.value.length
  const next = event.key === 'Home' ? 0 : event.key === 'End' ? length - 1 : (index + (event.key === 'ArrowRight' ? 1 : length - 1)) % length
  const target = tabs.value[next]
  if (!target) return
  const button = tabButton(target.id)
  if (!button) return
  event.preventDefault()
  // Focus does not mount a page or start its readers. Native Enter/Space and
  // click activate the tab after the user chooses the destination.
  focused.value = target.id
  button.focus({ preventScroll: true })
}
</script>
<template>
  <div v-if="tabs.length > 1" ref="tablist" class="context-tabs" role="tablist" aria-orientation="horizontal"
    :aria-label="'Подразделы: ' + currentSection.label" @focusout="onFocusOut">
    <button v-for="(tab, index) in tabs" :id="'tab-' + tab.id" :key="tab.id" type="button" class="context-tab" role="tab" :title="tab.caption"
      :class="{ active: tab.id === active }" :aria-selected="tab.id === active" :aria-controls="'page-' + tab.id"
      :aria-label="tab.label" :tabindex="tab.id === focused ? 0 : -1" @focus="onFocus(tab.id)" @keydown="onKey($event, index)" @click="emit('navigate', tab.id)">
      <Icon :name="tab.icon" :size="18" />
      <span class="tab-copy"><strong>{{ tab.label }}</strong><small>{{ tab.caption }}</small></span>
    </button>
  </div>
  <div v-else class="tabs-spacer" aria-hidden="true"></div>
</template>
<style scoped>
.context-tabs {
  width: min(1280px, 100%);
  margin: -1px auto 18px;
  min-height: 66px;
  display: flex;
  min-width: 0;
  border: 1px solid #1b2a36;
  border-top-color: rgba(255, 98, 38, .35);
  background: rgba(5, 12, 18, .86);
  clip-path: polygon(0 8px, 8px 0, 100% 0, 100% calc(100% - 8px), calc(100% - 8px) 100%, 0 100%);
  overflow-x: auto;
  overscroll-behavior-inline: contain;
}
.context-tabs:focus-within { clip-path: none; }
.tabs-spacer { height: 18px; }

.context-tab {
  min-width: max-content;
  flex: 1 0 auto;
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
.context-tab:focus-visible { outline: 2px solid var(--accent); outline-offset: -4px; }
.context-tab.active {
  color: #fff7f2;
  background: linear-gradient(180deg, rgba(255, 98, 38, .16), rgba(255, 98, 38, .045));
  box-shadow: inset 0 -2px #ff6226, inset 0 0 30px rgba(255, 98, 38, .06);
}
.context-tab.active > :first-child { color: #ff7540; filter: drop-shadow(0 0 6px rgba(255, 98, 38, .45)); }
.tab-copy { display: flex; flex-direction: column; min-width: 0; }
.tab-copy strong { font-size: 12.5px; font-weight: 600; white-space: nowrap; }
.tab-copy small { margin-top: 2px; color: var(--muted); font-size: 12px; white-space: nowrap; }
.context-tab.active small { color: var(--text-soft); }

@media (max-width: 1180px) {
  .context-tabs { min-height: 50px; margin-bottom: 12px; }
  .context-tab { min-height: 50px; padding: 10px 14px; gap: 9px; justify-content: center; }
  .tab-copy small { display: none; }
  .tab-copy strong { font-size: 12px; }
}

@media (max-width: 620px) {
  .context-tab { flex: 0 0 auto; padding: 10px 12px; }
}
@media (forced-colors: active) {
  .context-tab.active { border-bottom: 3px solid Highlight; }
  .context-tab:focus-visible { outline-color: Highlight; }
}
</style>
