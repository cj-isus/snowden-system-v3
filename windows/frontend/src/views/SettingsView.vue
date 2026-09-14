<script setup lang="ts">
/**
 * Настройки (concept): профиль доставки (подписанный metadata-envelope),
 * сводка секретов, системная информация, «О защите». Все значения — факты
 * из биндингов (GetDeliveryProfile, ListSecrets), выдуманных нет.
 */
import { computed, onMounted, ref } from 'vue'
import HintBox from '../components/HintBox.vue'
import { api, isBuildPhase } from '../api/backend'
import { APP_VERSION } from '../api/contract'
import type { DeliveryProfile, OnboardingPreview, UpdatePreview } from '../api/contract'
import { useSecretsStore } from '../composables/secretsStore'
import { fmtDateTime } from '../api/labels'

const profile = ref<DeliveryProfile | null>(null)
const profileError = ref('')
const { items } = useSecretsStore()

// Автозапуск (V2-048/F14): HKCU Run, без UAC. Ошибка — честно текстом.
const autostart = ref(false)
const autostartBusy = ref(false)
const autostartError = ref('')
async function toggleAutostart(): Promise<void> {
  if (isBuildPhase() || autostartBusy.value) return
  autostartBusy.value = true
  autostartError.value = ''
  try {
    await api.setAutostart(!autostart.value)
    autostart.value = !autostart.value
  } catch (e) {
    autostartError.value = e instanceof Error ? e.message : String(e)
  } finally {
    autostartBusy.value = false
  }
}

onMounted(async () => {
  if (isBuildPhase()) {
    profileError.value = 'Биндинги недоступны: приложение запущено вне Wails (предпросмотр).'
    return
  }
  try {
    profile.value = await api.getDeliveryProfile()
  } catch (e) {
    profileError.value = e instanceof Error ? e.message : String(e)
  }
  try {
    autostart.value = await api.getAutostart()
  } catch {
    /* справочная настройка; ошибка уйдёт при переключении */
  }
})

const statusPill = computed(() => {
  const p = profile.value
  if (!p) return { cls: 'neutral', text: 'Нет данных' }
  if (p.present && !p.error) return { cls: 'success', text: 'Установлено' }
  if (p.present && p.error) return { cls: 'warning', text: 'С замечанием' }
  return { cls: 'neutral', text: 'Встроенный набор' }
})

function fmtDate(iso: string): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' })
}

function shortId(id: string): string {
  if (!id) return '—'
  return id.length > 10 ? id.slice(0, 8) + '…' : id
}

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
    obImpPreview.value = await api.previewOnboarding(t, obImpPass.value)
  } catch (e) {
    obImpPreview.value = null
    obError.value = e instanceof Error ? e.message : String(e)
  } finally { obBusy.value = false }
}

