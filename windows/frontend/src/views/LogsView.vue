<script setup lang="ts">
import { computed, nextTick, onUnmounted, ref, watch } from 'vue'
import PageHeading from '../components/PageHeading.vue'
import HintBox from '../components/HintBox.vue'
import SupportLogPreview from '../components/SupportLogPreview.vue'
import { uiLogLines, clearUiLog } from '../composables/uiLog'
import { uiLogSource } from '../composables/supportLog'
import type { UiLogSource } from '../composables/supportLog'
import { fmtDateTime } from '../api/labels'

const lines = uiLogLines()
const source = ref<'all' | UiLogSource>('all')
const level = ref<'all' | 'debug' | 'info' | 'warn' | 'error'>('all')
const query = ref('')
const autoscroll = ref(true)
const listEl = ref<HTMLElement | null>(null)
const atBottom = ref(true)
const contextRevision = ref(0)
let owner = true
let scrollGeneration = 0
onUnmounted(() => { owner = false; scrollGeneration++ })
watch([source, level, query], () => { contextRevision.value++ }, { flush: 'sync' })
// The preview can observe retirement before the next parent render/prop update.
function currentContextRevision(): number { return contextRevision.value }
const filtered = computed(() => {
  const search = query.value.trim().toLowerCase()
  return lines.value.filter(line => {
    const kind = uiLogSource(line.text)
    if (source.value === 'backend' ? kind !== 'backend' && kind !== 'engine' : source.value !== 'all' && source.value !== kind) return false
    return (level.value === 'all' || line.level === level.value) && (!search || line.text.toLowerCase().includes(search))
  })
})
function onClear(): void { contextRevision.value++; clearUiLog() }
function onScroll(): void {
  const list = listEl.value
  if (list) atBottom.value = list.scrollHeight - list.scrollTop - list.clientHeight < 40
}
function scrollToEnd(): void {
  const list = listEl.value
  if (list) { list.scrollTop = list.scrollHeight; atBottom.value = true }
}
watch([filtered, autoscroll], ([, auto]) => {
  const revision = ++scrollGeneration
  if (!auto || !atBottom.value) return
  void nextTick(() => { if (owner && revision === scrollGeneration && atBottom.value) scrollToEnd() })
})
</script>

<template>
  <div class="view">
    <PageHeading title="События" description="Локальный буфер интерфейса. Для передачи диагностических сведений сначала подготовьте и просмотрите сводку.">
      <button class="btn" @click="onClear">Очистить локальный буфер</button>
    </PageHeading>
    <div class="filters">
      <label>Источник по префиксу<select v-model="source"><option value="all">Все источники</option><option value="backend">Backend и ядро</option><option value="engine">Только ядро</option><option value="probe">Probe</option><option value="ui">UI / без префикса</option></select></label>
      <label>Уровень<select v-model="level"><option value="all">Все уровни</option><option value="debug">debug</option><option value="info">info</option><option value="warn">warn</option><option value="error">error</option></select></label>
      <label class="search">Текст<input v-model="query" type="search" maxlength="256" placeholder="Поиск в локальном буфере" spellcheck="false" /></label>
      <label class="follow"><input v-model="autoscroll" type="checkbox" /> Следовать за событиями</label>
    </div>
    <div class="selection-info">
      <span>Показано {{ filtered.length }} из {{ lines.length }} локальных записей.</span>
      <button v-if="!atBottom" class="btn" @click="scrollToEnd">К новым событиям</button>
    </div>
    <p class="scope-note">Источник определяется по текстовому префиксу, не подтверждает происхождение. Буфер ограничен 1000 записями, длинные сообщения усекаются. Очистка не удаляет журналы Windows или backend.</p>
    <div ref="listEl" class="log-list" tabindex="0" role="region" aria-label="Выбранные локальные события" @scroll="onScroll">
      <p v-if="!filtered.length" class="empty">{{ lines.length ? 'Записей по этим фильтрам нет. Измените условия поиска.' : 'Локальных записей нет. Это не подтверждает отсутствие сбоев.' }}</p>
      <ol v-else>
        <li v-for="(line, index) in filtered" :key="index" :class="line.level">
          <div class="line-meta"><time :datetime="line.t || undefined">{{ fmtDateTime(line.t) }}</time><span>{{ line.level }}</span><span>{{ uiLogSource(line.text) }}</span></div>
          <p class="mono">{{ line.text }}</p>
        </li>
      </ol>
    </div>
    <SupportLogPreview :lines="filtered" :context-revision="contextRevision" :current-context-revision="currentContextRevision" />
    <HintBox kind="info">Сырой текст может содержать сведения из ошибок, адреса или пользовательские данные. Этот экран не подтверждает отсутствие секретов в исходных сообщениях. Для экспорта используется отдельная сводка с ограниченным набором полей; проверьте её перед копированием.</HintBox>
  </div>
</template>

<style scoped>
.view { display: flex; flex-direction: column; gap: 16px; min-width: 0; overflow-wrap: anywhere; }
.filters { display: flex; flex-wrap: wrap; gap: 12px; align-items: flex-end; }
label { display: flex; flex-direction: column; gap: 7px; color: var(--muted); font-size: 12px; min-width: 0; }
.search { flex: 1 1 220px; }
input[type='search'], select { min-height: 40px; min-width: 0; max-width: 100%; padding: 8px 12px; color: var(--text); background: var(--panel); border: 1px solid var(--border-light); border-radius: var(--radius); font: inherit; }
.follow { flex-direction: row; align-items: center; gap: 8px; min-height: 40px; }
.selection-info { display: flex; flex-wrap: wrap; justify-content: space-between; align-items: center; gap: 10px; color: var(--muted); font-size: 12.5px; }
.scope-note, .empty { color: var(--muted); font-size: 12.5px; line-height: 1.65; margin: 0; }
.log-list { min-width: 0; min-height: 180px; max-height: min(52vh, 600px); overflow: auto; background: var(--panel); border: 1px solid var(--border); border-radius: var(--radius); padding: 12px 16px; }
ol { list-style: none; margin: 0; padding: 0; }
li { border-bottom: 1px solid var(--border); padding: 12px 0; }
li:last-child { border-bottom: 0; }
.line-meta { display: flex; flex-wrap: wrap; gap: 8px 14px; color: var(--muted); font-size: 11.5px; }
li p { color: var(--text-soft); margin: 7px 0 0; font-size: 12px; line-height: 1.65; white-space: pre-wrap; }
.warn .line-meta { color: var(--yellow); }
.error .line-meta { color: var(--red); }
.btn { min-height: 40px; white-space: normal; }
</style>
