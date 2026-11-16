<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { pages, sectionForPage } from '../composables/navigation'
import type { PageId } from '../composables/navigation'
import DialogFrame from './DialogFrame.vue'
import Icon from './Icon.vue'
const emit = defineEmits<{ close: []; navigate: [id: PageId] }>()
const query = ref('')
const index = ref(0)
const list = ref<HTMLElement | null>(null)
const destinations = pages.map(page => ({ ...page, section: sectionForPage(page.id).label }))
const results = computed(() => {
  const value = query.value.trim().toLocaleLowerCase('ru')
  return destinations.filter(page => `${page.section} ${page.label} ${page.caption}`.toLocaleLowerCase('ru').includes(value))
})
const selected = computed(() => results.value[index.value] ?? null)
watch(query, () => { index.value = 0 }, { flush: 'sync' })
watch(() => selected.value?.id, async id => {
  await nextTick()
  if (id && selected.value?.id === id && list.value?.isConnected) {
    list.value.querySelector<HTMLElement>('#result-' + id)?.scrollIntoView({ block: 'nearest', inline: 'nearest', behavior: 'auto' })
  }
}, { immediate: true })
function choose(id: PageId) { emit('navigate', id); emit('close') }
function onPointerMove(event: PointerEvent, position: number) {
  if (event.pointerType === 'mouse' && (event.movementX !== 0 || event.movementY !== 0)) index.value = position
}
function onKey(event: KeyboardEvent) {
  if (event.defaultPrevented || event.isComposing || event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) return
  if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    const length = results.value.length
    if (!length) return
    event.preventDefault()
    index.value = (index.value + (event.key === 'ArrowDown' ? 1 : length - 1)) % length
  } else if (event.key === 'Enter' && !event.repeat && selected.value) {
    event.preventDefault()
    choose(selected.value.id)
  }
}
</script>
<template>
  <DialogFrame label="Переход к разделу" @close="emit('close')">
    <div class="palette">
      <div class="search-field">
        <Icon name="search" :size="20" />
        <input v-model="query" autofocus autocomplete="off" :spellcheck="false" maxlength="256" aria-label="Поиск страниц" placeholder="Куда перейти?" role="combobox"
          aria-autocomplete="list" aria-controls="page-results" aria-describedby="page-search-help" aria-expanded="true"
          :aria-activedescendant="selected ? `result-${selected.id}` : undefined" @keydown="onKey" />
        <button type="button" class="btn small" aria-label="Закрыть поиск" @click="emit('close')">Esc</button>
      </div>
      <div id="page-results" ref="list" role="listbox" aria-label="Страницы приложения" class="results">
        <button v-for="(page, i) in results" :id="`result-${page.id}`" :key="page.id" type="button" role="option" tabindex="-1"
          :aria-selected="index === i" :class="{ selected: index === i }" @click="choose(page.id)" @pointermove="onPointerMove($event, i)">
          <Icon :name="page.icon" :size="20" />
          <span><strong>{{ page.label }}</strong><small>{{ page.section }} · {{ page.caption }}</small></span>
          <span class="enter" aria-hidden="true">↵</span>
        </button>
      </div>
      <p v-if="!results.length" role="status" class="empty">Страница не найдена. Попробуйте другое название.</p>
      <footer id="page-search-help">↑ ↓ выбрать · Enter перейти · Esc закрыть</footer>
    </div>
  </DialogFrame>
</template>
<style scoped>
.palette { display: flex; flex-direction: column; max-height: min(70dvh, 640px); min-height: 0; }
.search-field { flex-shrink: 0; display: flex; align-items: center; gap: 12px; padding: 18px; border-bottom: 1px solid var(--border); }
input { flex: 1; width: 0; border: 0; background: transparent; color: var(--text); font: inherit; min-height: 32px; }
.results { min-height: 0; overflow: auto; overscroll-behavior: contain; padding: 8px; scroll-padding: 8px; }
.results button { width: 100%; display: flex; align-items: center; gap: 14px; padding: 12px; background: transparent; border: 0; color: var(--text-soft); border-radius: 6px; text-align: left; }
.results button.selected { background: var(--accent-soft); color: var(--text); }
.results button > span:first-of-type { display: grid; gap: 3px; min-width: 0; overflow-wrap: anywhere; }
small { color: var(--muted); }
.enter { margin-left: auto; color: var(--accent); }
.empty { color: var(--muted); padding: 16px; }
footer { flex-shrink: 0; border-top: 1px solid var(--border); padding: 10px 18px; color: var(--muted); font-size: 12px; }
@media (forced-colors: active) { .results button.selected { outline: 2px solid Highlight; outline-offset: -2px; } }
</style>
