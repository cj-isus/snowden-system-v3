<script setup lang="ts">
/**
 * Карточка секрета. Значение не показывается; вместо него — fingerprint
 * (SHA256-префикс), статус проверки и число раскрытий (аудит).
 */
import { computed, ref } from 'vue'
import type { SecretMeta, SecretTestReport, ProbeStep } from '@/api/contract'
import {
  secretKindLabels,
  secretBackendLabels,
  fmtDateTime,
} from '../api/labels'
import { useSecretsStore } from '../composables/secretsStore'
import { toast } from '../composables/toast'
import Icon from './Icon.vue'

const props = defineProps<{ secret: SecretMeta }>()

const store = useSecretsStore()
const confirmingDelete = ref(false)
const revealing = ref(false)
const verifying = ref(false)

// «Задать значение» для пустого слота (предзаполнен сеялкой без значения)
const settingValue = ref(false)
const newValue = ref('')
const newConfirm = ref('')
const showNew = ref(false)

const valueMismatch = computed(() => newConfirm.value.length > 0 && newValue.value !== newConfirm.value)
const canSubmitValue = computed(() => newValue.value.trim().length > 0 && !valueMismatch.value)

async function onSubmitValue(): Promise<void> {
  if (!canSubmitValue.value) return
  const ok = await store.setValue(props.secret.id, newValue.value)
  if (ok) {
    newValue.value = ''
    newConfirm.value = ''
    showNew.value = false
    settingValue.value = false
  }
}

const kindLabel = computed(() => secretKindLabels[props.secret.kind] ?? props.secret.kind)
const backendLabel = computed(() => secretBackendLabels[props.secret.backend] ?? props.secret.backend)

async function onReveal(): Promise<void> {
  revealing.value = true
  try {
    const value = await store.reveal(props.secret.id)
    if (value !== null) {
      await navigator.clipboard.writeText(value)
      toast('info', 'Значение скопировано в буфер обмена (не отображается на экране)')
    }
  } finally {
    revealing.value = false
  }
}

async function onVerify(): Promise<void> {
  verifying.value = true
  try {
    const res = await store.verify(props.secret.id)
    if (res) {
      if (res.status === 'ok') toast('ok', 'Проверка: формат значения корректен')
      else if (res.status === 'failed') toast('error', `Проверка не пройдена: ${res.error}`)
      else toast('info', 'Проверка выполнена')
    }
  } finally {
    verifying.value = false
  }
}

// ---------- живой тест значения ----------

const testing = ref(false)
const testReport = ref<SecretTestReport | null>(null)
const testError = ref('')
const showTestReport = ref(true)

const testTitle = computed(() => {
  switch (props.secret.kind) {
    case 'vless-uuid':
    case 'hy2-password':
    case 'hy2-obfs-password':
      return 'Сверка с сервером: сервер сам посчитает хеши своего конфига, сравним с локальным значением. Значение не передаётся — только хеши'
    case 'vps-ssh-key':
      return 'Реальный вход на сервер ключом из хранилища (значение не показывается)'
    case 'cf-api-token':
      return 'Живая проверка токена в API Cloudflare (/user/tokens/verify)'
    default:
      return 'Живой тест для этого типа не определён — будет честный «пропущено»'
  }
})

async function onTest(): Promise<void> {
  if (testing.value) return
  testing.value = true
  testReport.value = null
  testError.value = ''
  showTestReport.value = true
  try {
    testReport.value = await store.runTest(props.secret.id)
  } catch (e) {
    testError.value = e instanceof Error ? e.message : String(e)
    toast('error', `Тест не выполнен: ${testError.value}`)
  } finally {
    testing.value = false
  }
}

function stepWord(s: ProbeStep['status']): string {
  switch (s) {
    case 'pass':
      return 'пройдено'
    case 'fail':
      return 'не пройдено'
    case 'skipped':
      return 'пропущено'
    case 'running':
      return 'выполняется'
    default:
      return 'ожидает'
  }
}

function stepIcon(s: ProbeStep['status']): 'check' | 'cross' | 'refresh' | 'info' {
  switch (s) {
    case 'pass':
      return 'check'
    case 'fail':
      return 'cross'
    case 'running':
      return 'refresh'
    default:
      return 'info'
  }
}

async function onDelete(): Promise<void> {
  if (!confirmingDelete.value) {
    confirmingDelete.value = true
    window.setTimeout(() => (confirmingDelete.value = false), 4000)
    return
  }
  await store.remove(props.secret.id, props.secret.title)
  confirmingDelete.value = false
}
</script>

