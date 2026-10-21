<script setup lang="ts">
import { nextTick, onMounted, onUnmounted, ref } from 'vue'

defineProps<{ label: string }>()
const emit = defineEmits<{ close: [] }>()
const panel = ref<HTMLElement>()
let previous: HTMLElement | null = null
const focusable = () => [...(panel.value?.querySelectorAll<HTMLElement>(
  'button:not(:disabled), input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex="0"]',
) ?? [])].filter(element => !element.hidden)

function onKey(event: KeyboardEvent) {
  if (event.key === 'Escape') { event.preventDefault(); emit('close') }
  if (event.key !== 'Tab') return
  const items = focusable()
  const first = items[0] ?? panel.value
  const last = items[items.length - 1] ?? panel.value
  if (event.shiftKey && (document.activeElement === first || document.activeElement === panel.value)) {
    event.preventDefault(); last?.focus()
  } else if (!event.shiftKey && (document.activeElement === last || items.length === 0)) {
    event.preventDefault(); first?.focus()
  }
}
onMounted(async () => {
  previous = document.activeElement as HTMLElement | null
  await nextTick()
  ;(focusable()[0] ?? panel.value)?.focus()
})
onUnmounted(() => { if (previous?.isConnected) previous.focus() })
</script>

<template>
  <Teleport to="body">
    <div class="dialog-overlay" @click.self="emit('close')">
      <section ref="panel" class="dialog-frame" role="dialog" aria-modal="true" :aria-label="label" tabindex="-1" @keydown="onKey">
        <slot />
      </section>
    </div>
  </Teleport>
</template>

<style scoped>
.dialog-overlay { position: fixed; inset: 0; z-index: 100; display: flex; align-items: flex-start; justify-content: center; padding: min(12vh, 90px) 16px 16px; background: #02070ac9; backdrop-filter: blur(8px); }
.dialog-frame { width: min(640px, 100%); max-height: 80dvh; overflow: auto; background: var(--panel-2); border: 1px solid var(--border-light); border-top: 2px solid var(--accent); border-radius: 12px; box-shadow: 0 24px 80px #0009; }
</style>
