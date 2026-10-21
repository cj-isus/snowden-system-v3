<script setup lang="ts">
import { computed } from 'vue'
import DialogFrame from './DialogFrame.vue'
import { uiLogLines } from '../composables/uiLog'
import { fmtTime } from '../api/labels'
defineEmits<{ close: [] }>()
const lines = uiLogLines()
const recent = computed(() => lines.value.slice(-20).reverse())
</script>
<template>
  <DialogFrame label="Последние события" @close="$emit('close')">
    <header><h2>Последние события</h2><button class="btn small" @click="$emit('close')">Закрыть</button></header>
    <p v-if="!recent.length" class="empty">Событий пока нет. Здесь появятся результаты действий.</p>
    <ol v-else>
      <li v-for="(line, i) in recent" :key="i" :class="line.level">
        <time class="mono">{{ fmtTime(line.t) }}</time><span>{{ line.text }}</span>
      </li>
    </ol>
  </DialogFrame>
</template>
<style scoped>
header { display: flex; justify-content: space-between; gap: 12px; padding: 20px; align-items: center; }
h2 { font-size: 18px; }
.empty { padding: 0 20px 20px; color: var(--muted); }
ol { list-style: none; margin: 0; padding: 0 20px 20px; }
li { display: flex; gap: 14px; border-top: 1px solid var(--border); padding: 12px 0; color: var(--text-soft); font-size: 13px; }
li.error { color: var(--red); } li.warn { color: var(--yellow); }
time { color: var(--muted); flex-shrink: 0; } li span { overflow-wrap: anywhere; }
</style>
