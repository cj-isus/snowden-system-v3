<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { api, hasPinPolicySource } from '../api/backend'
import { uiFailure } from '../api/failureMessages'
import type { PinPolicyPreview, PinPolicySourcePreview, PublicProfileRoute } from '../api/contract'
import { usePreviewExpiry } from '../composables/previewExpiry'
import { useProfileActions } from '../composables/profileActions'

const emit = defineEmits<{ changed: [] }>()
const actions = useProfileActions()
const { blocked } = actions
const available = computed(hasPinPolicySource)
const repository = ref('')
const route = ref<PublicProfileRoute>('protected')
const directConsent = ref(false)
const confirmed = ref(false)
const attempted = ref(false)
const phase = ref<'idle' | 'loading' | 'applying'>('idle')
const error = ref('')
const notice = ref('')
const receipt = ref<PinPolicySourcePreview | null>(null)
const done = ref<PinPolicyPreview | null>(null)
const { expired, arm: armExpiry, clear: clearExpiry, check: checkExpiry } = usePreviewExpiry(() => { confirmed.value = false; cancelOwned() })
const repositoryInput = ref<HTMLInputElement | null>(null)
const previewHeading = ref<HTMLElement | null>(null)
const doneMessage = ref<HTMLElement | null>(null)
const canApply = computed(() => available.value && !actions.busy.value && phase.value === 'idle' && receipt.value !== null && !expired.value && confirmed.value)
let generation = 0
let ownedRequest = ''
let runningRequest = ''
let mounted = true
function cancelOwned(): void {
  const id = ownedRequest
  ownedRequest = ''
  if (id && phase.value !== 'applying') void api.cancelPinPolicySource(id).catch(() => {})
}
function invalidate(): void {
  generation++
  clearExpiry()
  cancelOwned()
  receipt.value = null
  done.value = null
  confirmed.value = false
  attempted.value = false
  error.value = ''
  notice.value = ''
}
watch([repository, route, directConsent], invalidate, { flush: 'sync' })
watch(route, () => { directConsent.value = false }, { flush: 'sync' })
watch(actions.revision, () => { if (!actions.ownsPending.value) invalidate() }, { flush: 'sync' })
onBeforeUnmount(() => { mounted = false; generation++; clearExpiry(); cancelOwned(); receipt.value = null; done.value = null; confirmed.value = false })
function owned(current: number): boolean { return mounted && actions.current() && current === generation }

function cancel(): void {
  if (phase.value === 'applying') { notice.value = 'Применение уже запрошено. Закрытие превью не отменяет этот запрос; дождитесь результата.'; return }
  if (phase.value !== 'loading' && !receipt.value) return
  invalidate()
  notice.value = phase.value === 'loading' ? 'Отмена запрошена. Дождитесь ответа; если открыт выбор файла, закройте его.' : 'Превью закрыто. Применение по этому превью не запрашивалось; история проверки могла измениться.'
}
function displayTime(value: string): string {
  const time = new Date(value)
  return Number.isFinite(time.getTime()) ? time.toLocaleString('ru-RU') : 'Срок недоступен'
}
function displayRoute(value: PinPolicySourcePreview['route']): string {
  return { protected: 'Через защищённое подключение', bootstrap: 'Без VPN, с вашим согласием', local: 'Локальный источник', file: 'Выбранный файл' }[value]
}

async function preview(fromFile = false): Promise<void> {
  if (!available.value || actions.busy.value || phase.value !== 'idle' || !actions.current()) return
  invalidate()
  attempted.value = !fromFile
  if (!fromFile && !repository.value.trim()) {
    error.value = 'Укажите адрес источника или выберите файл списка отзывов.'
    repositoryInput.value?.focus()
    return
  }
  if (!fromFile && route.value === 'bootstrap' && !directConsent.value) {
    error.value = 'Подтвердите загрузку без VPN или выберите другой маршрут.'
    return
  }
  const original = actions.begin('policy_preview')
  if (!original) return
  const current = ++generation
  // This ID correlates UI ownership only. Go creates the approval nonce.
  const requestId = `policy-${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`
  ownedRequest = runningRequest = requestId
  phase.value = 'loading'
  try {
    const result = fromFile
      ? await api.previewPinPolicyFile(requestId)
      : await api.previewPinPolicySource({ request_id: requestId, source: { repository: repository.value.trim(), target_path: 'trust/pin-history.json', route: route.value, allow_direct: route.value === 'bootstrap' && directConsent.value } })
    original.received(false)
    if (!owned(current) || !original.current() || ownedRequest !== requestId) return
    if (!result) {
      ownedRequest = ''
      original.cancelled()
      notice.value = 'Выбор файла отменён. Применение по этому запросу не запрашивалось.'
      return
    }
    receipt.value = result
    armExpiry(result.expiresAt)
    original.received(true)
    await nextTick()
    if (!owned(current) || !original.current()) original.received(false)
    if (owned(current)) previewHeading.value?.focus()
  } catch (failure) {
    const message = original.failed(failure)
    if (owned(current)) {
      receipt.value = null
      clearExpiry()
      error.value = message
      cancelOwned()
    }
  } finally {
    if (mounted && runningRequest === requestId) { runningRequest = ''; phase.value = 'idle' }
    original.finish()
  }
}

