<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { pages } from '../composables/navigation'
import type { PageId } from '../composables/navigation'
import DialogFrame from './DialogFrame.vue'
import Icon from './Icon.vue'
const emit = defineEmits<{ close: []; navigate: [id: PageId] }>()
const query = ref('')
const index = ref(0)
const results = computed(() => {
  const value = query.value.trim().toLocaleLowerCase('ru')
  return pages.filter(page => `${page.label} ${page.caption}`.toLocaleLowerCase('ru').includes(value))
})
watch(query, () => { index.value = 0 })
function choose(id: PageId) { emit('navigate', id); emit('close') }
function onKey(event: KeyboardEvent) {
  if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    event.preventDefault()
    const length = results.value.length
    index.value = length ? (index.value + (event.key === 'ArrowDown' ? 1 : length - 1)) % length : 0
  } else if (event.key === 'Enter' && results.value[index.value]) {
    event.preventDefault()
    choose(results.value[index.value].id)
  }
}
</script>
<template>
  <DialogFrame label="Переход к разделу" @close="emit('close')">
    <div class="search-field">
      <Icon name="search" :size="20" />
      <input v-model="query" aria-label="Поиск раздела" placeholder="Куда перейти?" role="combobox"
        aria-autocomplete="list" aria-controls="page-results" aria-expanded="true"
        :aria-activedescendant="results.length ? `result-${results[index]?.id}` : undefined" @keydown="onKey" />
      <button class="btn small" aria-label="Закрыть поиск" @click="emit('close')">Esc</button>
    </div>
    <div id="page-results" role="listbox" aria-label="Разделы" class="results">
      <button v-for="(page, i) in results" :id="`result-${page.id}`" :key="page.id" role="option"
        :aria-selected="index === i" :class="{ selected: index === i }" @click="choose(page.id)" @mousemove="index = i">
        <Icon :name="page.icon" :size="20" />
        <span><strong>{{ page.label }}</strong><small>{{ page.caption }}</small></span>
        <span class="enter" aria-hidden="true">↵</span>
      </button>
      <p v-if="!results.length" role="status" class="empty">Раздел не найден. Попробуйте другое название.</p>
    </div>
  </DialogFrame>
</template>
<style scoped>
.search-field { display: flex; align-items: center; gap: 12px; padding: 18px; border-bottom: 1px solid var(--border); }
input { flex: 1; width: 0; border: 0; background: transparent; color: var(--text); font: inherit; min-height: 32px; }
.results { padding: 8px; }
.results button { width: 100%; display: flex; align-items: center; gap: 14px; padding: 12px; background: transparent; border: 0; color: var(--text-soft); border-radius: 6px; text-align: left; }
.results button.selected { background: var(--accent-soft); color: var(--text); }
.results button > span:first-of-type { display: grid; gap: 3px; }
small { color: var(--muted); }
.enter { margin-left: auto; color: var(--accent); }
.empty { color: var(--muted); padding: 16px; }
</style>
