<script setup lang="ts">
import { computed, nextTick, onUnmounted, ref, watch } from 'vue'
import DialogFrame from './DialogFrame.vue'
import { useBackendState } from '../composables/backendState'
import { useProxyRecovery } from '../composables/proxyRecovery'

const { state, isBusy } = useBackendState()
const recovery = useProxyRecovery()
const { phase, holding, canCheck, canResume, visible, preview, result, message, remaining } = recovery
const acknowledged = ref(false)
const closeControl = ref<HTMLButtonElement | null>(null)
const intent = computed(() => state.value?.diagnosticContext?.intent ?? null)
const localState = computed(() => state.value?.state)
const canPrepare = computed(() => recovery.available && !holding.value && !isBusy.value && intent.value !== null &&
  (localState.value === 'stopped' || localState.value === 'error'))
const canContinue = computed(() => recovery.available && canResume.value && !isBusy.value && intent.value !== null &&
  (localState.value === 'stopped' || localState.value === 'error'))
const resumeReason = computed(() => {
  switch (preview.value?.preview.resumeReason) {
    case 'unchanged_write': return 'Предыдущая запись вернула ошибку; чтение подтвердило, что значение не изменилось.'
    case 'notification_failed': return 'Исходные значения уже восстановлены; уведомление Windows завершилось ошибкой.'
    case 'cancelled_before_intent': return 'Предыдущая попытка была отменена до следующего системного действия.'
    default: return ''
  }
})
watch(() => preview.value?.preview.token, () => { acknowledged.value = false })
watch(phase, async next => {
  if (!visible.value || !['preparing', 'confirming', 'cancelling'].includes(next)) return
  await nextTick()
  if (visible.value && phase.value === next) closeControl.value?.focus()
})
watch([intent, localState], ([next, lifecycle]) => recovery.invalidate(next, lifecycle))
onUnmounted(recovery.close)
function open() { if (canPrepare.value && intent.value !== null) void recovery.open(intent.value) }
function resume() { if (canContinue.value && intent.value !== null) void recovery.open(intent.value, true) }
</script>

<template>
  <div class="proxy-recovery">
    <button class="btn" :disabled="!canPrepare" @click="open">Восстановить изменения Snowden</button>
    <p v-if="!recovery.available" class="recovery-note">Ручное восстановление недоступно в этой версии приложения.</p>
    <p v-else-if="!canPrepare && !holding" class="recovery-note">Дождитесь состояния приложения и остановите VPN перед восстановлением.</p>
    <p v-if="message && !visible" class="recovery-note" role="status" aria-live="polite">{{ message }}</p>
    <div v-if="!visible && holding && (phase === 'blocked' || phase === 'held')" class="recovery-actions">
      <button class="btn" :disabled="!canContinue" @click="resume">Подготовить продолжение</button>
      <button class="btn" :disabled="!canCheck" @click="recovery.retryCancel">Проверить завершение</button>
    </div>
    <p v-if="result" class="recovery-note">Это результат операции с прокси. VPN не подключён автоматически; состояние защиты и доступность сети проверяются отдельно.</p>

    <DialogFrame v-if="visible" label="Восстановление изменений прокси Snowden" @close="recovery.close">
      <div class="recovery-dialog">
        <header><h2>{{ preview?.preview.resume ? 'Продолжение восстановления' : 'Восстановление прокси' }}</h2><button ref="closeControl" class="btn" @click="recovery.close">Закрыть</button></header>
        <p v-if="phase === 'preparing'" role="status">Подготавливаем сохранённые изменения. Системные настройки пока не восстанавливаются.</p>
        <template v-if="phase === 'awaiting' && preview">
          <p>Будут восстановлены только изменения из журнала Snowden. Другие экземпляры приложения должны быть закрыты. Обнаруженные чужие настройки сохранятся.</p>
          <p v-if="preview.preview.resume">{{ resumeReason }} Продолжение использует исходную транзакцию и требует нового разрешения.</p>
          <dl><div><dt>Параметров с записанными изменениями</dt><dd>{{ preview.preview.recordedValues }}</dd></div><div><dt>Подтверждение действует</dt><dd>{{ remaining }} с по часам экрана</dd></div></dl>
          <p v-if="!preview.preview.resume && preview.preview.priorProcess !== 'observed_exited'" class="recovery-note">Завершение прежнего процесса не подтверждено. Разрешение относится только к сохранённым изменениям прокси.</p>
          <label class="recovery-ack"><input v-model="acknowledged" type="checkbox" />{{ preview.preview.resume ? 'Разрешаю продолжить восстановление записанных изменений на исходной операции.' : 'Я закрыл другие экземпляры Snowden и разрешаю восстановить записанные изменения прокси.' }}</label>
          <div class="recovery-actions"><button class="btn" @click="recovery.close">Отмена</button><button class="btn primary" :disabled="!acknowledged || remaining <= 0" @click="recovery.confirm">{{ preview.preview.resume ? 'Подтвердить продолжение' : 'Подтвердить восстановление' }}</button></div>
        </template>
        <p v-if="phase === 'confirming' || phase === 'cancelling'" role="status">Ожидаем завершения исходной операции. Закрытие окна не прерывает уже начатый системный вызов.</p>
        <p v-if="message" role="status" aria-live="polite">{{ message }}</p>
        <div v-if="phase === 'blocked' || phase === 'held'" class="recovery-actions">
          <button class="btn" :disabled="!canContinue" @click="resume">Подготовить продолжение</button>
          <button class="btn" :disabled="!canCheck" @click="recovery.retryCancel">Проверить завершение</button>
        </div>
        <p v-if="result" class="recovery-note">Подключение VPN выполняется отдельно.</p>
      </div>
    </DialogFrame>
  </div>
</template>

<style scoped>
.proxy-recovery { min-width: 0; margin-top: 16px; }
.recovery-note { margin: 10px 0; color: var(--muted); font-size: 13px; line-height: 1.6; overflow-wrap: anywhere; }
.recovery-dialog { padding: 24px; color: var(--text-soft); line-height: 1.65; }
header, .recovery-actions { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 12px; }
h2 { margin: 0; color: var(--text); font-size: 19px; }
dl { margin: 16px 0; } dl > div { display: grid; grid-template-columns: minmax(0, 2fr) minmax(0, 1fr); gap: 12px; border-bottom: 1px solid var(--border); padding: 10px 0; }
dt { color: var(--muted); } dd { margin: 0; }
.recovery-ack { display: flex; align-items: flex-start; gap: 10px; margin: 20px 0; cursor: pointer; }
input { flex: 0 0 auto; margin-top: 6px; accent-color: var(--accent); }
.btn { min-height: 42px; white-space: normal; }
@media (max-width: 600px) { .recovery-dialog { padding: 16px; } dl > div { grid-template-columns: minmax(0, 1fr); gap: 4px; } }
</style>