async function apply(): Promise<void> {
  const selected = receipt.value
  if (!canApply.value || !selected) return
  if (checkExpiry()) return
  const original = actions.begin('policy_apply')
  if (!original) return
  const current = generation
  clearExpiry()
  phase.value = 'applying'
  receipt.value = null // consume once, including a failed apply
  confirmed.value = false
  error.value = ''
  notice.value = ''
  try {
    const result = await api.applyPinPolicySource(selected.token)
    original.received(false)
    if (!result.committed || result.policyVersion !== selected.policy.policyVersion || result.approval.policySHA256 !== selected.policy.approval.policySHA256 || result.approval.capsuleSHA256 !== selected.packageSHA256) {
      if (owned(current)) error.value = uiFailure.pinApply
      return
    }
    if (owned(current) && original.current()) {
      original.received(true)
      done.value = result
      await nextTick()
      if (!owned(current) || !original.current()) original.received(false)
      if (owned(current)) doneMessage.value?.focus()
    }
  } catch (failure) {
    const message = original.failed(failure)
    if (owned(current)) error.value = message
  } finally {
    if (mounted) { phase.value = 'idle'; ownedRequest = ''; emit('changed') }
    original.finish()
  }
}
</script>

<template>
  <section class="pin-policy-source" aria-labelledby="pin-policy-title">
    <h2 id="pin-policy-title">Список отозванных сертификатов</h2>
    <p class="muted">Обновите запреты отдельно от профиля на уже настроенном устройстве. Это помогает отключить небезопасные каналы, даже если старый профиль больше не подходит.</p>
    <p v-if="!available" class="muted" role="status">Отдельное обновление списка отзывов недоступно в этой версии приложения.</p>
    <form novalidate :aria-busy="phase !== 'idle'" @submit.prevent="preview()">
      <fieldset :disabled="blocked || !available || phase === 'applying'">
        <legend class="sr-title">Получение списка отзывов</legend>
        <div class="source-fields">
          <div>
            <label for="pin-policy-repository">Адрес источника</label>
            <input id="pin-policy-repository" ref="repositoryInput" v-model="repository" type="url" autocomplete="off" spellcheck="false" placeholder="https://ваш-источник" :aria-invalid="attempted && !repository.trim() ? 'true' : undefined" aria-describedby="pin-policy-source-help pin-policy-error" />
          </div>
          <div>
            <label for="pin-policy-route">Маршрут загрузки</label>
            <select id="pin-policy-route" v-model="route">
              <option value="protected">Через защищённое подключение</option>
              <option value="local">Локальный источник на этом ПК</option>
              <option value="bootstrap">Без VPN, с отдельным согласием</option>
            </select>
          </div>
        </div>
        <p id="pin-policy-source-help" class="muted">Используйте источник своего издателя или выберите полученный от него публичный файл. Новый просмотр заменяет ранее подготовленное превью профиля или списка.</p>
        <label v-if="route === 'bootstrap'" class="consent"><input v-model="directConsent" type="checkbox" />Разрешаю скачать этот публичный список без VPN.</label>
        <div class="actions">
          <button class="btn" type="submit" :disabled="blocked || !available || phase !== 'idle'">Получить превью</button>
          <button class="btn" type="button" :disabled="blocked || !available || phase !== 'idle'" @click="preview(true)">Выбрать файл…</button>
        </div>
      </fieldset>
    </form>
    <p v-if="phase === 'loading'" class="muted" role="status">Получение и проверка списка… Изменения ещё не применяются.</p>
    <p v-if="phase === 'applying'" class="muted" role="status">Применение запретов… Доступность подключения отображается отдельно в состоянии приложения.</p>
    <button v-if="phase !== 'applying' && (receipt || phase === 'loading')" class="btn cancel" type="button" @click="cancel">{{ phase === 'loading' ? 'Отменить получение' : 'Отменить превью' }}</button>
    <p id="pin-policy-error" class="source-error" :role="error ? 'alert' : undefined">{{ error }}</p>
    <p v-if="notice" class="muted" role="status">{{ notice }}</p>
    <div v-if="receipt" class="policy-preview">
      <h3 ref="previewHeading" tabindex="-1">Проверьте список перед применением</h3>
      <dl>
        <dt>Получен</dt><dd>{{ displayRoute(receipt.route) }}</dd>
        <dt>Версия списка</dt><dd>{{ receipt.policy.policyVersion }}</dd>
        <dt>Применённая версия</dt><dd>{{ receipt.policy.appliedVersion || 'Список ещё не применялся' }}</dd>
        <dt>Профиль устройства</dt><dd>{{ receipt.policy.profileIdentity }}</dd>
        <dt>Подтвердить до</dt><dd>{{ displayTime(receipt.expiresAt) }}</dd>
      </dl>
      <p v-if="receipt.policy.willRetireSession" class="muted">Применение остановит текущую или ожидающую сессию. Подключение возобновится только через разрешённый канал после обычных проверок; доступных каналов может не остаться.</p>
      <p v-else class="muted">Профиль и пароли остаются прежними. Сохранение списка само по себе не подтверждает доступность VPN.</p>
      <details>
        <summary>Отпечатки проверенного пакета</summary>
        <dl class="fingerprints"><dt>Список</dt><dd class="mono">{{ receipt.policy.approval.policySHA256 }}</dd><dt>Пакет</dt><dd class="mono">{{ receipt.packageSHA256 }}</dd></dl>
      </details>
      <p v-if="expired" class="source-error" role="status">Срок превью истёк. Получите новое перед применением.</p>
      <template v-else>
        <label class="consent"><input v-model="confirmed" type="checkbox" :disabled="blocked || phase !== 'idle'" />Подтверждаю применение показанного списка и возможную остановку подключения.</label>
        <button class="btn primary" type="button" :disabled="!canApply" @click="apply">Применить список отзывов</button>
      </template>
    </div>
    <p v-if="done" ref="doneMessage" class="source-done" role="status" tabindex="-1">Список версии {{ done.policyVersion }} сохранён. Состояние подключения и результат новых проверок смотрите в разделе «Состояние».</p>
  </section>