<template>
  <section class="secret">
    <div class="row1">
      <div class="title">
        <Icon name="lock" :size="14" />
        <strong>{{ props.secret.title }}</strong>
      </div>
      <span class="kind">{{ kindLabel }}</span>
      <span class="backend" :title="`Хранение: ${backendLabel}`">{{ backendLabel }}</span>
    </div>

    <div class="row2">
      <span class="fp mono" :title="'SHA256-префикс значения (само значение не хранится в UI)'">
        fingerprint: {{ props.secret.fingerprint }}
      </span>
      <span class="audit">раскрытий: {{ props.secret.reveals }}</span>
      <span class="audit">обновлён: {{ fmtDateTime(props.secret.updatedAt) }}</span>
    </div>

    <div class="row3">
      <span class="vstat" :class="props.secret.verifyStatus">
        {{ props.secret.verifyStatus === 'ok' ? 'проверен' : props.secret.verifyStatus === 'failed' ? 'проверка провалена' : 'не проверялся' }}
      </span>
      <span v-if="props.secret.verifyError" class="vverr mono">{{ props.secret.verifyError }}</span>
    </div>

    <p v-if="props.secret.hint" class="hint-text">{{ props.secret.hint }}</p>

    <div v-if="settingValue" class="set-value">
      <label class="sv-field">
        <span class="sv-lbl">Значение</span>
        <div class="sv-row">
          <input
            v-model="newValue"
            :type="showNew ? 'text' : 'password'"
            placeholder="Вставьте значение — оно шифруется DPAPI и не покидает этот компьютер"
            autocomplete="off"
            spellcheck="false"
          />
          <button class="eye" type="button" :title="showNew ? 'Скрыть' : 'Показать (осторожно: скриншот)'" @click="showNew = !showNew">
            <Icon name="eye" :size="15" />
          </button>
        </div>
      </label>
      <label class="sv-field">
        <span class="sv-lbl">Повторите значение</span>
        <input
          v-model="newConfirm"
          :type="showNew ? 'text' : 'password'"
          placeholder="Ещё раз — исключаем опечатку"
          autocomplete="off"
          spellcheck="false"
        />
      </label>
      <p v-if="valueMismatch" class="sv-mismatch">Значения не совпадают.</p>
      <p v-if="props.secret.verifyError" class="sv-mismatch">Предыдущая ошибка: {{ props.secret.verifyError }}</p>
      <div class="sv-actions">
        <button class="btn primary" :disabled="!canSubmitValue" @click="onSubmitValue">
          <Icon name="lock" :size="13" />
          Сохранить
        </button>
        <button class="btn" @click="settingValue = false">Отмена</button>
      </div>
    </div>

    <div class="actions">
      <button
        v-if="props.secret.storedValue === null"
        class="btn primary"
        @click="settingValue = !settingValue"
        title="Слот создан заранее — вставьте значение"
      >
        <Icon name="key" :size="13" />
        Задать значение
      </button>
      <button
        class="btn"
        :disabled="testing"
        :title="testTitle"
        @click="onTest"
      >
        <Icon name="flask" :size="13" />
        {{ testing ? 'Идёт тест…' : 'Тест' }}
      </button>
      <button class="btn" :disabled="revealing" @click="onReveal" title="Скопировать значение в буфер обмена">
        <Icon name="copy" :size="13" />
        Показать
      </button>
      <button class="btn" :disabled="verifying" @click="onVerify" title="Проверить формат значения (без сети)">
        <Icon name="check" :size="13" />
        Проверить
      </button>
      <button class="btn danger" :class="{ confirm: confirmingDelete }" @click="onDelete" title="Удалить значение из локального хранилища">
        <Icon name="trash" :size="13" />
        {{ confirmingDelete ? 'Точно удалить?' : 'Удалить' }}
      </button>
    </div>

    <div v-if="testReport || testError" class="test-report">
      <div class="tr-head" @click="showTestReport = !showTestReport">
        <span class="tr-title">
          <Icon :name="testReport?.ok ? 'check' : 'error'" :size="14" />
          Живой тест: {{ testReport ? (testReport.ok ? 'пройден' : 'не пройден') : 'ошибка' }}
        </span>
        <Icon :name="showTestReport ? 'chevron' : 'chevron'" :size="13" class="tr-chev" :class="{ open: showTestReport }" />
      </div>
      <div v-if="showTestReport" class="tr-steps">
        <div v-if="testError" class="tr-step">
          <Icon name="error" :size="13" class="st-ic fail" />
          <span class="st-txt">{{ testError }}</span>
        </div>
        <div v-for="(st, i) in testReport?.steps ?? []" :key="i" class="tr-step">
          <Icon :name="stepIcon(st.status)" :size="13" class="st-ic" :class="st.status" />
          <span class="st-name">{{ st.name }}</span>
          <span class="st-status" :class="st.status">{{ stepWord(st.status) }}</span>
          <span class="st-txt">{{ st.detail }}</span>
        </div>
      </div>
      <p class="tr-note">Значение секрета не передаётся и не отображается — только хеши и статусы шагов.</p>
    </div>
  </section>
</template>

<style scoped>
.secret {
  padding: 14px 16px;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--bg-panel);
}

