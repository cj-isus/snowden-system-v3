<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import type { SecretMeta, SecretTestReport } from '../api/contract'
import { secretKindLabels, secretBackendLabels, fmtDateTime, probeStepStatus, probeStepStatusLabels, probeStepName, probeStepDetail } from '../api/labels'
import { useSecretsStore } from '../composables/secretsStore'
import { useClipboardAction } from '../composables/clipboard'

const props = defineProps<{ secret: SecretMeta }>()
const emit = defineEmits<{ deleting: [] }>()
const store = useSecretsStore()
const clipboard = useClipboardAction()
const setting = ref(false)
const draft = ref('')
const repeated = ref('')
const visible = ref(false)
const saving = ref(false)
const revealing = ref(false)
const verifying = ref(false)
const testing = ref(false)
const deleting = ref(false)
const confirmDelete = ref(false)
const outcome = ref('')
const report = ref<SecretTestReport | null>(null)
let owner = true
let generation = 0
function clearDraft(): void { draft.value = ''; repeated.value = ''; visible.value = false }
function retire(): void { generation++; clearDraft(); setting.value = false; confirmDelete.value = false; report.value = null; outcome.value = '' }
onBeforeUnmount(() => { owner = false; retire() })
watch(() => [props.secret.id, props.secret.updatedAt, props.secret.fingerprint] as const, retire, { flush: 'sync' })
const busy = computed(() => store.actionBusy.value || saving.value || revealing.value || verifying.value || testing.value || deleting.value)
const kindLabel = computed(() => typeof secretKindLabels[props.secret.kind] === 'string' ? secretKindLabels[props.secret.kind] : 'Тип не описан интерфейсом')
const backendLabel = computed(() => props.secret.backend ? secretBackendLabels[props.secret.backend] : 'Хранилище не сообщено')
const hasValue = computed(() => props.secret.storedValue !== null && props.secret.storedValue !== 'unknown')
const canSave = computed(() => !busy.value && !!draft.value && draft.value === repeated.value)
const formatHelp = computed(() => {
  switch (props.secret.kind) {
    case 'vless-uuid': return 'UUID: 36 символов hex, группы 8-4-4-4-12.'
    case 'hy2-password': case 'hy2-obfs-password': return 'Локальное правило: не менее 8 символов. Совпадение с сервером проверяется отдельно.'
    case 'vps-ssh-key': return 'Ожидается приватный ключ PEM. Проверка формата не подтверждает вход или доверие серверу.'
    case 'cf-api-token': return 'Локальное правило: не менее 30 символов. Действительность и права токена проверяются отдельно.'
    case 'custom': return 'Локальное правило: не менее 4 символов.'
    default: return 'Правила этого типа текущему интерфейсу не переданы.'
  }
})
const networkScope = computed(() => props.secret.kind === 'cf-api-token' ? 'Запрос к API Cloudflare использует токен по назначению в Authorization.' : props.secret.kind === 'vps-ssh-key' ? 'Проверка выполняет SSH-вход ключом. Успешный вход не является проверкой VPN-пути.' : 'Проверка может обращаться к VPS по SSH для сверки данных. Результат относится только к шагам отчёта; для custom запрос может быть пропущен.')
const reportLabel = computed(() => {
  const value = report.value
  if (!value || !value.steps.length) return 'Измеренные шаги не переданы'
  if (value.steps.every(step => probeStepStatus(step) === 'skipped')) return 'Все шаги пропущены'
  return value.ok && value.steps.every(step => probeStepStatus(step) === 'pass' || probeStepStatus(step) === 'skipped') ? 'Переданные шаги пройдены' : 'Положительный результат не подтверждён'
})
async function saveValue(): Promise<void> {
  if (!canSave.value) return
  const current = ++generation
  const id = props.secret.id
  saving.value = true
  outcome.value = ''
  const request = store.setValue(id, draft.value)
  clearDraft()
  try {
    const result = await request
    if (!owner || current !== generation) return
    if (result.ok) { setting.value = false; outcome.value = 'Приложение подтвердило запись. Это не подтверждение работы канала.' }
    else outcome.value = result.message
  } finally { if (owner) saving.value = false }
}
async function copyValue(): Promise<void> {
  if (busy.value || !hasValue.value || !clipboard.canStart.value) return
  const current = ++generation
  const id = props.secret.id
  const interested = () => owner && current === generation && props.secret.id === id
  revealing.value = true
  outcome.value = ''
  try {
    const result = await clipboard.run('secret', interested, async lease => {
      if (!lease.current()) return
      const revealed = await store.reveal(id)
      if (!revealed.ok) { if (lease.current()) outcome.value = revealed.message; return }
      let value = revealed.value.value
      revealed.value.value = null // also clear a response that belongs to a retired view
      if (!lease.current()) { value = null; return }
      if (typeof value !== 'string' || !value.length) { value = null; outcome.value = 'Непустое значение не получено; копирование не выполнялось.'; return }
      const writing = lease.writeText(value) // original interest checked again without await
      value = null // release this reference; JavaScript strings cannot be zeroised
      const copied = await writing
      if (!lease.current()) return
      outcome.value = copied === 'confirmed'
        ? 'Служба буфера подтвердила запись. Буфер обмена и его история контролируются системой.'
        : 'Копирование не подтверждено. Выдача значения могла быть учтена в аудите.'
    })
    if (clipboard.current() && interested() && result !== 'finished' && result !== 'retired') outcome.value = 'Копирование не подтверждено. Выдача значения могла быть учтена в аудите.'
  } catch {
    if (owner && current === generation) outcome.value = 'Копирование не подтверждено. Выдача значения могла быть учтена в аудите.'
  } finally { if (owner) revealing.value = false }
}
async function verify(): Promise<void> {
  if (busy.value || !hasValue.value) return
  const current = ++generation
  verifying.value = true
  outcome.value = ''
  try {
    const result = await store.verify(props.secret.id)
    if (owner && current === generation) outcome.value = !result.ok ? result.message : result.value.status === 'ok' ? 'Приложение подтвердило локальный формат. Сервер не проверялся.' : 'Локальный формат не подтверждён; сверьте требования к типу.'
  } finally { if (owner) verifying.value = false }
}
async function networkCheck(): Promise<void> {
  if (busy.value || !hasValue.value) return
  const current = ++generation
  testing.value = true
  report.value = null
  outcome.value = ''
  try {
    const result = await store.runTest(props.secret.id)
    if (owner && current === generation) {
      if (result.ok) report.value = result.value
      else outcome.value = result.message
    }
  } finally { if (owner) testing.value = false }
}
async function remove(): Promise<void> {
  if (busy.value || !confirmDelete.value) return
  const current = ++generation
  deleting.value = true
  confirmDelete.value = false
  emit('deleting')
  try {
    const result = await store.remove(props.secret.id)
    if (owner && current === generation && !result.ok) outcome.value = result.message
  } finally { if (owner) deleting.value = false }
}
</script>

