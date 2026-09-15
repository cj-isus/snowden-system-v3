<script setup lang="ts">
/**
 * Журнал: backend-события и локальный лог UI. Фильтры: источник (все/backend/
 * UI/движок), уровень, поиск по подстроке. Автопрокрутка с честной паузой:
 * пользователь проскроллил вверх — прокрутка останавливается, вернулся вниз —
 * продолжилась. Значения секретов сюда не попадают никогда (AGENTS.md §2.4).
 */
import { computed, nextTick, ref, watch } from 'vue'
import Icon from '../components/Icon.vue'
import HintBox from '../components/HintBox.vue'
import { uiLogLines, clearUiLog } from '../composables/uiLog'
import { fmtTime } from '../api/labels'
import { toast } from '../composables/toast'

const lines = uiLogLines()
const source = ref<'all' | 'backend' | 'engine' | 'ui'>('all')
const level = ref<'all' | 'info' | 'warn' | 'error'>('all')
const query = ref('')
const autoscroll = ref(true)
const listEl = ref<HTMLElement | null>(null)
const atBottom = ref(true)

const filtered = computed(() => {
  const q = query.value.trim().toLowerCase()
  return lines.value.filter((l) => {
    if (source.value === 'backend' && !l.text.startsWith('[backend]')) return false
    if (source.value === 'engine' && !l.text.startsWith('[backend] движок [')) return false
    if (source.value === 'ui' && (l.text.startsWith('[backend]') || l.text.startsWith('[probe]')))
      return false
    if (level.value !== 'all' && l.level !== level.value) return false
    if (q && !l.text.toLowerCase().includes(q)) return false
    return true
  })
})

function levelClass(l: string): string {
  return l
}

function onClear(): void {
  clearUiLog()
}

function onCopy(): void {
  const text = filtered.value.map((l) => `${fmtTime(l.t)} [${l.level}] ${l.text}`).join('\n')
  void navigator.clipboard.writeText(text)
  toast('info', `Скопировано строк: ${filtered.value.length} (журнал не содержит секретов)`)
}

/** Держим низ списка, только если пользователь и так внизу (честная пауза). */
function onScroll(): void {
  const el = listEl.value
  if (!el) return
  atBottom.value = el.scrollHeight - el.scrollTop - el.clientHeight < 40
}

watch(
  () => [filtered.value.length, autoscroll.value] as const,
  ([, auto]) => {
    if (!auto) return
    void nextTick(() => {
      const el = listEl.value
      if (!el || !atBottom.value) return
      el.scrollTop = el.scrollHeight
    })
  },
)
</script>

<template>
  <div class="view">
    <header class="head">
      <h2>Журнал</h2>
      <p class="sub">
        Диагностика без секретов: события backend помечены [backend], локальные — без метки,
        строки ядра sing-box — меткой «движок [канал]». Уровни: info/warn/error.
      </p>
    </header>

    <div class="toolbar">
      <div class="seg">
        <button :class="{ on: source === 'all' }" @click="source = 'all'">Все</button>
        <button :class="{ on: source === 'backend' }" @click="source = 'backend'">Backend</button>
        <button
          :class="{ on: source === 'engine' }"
          title="Строки ядра sing-box с тегом активного канала"
          @click="source = 'engine'"
        >
          Движок
        </button>
        <button :class="{ on: source === 'ui' }" @click="source = 'ui'">UI</button>
      </div>
      <div class="seg">
        <button :class="{ on: level === 'all' }" @click="level = 'all'">Любой уровень</button>
        <button :class="{ on: level === 'info' }" @click="level = 'info'">info</button>
        <button :class="{ on: level === 'warn' }" @click="level = 'warn'">warn</button>
        <button :class="{ on: level === 'error' }" @click="level = 'error'">error</button>
      </div>
      <div class="search">
        <Icon name="search" :size="13" />
        <input v-model="query" type="text" placeholder="поиск по тексту…" spellcheck="false" />
        <button v-if="query" class="clear" aria-label="Очистить поиск" @click="query = ''">×</button>
      </div>
      <label class="chk">
        <input v-model="autoscroll" type="checkbox" />
        автопрокрутка
      </label>
      <div class="spacer"></div>
      <button class="btn" @click="onCopy">
        <Icon name="copy" :size="13" />
        Копировать
      </button>
      <button class="btn" @click="onClear">
        <Icon name="trash" :size="13" />
        Очистить
      </button>
    </div>

    <div ref="listEl" class="loglist" @scroll="onScroll">
      <div v-if="filtered.length === 0" class="empty">Пока пусто — это норма до первого действия.</div>
      <div v-for="(l, i) in filtered" :key="i" class="line" :class="levelClass(l.level)">
        <span class="t mono">{{ fmtTime(l.t) }}</span>
        <span class="lv mono">{{ l.level }}</span>
        <span class="tx mono">{{ l.text }}</span>
      </div>
    </div>

    <HintBox kind="info" title="Что нельзя вставлять в журнал">
      UUID, пароли, токены и приватные ключи не логируются даже временно. Если нужно
      поделиться диагностикой — скопируйте журнал целиком: он по построению free of secrets.
    </HintBox>
  </div>
</template>

<style scoped>
.view {
  display: flex;
  flex-direction: column;
  gap: 14px;
  height: 100%;
}

.sub {
  margin: 5px 0 0;
  color: var(--muted);
  font-size: 11.5px;
}

.toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.seg {
  display: inline-flex;
  border: 1px solid var(--border);
  border-radius: 9px;
  overflow: hidden;
}
.seg button {
  padding: 6px 12px;
  border: none;
  background: var(--bg-elevated);
  color: var(--text-dim);
  font-size: 12px;
  cursor: pointer;
}
.seg button + button {
  border-left: 1px solid var(--border);
}
.seg button.on {
  background: var(--accent-soft);
  color: var(--text);
}

.search {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 5px 8px;
  border: 1px solid var(--border);
  border-radius: 9px;
  background: var(--bg-elevated);
  color: var(--text-faint);
}
.search input {
  border: none;
  background: transparent;
  color: var(--text);
  font-size: 12px;
  width: 180px;
  outline: none;
}
.search input::placeholder {
  color: var(--text-faint);
}
.search .clear {
  border: none;
  background: transparent;
  color: var(--text-faint);
  font-size: 14px;
  cursor: pointer;
  padding: 0 2px;
}
.search .clear:hover {
  color: var(--text);
}

.chk {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--text-dim);
}

.spacer {
  flex: 1;
}

.loglist {
  flex: 1;
  min-height: 220px;
  overflow: auto;
  padding: 10px 12px;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: #0a0c11;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.line {
  display: flex;
  gap: 10px;
  padding: 2px 6px;
  border-radius: 5px;
  font-size: 11.5px;
}
.line:hover {
  background: var(--bg-hover);
}
.line .t {
  color: var(--text-faint);
  flex: 0 0 auto;
}
.line .lv {
  flex: 0 0 44px;
  color: var(--text-faint);
}
.line .tx {
  color: var(--text-dim);
  word-break: break-word;
}
.line.warn .lv,
.line.warn .tx {
  color: var(--warn);
}
.line.error .lv,
.line.error .tx {
  color: var(--danger);
}

.empty {
  color: var(--text-faint);
  font-size: 12.5px;
  padding: 8px;
}
</style>