async function doImportApply(): Promise<void> {
  if (isBuildPhase() || obBusy.value || !obImpPreview.value) return
  if (obImpPreview.value.wouldDowngrade) return // кнопка скрыта, но guard честный
  obBusy.value = true; obError.value = ''
  try {
    obImpDone.value = await api.applyOnboarding(obImpTransport.value, obImpPass.value, true)
    obImpPreview.value = null
  } catch (e) {
    obError.value = e instanceof Error ? e.message : String(e)
  } finally { obBusy.value = false }
}

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
  <div class="view">
    <header class="head">
      <div class="section-title-group">
        <h2>Настройки</h2>
      </div>
      <p class="sub">
        Профиль доставки обновлений, сводка локального хранилища секретов и сведения о защите.
      </p>
    </header>

    <div class="grid">
      <!-- Профиль доставки -->
      <section class="card delivery">
        <div class="card-header">
          <h2>Профиль доставки</h2>
          <span class="pill" :class="statusPill.cls"><span class="dot"></span>{{ statusPill.text }}</span>
        </div>

        <div v-if="profileError" class="empty-block">{{ profileError }}</div>

        <template v-else-if="profile">
          <div class="rows">
            <div class="row"><span>Версия обновления</span><strong>{{ profile.present ? profile.version : '—' }}</strong></div>
            <div class="row"><span>key_id</span><strong class="mono">{{ profile.present ? shortId(profile.keyId) : '—' }}</strong></div>
            <div class="row"><span>Действует до</span><strong>{{ profile.present ? fmtDate(profile.expiresAt) : '—' }}</strong></div>
            <div class="row"><span>Доверенные ключи</span><strong>{{ profile.trustedKeys || '—' }}</strong></div>
            <div class="row"><span>Каналы из обновления</span><strong>{{ profile.present ? `${profile.channelsApplied} · из подписанного обновления` : 'встроенный набор' }}</strong></div>
          </div>
          <p v-if="profile.error" class="profile-error">{{ profile.error }}</p>
        </template>

        <HintBox kind="info" title="Что это">
          Подписанный metadata-envelope (FR-008) заменяет встроенный набор каналов. Envelope без
          действующей подписи, с даунгрейдом версии или истёкший — отвергается (fail-closed).
        </HintBox>
      </section>

      <!-- Системные настройки (V2-048/F14) -->
      <section class="card">
        <div class="card-header">
          <h2>Система</h2>
        </div>
        <div class="rows">
          <div class="row">
            <span>Автозапуск с системой</span>
            <button class="btn" :disabled="autostartBusy" @click="toggleAutostart">
              {{ autostart ? 'Включён — выключить' : 'Выключен — включить' }}
            </button>
          </div>
        </div>
        <p v-if="autostartError" class="profile-error">{{ autostartError }}</p>
        <HintBox kind="info" title="Как работает">
          Обычная пользовательская запись в реестре (HKCU\...\Run, без прав администратора).
          Приложение стартует с флагом --auto-connect: подключение идёт через честный
          failover-on-start с probe-проверкой, как и при ручном «Подключить».
        </HintBox>
      </section>

      <!-- Секреты (сводка) -->
      <section class="card secrets">
        <div class="card-header">
          <h2>Секреты</h2>
          <button class="btn" @click="$emit('navigate', 'secrets')">Управление секретами ›</button>
        </div>

        <div v-if="items === null" class="empty-block">Список секретов недоступен: источник данных не отвечает.</div>
        <div v-else-if="items.length === 0" class="empty-block">Хранилище пусто. Добавьте значения на странице «Секреты».</div>
        <div v-else class="rows">
          <div v-for="s in items" :key="s.id" class="row secret-row">
            <span class="sr-title">{{ s.title }}</span>
            <span class="sr-fp mono">{{ s.fingerprint ? s.fingerprint.slice(0, 10) + '…' : '—' }}</span>
            <span class="pill" :class="s.verifyStatus === 'ok' ? 'success' : s.verifyStatus === 'failed' ? 'danger' : 'neutral'">
              <span class="dot"></span>
              {{ s.verifyStatus === 'ok' ? 'Verify OK' : s.verifyStatus === 'failed' ? 'Ошибка' : 'Не проверялся' }}
            </span>
            <span class="sr-reveals">раскрытий: {{ s.reveals }}</span>
            <span class="sr-updated">{{ fmtDateTime(s.updatedAt) }}</span>
          </div>
        </div>

        <p class="secrets-note">
          Значения никогда не показываются: только SHA256-отпечатки и статусы проверки.
        </p>
      </section>

      <!-- Перенос на новое устройство (V2-049/F8) -->
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
              type="password"
              class="ob-input"
              placeholder="Парольная фраза бандла"
              autocomplete="off"
            />
            <textarea
              v-model="obImpTransport"
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
    </div>

    <!-- О защите -->
    <section class="card about">
      <div class="card-header"><h2>О защите</h2></div>
      <div class="about-grid">
        <div>
          <h3>Что делает snowden.system</h3>
          <p>Защищённый трафик проходит только через проверенные каналы.</p>
          <p>Если рабочего канала нет — соединение останавливается.</p>
        </div>
        <div>
          <h3>Что продукт НЕ обещает</h3>
          <ul>
            <li>работу при полном shutdown / allowlist;</li>
            <li>работу QUIC при фильтрации UDP;</li>
            <li>вечную работоспособность одного сервера.</li>
          </ul>
        </div>
        <div>
          <h3>Почему VPN иногда отключается сам</h3>
          <p>Fail-closed — лучше остановить передачу, чем отправить трафик напрямую.</p>
        </div>
      </div>
    </section>

    <!-- Обновления приложения (V2-050/F15) -->
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

    <!-- Системная информация -->
    <section class="card sysinfo">
      <div class="card-header"><h2>Системная информация</h2></div>
      <div class="about-grid sys-grid">
        <div class="rows">
          <div class="row"><span>Версия</span><strong>v{{ APP_VERSION }}</strong></div>
          <div class="row"><span>Ядро</span><strong>sing-box</strong></div>
          <div class="row"><span>Сборка</span><strong>pinned</strong></div>
        </div>
        <div class="rows">
          <div class="row"><span>Хранилище секретов</span><strong>DPAPI (%AppData%)</strong></div>
          <div class="row"><span>Один экземпляр</span><strong>SingleInstanceLock</strong></div>
          <div class="row"><span>Журнал</span><strong>%AppData%\snowden-system\logs</strong></div>
        </div>
      </div>
    </section>
  </div>
