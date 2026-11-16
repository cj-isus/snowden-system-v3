<script setup lang="ts">
import { computed, watch } from 'vue'
import HintBox from './HintBox.vue'
import { fmtDateTime } from '../api/labels'
import { useApplicationUpdate } from '../composables/applicationUpdate'

const emit = defineEmits<{ changed: [] }>()
const { preview, problem, settledMessage, changed, busy, phase, disabledReason, canChoose, canApply, message, choose, apply } = useApplicationUpdate()
watch(changed, () => emit('changed'))
const phaseLabel = computed(() => phase.value === 'picker' ? 'Ожидается выбор каталога…' : phase.value === 'check' ? 'Проверяется выбранный пакет…' : phase.value === 'apply' ? 'Ожидается ответ на применение…' : '')

function fmtSize(value: number | null): string {
  if (value === null) return 'Размер не передан'
  return value >= 1024 * 1024 ? (value / (1024 * 1024)).toFixed(1) + ' МБ' : Math.ceil(value / 1024) + ' КБ'
}
</script>

<template>
  <section class="update" aria-labelledby="application-update-title" :aria-busy="busy">
    <h2 id="application-update-title">Обновление приложения</h2>
    <p id="application-update-help" class="update-note">
      Выберите пакет из доверенного источника. Применение связано с показанным манифестом
      и пакетом доставки; перед установкой подпись, версия и содержимое проверяются повторно.
    </p>
    <div class="update-actions">
      <button class="btn" type="button" :aria-disabled="!canChoose" aria-describedby="application-update-help application-update-status" @click="choose">Выбрать каталог пакета…</button>
    </div>
    <p id="application-update-status" class="update-note" role="status" aria-live="polite">{{ phaseLabel || disabledReason || 'Выбор пакета не устанавливает обновление.' }}</p>

    <div v-if="preview" class="update-preview">
      <template v-if="preview.ok && preview.approval">
        <h3>Пакет v{{ preview.version }} прошёл проверку приложения</h3>
        <dl class="update-facts">
          <div><dt>Текущая версия</dt><dd>{{ preview.currentVersion || 'Не передана' }}</dd></div>
          <div><dt>Размер</dt><dd>{{ fmtSize(preview.size) }}</dd></div>
          <div><dt>Подтверждение до</dt><dd>{{ fmtDateTime(preview.approval.expiresAt) }}</dd></div>
          <div><dt>SHA-256 приложения</dt><dd class="mono-small">{{ preview.sha256 }}</dd></div>
        </dl>
        <p v-if="preview.notes" class="update-notes">{{ preview.notes }}</p>
        <div class="update-actions">
          <button class="btn primary" type="button" :aria-disabled="!canApply" aria-describedby="application-update-confirm application-update-status" @click="apply">Применить v{{ preview.version }} и перезапустить</button>
        </div>
        <p id="application-update-confirm" class="update-note">Подтверждение одноразовое и ограничено по времени. После изменения пакета или состояния приложения нужна новая проверка.</p>
      </template>
      <HintBox v-else kind="warn" title="Обновление не принято">{{ preview.reason }}</HintBox>
    </div>
    <p v-if="problem" class="update-warning" role="status">{{ problem }}</p>
    <p class="update-note" role="status" aria-atomic="true">{{ settledMessage }}</p>
    <p v-if="message && message !== disabledReason" class="update-note" role="status">{{ message }}</p>
    <p class="update-note">При отказе обновите сведения о профиле и версии: проверенная история доставки уже могла сохраниться. Ответ команды не подтверждает запуск новой версии.</p>
  </section>
</template>

<style scoped>
.update { min-width: 0; padding: 18px 0; }
h2 { font-size: 15px; margin: 0 0 12px; }
h3 { font-size: 14px; margin: 0 0 14px; }
.update-note { color: var(--muted); font-size: 13px; line-height: 1.6; overflow-wrap: anywhere; }
.update-actions { display: flex; flex-wrap: wrap; gap: 10px; margin: 14px 0; }
.update-actions .btn { max-width: 100%; white-space: normal; overflow-wrap: anywhere; }
.btn[aria-disabled='true'] { color: var(--muted); cursor: not-allowed; }
.update-preview { min-width: 0; margin-top: 18px; padding-top: 18px; border-top: 1px solid var(--border); }
.update-facts { display: grid; gap: 10px; margin: 0; font-size: 13px; }
.update-facts > div { display: grid; grid-template-columns: minmax(120px, 1fr) minmax(0, 2fr); gap: 12px; }
dt { color: var(--muted); }
dd { margin: 0; overflow-wrap: anywhere; }
.update-notes { white-space: pre-wrap; overflow-wrap: anywhere; font-size: 13px; line-height: 1.6; }
.update-warning { color: var(--yellow); font-size: 13px; line-height: 1.6; }
@media (max-width: 700px) { .update-facts > div { grid-template-columns: 1fr; gap: 4px; } }
</style>
