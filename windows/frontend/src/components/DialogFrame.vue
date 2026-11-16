<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { toast } from '../composables/toast'

defineProps<{ label: string }>()
const emit = defineEmits<{ close: [] }>()
const dialog = ref<HTMLDialogElement | null>(null)
let active = false
let backdropPointer: number | null = null
let dismissClick = false

function requestClose() { if (active) emit('close') }
function onCancel(event: Event) {
  event.preventDefault() // the operation owner decides when to remove its modal
  requestClose()
}
function onPointerDown(event: PointerEvent) {
  dismissClick = false
  backdropPointer = event.isPrimary && event.button === 0 && event.target === event.currentTarget ? event.pointerId : null
}
function onPointerUp(event: PointerEvent) {
  dismissClick = backdropPointer !== null && backdropPointer === event.pointerId && event.target === event.currentTarget
  backdropPointer = null
}
function resetPointer() { backdropPointer = null; dismissClick = false }
function onBackdropClick(event: MouseEvent) {
  const dismiss = dismissClick && event.detail > 0 && event.target === event.currentTarget
  resetPointer()
  if (dismiss) { event.preventDefault(); event.stopPropagation(); requestClose() }
}
onMounted(() => {
  active = true
  const element = dialog.value
  try {
    if (!element?.isConnected || typeof element.showModal !== 'function') throw new Error('modal unavailable')
    element.showModal() // native top layer, background inertness and focus ownership
  } catch {
    toast('error', 'Не удалось открыть диалог. Повторите действие; если ошибка сохраняется, обновите WebView2.')
    requestClose() // no visible nonmodal fallback for a confirmation dialog
  }
})
onBeforeUnmount(() => {
  active = false
  resetPointer()
  const element = dialog.value
  if (element?.open) element.close() // while connected: native focus restoration, no late callback
})
</script>

<template>
  <Teleport to="body">
    <dialog ref="dialog" class="dialog-overlay" :aria-label="label" aria-modal="true"
      @cancel="onCancel" @close="requestClose" @pointerdown="onPointerDown" @pointerup="onPointerUp" @pointercancel="resetPointer" @click="onBackdropClick">
      <section class="dialog-frame">
        <slot />
      </section>
    </dialog>
  </Teleport>
</template>

<style scoped>
.dialog-overlay { position: fixed; inset: 0; margin: 0; width: 100%; height: 100dvh; max-width: none; max-height: none; border: 0; padding: min(12vh, 90px) 16px 16px; overflow: auto; background: transparent; color: var(--text); }
.dialog-overlay:not([open]) { display: none; }
.dialog-overlay[open] { display: flex; align-items: flex-start; justify-content: center; }
.dialog-overlay::backdrop { background: #02070ac9; backdrop-filter: blur(8px); }
.dialog-frame { flex: 0 1 640px; min-width: 0; max-width: 100%; max-height: 100%; overflow: auto; background: var(--panel-2); border: 1px solid var(--border-light); border-top: 2px solid var(--accent); border-radius: 12px; box-shadow: 0 24px 80px #0009; }
</style>
