<script setup lang="ts">
defineProps<{ title: string; description: string; count?: number | null }>()
</script>
<template>
  <header class="page-heading">
    <div class="page-heading-copy">
      <div class="page-title"><h2>{{ title }}</h2><span v-if="count != null" class="count">{{ count }}</span></div>
      <p>{{ description }}</p>
    </div>
    <div v-if="$slots.default" class="page-actions"><slot /></div>
  </header>
</template>
<style scoped>
.page-heading { display: flex; align-items: center; justify-content: space-between; gap: 20px; margin-bottom: 4px; }
.page-heading-copy { min-width: 0; }
.page-title { display: flex; align-items: center; gap: 12px; }
h2 { font-size: 20px; letter-spacing: -.025em; font-weight: 600; }
p { margin: 7px 0 0; color: var(--muted); max-width: 76ch; font-size: 13px; line-height: 1.6; }
.count { color: var(--accent); background: var(--accent-soft); padding: 1px 8px; border-radius: 4px; font: 12px var(--mono); }
.page-actions { display: flex; gap: 8px; flex-shrink: 0; }
@media(max-width: 760px) { .page-heading { align-items: flex-start; flex-direction: column; gap: 12px; } h2 { font-size: 18px; } }
</style>