<template>
  <section class="secret">
    <div class="secret-heading"><h2>{{ secret.title || 'Слот без названия' }}</h2><span>{{ kindLabel }}</span></div>
    <dl class="facts"><div><dt>Хранение</dt><dd>{{ backendLabel }}</dd></div><div><dt>Значение</dt><dd>{{ secret.storedValue === null ? 'Не задано по метаданным' : secret.storedValue === 'unknown' ? 'Наличие не установлено' : 'Сообщено сохранённым' }}</dd></div><div><dt>Отпечаток</dt><dd class="mono">{{ secret.fingerprint || 'Не передан' }}</dd></div><div><dt>Выдач значения</dt><dd>{{ secret.reveals ?? 'Не сообщено' }}</dd></div></dl>
    <p class="note">Изменено: {{ fmtDateTime(secret.updatedAt) }} · Локальная проверка: {{ secret.verifyStatus === 'ok' ? 'формат подтверждён' : secret.verifyStatus === 'failed' ? 'формат не подтверждён' : 'положительного результата нет' }} · {{ fmtDateTime(secret.lastVerifiedAt) }}.</p>
    <p class="note">{{ formatHelp }}</p>
    <p v-if="secret.verifyError" class="note">{{ secret.verifyError }}</p>
    <div class="actions">
      <button class="btn" :disabled="busy || secret.storedValue === 'unknown'" @click="retire(); setting = true">{{ secret.storedValue === null ? 'Задать значение' : 'Заменить значение' }}</button>
      <button class="btn" :disabled="busy || !hasValue || !clipboard.canStart.value" :aria-busy="clipboard.ownsPending.value && clipboard.pending.value || undefined" @click="copyValue">{{ revealing ? 'Выдача / копирование…' : 'Скопировать значение' }}</button>
      <button class="btn" :disabled="busy || !hasValue" @click="verify">{{ verifying ? 'Проверка формата…' : 'Проверить формат' }}</button>
    </div>
    <p v-if="clipboard.busy.value" class="note">{{ clipboard.pendingLabel.value }}</p>
    <p v-else-if="!clipboard.available.value" class="note">Служба копирования недоступна. Значение не запрашивается для недоступного копирования.</p>
    <div v-if="setting" class="edit-value">
      <label>Новое значение<input v-model="draft" :disabled="saving" :type="visible ? 'text' : 'password'" autocomplete="new-password" autocapitalize="off" spellcheck="false" /></label>
      <label>Повторите значение<input v-model="repeated" :disabled="saving" :type="visible ? 'text' : 'password'" autocomplete="new-password" autocapitalize="off" spellcheck="false" /></label>
      <label class="show-value"><input v-model="visible" type="checkbox" :disabled="saving" /> Показать ввод на экране</label>
      <p v-if="repeated && draft !== repeated" class="note">Значения не совпадают.</p>
      <div class="actions"><button class="btn primary" :disabled="!canSave" @click="saveValue">{{ saving ? 'Ожидается запись…' : 'Записать в слот' }}</button><button class="btn" @click="retire">Закрыть и очистить поля</button></div>
    </div>
    <details class="advanced">
      <summary>Сетевая проверка и удаление</summary>
      <p class="note">{{ networkScope }} Эти действия не выполняются автоматически.</p>
      <button class="btn" :disabled="busy || !hasValue" @click="networkCheck">{{ testing ? 'Ожидается отчёт…' : 'Выполнить сетевую проверку' }}</button>
      <button class="btn" :disabled="busy" @click="confirmDelete = true">Удалить слот…</button>
      <div v-if="confirmDelete" class="delete-confirm"><p>Удалить этот слот из локального хранилища? Привязки профиля могут потребовать его повторного добавления.</p><div class="actions"><button class="btn danger" :disabled="busy" @click="remove">Удалить этот слот</button><button class="btn" @click="confirmDelete = false">Отмена</button></div></div>
    </details>
    <p v-if="outcome" class="outcome" role="status">{{ outcome }}</p>
    <details v-if="report" class="report" open><summary>Отчёт: {{ reportLabel }}</summary><ol><li v-for="(step, index) in report.steps" :key="index"><strong>{{ probeStepName(step) }} · {{ probeStepStatusLabels[probeStepStatus(step)] }}</strong><p>{{ probeStepDetail(step) }}</p></li></ol><p class="note">Отчёт относится к этой проверке. Применение профиля, текущее подключение и общий охват трафика он не подтверждает.</p></details>
    <p class="note">Счётчик выдач не доказывает успешное копирование или вставку. Закрытие страницы не отменяет начатую выдачу значения или запись в буфер.</p>
  </section>
