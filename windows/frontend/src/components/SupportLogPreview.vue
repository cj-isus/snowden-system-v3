<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'
import type { UiLogLine } from '../api/contract'
import { createSupportLogSnapshot } from '../composables/supportLog'
import type { SupportLogSnapshot } from '../composables/supportLog'
import { useClipboardAction } from '../composables/clipboard'

const props = defineProps<{ lines: readonly UiLogLine[]; contextRevision: number; currentContextRevision?: () => number }>()
const clipboard = useClipboardAction()
const snapshot = ref<SupportLogSnapshot | null>(null)
const panel = ref<HTMLElement | null>(null)
const prepareButton = ref<HTMLButtonElement | null>(null)
const heading = ref<HTMLElement | null>(null)
const outcome = ref('')
let owner = true
let generation = Symbol()
const contextRevision = () => props.currentContextRevision?.() ?? props.contextRevision
function clear(): void { generation = Symbol(); snapshot.value = null; outcome.value = '' }
async function close(): Promise<void> {
  clear()
  const revision = generation
  await nextTick()
  if (owner && revision === generation) (prepareButton.value?.disabled ? panel.value : prepareButton.value)?.focus()
}
watch(contextRevision, clear, { flush: 'sync' })
onBeforeUnmount(() => { owner = false; clear() })
async function prepare(): Promise<void> {
  if (!owner || !clipboard.current() || !props.lines.length || props.contextRevision !== contextRevision()) return
  clear()
  const revision = generation
  snapshot.value = createSupportLogSnapshot(props.lines)
  await nextTick()
  if (owner && revision === generation) heading.value?.focus()
}
async function copy(): Promise<void> {
  const selected = snapshot.value
  if (!selected || !clipboard.canStart.value) return
  const revision = generation, context = contextRevision()
  const interested = () => owner && revision === generation && context === contextRevision() && snapshot.value === selected
  outcome.value = ''
  const result = await clipboard.run('support', interested, async lease => {
    const copied = await lease.writeText(selected.text)
    if (!lease.current()) return
    outcome.value = copied === 'confirmed'
      ? 'Служба буфера подтвердила запись этой сводки. Последующие изменения буфера не проверялись.'
      : copied === 'unavailable'
        ? 'Служба копирования недоступна. Можно выделить текст сводки и скопировать вручную.'
        : 'Копирование не подтверждено. Проверьте результат перед передачей сводки.'
  })
  if (clipboard.current() && interested() && result !== 'finished' && result !== 'retired') {
    outcome.value = result === 'busy' ? 'Предыдущая операция копирования ещё не завершена.' : 'Копирование не подтверждено. Текст сводки доступен для ручного выделения.'
  }
}
</script>

<template>
  <section ref="panel" class="support-preview" tabindex="-1" aria-label="Подготовка сводки журнала">
    <button ref="prepareButton" class="btn" :disabled="!lines.length" @click="prepare">Подготовить сводку для поддержки</button>
    <p class="scope-note">Экспортируется сводка времени, уровней, источников и известных кодов. Исходные сообщения и поисковый запрос в неё не входят. Полный диагностический пакет этим не создаётся.</p>
    <template v-if="snapshot">
      <h2 ref="heading" tabindex="-1">Предварительный просмотр</h2>
      <p class="scope-note">Подготовлено: {{ snapshot.generatedAt }} · Событий: {{ snapshot.records }} · Не включено: {{ snapshot.omitted }}. Новые события не изменяют эту зафиксированную выборку.</p>
      <label for="support-log-text">Точный текст для копирования</label>
      <textarea id="support-log-text" :value="snapshot.text" readonly spellcheck="false" rows="9"></textarea>
      <div class="actions-row">
        <button class="btn primary" :disabled="!clipboard.canStart.value" :aria-busy="clipboard.ownsPending.value && clipboard.pending.value || undefined" @click="copy">{{ clipboard.ownsPending.value && clipboard.pending.value ? 'Ожидается копирование…' : 'Копировать эту сводку' }}</button>
        <button class="btn" @click="close">Закрыть просмотр</button>
      </div>
    </template>
    <p v-if="clipboard.busy.value" class="scope-note" role="status">{{ clipboard.pendingLabel.value }} Подготовка и закрытие просмотра не отменяют начатую запись.</p>
    <p v-else-if="!clipboard.available.value" class="scope-note">Автоматическое копирование недоступно. Подготовленную сводку можно выделить и скопировать вручную.</p>
    <p class="copy-outcome" role="status" aria-atomic="true">{{ outcome }}</p>
  </section>
</template>

<style scoped>
.support-preview { min-width: 0; padding-top: 16px; border-top: 1px solid var(--border); overflow-wrap: anywhere; }
h2 { margin: 18px 0 0; font-size: 17px; }
.scope-note { color: var(--muted); font-size: 12.5px; line-height: 1.65; }
label { display: block; color: var(--text-soft); font-size: 13px; margin: 12px 0 8px; }
textarea { width: 100%; min-width: 0; resize: vertical; min-height: 160px; max-height: 400px; padding: 12px; font: 12px/1.6 var(--mono); background: var(--panel); color: var(--text-soft); border: 1px solid var(--border-light); border-radius: var(--radius); }
.actions-row { display: flex; flex-wrap: wrap; gap: 10px; margin-top: 12px; }
.btn { min-height: 40px; white-space: normal; }
.copy-outcome { color: var(--text-soft); font-size: 13px; line-height: 1.6; }
</style>
