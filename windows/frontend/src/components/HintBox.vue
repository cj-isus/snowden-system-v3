<script setup lang="ts">
/**
 * Поясняющий хинт. kind=info — нейтральный, warn — предостережение, danger —
 * про безопасность. Текст даёт факт или правило, не гипотезу.
 */
const props = withDefaults(
  defineProps<{
    kind?: 'info' | 'warn' | 'danger'
    title?: string
  }>(),
  { kind: 'info', title: '' },
)
</script>

<template>
  <div class="hint" :class="props.kind" role="note">
    <span class="hint-dot" aria-hidden="true"></span>
    <div>
      <div v-if="props.title" class="hint-title">{{ props.title }}</div>
      <div class="hint-body"><slot /></div>
    </div>
  </div>
</template>

<style scoped>
.hint {
  display: flex;
  gap: 10px;
  padding: 10px 12px;
  border-radius: 10px;
  font-size: 12.5px;
  line-height: 1.55;
  background: var(--bg-elevated);
  border: 1px solid var(--border-soft);
  color: var(--text-dim);
}

.hint-dot {
  flex: 0 0 auto;
  width: 7px;
  height: 7px;
  margin-top: 6px;
  border-radius: 50%;
  background: var(--accent);
}

.hint.warn {
  background: var(--warn-soft);
  border-color: #f0b42933;
}
.hint.warn .hint-dot {
  background: var(--warn);
}
.hint.warn .hint-title,
.hint.warn .hint-body {
  color: #e8d5a4;
}

.hint.danger {
  background: var(--danger-soft);
  border-color: #ff636333;
}
.hint.danger .hint-dot {
  background: var(--danger);
}
.hint.danger .hint-title,
.hint.danger .hint-body {
  color: #eec6c6;
}

.hint-title {
  font-weight: 600;
  color: var(--text);
  margin-bottom: 2px;
}
</style>
