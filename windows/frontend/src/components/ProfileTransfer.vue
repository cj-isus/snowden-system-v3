<script setup lang="ts">
import { ref, watch } from 'vue'
import { api, isBuildPhase } from '../api/backend'
import type { OnboardingPreview } from '../api/contract'
// ---------- Onboarding (V2-049/F8): перенос на новое устройство ----------
const obPass = ref('')
const obBusy = ref(false)
const obError = ref('')
const obQr = ref('') // data-URL PNG
const obSavedPath = ref('')
const obImpPass = ref('')
const obImpTransport = ref('')
const obImpPreview = ref<OnboardingPreview | null>(null)
const obImpDone = ref<OnboardingPreview | null>(null)
// Approval belongs to the exact bytes and passphrase that were previewed.
let previewInput = ''
watch([obImpTransport, obImpPass], () => { obImpPreview.value = null })

async function doExportQr(): Promise<void> {
  if (isBuildPhase() || obBusy.value) return
  obBusy.value = true; obError.value = ''; obQr.value = ''; obSavedPath.value = ''
  try {
    const r = await api.exportOnboardingQr(obPass.value)
    obQr.value = r.dataUrl
  } catch (e) {
    obError.value = e instanceof Error ? e.message : String(e)
  } finally { obBusy.value = false }
}

async function doSaveFile(): Promise<void> {
  if (isBuildPhase() || obBusy.value) return
  obBusy.value = true; obError.value = ''; obQr.value = ''; obSavedPath.value = ''
  try {
    const p = await api.saveOnboardingFile(obPass.value)
    if (p) obSavedPath.value = p
  } catch (e) {
    obError.value = e instanceof Error ? e.message : String(e)
  } finally { obBusy.value = false }
}

async function doImportPreview(): Promise<void> {
  if (isBuildPhase() || obBusy.value) return
  obBusy.value = true; obError.value = ''; obImpDone.value = null
  try {
    let t = obImpTransport.value
    if (!t) t = await api.loadOnboardingFile()
    if (!t) { obError.value = 'Бандл не выбран или пуст.'; return }
    obImpTransport.value = t
    const passphrase = obImpPass.value
    const result = await api.previewOnboarding(t, passphrase)
    if (obImpTransport.value === t && obImpPass.value === passphrase) {
      previewInput = t + '\0' + passphrase
      obImpPreview.value = result
    }
  } catch (e) {
    obImpPreview.value = null
    obError.value = e instanceof Error ? e.message : String(e)
  } finally { obBusy.value = false }
}

async function doImportApply(): Promise<void> {
  if (isBuildPhase() || obBusy.value || !obImpPreview.value) return
  if (previewInput !== obImpTransport.value + '\0' + obImpPass.value) return
  if (obImpPreview.value.wouldDowngrade) return // кнопка скрыта, но guard честный
  obBusy.value = true; obError.value = ''
  try {
    obImpDone.value = await api.applyOnboarding(obImpTransport.value, obImpPass.value, true)
    obImpPreview.value = null
  } catch (e) {
    obError.value = e instanceof Error ? e.message : String(e)
  } finally { obBusy.value = false }
}


</script>
<template>
<section class="card">
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
            <p v-if="obSavedPath" class="secrets-note">Сохранено: {{ obSavedPath }}</p>
          </div>

          <div class="ob-col">
            <h3>Импорт (на новом устройстве)</h3>
            <input
              v-model="obImpPass"
              aria-label="Пароль импорта"
              type="password"
              class="ob-input"
              placeholder="Парольная фраза бандла"
              autocomplete="off"
            />
            <textarea
              v-model="obImpTransport"
              aria-label="Данные бандла"
              class="ob-input ob-textarea"
              rows="3"
              placeholder="…или вставьте transport-строку бандла (SNOB1.…)"
            ></textarea>
            <div class="ob-actions">
              <button class="btn" :disabled="obBusy || !obImpPass" @click="doImportPreview">Проверить бандл</button>
            </div>

            <div v-if="obImpPreview" class="ob-preview">
              <p><strong>Внутри бандла:</strong> каналов — {{ obImpPreview.channels }}, секретов — {{ obImpPreview.secrets }}, версия профиля — {{ obImpPreview.bundleVersion }}.</p>
              <p v-if="obImpPreview.wouldDowngrade" class="ob-warn">Версия бандла ниже уже принятой ({{ obImpPreview.currentVersion }}) — импорт будет отклонён.</p>
              <div v-if="obImpPreview.keys.length" class="ob-keys">
                <p>Ключи подписи (сверьте отпечатки с устройством-источником):</p>
                <div v-for="k in obImpPreview.keys" :key="k.keyId" class="row">
                  <span class="mono">{{ k.fingerprint }}</span>
                  <span class="sr-title">{{ k.keyId }}</span>
                </div>
              </div>
              <div class="ob-actions">
                <button
                  class="btn primary"
                  :disabled="obBusy || obImpPreview.wouldDowngrade"
                  @click="doImportApply"
                >Применить на этом устройстве</button>
              </div>
            </div>

            <div v-if="obImpDone" class="ob-preview ob-done">
              ✔ Импорт применён: каналов — {{ obImpDone.channels }}, секретов — {{ obImpDone.secrets }}. Перезапустите подключение, чтобы новый профиль вступил в силу.
            </div>
          </div>
        </div>

        <p v-if="obError" class="ob-warn">{{ obError }}</p>
      </section>
</template>
<style scoped>
.card { padding: 22px; }
.secrets-note { font-size: 13px; line-height: 1.6; color: var(--muted); }
.ob-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 24px; margin-top: 22px; }
h3 { font-size: 14px; margin-bottom: 12px; }
.ob-input { display: block; width: 100%; margin: 10px 0; padding: 10px 12px; background: var(--bg); border: 1px solid var(--border-light); border-radius: 6px; color: var(--text); font: inherit; }
.ob-actions { display: flex; gap: 8px; flex-wrap: wrap; margin: 12px 0; }
.ob-preview { padding: 14px; margin-top: 12px; border-left: 2px solid var(--accent); background: var(--accent-soft); overflow-wrap: anywhere; font-size: 13px; }
.ob-warn { color: var(--yellow); font-size: 13px; }
.ob-done { color: var(--green); }
.ob-qr { max-width: 220px; width: 100%; }
.ob-textarea { resize: vertical; }
.row { display: grid; gap: 6px; font-size: 12px; }
@media(max-width: 980px) { .ob-grid { grid-template-columns: 1fr; } .card { padding: 16px; } }
</style>
