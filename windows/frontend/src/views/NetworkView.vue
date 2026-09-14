<script setup lang="ts">
/**
 * Сеть (concept): сетевая информация из фактов ОС (GetNetworkFacts), NetGuard
 * и диагностика. Пустой факт = «нет данных», никаких выдуманных провайдеров.
 * Поллинг фактов — общий useInfoFacts (один PowerShell-опрос на всё приложение).
 */
import { computed, onMounted, onUnmounted, ref } from 'vue'
import Icon from '../components/Icon.vue'
import type { IconName } from '../components/Icon.vue'
import HintBox from '../components/HintBox.vue'
import { api, isBuildPhase, netGuardStatus } from '../api/backend'
import type { AdaptiveStatus, NetGuardStatus } from '../api/contract'
import { useBackendState, initBackendState } from '../composables/backendState'
import { useInfoFacts } from '../composables/infoFacts'
import { fmtDateTime } from '../api/labels'

const { isRunning } = useBackendState()
const { facts, refresh: refreshFacts } = useInfoFacts()

const factsError = ref('')
const netGuard = ref<NetGuardStatus | null>(null)
const adaptive = ref<AdaptiveStatus | null>(null)
const adaptiveBusy = ref(false)
const splitList = ref<string[]>([])
const loading = ref(false)
let timer: number | undefined

async function refresh(): Promise<void> {
  loading.value = true
  try {
    if (!isBuildPhase()) splitList.value = await api.getSplitDirect()
  } catch { /* read-only справка */ }
  try {
    if (isBuildPhase()) {
      factsError.value = 'Биндинги недоступны: приложение запущено вне Wails (предпросмотр).'
    } else {
      await refreshFacts()
      factsError.value = ''
    }
  } catch (e) {
    factsError.value = e instanceof Error ? e.message : String(e)
  }
  try {
    netGuard.value = await netGuardStatus()
  } catch {
    netGuard.value = null
  }
  try {
    adaptive.value = await api.getAdaptiveStatus()
  } catch {
    adaptive.value = null
  }
  loading.value = false
}

async function runAdaptive(): Promise<void> {
  adaptiveBusy.value = true
  try {
    adaptive.value = await api.runAdaptiveCheck()
  } catch (e) {
    factsError.value = e instanceof Error ? e.message : String(e)
  }
  adaptiveBusy.value = false
}

onMounted(() => {
  void initBackendState()
  void refresh()
  timer = window.setInterval(() => void refresh(), 30000)
})
onUnmounted(() => window.clearInterval(timer))

const rows = computed<{ icon: IconName; label: string; value: string | null }[]>(() => {
  const f = facts.value
  return [
    { icon: 'pulse', label: 'Тип соединения', value: f?.connectionType || null },
    { icon: 'pulse', label: 'Название сети', value: f?.ssid || null },
    { icon: 'server', label: 'Локальный IP', value: f?.localIP || null },
    { icon: 'key', label: 'Провайдер', value: isRunning.value ? 'Скрыт туннелем' : (f?.isp || null) },
    { icon: 'key', label: 'Публичный IP (прямой egress)', value: f?.publicIP || null },
    { icon: 'shield', label: 'Страна egress', value: f?.country || null },
    { icon: 'pulse', label: 'Путь через туннель', value: f ? (f.dnsViaTunnel ? 'работает (socks5h)' : 'не работает / VPN выключен') : null },
    { icon: 'pulse', label: 'Проверено', value: f?.checkedAt ? fmtDateTime(f.checkedAt) : null },
  ]
})

const countryOriginNote = computed(() => facts.value?.countryOrigin || '')
</script>