</template>

<style scoped>
.secret { min-width: 0; padding: 20px 0; border-bottom: 1px solid var(--border); overflow-wrap: anywhere; }
.secret-heading { display: flex; flex-wrap: wrap; gap: 8px 20px; justify-content: space-between; align-items: baseline; }
h2 { margin: 0; font-size: 16px; }
.secret-heading span, .note { color: var(--muted); font-size: 12.5px; line-height: 1.65; }
.facts { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px 20px; margin: 16px 0; }
dt { color: var(--muted); font-size: 11.5px; }
dd { margin: 5px 0 0; color: var(--text-soft); font-size: 12.5px; }
.actions { display: flex; flex-wrap: wrap; gap: 10px; margin: 12px 0; }
.btn { min-height: 40px; white-space: normal; }
.edit-value { max-width: 700px; margin-top: 18px; padding-left: 16px; border-left: 2px solid var(--accent); }
label { display: flex; flex-direction: column; gap: 7px; margin: 12px 0; font-size: 13px; color: var(--text-soft); }
input:not([type='checkbox']) { width: 100%; min-width: 0; min-height: 40px; padding: 9px 12px; color: var(--text); background: var(--panel); border: 1px solid var(--border-light); border-radius: var(--radius); font: inherit; }
.show-value { flex-direction: row; align-items: center; gap: 8px; }
summary { padding: 10px 0; color: var(--text-soft); cursor: pointer; font-size: 13px; }
.advanced > .btn { margin: 0 10px 10px 0; }
.delete-confirm { border-left: 2px solid var(--yellow); padding-left: 12px; color: var(--text-soft); font-size: 13px; }
.outcome { color: var(--text-soft); line-height: 1.6; font-size: 13px; }
ol { padding-left: 22px; color: var(--text-soft); font-size: 12.5px; line-height: 1.6; }
@media (max-width: 1180px) { .facts { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 620px) { .facts { grid-template-columns: minmax(0, 1fr); } }
</style>
