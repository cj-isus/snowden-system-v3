<script setup lang="ts">
/**
 * Секреты: локальное хранилище значений с fingerprint, проверкой и аудитом
 * раскрытий. Значения никогда не отображаются и не логируются (AGENTS.md §2.4):
 * копирование — только в буфер обмена, по явному действию владельца.
 */
import { computed, onMounted, ref } from 'vue'
import SecretCard from '../components/SecretCard.vue'
import HintBox from '../components/HintBox.vue'
import Icon from '../components/Icon.vue'
import { useSecretsStore } from '../composables/secretsStore'
import { isBuildPhase } from '../api/backend'
import { secretKindLabels, secretKindHints } from '../api/labels'
import type { SecretKind } from '../api/contract'

const { items, loading, refresh, save } = useSecretsStore()

const buildPhase = ref(false)
onMounted(() => {
  buildPhase.value = isBuildPhase()
  void refresh()
})

const showForm = ref(false)

// Форма добавления
const kind = ref<SecretKind>('custom')
const title = ref('')
const value = ref('')
const confirmValue = ref('')
const showValue = ref(false)

const valueMismatch = computed(() => confirmValue.value.length > 0 && value.value !== confirmValue.value)
const canSave = computed(
  () => title.value.trim().length > 0 && value.value.length > 0 && !valueMismatch.value,
)

const kindHint = computed(() => secretKindHints[kind.value] ?? '')

function onKindChange(): void {
  if (!title.value.trim() || title.value.trim() === secretKindLabels[kind.value]) {
    // Автоподпис поля — удобно, но не обязательно.
    title.value = secretKindLabels[kind.value]
  }
}

async function onSave(): Promise<void> {
  if (!canSave.value) return
  const ok = await save(kind.value, title.value.trim(), value.value, kindHint.value)
  if (ok) {
    value.value = ''
    confirmValue.value = ''
    showValue.value = false
    showForm.value = false
  }
}

function onPaste(e: ClipboardEvent): void {
  // Значение вставляется целиком; в DOM оно живёт только в поле ввода.
  void e
}
</script>