<template>
  <div class="view">
    <header class="head">
      <div class="section-title-group">
        <h2>Сеть</h2>
        <span v-if="loading" class="pill neutral"><span class="dot"></span>обновление…</span>
      </div>
      <p class="sub">
        Факты о сетевом подключении: адаптер маршрута по умолчанию, SSID, прямой egress.
        Значения собирает ОС-сборщик (PowerShell/netsh) и HTTP-наблюдение; пусто = «нет данных».
      </p>
    </header>

    <div class="grid">
      <section class="card netinfo">
        <div class="card-header">
          <h2>Сетевая информация</h2>
          <span class="connection-pill" :class="{ wifi: facts?.connectionType === 'Wi-Fi' }">
            {{ facts?.connectionType || 'нет данных' }}
          </span>
        </div>

        <div v-if="factsError" class="empty-block">{{ factsError }}</div>

        <div class="network-list">
          <div v-for="r in rows" :key="r.label" class="network-row">
            <span class="net-ico"><Icon :name="r.icon" :size="12" /></span>
            <span class="net-label">{{ r.label }}</span>
            <strong :class="{ 'no-data': !r.value }">{{ r.value ?? 'нет данных' }}</strong>
          </div>
        </div>

        <p v-if="countryOriginNote" class="origin-note mono">источник страны: {{ countryOriginNote }}</p>
      </section>

      <section class="card">
        <div class="card-header">
          <h2>Прямой обход процессов</h2>
        </div>
        <p v-if="splitList.length" class="sub">
          Эти приложения ходят мимо туннеля (правило доставлено подписанным конвертом):
        </p>
        <p v-else class="sub">
          Список пуст: весь трафик идёт через туннель. Изменить список может только владелец —
          он приходит в подписанном конверте конфигурации, вручную в приложении не редактируется.
        </p>
        <div class="split-chips">
          <span v-for="p in splitList" :key="p" class="adapter-pill mono">{{ p }}</span>
        </div>
      </section>

      <section class="card netguard">
        <div class="card-header">
          <h2>NetGuard</h2>
          <button class="btn" @click="$emit('navigate', 'netguard')">Открыть раздел ›</button>
        </div>

        <template v-if="netGuard">
          <div class="network-list">
            <div class="network-row">
              <span class="net-ico"><Icon name="pulse" :size="12" /></span>
              <span class="net-label">Системный прокси</span>
              <strong>{{ netGuard.proxy.enabled ? `Включён · ${netGuard.proxy.server}:${netGuard.proxy.port}` : 'Выключен' }}</strong>
            </div>
            <div class="network-row">
              <span class="net-ico"><Icon name="pulse" :size="12" /></span>
              <span class="net-label">Listener</span>
              <strong :class="netGuard.proxy.listenerAlive ? 'green-value' : 'red-value'">
                {{ netGuard.proxy.listenerAlive ? '● активен' : '○ не отвечает' }}
              </strong>
            </div>
            <div class="network-row">
              <span class="net-ico"><Icon name="pulse" :size="12" /></span>
              <span class="net-label">SOCKS VPN :1080</span>
              <strong :class="netGuard.socksAlive ? 'green-value' : ''">{{ netGuard.socksAlive ? 'слушается' : 'не слушается' }}</strong>
            </div>
            <div class="network-row">
              <span class="net-ico"><Icon name="warn" :size="12" /></span>
              <span class="net-label">Stale-прокси</span>
              <strong :class="netGuard.proxy.stale ? 'red-value' : 'green-value'">{{ netGuard.proxy.stale ? 'обнаружен' : 'нет' }}</strong>
            </div>
          </div>

          <div v-if="netGuard.events.length" class="netguard-events">
            <div v-for="(e, i) in netGuard.events.slice(0, 2)" :key="i" class="netguard-event" :class="e.kind === 'PROBLEM' ? 'problem' : 'healed'">
              <span>{{ e.kind === 'PROBLEM' ? '⚠' : '✓' }}</span>
              <div>
                <time>{{ e.time }}</time>
                <strong>{{ e.kind }} · {{ e.text }}</strong>
              </div>
            </div>
          </div>
        </template>
        <div v-else class="empty-block">Данные NetGuard недоступны (биндинг не отвечает или NetGuard не установлен).</div>
      </section>

      <section class="card adaptive">
        <div class="card-header">
          <h2>Адаптивный обход</h2>
          <button class="btn" :disabled="adaptiveBusy" @click="runAdaptive">
            {{ adaptiveBusy ? 'Проверка…' : 'Проверить сайты' }}
          </button>
        </div>
        <p class="sub adaptive-note">{{ adaptive?.note || 'Проверка доступности популярных сайтов через туннель и классификация блоков egress-IP.' }}</p>
        <template v-if="adaptive?.sites?.length">
          <div class="network-list">
            <div v-for="s in adaptive.sites" :key="s.domain" class="network-row">
              <span class="net-ico"><Icon :name="s.ok ? 'check' : s.challenge ? 'warn' : 'pulse'" :size="12" /></span>
              <span class="net-label">{{ s.domain }}</span>
              <strong :class="s.ok ? 'green-value' : s.challenge ? 'red-value' : 'no-data'">{{ s.detail }}</strong>
            </div>
          </div>
          <p class="origin-note mono">через WARP-egress на сервере: {{ adaptive.warpDomains.join(', ') }}</p>
        </template>
        <div v-else class="empty-block">Пока не проверялось — нажмите «Проверить сайты» при включённом VPN.</div>
      </section>
    </div>

    <HintBox kind="info" title="Почему «страна» может быть пустой">
      Страна egress берётся из эха сервиса наблюдения (ipapi.co). Если сервис недоступен или
      запрос заблокирован — честно «нет данных», без подстановки предполагаемой страны.
    </HintBox>
    <HintBox kind="warn" title="Приватность">
      Прямой egress-IP — это то, что видит внешний сервис без туннеля. При работающем VPN
      «провайдер» для целевых сайтов скрыт туннелем; сам факт прямого egress нужен для
      диагностики утечек.
    </HintBox>
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
  grid-template-columns: 1.15fr 1fr;
  gap: 10px;
}

