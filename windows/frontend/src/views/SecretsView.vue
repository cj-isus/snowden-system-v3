<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import PageHeading from '../components/PageHeading.vue'
import SecretCard from '../components/SecretCard.vue'
import HintBox from '../components/HintBox.vue'
import Icon from '../components/Icon.vue'
import { useSecretsStore } from '../composables/secretsStore'
import { useNav } from '../composables/nav'
import { isBuildPhase } from '../api/backend'
import { secretKindLabels, secretKindHints, fmtDateTime } from '../api/labels'
import type { SecretKind } from '../api/contract'

const { items, loading, loadError, receivedAt, actionBusy, lastOutcome, refresh, save } = useSecretsStore()
const { navTo } = useNav()
const showForm = ref(false)
const saving = ref(false)
const kind = ref<SecretKind>('custom')
const title = ref('')
const value = ref('')
const confirmValue = ref('')
const showValue = ref(false)
const formOutcome = ref('')
const page = ref<HTMLElement | null>(null)
const addButton = ref<HTMLButtonElement | null>(null)
let owner = true
let generation = 0
function clearDraft(): void { value.value = ''; confirmValue.value = ''; title.value = ''; showValue.value = false }
onBeforeUnmount(() => { owner = false; generation++; clearDraft() })
onMounted(() => { void refresh() })
const valueMismatch = computed(() => confirmValue.value.length > 0 && value.value !== confirmValue.value)
const canSave = computed(() => !isBuildPhase() && !actionBusy.value && !saving.value && !!title.value.trim() && !!value.value && value.value === confirmValue.value)
function onKindChange(): void { if (!title.value.trim()) title.value = secretKindLabels[kind.value] }
function onDeleting(): void { page.value?.focus() }
async function closeForm(): Promise<void> {
  generation++
  clearDraft()
  showForm.value = false
  formOutcome.value = ''
  const revision = generation
  await nextTick()
  if (owner && revision === generation) (addButton.value?.disabled ? page.value : addButton.value)?.focus()
}
async function onSave(): Promise<void> {
  if (!canSave.value) return
  const revision = ++generation
  saving.value = true
  formOutcome.value = ''
  const request = save(kind.value, title.value.trim(), value.value, secretKindHints[kind.value])
  clearDraft()
  try {
    const result = await request
    if (!owner || revision !== generation) return
    if (result.ok) showForm.value = false
    else formOutcome.value = result.message
  } finally { if (owner) saving.value = false }
}
</script>

<template>
  <div ref="page" class="view" tabindex="-1">
    <PageHeading title="Локальные секреты" description="Метаданные слотов и явные действия со значением. Проверка формата не подтверждает сервер или защищённый путь." :count="items?.length">
      <button ref="addButton" class="btn primary" :disabled="actionBusy || isBuildPhase()" @click="showForm = true">Добавить секрет</button>
      <button class="btn" :disabled="loading || actionBusy" @click="refresh">{{ loading ? 'Загрузка…' : 'Обновить метаданные' }}</button>
    </PageHeading>
    <p class="source-note">Получено экраном: {{ fmtDateTime(receivedAt) }}. Хранилище и наличие значения показываются по ответу приложения; неизвестное состояние не считается пустым слотом.</p>
    <p v-if="actionBusy" class="operation-note" role="status">Операция ожидает ответа. Уход со страницы отзывает отображение результата, но не отменяет уже отправленный вызов.</p>
    <p v-if="lastOutcome" class="operation-note" role="status">Последнее действие: {{ lastOutcome }}</p>

    <section v-if="showForm" class="new-secret" :aria-busy="saving">
      <div class="section-heading"><h2>Новый секрет</h2><button class="btn" @click="closeForm">Закрыть и очистить поля</button></div>
      <p class="source-note">Название видно в метаданных. Вводите само значение только в предназначенные для него поля.</p>
      <form @submit.prevent="onSave">
        <label>Тип<select v-model="kind" :disabled="saving" @change="onKindChange"><option v-for="(label, key) in secretKindLabels" :key="key" :value="key">{{ label }}</option></select></label>
        <p class="source-note">{{ secretKindHints[kind] }}</p>
        <label>Название<input v-model="title" :disabled="saving" type="text" autocomplete="off" spellcheck="false" /></label>
        <label for="new-secret-value">Значение</label>
        <div class="value-row"><input id="new-secret-value" v-model="value" :disabled="saving" :type="showValue ? 'text' : 'password'" autocomplete="new-password" autocapitalize="off" spellcheck="false" /><button type="button" class="btn" :disabled="saving" :aria-pressed="showValue" :aria-label="showValue ? 'Скрыть введённое значение' : 'Показать введённое значение'" @click="showValue = !showValue"><Icon name="eye" :size="15" /></button></div>
        <label>Повторите значение<input v-model="confirmValue" :disabled="saving" :type="showValue ? 'text' : 'password'" autocomplete="new-password" autocapitalize="off" spellcheck="false" /></label>
        <p v-if="valueMismatch" class="error">Значения не совпадают.</p>
        <p v-if="showValue" class="source-note">Значение видно на экране и может попасть в снимок.</p>
        <button class="btn primary" type="submit" :disabled="!canSave">{{ saving ? 'Ожидается запись…' : 'Сохранить локально' }}</button>
      </form>
      <p v-if="formOutcome" class="error" role="status">{{ formOutcome }}</p>
    </section>

    <p v-if="items === null" class="empty" role="status">{{ loading ? 'Получаем метаданные слотов…' : loadError || 'Метаданные недоступны.' }}</p>
    <div v-else-if="!items.length" class="empty"><p>В ответе приложения нет слотов. Для подписанного профиля используйте импорт с точными привязками секретов.</p><button class="btn" @click="navTo('settings')">Профиль и импорт</button></div>
    <div v-else class="secret-list"><SecretCard v-for="secret in items" :key="secret.id" :secret="secret" @deleting="onDeleting" /></div>
    <HintBox kind="info">Копирование значения — отдельное действие: буфер обмена и его история могут быть доступны другим приложениям. Закрытие формы очищает её ссылки на ввод; гарантированное стирание строк из памяти JavaScript не обещается.</HintBox>
  </div>
</template>

<style scoped>
.view { display: flex; flex-direction: column; gap: 18px; min-width: 0; overflow-wrap: anywhere; }
h2 { margin: 0; font-size: 17px; }
.source-note, .empty { color: var(--muted); font-size: 12.5px; line-height: 1.65; margin: 0; }
.operation-note { color: var(--text-soft); font-size: 13px; line-height: 1.6; margin: 0; }
.new-secret { border-top: 2px solid var(--accent); padding: 20px; background: var(--panel); border-radius: var(--radius); }
.section-heading { display: flex; flex-wrap: wrap; align-items: baseline; justify-content: space-between; gap: 12px; margin-bottom: 12px; }
form { display: flex; flex-direction: column; gap: 12px; max-width: 720px; margin-top: 16px; }
label { display: flex; flex-direction: column; gap: 7px; color: var(--text-soft); font-size: 13px; }
input, select { width: 100%; min-width: 0; min-height: 40px; padding: 9px 12px; background: var(--panel-2); color: var(--text); border: 1px solid var(--border-light); border-radius: var(--radius); font: inherit; }
.value-row { display: flex; gap: 8px; }
.value-row input { flex: 1; }
.btn { min-height: 40px; white-space: normal; }
form > .btn { align-self: flex-start; }
.secret-list { min-width: 0; border-top: 1px solid var(--border); }
.error { color: var(--red); font-size: 13px; line-height: 1.6; }
@media (max-width: 760px) { .new-secret { padding: 16px; } }
</style>