</template>

<style scoped>
.pin-policy-source { min-width: 0; border-bottom: 1px solid var(--border-light); padding-bottom: 24px; margin-bottom: 24px; }
h2 { font-size: 18px; margin-bottom: 10px; }
h3 { font-size: 15px; margin-bottom: 14px; }
.muted { color: var(--muted); font-size: 13px; line-height: 1.6; margin: 8px 0 14px; }
fieldset { border: 0; padding: 0; margin: 0; min-width: 0; }
.source-fields { display: grid; grid-template-columns: minmax(0, 1.3fr) minmax(0, 1fr); gap: 16px; max-width: 900px; }
label { display: block; font-size: 13px; }
input:not([type="checkbox"]), select { width: 100%; min-width: 0; margin-top: 6px; padding: 10px 12px; border: 1px solid var(--border-light); border-radius: 6px; background: var(--bg); color: var(--text); font: inherit; }
input[aria-invalid="true"] { border-color: var(--yellow); }
:is(input, select, button, summary, [tabindex]):focus-visible { outline: 2px solid var(--accent); outline-offset: 3px; }
.actions { display: flex; gap: 10px; flex-wrap: wrap; margin-top: 18px; }
.cancel { margin-top: 12px; }
.consent { display: flex; align-items: flex-start; gap: 10px; margin: 16px 0; line-height: 1.6; }
.consent input { margin-top: 5px; flex-shrink: 0; }
.policy-preview { min-width: 0; border-left: 2px solid var(--accent); padding-left: 16px; margin-top: 24px; }
dl { display: grid; grid-template-columns: minmax(120px, 180px) minmax(0, 1fr); gap: 10px 18px; font-size: 13px; }
dt { color: var(--muted); }
dd { margin: 0; overflow-wrap: anywhere; }
summary { cursor: pointer; font-size: 13px; margin: 16px 0; }
.fingerprints { margin-bottom: 16px; }
.source-error { color: var(--yellow); overflow-wrap: anywhere; line-height: 1.6; }
.source-error:empty { display: none; }
.source-done { color: var(--green); line-height: 1.6; }
@media(max-width: 700px) { .source-fields { grid-template-columns: minmax(0, 1fr); } dl { grid-template-columns: 1fr; gap: 4px; } dd { margin-bottom: 10px; } .policy-preview { padding-left: 12px; } }
</style>
