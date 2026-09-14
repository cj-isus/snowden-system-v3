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
import type { DeliveryProfile } from '../api/contract'
import { useSecretsStore } from '../composables/secretsStore'
import { fmtDateTime } from '../api/labels'

const profile = ref<DeliveryProfile | null>(null)
const profileError = ref('')
const { items } = useSecretsStore()

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
</style>
