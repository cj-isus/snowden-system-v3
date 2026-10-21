<script setup lang="ts">
import { ref } from 'vue'
import { api, isBuildPhase } from '../api/backend'
import type { UpdatePreview } from '../api/contract'
// ---------- Обновления (V2-050/F15) ----------
const updBusy = ref(false)
const updError = ref('')
const updPreview = ref<UpdatePreview | null>(null)
let updDir = ''

function fmtSize(n: number): string {
  if (!n) return '—'
  if (n > 1024 * 1024) return (n / (1024 * 1024)).toFixed(1) + ' МБ'
  return Math.round(n / 1024) + ' КБ'
}

async function updPickAndCheck(): Promise<void> {
  if (isBuildPhase() || updBusy.value) return
  updBusy.value = true; updError.value = ''; updPreview.value = null
  try {
    const dir = await api.pickUpdateDir()
    if (!dir) return // отмена
    updDir = dir
    updPreview.value = await api.checkUpdate(dir)
  } catch (e) {
    updError.value = e instanceof Error ? e.message : String(e)
  } finally { updBusy.value = false }
}

async function updApply(): Promise<void> {
  if (isBuildPhase() || updBusy.value || !updDir) return
  if (!updPreview.value?.ok) return
  updBusy.value = true; updError.value = ''
  try {
    // Успех = процесс перезапустится; окно закроется само.
    await api.applyUpdate(updDir)
  } catch (e) {
    updError.value = e instanceof Error ? e.message : String(e)
    updBusy.value = false
  }
  // При успехе не снимаем busy: приложение уходит на перезапуск.
}

</script>
<template>
<section class="card">
      <div class="card-header"><h2>Обновления</h2></div>
      <p class="secrets-note">
        Обновление — подписанный манифест (та же цепочка Ed25519, что у профиля каналов)
        и payload с SHA-256. Применяется только версия строго выше текущей и выше этажа
        уже принятых (антидаунгрейд переживает переустановку старой версии). Подмена
        бинарника атомарна: старый exe сохраняется как .old до успешного старта нового.
      </p>
      <div class="ob-actions">
        <button class="btn" :disabled="updBusy" @click="updPickAndCheck">Выбрать каталог обновления…</button>
      </div>
      <div v-if="updPreview" class="ob-preview">
        <template v-if="updPreview.ok">
          <p><strong>Доступно:</strong> v{{ updPreview.version }} (текущая v{{ updPreview.currentVersion }})</p>
          <p v-if="updPreview.notes">{{ updPreview.notes }}</p>
          <p class="secrets-note mono-small">sha256: {{ updPreview.sha256.slice(0, 16) }}… · {{ fmtSize(updPreview.size) }} · key {{ updPreview.keyId.slice(0, 10) }}…</p>
          <div class="ob-actions">
            <button class="btn primary" :disabled="updBusy" @click="updApply">Применить и перезапустить</button>
          </div>
        </template>
        <template v-else>
          <p class="ob-warn">Обновление не принято: {{ updPreview.reason || 'причина не указана' }}</p>
          <p v-if="updPreview.version" class="secrets-note">Проверялась версия v{{ updPreview.version }} (текущая v{{ updPreview.currentVersion }}, этаж {{ updPreview.floor }}).</p>
        </template>
      </div>
      <p v-if="updError" class="ob-warn">{{ updError }}</p>
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