.netinfo,
.netguard {
  padding: 13px;
}

.adaptive {
  padding: 13px;
  grid-column: 1 / -1;
}
.adaptive-note {
  margin: 4px 0 0;
  font-size: 10.5px;
  color: var(--muted);
}
.adaptive .card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.connection-pill {
  margin-left: auto;
  padding: 3px 9px;
  border-radius: 5px;
  font-size: 9.5px;
  color: #a1b2c0;
  border: 1px solid var(--gray-border);
  background: var(--gray-soft);
}
.connection-pill.wifi {
  color: #42e8ad;
  border-color: var(--green-border);
  background: var(--green-soft);
}

.network-list {
  margin-top: 8px;
}
.network-row {
  min-height: 28px;
  display: flex;
  align-items: center;
  gap: 8px;
  border-bottom: 1px solid #142d40;
  font-size: 10.5px;
}
.network-row .net-ico {
  width: 16px;
  color: #d2e0e9;
  display: inline-flex;
}
.network-row .net-label {
  flex: 1;
  color: #8399ad;
}
.network-row strong {
  color: #dce7ee;
  font-weight: 500;
  white-space: nowrap;
}
.network-row strong.no-data {
  color: #55708a;
  font-weight: 400;
}

.origin-note {
  margin: 8px 0 0;
  color: #55708a;
  font-size: 8.5px;
}

.netguard-events {
  margin-top: 10px;
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
}
.netguard-event {
  min-height: 48px;
  display: flex;
  gap: 8px;
  padding: 8px;
  border: 1px solid #1b374d;
  border-radius: 6px;
  background: #091b2a;
}
.netguard-event > span {
  font-size: 13px;
}
.netguard-event.healed > span {
  color: var(--green);
}
.netguard-event.problem > span {
  color: var(--yellow);
}
.netguard-event div {
  display: flex;
  flex-direction: column;
}
.netguard-event time {
  color: #6f879a;
  font-size: 8px;
}
.netguard-event strong {
  margin-top: 3px;
  color: #cbd8e2;
  font-size: 8.5px;
  font-weight: 500;
}

.empty-block {
  margin-top: 10px;
  padding: 10px 12px;
  border: 1px dashed var(--border);
  border-radius: 6px;
  color: var(--muted);
  font-size: 11px;
}

@media (max-width: 1000px) {
  .grid {
    grid-template-columns: 1fr;
  }
}
</style>
<style scoped>
.split-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
</style>