</template>

<script lang="ts">
export default { emits: ['navigate'] }
</script>

<style scoped>
.view {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.head h2 {
  margin: 0;
  font-size: 18px;
  font-weight: 650;
}
.sub {
  margin: 5px 0 0;
  color: var(--muted);
  font-size: 11.5px;
  max-width: 720px;
}

.grid {
  display: grid;
  grid-template-columns: 1fr 1.2fr;
  gap: 10px;
}

.delivery,
.secrets,
.about,
.sysinfo {
  padding: 13px;
}

.rows {
  margin-top: 8px;
}
.row {
  min-height: 28px;
  display: flex;
  align-items: center;
  gap: 10px;
  border-bottom: 1px solid #142d40;
  color: #8298ab;
  font-size: 10.5px;
}
.row strong {
  color: #dce7ee;
  font-weight: 500;
  margin-left: auto;
  text-align: right;
}

.secret-row .sr-title {
  color: #dbe6ed;
  flex: 0 0 auto;
  max-width: 34%;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.secret-row .sr-fp {
  color: #6d8497;
  font-size: 9.5px;
}
.secret-row .pill {
  margin-left: auto;
}
.secret-row .sr-reveals,
.secret-row .sr-updated {
  color: #55708a;
  font-size: 9px;
  white-space: nowrap;
}

.profile-error {
  margin: 8px 0 0;
  color: #ff8a90;
  font-size: 10.5px;
}
.secrets-note {
  margin: 9px 0 0;
  color: #6f8699;
  font-size: 9px;
}

.empty-block {
  margin-top: 10px;
  padding: 10px 12px;
  border: 1px dashed var(--border);
  border-radius: 6px;
  color: var(--muted);
  font-size: 11px;
}

.about-grid {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr;
  gap: 20px;
  margin-top: 10px;
}
.sys-grid {
  grid-template-columns: 1fr 1fr;
}
.about-grid h3 {
  margin: 0 0 6px;
  color: #e2ecf2;
  font-size: 11px;
}
.about-grid p,
.about-grid li {
  color: #8499ab;
  font-size: 10px;
  line-height: 1.5;
}
.about-grid p {
  margin: 0 0 6px;
}
.about-grid ul {
  margin: 0;
  padding-left: 15px;
}

@media (max-width: 1000px) {
  .grid {
    grid-template-columns: 1fr;
  }
}

/* Onboarding (F8) */
.ob-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
  margin-top: 8px;
}
.ob-col h3 {
  margin: 0 0 8px;
  font-size: 14px;
}
.ob-input {
  width: 100%;
  box-sizing: border-box;
  margin-bottom: 8px;
  padding: 8px 10px;
  border: 1px solid var(--border, #3a3a4a);
  border-radius: 6px;
  background: var(--bg-input, #1c1c28);
  color: inherit;
  font: inherit;
}
.ob-textarea {
  resize: vertical;
  font-family: monospace;
  font-size: 12px;
}
.ob-actions {
  display: flex;
  gap: 8px;
  margin: 4px 0 8px;
}
.ob-qr {
  width: 220px;
  height: 220px;
  border-radius: 8px;
  background: #fff;
  padding: 8px;
}
.ob-preview {
  border: 1px solid var(--border, #3a3a4a);
  border-radius: 8px;
  padding: 10px 12px;
  margin-top: 8px;
}
.ob-keys .mono {
  font-size: 12px;
  letter-spacing: 1px;
}
.ob-warn {
  color: #e2b344;
  margin: 6px 0 0;
}
.ob-done {
  border-color: #3f9d63;
  color: #7dd8a0;
}
.mono-small {
  font-family: monospace;
  font-size: 11px;
}
@media (max-width: 1000px) {
  .ob-grid {
    grid-template-columns: 1fr;
  }
}
</style>
