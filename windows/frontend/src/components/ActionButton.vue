<script setup lang="ts">
import { ref } from 'vue'

// busy describes an outstanding action; disabled alone gates activation.
// A batch's stop-next action can therefore remain usable while work is pending.
const props = withDefaults(defineProps<{ disabled?: boolean; busy?: boolean }>(), { disabled: false, busy: false })
const emit = defineEmits<{ click: [event: MouseEvent] }>()
const button = ref<HTMLButtonElement | null>(null)
function onClick(event: MouseEvent): void {
  if (props.disabled || event.defaultPrevented) {
    event.preventDefault()
    event.stopPropagation()
    return
  }
  emit('click', event)
}
function onEnter(event: KeyboardEvent): void { if (event.repeat) event.preventDefault() }
defineExpose({ focus: (options?: FocusOptions) => button.value?.focus(options) })
</script>

<template>
  <button ref="button" type="button" class="btn action-button" :aria-disabled="props.disabled" :aria-busy="props.busy"
    @click="onClick" @keydown.enter="onEnter"><slot /></button>
</template>

<style scoped>
.action-button[aria-disabled='true'] { opacity: .6; cursor: default; }
.action-button[aria-disabled='true']:active { transform: none; }
</style>
