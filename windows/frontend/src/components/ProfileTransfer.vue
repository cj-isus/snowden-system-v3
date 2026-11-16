<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { api, isBuildPhase, hasProtectedProfileTransfer } from '../api/backend'
import type { OnboardingPreview, PublicProfileMode, ProtectedProfileTransferRequest } from '../api/contract'
import PublicProfileSource from './PublicProfileSource.vue'
import PinPolicySource from './PinPolicySource.vue'
import HintBox from './HintBox.vue'
import { usePreviewExpiry } from '../composables/previewExpiry'
import { useProfileActions } from '../composables/profileActions'
const emit = defineEmits<{ changed: [] }>()
const actions = useProfileActions()
const { pendingLabel } = actions
const obPass = ref('')
const obBusy = actions.busy
const obError = ref('')
const obQr = ref('')
const obSavedPath = ref('')
const obImpPass = ref('')
const obImpTransport = ref('')
const obImpMode = ref<PublicProfileMode>('profile')
const obImpRoot = ref('')
const obImpPolicy = ref('')
const obImpTarget = ref('')
const protectedTransferAvailable = computed(hasProtectedProfileTransfer)
const importNeedsRoot = computed(() => obImpMode.value === 'enroll' || obImpMode.value === 'epoch_enroll')
const importNeedsTarget = computed(() => ['recover', 'epoch_recover', 'epoch_enroll'].includes(obImpMode.value))
const obImpPreview = ref<OnboardingPreview | null>(null)
const obImpDone = ref<OnboardingPreview | null>(null)
let inputRevision = Symbol()
let previewInput: symbol | null = null
let mounted = true
let generation = 0
let clearingInput = false
function clearImportInputs(): void {
  clearingInput = true
  try { obImpPass.value = ''; obImpTransport.value = ''; obImpTarget.value = ''; inputRevision = Symbol(); previewInput = null }
  finally { clearingInput = false }
}
const { arm: armImportExpiry, clear: clearImportExpiry, check: checkImportExpiry } = usePreviewExpiry(() => {
  obImpPreview.value = null
  clearImportInputs()
  obError.value = 'Срок превью истёк. Введите данные и проверьте перенос заново.'
})
watch([obImpTransport, obImpPass, obImpMode, obImpRoot, obImpPolicy], () => {
  if (clearingInput) return
  generation++; inputRevision = Symbol()
  clearImportExpiry(); obImpPreview.value = null; obImpTarget.value = ''; obImpDone.value = null; obError.value = ''; previewInput = null
}, { flush: 'sync' })
watch(actions.revision, () => {
  if (actions.ownsPending.value) return
  generation++; clearImportExpiry(); clearImportInputs()
  obPass.value = ''; obQr.value = ''; obSavedPath.value = ''; obImpPreview.value = null; obImpDone.value = null; obError.value = ''
}, { flush: 'sync' })
watch(actions.changed, () => { if (mounted && actions.current()) emit('changed') })
onBeforeUnmount(() => {
  mounted = false; generation++; clearImportExpiry(); clearImportInputs()
  obPass.value = ''; obQr.value = ''; obSavedPath.value = ''; obImpPreview.value = null; obImpDone.value = null
})
function owned(current: number): boolean { return mounted && actions.current() && current === generation }
function encryptedRequest(): ProtectedProfileTransferRequest {
  return { transport: obImpTransport.value, passphrase: obImpPass.value, mode: obImpMode.value, approved_root_sha256: importNeedsRoot.value ? obImpRoot.value : '', approved_pin_policy_sha256: importNeedsRoot.value ? obImpPolicy.value : '', approved_target_sha256: importNeedsTarget.value ? obImpTarget.value : '' }
}
async function doExportQr(): Promise<void> {
  if (!mounted || isBuildPhase() || obBusy.value || obPass.value.length < 8) return
  const original = actions.begin('transfer_export')
  if (!original) return
  const current = ++generation
  clearImportExpiry(); obImpPreview.value = null; previewInput = null
  obError.value = ''; obQr.value = ''; obSavedPath.value = ''; obImpDone.value = null
  try {
    const request = api.exportOnboardingQr(obPass.value)
    obPass.value = ''
    const result = await request
    const accepted = owned(current) && original.current()
    original.received(accepted)
    if (accepted) obQr.value = result.dataUrl
  } catch (error) { const message = original.failed(error); if (owned(current)) obError.value = message }
  finally { obPass.value = ''; original.finish() }
}
async function doSaveFile(): Promise<void> {
  if (!mounted || isBuildPhase() || obBusy.value || obPass.value.length < 8) return
  const original = actions.begin('transfer_save')
  if (!original) return
  const current = ++generation
  clearImportExpiry(); obImpPreview.value = null; previewInput = null
  obError.value = ''; obQr.value = ''; obSavedPath.value = ''; obImpDone.value = null
  try {
    const request = api.saveOnboardingFile(obPass.value)
    obPass.value = ''
    const path = await request
    if (path === '') original.cancelled()
    else original.received(owned(current) && original.current())
    if (owned(current) && original.current()) obSavedPath.value = path
  } catch (error) { const message = original.failed(error); if (owned(current)) obError.value = message }
  finally { obPass.value = ''; original.finish() }
}
async function doImportPreview(): Promise<void> {
  if (!mounted || isBuildPhase() || obBusy.value || !obImpPass.value) return
  if (importNeedsRoot.value && (!/^[a-f0-9]{64}$/.test(obImpRoot.value) || !/^[a-f0-9]{64}$/.test(obImpPolicy.value))) { obError.value = 'Независимо подтвердите оба SHA256: root.json и списка отзывов.'; return }
  const original = actions.begin('transfer_preview')
  if (!original) return
  const current = ++generation
  clearImportExpiry(); obImpPreview.value = null; previewInput = null
  obError.value = ''; obImpDone.value = null; obQr.value = ''; obSavedPath.value = ''
  try {
    let transport = obImpTransport.value
    if (!transport) {
      transport = await api.loadOnboardingFile()
      if (transport === '') { original.cancelled(); return }
      original.received(false) // selection alone is not a completed preview
    }
    if (!owned(current) || !original.current()) return
    clearingInput = true
    try { obImpTransport.value = transport }
    finally { clearingInput = false }
    const snapshot = inputRevision // no second JSON/password-bearing input copy
    const result = obImpMode.value === 'profile' ? await api.previewOnboarding(transport, obImpPass.value) : await api.previewProtectedProfileTransfer(encryptedRequest())
    original.received(false)
    if (!owned(current) || inputRevision !== snapshot || !original.current()) return
    if (!result.validUntil) { obError.value = 'Полный срок действия превью не передан; применение недоступно.'; clearImportInputs(); return }
    previewInput = snapshot
    obImpPreview.value = result
    armImportExpiry(result.validUntil)
    original.received(true)
  } catch (error) {
    const message = original.failed(error)
    if (owned(current)) { obImpPreview.value = null; previewInput = null; clearImportInputs(); obError.value = message }
  } finally { original.finish() }
}
async function doImportApply(): Promise<void> {
  const selected = obImpPreview.value
  if (!mounted || isBuildPhase() || obBusy.value || !selected) return
  if (previewInput !== inputRevision || importNeedsTarget.value && obImpTarget.value !== selected.targetSHA256 || selected.wouldDowngrade || checkImportExpiry()) return
  const original = actions.begin('transfer_apply')
  if (!original) return
  const current = ++generation
  const request = encryptedRequest(), mode = obImpMode.value
  clearImportExpiry(); obImpPreview.value = null; previewInput = null
  obError.value = ''
  try {
    const pending = mode === 'profile' ? api.applyOnboarding(request.transport, request.passphrase, true) : api.applyProtectedProfileTransfer(request)
    clearImportInputs()
    const result = await pending
    original.received(false)
    if ((selected.targetSHA256 && result.targetSHA256 !== selected.targetSHA256) || (selected.bundleIdentity && result.bundleIdentity !== selected.bundleIdentity)) {
      if (owned(current)) obError.value = original.failed(undefined)
      return
    }
    const accepted = owned(current) && original.current()
    original.received(accepted)
    if (accepted) obImpDone.value = result
  } catch (error) { const message = original.failed(error); if (owned(current)) obError.value = message }
  finally { clearImportInputs(); original.finish() }
}
</script>
<template>
<section class="profile-transfer" :aria-busy="obBusy">
        <p v-if="pendingLabel" class="secrets-note" role="status">{{ pendingLabel }} Закрытие панели не отменяет отправленный вызов.</p>
        <p v-if="actions.settlementLabel.value && !actions.attentionLabel.value" class="secrets-note" role="status">{{ actions.settlementLabel.value }}</p>
        <HintBox v-if="actions.attentionLabel.value" kind="warn" title="Предыдущий результат требует проверки">
          <p role="status">{{ actions.attentionLabel.value }}</p>
          <button class="btn" type="button" @click="actions.dismissAttention">Скрыть напоминание</button>
          <p>Скрытие сообщения не отменяет операцию и не подтверждает откат.</p>
        </HintBox>
        <PublicProfileSource />
        <PinPolicySource />
        <div class="card-header"><h2>Перенос на новое устройство</h2></div>
        <p class="secrets-note">
          Бандл содержит подписанный профиль каналов, ключи доверия и значения секретов,
          зашифрованные (scrypt + AES-256-GCM) парольной фразой. Фраза нигде не сохраняется —
          без неё бандл бесполезен. Импорт проверяет подпись профиля и отвергает бандл старее
          уже принятого (антидаунгрейд).
        </p>

        <div class="ob-grid">
          <div class="ob-col">
            <h3>Экспорт (с этого устройства)</h3>
            <input
              v-model="obPass"
              :disabled="obBusy"
              aria-label="Пароль экспорта"
              type="password"
              class="ob-input"
              placeholder="Парольная фраза (минимум 8 символов)"
              autocomplete="new-password"
            />
            <div class="ob-actions">
              <button class="btn" :disabled="obBusy || obPass.length < 8" @click="doExportQr">Показать QR</button>
              <button class="btn" :disabled="obBusy || obPass.length < 8" @click="doSaveFile">Сохранить в файл…</button>
            </div>
            <img v-if="obQr" :src="obQr" class="ob-qr" alt="QR-код бандла переноса" />
            <p class="secrets-note">После отправки команды фраза экспорта очищается. Для следующего экспорта введите её снова.</p>
            <p v-if="obSavedPath" class="secrets-note">Приложение вернуло файл переноса: {{ obSavedPath }}</p>
          </div>

          <div class="ob-col">
            <h3>Импорт (на новом устройстве)</h3>
            <label for="encrypted-import-mode">Режим импорта</label>
            <select id="encrypted-import-mode" v-model="obImpMode" class="ob-input" :disabled="obBusy">
              <option value="profile">Обычный импорт</option>
              <option value="enroll" :disabled="!protectedTransferAvailable">Первая установка</option>
              <option value="epoch_enroll" :disabled="!protectedTransferAvailable">Первая установка с новым ключом издателя</option>
              <option value="recover" :disabled="!protectedTransferAvailable">Восстановить профиль</option>
              <option value="epoch_recover" :disabled="!protectedTransferAvailable">Восстановить после смены ключа издателя</option>
            </select>
            <template v-if="importNeedsRoot">
              <label for="encrypted-import-root">SHA256 root.json с доверенного устройства</label>
              <input id="encrypted-import-root" v-model.trim="obImpRoot" class="ob-input" autocomplete="off" spellcheck="false" :disabled="obBusy" aria-describedby="encrypted-import-approval-help" />
              <label for="encrypted-import-policy">SHA256 списка отзывов с доверенного устройства</label>
              <input id="encrypted-import-policy" v-model.trim="obImpPolicy" class="ob-input" autocomplete="off" spellcheck="false" :disabled="obBusy" aria-describedby="encrypted-import-approval-help" />
              <p id="encrypted-import-approval-help" class="secrets-note">Сверьте оба отпечатка отдельно с отправителем. Файл переноса не подтверждает собственный корень и актуальность списка отзывов.</p>
            </template>
            <input
              v-model="obImpPass"
              :disabled="obBusy"
              aria-label="Пароль импорта"
              type="password"
              class="ob-input"
              placeholder="Парольная фраза бандла"
              autocomplete="off"
            />
            <textarea
              v-model="obImpTransport"
              :disabled="obBusy"
              aria-label="Данные бандла"
              class="ob-input ob-textarea"
              rows="3"
              placeholder="…или вставьте transport-строку бандла (SNOB1.…)"
            ></textarea>
            <div class="ob-actions">
              <button class="btn" :disabled="obBusy || !obImpPass" @click="doImportPreview">Проверить бандл</button>
            </div>

            <div v-if="obImpPreview" class="ob-preview">
              <p><strong>Внутри бандла:</strong> каналов — {{ obImpPreview.channels }}, секретов — {{ obImpPreview.secrets }}, версия профиля — {{ obImpPreview.bundleIdentity || (Number.isSafeInteger(obImpPreview.bundleVersion) ? obImpPreview.bundleVersion : 'не отображается') }}.</p>
              <p v-if="obImpPreview.bootstrapPolicySHA256">Подтверждённый первый список отзывов: <span class="mono">{{ obImpPreview.bootstrapPolicySHA256 }}</span></p>
              <p v-if="obImpPreview.wouldDowngrade" class="ob-warn">Версия бандла ниже уже принятой ({{ obImpPreview.currentIdentity || (Number.isSafeInteger(obImpPreview.currentVersion) ? obImpPreview.currentVersion : 'не отображается') }}) — импорт будет отклонён.</p>
              <div v-if="obImpPreview.keys.length" class="ob-keys">
                <p>Ключи подписи (сверьте отпечатки с устройством-источником):</p>
                <div v-for="k in obImpPreview.keys" :key="k.keyId" class="row">
                  <span class="mono">{{ k.fingerprint }}</span>
                  <span class="sr-title">{{ k.keyId }}</span>
                </div>
              </div>
              <div class="ob-actions">
                <template v-if="importNeedsTarget">
                  <label for="encrypted-import-target">Подтвердите SHA256 профиля: <span class="mono">{{ obImpPreview.targetSHA256 }}</span></label>
                  <input id="encrypted-import-target" v-model.trim="obImpTarget" class="ob-input" autocomplete="off" spellcheck="false" :disabled="obBusy" />
                </template>
                <button
                  class="btn primary"
                  :disabled="obBusy || obImpPreview.wouldDowngrade || importNeedsTarget && obImpTarget !== obImpPreview.targetSHA256"
                  @click="doImportApply"
                >Применить на этом устройстве</button>
              </div>
            </div>

            <div v-if="obImpDone" class="ob-preview">
              Ответ на импорт получен: в результате перечислено каналов — {{ obImpDone.channels }}, секретов — {{ obImpDone.secrets }}. Перечитайте принятую версию профиля; применение к работающему соединению и готовность пути показываются отдельно.
            </div>
          </div>
        </div>

        <p v-if="obError" class="ob-warn" role="status">{{ obError }}</p>
        <p class="secrets-note">Новое действие с профилем заменяет прежнее превью. Закрытие панели очищает поля и отзывает отображение позднего ответа, но не отменяет отправленную операцию: повторный запрос ждёт её ответа. Строки запроса могут оставаться в памяти Wails до завершения.</p>
      </section>
</template>
<style scoped>
.profile-transfer { min-width: 0; padding: 18px 0; }
.ob-col { min-width: 0; }
.secrets-note { font-size: 13px; line-height: 1.6; color: var(--muted); }
.ob-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 24px; margin-top: 22px; }
h3 { font-size: 14px; margin-bottom: 12px; }
.ob-input { display: block; width: 100%; margin: 10px 0; padding: 10px 12px; background: var(--bg); border: 1px solid var(--border-light); border-radius: 6px; color: var(--text); font: inherit; }
.ob-actions { display: flex; gap: 8px; flex-wrap: wrap; margin: 12px 0; }
.ob-actions .btn { max-width: 100%; white-space: normal; overflow-wrap: anywhere; }
.ob-preview { padding: 14px; margin-top: 12px; border-left: 2px solid var(--accent); background: var(--accent-soft); overflow-wrap: anywhere; font-size: 13px; }
.ob-warn { color: var(--yellow); font-size: 13px; }
.ob-done { color: var(--green); }
.ob-qr { max-width: 220px; width: 100%; }
.ob-textarea { resize: vertical; }
.row { display: grid; gap: 6px; font-size: 12px; }
@media(max-width: 980px) { .ob-grid { grid-template-columns: 1fr; } }
</style>