.row1 {
  display: flex;
  align-items: center;
  gap: 10px;
}
.title {
  display: flex;
  align-items: center;
  gap: 7px;
  flex: 1;
  color: var(--accent);
}
.title strong {
  color: var(--text);
  font-size: 13.5px;
}

.kind,
.backend {
  font-size: 11.5px;
  color: var(--text-faint);
  background: var(--bg-hover);
  border-radius: 999px;
  padding: 2px 9px;
}

.row2,
.row3 {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-top: 8px;
  font-size: 11.5px;
  color: var(--text-faint);
  flex-wrap: wrap;
}

.fp {
  color: var(--text-dim);
}
.audit {
  color: var(--text-faint);
}

.vstat {
  font-size: 11.5px;
  padding: 2px 9px;
  border-radius: 999px;
}
.vstat.ok {
  color: var(--ok);
  background: var(--ok-soft);
}
.vstat.failed {
  color: var(--danger);
  background: var(--danger-soft);
}
.vstat.unverified {
  color: var(--text-faint);
  background: var(--bg-hover);
}
.vverr {
  color: var(--danger);
  font-size: 11.5px;
}

.hint-text {
  margin: 8px 0 0;
  font-size: 12px;
  color: var(--text-dim);
  line-height: 1.5;
}

.set-value {
  margin-top: 12px;
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--bg-elevated);
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.sv-field {
  display: flex;
  flex-direction: column;
  gap: 5px;
}
.sv-lbl {
  font-size: 11.5px;
  color: var(--text-faint);
}
.sv-row {
  display: flex;
  gap: 8px;
}
.sv-row input,
.sv-field input {
  flex: 1;
  padding: 8px 11px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--bg-panel);
  color: var(--text);
  font-size: 12.5px;
  outline: none;
}
.sv-row input:focus,
.sv-field input:focus {
  border-color: var(--accent);
}
.eye {
  width: 36px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--bg-panel);
  color: var(--text-faint);
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.eye:hover {
  color: var(--text);
}
.sv-mismatch {
  margin: 0;
  font-size: 11.5px;
  color: var(--danger);
}
.sv-actions {
  display: flex;
  gap: 8px;
}

.actions {
  display: flex;
  gap: 8px;
  margin-top: 12px;
  flex-wrap: wrap;
}
.btn.primary {
  background: var(--accent);
  border-color: var(--accent);
  color: #0b1020;
  font-weight: 600;
}

.btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 11px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--bg-elevated);
  color: var(--text);
  font-size: 12px;
  cursor: pointer;
  transition: background 0.12s;
}
.btn:hover:not(:disabled) {
  background: var(--bg-hover);
}
.btn:disabled {
  opacity: 0.55;
  cursor: default;
}
.btn.danger {
  color: var(--danger);
  border-color: #ff636333;
}
.btn.danger:hover {
  background: var(--danger-soft);
}
.btn.danger.confirm {
  background: var(--danger-soft);
  border-color: #ff636366;
}

.test-report {
  margin-top: 12px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--bg-elevated);
  overflow: hidden;
}
.tr-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 9px 12px;
  cursor: pointer;
  user-select: none;
}
.tr-head:hover {
  background: var(--bg-hover);
}
.tr-title {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  font-size: 12.5px;
  color: var(--text);
}
.tr-chev {
  color: var(--text-faint);
  transition: transform 0.15s;
}
.tr-chev.open {
  transform: rotate(90deg);
}
.tr-steps {
  display: flex;
  flex-direction: column;
  border-top: 1px solid var(--border);
}
.tr-step {
  display: grid;
  grid-template-columns: 18px auto auto 1fr;
  align-items: baseline;
  gap: 8px;
  padding: 7px 12px;
  font-size: 12px;
}
.tr-step + .tr-step {
  border-top: 1px dashed var(--border-soft);
}
.st-ic {
  align-self: center;
}
.st-ic.pass {
  color: var(--ok);
}
.st-ic.fail {
  color: var(--danger);
}
.st-ic.running {
  color: var(--accent);
}
.st-ic.skipped,
.st-ic.info {
  color: var(--text-faint);
}
.st-name {
  color: var(--text);
}
.st-status {
  font-size: 11px;
  padding: 1px 8px;
  border-radius: 999px;
  white-space: nowrap;
}
.st-status.pass {
  color: var(--ok);
  background: var(--ok-soft);
}
.st-status.fail {
  color: var(--danger);
  background: var(--danger-soft);
}
.st-status.skipped {
  color: var(--text-faint);
  background: var(--bg-hover);
}
.st-status.running {
  color: var(--accent);
  background: var(--accent-soft);
}
.st-txt {
  color: var(--text-dim);
  line-height: 1.45;
}
.tr-note {
  margin: 0;
  padding: 7px 12px;
  font-size: 11px;
  color: var(--text-faint);
  border-top: 1px dashed var(--border-soft);
}
</style>