<template>
  <div class="view">
    <header class="head">
      <h1>Секреты</h1>
      <p class="sub">
        Значения живут только в локальном игнорируемом хранилище. В UI виден только
        SHA256-отпечаток — сверка конфигураций идёт по хешам, не по значениям.
      </p>
    </header>

    <div class="actions-row">
      <button class="btn primary" @click="showForm = !showForm">
        <Icon name="key" :size="14" />
        {{ showForm ? 'Скрыть форму' : 'Добавить секрет' }}
      </button>
      <button class="btn" :disabled="loading" @click="() => void refresh()">
        <Icon name="refresh" :size="14" />
        Обновить
      </button>
    </div>

    <section v-if="showForm" class="form">
      <h2>Новый секрет</h2>

      <label class="field">
        <span class="lbl">Тип</span>
        <select v-model="kind" @change="onKindChange">
          <option v-for="(label, k) in secretKindLabels" :key="k" :value="k">{{ label }}</option>
        </select>
      </label>

      <p class="kind-hint">{{ kindHint }}</p>

      <label class="field">
        <span class="lbl">Название</span>
        <input
          v-model="title"
          type="text"
          placeholder="Например: Канал A — UUID"
          autocomplete="off"
          spellcheck="false"
        />
      </label>

      <label class="field">
        <span class="lbl">Значение</span>
        <div class="value-row">
          <input
            v-model="value"
            :type="showValue ? 'text' : 'password'"
            placeholder="Вставьте значение — оно не попадёт в git, логи или скриншоты"
            autocomplete="off"
            spellcheck="false"
            @paste="onPaste"
          />
          <button class="eye" type="button" :title="showValue ? 'Скрыть значение' : 'Показать значение (осторожно: попадёт в скриншот)'"
            @click="showValue = !showValue">
            <Icon name="eye" :size="15" />
          </button>
        </div>
      </label>

      <label class="field">
        <span class="lbl">Повторите значение</span>
        <input
          v-model="confirmValue"
          :type="showValue ? 'text' : 'password'"
          placeholder="Ещё раз — чтобы исключить опечатку"
          autocomplete="off"
          spellcheck="false"
        />
      </label>

      <p v-if="valueMismatch" class="mismatch">Значения не совпадают — сохранение недоступно.</p>

      <div class="form-actions">
        <button class="btn primary" :disabled="!canSave" @click="onSave">
          <Icon name="lock" :size="14" />
          Сохранить локально
        </button>
      </div>

      <HintBox kind="danger" title="Правило проекта">
        Секрет из чата, скриншота или git-истории считается скомпрометированным: его нужно
        отозвать/перевыпустить, а не «спрятать» в .gitignore.
      </HintBox>
    </section>

    <div v-if="buildPhase && (items === null || items.length === 0)" class="empty-block">
      <Icon name="warn" :size="18" />
      <p>
        Хранилище недоступно в build phase: сохранение и проверка появятся вместе с backend
        (app.go: SaveSecret / VerifySecret / RevealSecret). Ничего не выдумываем.
      </p>
    </div>

    <div v-else-if="items === null" class="empty-block">
      <Icon name="warn" :size="18" />
      <p>Список секретов недоступен: источник данных не отвечает.</p>
    </div>

    <div v-else-if="items.length === 0" class="empty-block">
      <Icon name="info" :size="18" />
      <p>Хранилище пусто. Добавьте UUID VLESS, пароли HY2 или своё значение — они понадобятся при рендере конфига.</p>
    </div>

    <div v-else class="cards">
      <SecretCard v-for="s in items" :key="s.id" :secret="s" />
    </div>

    <HintBox kind="info" title="Что значит «проверен» и что делает «Тест»">
      «Проверен» у карточки — локальная проверка формата значения (например, UUID-форма или
      непустой пароль). Кнопка «Тест» — живая проверка: для канальных секретов сервер сам
      считает хеши своего конфига и мы сравниваем их с вашим значением (значение не
      передаётся); для SSH-ключа — реальный вход на сервер; для CF-токена — запрос к API
      Cloudflare. Для произвольных значений живой тест честно помечается «пропущено».
    </HintBox>
  </div>
</template>

<style scoped>
.view {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.head h1 {
  font-size: 20px;
}
.sub {
  margin: 4px 0 0;
  color: var(--text-dim);
  font-size: 12.5px;
  max-width: 640px;
}

.actions-row {
  display: flex;
  gap: 10px;
}

.btn {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 9px 16px;
  border-radius: 10px;
  border: 1px solid var(--border);
  background: var(--bg-elevated);
  color: var(--text);
  font-size: 13px;
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
.btn.primary {
  background: var(--accent);
  border-color: var(--accent);
  color: #0b1020;
  font-weight: 600;
}

.form {
  padding: 18px;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--bg-panel);
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.form h2 {
  font-size: 15px;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.lbl {
  font-size: 12px;
  color: var(--text-faint);
}

input,
select {
  padding: 9px 12px;
  border-radius: 9px;
  border: 1px solid var(--border);
  background: var(--bg-elevated);
  color: var(--text);
  font-size: 13px;
  outline: none;
  transition: border-color 0.12s;
}
input:focus,
select:focus {
  border-color: var(--accent);
}
input::placeholder {
  color: var(--text-faint);
}

.value-row {
  display: flex;
  gap: 8px;
}
.value-row input {
  flex: 1;
}

.eye {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 38px;
  border-radius: 9px;
  border: 1px solid var(--border);
  background: var(--bg-elevated);
  color: var(--text-faint);
  cursor: pointer;
}
.eye:hover {
  color: var(--text);
  background: var(--bg-hover);
}

.kind-hint {
  margin: -4px 0 0;
  font-size: 12px;
  color: var(--text-dim);
  line-height: 1.5;
}

.mismatch {
  margin: 0;
  font-size: 12px;
  color: var(--danger);
}

.form-actions {
  display: flex;
  gap: 10px;
}

.empty-block {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 14px 16px;
  border: 1px dashed var(--border);
  border-radius: 12px;
  color: var(--text-dim);
  font-size: 12.5px;
}

.cards {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
</style>
