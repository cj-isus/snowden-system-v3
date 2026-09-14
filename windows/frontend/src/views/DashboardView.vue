<script setup lang="ts">
/**
 * Главная (concept): hero с power-кнопкой, карточка текущего канала,
 * карта маршрута, сводка защиты, быстрые факты, проверка пути, трафик,
 * таблица каналов, автозащита, события, нижняя сетка (статистика, сеть,
 * инструменты, система). Все данные — факты из backend; пусто = «нет данных».
 */
import { computed, onMounted, ref } from 'vue'
import Icon from '../components/Icon.vue'
import type { IconName } from '../components/Icon.vue'
import { useBackendState, initBackendState } from '../composables/backendState'
import { useLifecycle } from '../composables/lifecycle'
import { useInfoFacts } from '../composables/infoFacts'
import { validationStatusLabels, lifecycleLabels, fmtDateTime } from '../api/labels'
import { APP_VERSION } from '../api/contract'
import { netGuardStatus } from '../api/backend'
import type { NetGuardStatus } from '../api/contract'
import { uiLogLines } from '../composables/uiLog'

const { state, stateLoaded, isRunning, isBusy, primaryAction, channels } = useBackendState()
const { start, stop, runProbe } = useLifecycle()
const { facts, failover } = useInfoFacts()
const logLines = uiLogLines()

onMounted(() => {
  void initBackendState()
})

function onPrimary(): void {
  if (primaryAction.value.kind === 'start') void start()
  else void stop()
}

// ---------- hero: фактическое состояние ----------

const heroHeading = computed(() => {
  const s = state.value?.state
  if (!stateLoaded.value || !s) return 'Нет данных'
  return lifecycleLabels[s]
})

const heroTone = computed(() => {
  const s = state.value?.state
  if (s === 'running') return 'running'
  if (s === 'error') return 'error'
  if (s === 'starting' || s === 'stopping' || s === 'reloading') return 'busy'
  return 'stopped'
})

const heroDescription = computed(() => {
  if (state.value?.blockedReason) return `BLOCKED — запуск остаётся закрытым (fail-closed)`
  if (isRunning.value) return 'Трафик проходит через защищённый канал'
  if (state.value?.error) return state.value.error
  return 'Соединение не установлено. Запуск — строго после успешного protected-probe.'
})

const disconnectLabel = computed(() => {
  if (isBusy.value) return 'Подождите…'
  return isRunning.value ? 'Отключить' : 'Подключить'
})

const powerGlyph = computed(() => '⏻')

// ---------- автозащита (факты из failoverPolicy через useInfoFacts) ----------

const watchdogLabel = computed(() => {
  const f = failover.value
  if (!f) return 'нет данных сторожа'
  if (!f.enabled) return 'сторож запустится вместе с VPN'
  if (f.state === 'closed') return 'сторож активен'
  if (f.state === 'half-open') return 'идёт автопопытка'
  if (f.state === 'open') return 'cooldown после сбоя'
  return f.state
})

const nextCheckLabel = computed(() => {
  const f = failover.value
  if (!f || !f.enabled) return '—'
  return f.nextCheckIn <= 0 ? 'сейчас' : `≈ через ${f.nextCheckIn} с`
})

// Инциденты — факты журнала (failover/переключения/BLOCKED).
const incidentLines = computed(() =>
  logLines.value
    .filter((l) => /failover|переключ|BLOCKED|heal|probe/i.test(l.text))
    .slice(-3)
    .reverse(),
)

const failoverBadgeClass = computed(() => {
  const f = failover.value
  if (!f || !f.enabled) return 'neutral'
  if (f.state === 'closed') return 'success'
  if (f.state === 'open') return 'danger'
  return 'warning'
})

// ---------- текущий канал ----------

const activeChannel = computed(() => state.value?.activeChannel ?? null)
const channelStatusPill = computed(() => {
  const v = activeChannel.value?.validation
  if (!v) return null
  const cls =
    v === 'live-verified' ? 'live' : v === 'degraded' ? 'degraded' : v === 'blocked' ? 'blocked' : v === 'retired' || v === 'planned' ? 'neutral' : 'configured'
  return { cls, text: validationStatusLabels[v] ?? v }
})

const channelDetails = computed(() => {
  const c = activeChannel.value
  if (!c) return []
  return [
    { label: 'Транспорт', value: c.transport || '—' },
    { label: 'Порт', value: c.port ? String(c.port) : '—' },
    { label: 'Сервер', value: c.server || '—' },
  ]
})

// ---------- сводка защиты (факты из probe) ----------

const probeSteps = computed(() => state.value?.probe?.steps ?? [])
const summaryItems = computed(() => {
  const items: { icon: IconName; text: string; ok: boolean | null }[] = []
  const find = (needle: string): 'pass' | 'fail' | null => {
    const st = probeSteps.value.find((s) => s.name.toLowerCase().includes(needle))
    if (!st) return null
    return st.status === 'pass' ? 'pass' : st.status === 'fail' ? 'fail' : null
  }
  const dns = find('dns')
  const egress = find('egress')
  const leak = find('leak')
  const https = probeSteps.value.filter((s) => s.name.toLowerCase().startsWith('https'))
  const httpsOk = https.length > 0 && https.every((s) => s.status === 'pass')

  items.push({ icon: 'shield', text: 'Соединение защищено', ok: state.value?.probe?.ok ?? null })
  items.push({ icon: 'key', text: 'IP-адрес скрыт', ok: egress === 'pass' ? true : egress === 'fail' ? false : null })
  items.push({ icon: 'pulse', text: 'DNS через туннель', ok: dns === 'pass' ? true : dns === 'fail' ? false : null })
  items.push({ icon: 'server', text: 'Доступ к ресурсам открыт', ok: httpsOk ? true : https.some((s) => s.status === 'fail') ? false : null })
  items.push({ icon: 'warn', text: 'Утечки прямого трафика нет', ok: leak === 'pass' ? true : leak === 'fail' ? false : null })
  return items
})

// Ядро: факт из расширенного стейта (coreReady приходит из backend).
const coreReadyLabel = computed(() => {
  const s = state.value as (typeof state.value & { coreReady?: boolean }) | null
  if (!s || typeof s.coreReady !== 'boolean') return '—'
  return s.coreReady ? 'да' : 'нет'
})

// Egress-IP из последнего probe (факт из детализации шага «Expected egress»).
const egressFact = computed(() => {
  const st = probeSteps.value.find((s) => s.name.toLowerCase().includes('egress'))
  if (!st) return '—'
  const m = /egress IP: ([0-9a-fA-F.:]+)/.exec(st.detail) ?? /egress ([0-9a-fA-F.:]+),/.exec(st.detail)
  return m ? m[1] : st.detail || '—'
})

// ---------- карта маршрута ----------

// Позиция точки устройства фиксирована (ваше устройство); точка сервера
// появляется только при факте активного канала. Города не выдумываем.
const routeActive = computed(() => isRunning.value && !!activeChannel.value)
const routeServerLabel = computed(() => activeChannel.value?.id ?? '')
const routeServerHint = computed(() => activeChannel.value?.server ?? '')

// ---------- быстрые факты ----------

const netGuard = ref<NetGuardStatus | null>(null)
onMounted(async () => {
  try {
    netGuard.value = await netGuardStatus()
  } catch {
    netGuard.value = null
  }
})

const routingMode = computed(() => {
  if (state.value?.proxyMode === 'tun') return 'TUN (вся система)'
  if (state.value?.proxyMode === 'socks') return 'SOCKS 127.0.0.1:1080'
  return null
})

const activeNetwork = computed(() => {
  const p = netGuard.value?.proxy
  if (!p) return null
  return p.enabled ? `Системный прокси — ${p.server}:${p.port}` : 'Системный прокси выключен'
})

// Локальный IP — факт ОС (GetNetworkFacts), а не адрес прокси.
const localIP = computed(() => facts.value?.localIP || null)

// Прямой egress — факт HTTP-наблюдения из тех же фактов сети.
const directEgress = computed(() => facts.value?.publicIP || null)
</script>

<template>
  <div class="dashboard">
    <!-- ================= HERO ================= -->
    <section class="protection-hero card" :class="heroTone">
      <div class="protection-control">
        <button
          class="protection-button"
          :class="heroTone"
          :disabled="primaryAction.disabled"
          :title="isRunning ? 'Остановить защиту' : 'Подключить (probe перед активацией — обязателен)'"
          @click="onPrimary"
        >
          <span class="power-symbol">{{ powerGlyph }}</span>
        </button>

        <div class="protection-state" :class="heroTone">
          <span class="state-dot"></span>
          <span>{{ heroHeading.toUpperCase() }}</span>
        </div>

        <button class="disconnect-button" :disabled="primaryAction.disabled" @click="onPrimary">
          {{ disconnectLabel }}
        </button>
      </div>

      <div class="protection-information">
        <div class="protection-heading" :class="heroTone">{{ heroHeading }}</div>
        <div class="protection-description">{{ heroDescription }}</div>

        <!-- Текущий канал -->
        <article v-if="activeChannel" class="current-channel-card">
          <div class="current-channel-header">
            <h2>Текущий канал</h2>
            <span v-if="channelStatusPill" class="channel-status" :class="channelStatusPill.cls">
              <span class="status-dot"></span>
              {{ channelStatusPill.text }}
            </span>
          </div>

          <div class="current-channel-name">
            <span>{{ activeChannel.id }}</span>
            <span class="channel-arrow">›</span>
          </div>

          <div class="current-channel-details">
            <div v-for="d in channelDetails" :key="d.label" class="channel-detail">
              <span class="detail-label">{{ d.label }}</span>
              <strong>{{ d.value }}</strong>
            </div>
          </div>

          <div class="current-channel-actions">
            <button class="change-channel-button" @click="$emit('navigate', 'channels')">
              <span>⇄</span>
              <span>Сменить канал</span>
            </button>
            <button class="channel-details-button" @click="$emit('navigate', 'channels')">
              <span>Подробнее</span>
              <span>›</span>
            </button>
          </div>
        </article>

        <!-- Карта маршрута -->
        <div class="route-visualization" :class="{ active: routeActive }">
          <div class="map-background"><div class="map-dots"></div></div>

          <svg class="route-line" viewBox="0 0 350 130" preserveAspectRatio="none">
            <path d="M70 100 C125 78 180 48 270 38" />
          </svg>

          <div class="route-point device">
            <span class="point"></span>
            <div class="route-location">
              <strong>Ваше устройство</strong>
              <small>{{ localIP ? 'локальный IP: ' + localIP : 'локальный IP: нет данных' }}</small>
            </div>
          </div>

          <div v-if="routeActive" class="route-point server">
            <span class="point"></span>
            <div class="route-location">
              <strong>{{ routeServerLabel }}</strong>
              <small>{{ routeServerHint }}</small>
            </div>
          </div>
          <div v-else class="route-point server ghost">
            <span class="point"></span>
            <div class="route-location">
              <strong>Сервер</strong>
              <small>нет активного канала</small>
            </div>
          </div>
        </div>

        <!-- Прямой egress (факт наблюдения) -->
        <div class="egress-fact mono">
          {{ directEgress ? `прямой egress: ${directEgress}` : 'прямой egress: нет данных (наблюдение недоступно)' }}
        </div>

        <!-- Сводка защиты: только факты probe -->
        <div class="connection-summary">
          <div
            v-for="item in summaryItems"
            :key="item.text"
            class="connection-summary-item"
            :class="{ ok: item.ok === true, bad: item.ok === false }"
          >
            <span class="summary-icon"><Icon :name="item.icon" :size="12" /></span>
            <span>{{ item.text }}</span>
          </div>
        </div>
      </div>
    </section>

    <!-- ================= БЫСТРЫЕ ФАКТЫ ================= -->
    <section class="quick-information">
      <article class="information-card card">
        <div class="information-icon"><Icon name="dashboard" :size="20" /></div>
        <div class="information-content">
          <div class="information-label">Режим маршрутизации</div>
          <div class="information-value">{{ routingMode ?? 'Нет данных' }}</div>
          <div class="information-description">
            {{ state?.proxyMode === 'tun' ? 'Весь трафик через VPN' : 'Прокси-режим приложений' }}
          </div>
        </div>
      </article>

      <article class="information-card card">
        <div class="information-icon blue"><Icon name="pulse" :size="20" /></div>
        <div class="information-content">
          <div class="information-label">Системный прокси</div>
          <div class="information-value">{{ activeNetwork ?? 'Нет данных' }}</div>
          <div class="information-description">
            {{ netGuard?.proxy.listenerAlive ? 'листенер жив' : netGuard ? 'листенер не слушается' : 'нет данных NetGuard' }}
          </div>
        </div>
      </article>

      <article class="information-card card">
        <div class="information-icon blue"><Icon name="server" :size="20" /></div>
        <div class="information-content">
          <div class="information-label">Локальный IP</div>
          <div class="information-value">{{ localIP ?? 'Нет данных' }}</div>
          <div class="information-description"><span class="small-status">{{ facts?.connectionType || 'тип соединения: нет данных' }}</span></div>
        </div>
      </article>

      <article class="information-card card">
        <div class="information-icon"><Icon name="shield" :size="20" /></div>
        <div class="information-content">
          <div class="information-label">Публичный IP (egress)</div>
          <div class="information-value">{{ isRunning ? 'Скрыт туннелем' : (directEgress ?? 'Нет данных') }}</div>
          <div class="information-description">
            {{ isRunning ? 'трафик идёт через канал' : facts?.country ? `страна egress: ${facts.country}` : 'подключение не установлено' }}
          </div>
        </div>
      </article>
    </section>

    <!-- ================= ПРОВЕРКА ПУТИ ================= -->
    <aside class="security-check-card card">
      <div class="card-header">
        <h2>Проверка защищённого пути</h2>
        <span v-if="state?.probeRunning" class="status-badge warning"><span class="status-dot"></span>Выполняется…</span>
        <span v-else-if="state?.probe?.ok" class="status-badge success"><span class="status-dot"></span>Успешно</span>
        <span v-else-if="state?.probe" class="status-badge danger"><span class="status-dot"></span>Не пройдено</span>
        <span v-else class="status-badge neutral"><span class="status-dot"></span>Нет данных</span>
      </div>

      <div class="verification-progress">
        <span
          v-for="(s, i) in probeSteps"
          :key="i"
          class="verification-step"
          :class="{ completed: s.status === 'pass', failed: s.status === 'fail' }"
          :title="`${s.name}: ${s.detail}`"
        ></span>
        <template v-if="probeSteps.length === 0">
          <span class="verification-step"></span>
          <span class="verification-step"></span>
          <span class="verification-step"></span>
          <span class="verification-step"></span>
          <span class="verification-step"></span>
        </template>
      </div>

      <div class="verification-columns">
        <div class="verification-column">
          <div class="verification-label">Последняя проверка</div>
          <strong>{{ state?.probeLastAt ? fmtDateTime(state.probeLastAt) : '—' }}</strong>
        </div>
        <div class="verification-column">
          <strong>{{ probeSteps.length ? `${probeSteps.filter((s) => s.status === 'pass').length} / ${probeSteps.length}` : '—' }}</strong>
          <div class="verification-label">шагов пройдено</div>
        </div>
        <div class="verification-column">
          <div class="verification-label">Egress:</div>
          <strong>{{ egressFact }}</strong>
        </div>
        <div class="verification-column">
          <div class="verification-label">Вердикт:</div>
          <strong :class="{ 'green-value': state?.probe?.ok, 'red-value': state?.probe && !state.probe.ok }">
            {{ state?.probe ? (state.probe.ok ? 'путь подтверждён' : 'путь не подтверждён') : '—' }}
          </strong>
        </div>
      </div>

      <button class="secondary-button" :disabled="state?.probeRunning || isBusy" @click="runProbe">
        Результаты проверки
        <span>→</span>
      </button>
    </aside>

    <!-- ================= КАНАЛЫ (сводная таблица) ================= -->
    <section class="channels-section card">
      <div class="channels-header">
        <div class="section-title-group">
          <h2>Каналы</h2>
          <span class="channels-count">{{ channels ? channels.length : '—' }}</span>
        </div>
        <div class="channels-controls">
          <button class="add-channel-button" @click="$emit('navigate', 'channels')">
            <span>＋</span>
            Все каналы
          </button>
        </div>
      </div>

      <div v-if="!channels" class="channels-empty">Список каналов недоступен: backend bindings не отвечают.</div>
      <div v-else-if="channels.length === 0" class="channels-empty">В активном конфиге нет каналов — запуск останется заблокированным (fail-closed).</div>

      <div v-else class="channels-table-wrapper">
        <table class="channels-table">
          <thead>
            <tr>
              <th></th>
              <th>Имя</th>
              <th>Статус</th>
              <th>Транспорт</th>
              <th>Сервер</th>
              <th>Порт</th>
              <th>Действия</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="c in channels" :key="c.id" :class="{ 'selected-channel': c.id === state?.activeChannelId }">
              <td>
                <span class="favorite" :class="{ active: c.id === state?.activeChannelId }">{{ c.id === state?.activeChannelId ? '★' : '☆' }}</span>
              </td>
              <td><strong>{{ c.id }}</strong></td>
              <td>
                <span class="channel-status" :class="c.validation === 'live-verified' ? 'live' : c.validation === 'degraded' ? 'degraded' : c.validation === 'blocked' ? 'blocked' : c.validation === 'retired' || c.validation === 'planned' ? 'neutral' : 'configured'">
                  <span class="status-dot"></span>
                  {{ validationStatusLabels[c.validation] ?? c.validation }}
                </span>
              </td>
              <td>{{ c.transport }}</td>
              <td>{{ c.server }}</td>
              <td>{{ c.port }}</td>
              <td>
                <button v-if="c.id === state?.activeChannelId" class="table-action" disabled>Текущий</button>
                <button v-else-if="c.enabled" class="table-action" :disabled="!isRunning || isBusy" @click="$emit('navigate', 'channels')">Переключиться</button>
                <button v-else class="table-action disabled" disabled>Выключен</button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <!-- ================= АВТОЗАЩИТА ================= -->
    <section class="auto-protection-section card">
      <div class="auto-protection-header">
        <h2>Автоматическая защита</h2>
        <span class="status-badge" :class="failoverBadgeClass"><span class="status-dot"></span>{{ watchdogLabel }}</span>
        <span class="large-toggle" :class="{ active: failover?.enabled }"></span>
      </div>

      <div class="watcher-status">
        <div class="watcher-icon"><Icon name="shield" :size="16" /></div>
        <div>
          <strong>{{ failover?.enabled ? 'Сторож активен' : failover ? 'Сторож в ожидании VPN' : 'Состояние сторожа: нет данных' }}</strong>
          <p>
            Периодически проверяет защищённый путь (каждые 30 секунд при работающем VPN)
            и автоматически переключает канал при проблемах. Кандидаты — только
            live-verified каналы.
          </p>
        </div>
      </div>

      <div class="auto-information">
        <div class="auto-row"><span>Защита пути</span><strong :class="state?.probe?.ok ? 'green-value' : ''">{{ state?.probe ? (state.probe.ok ? 'подтверждена' : 'нарушена') : 'нет данных' }}</strong></div>
        <div class="auto-row"><span>Сторож</span><strong>{{ watchdogLabel }}</strong></div>
        <div class="auto-row"><span>Следующая проверка</span><strong>{{ nextCheckLabel }}</strong></div>
        <div class="auto-row"><span>Последняя проверка</span><strong>{{ state?.probeLastAt ? fmtDateTime(state.probeLastAt) : '—' }}</strong></div>
        <div class="auto-row"><span>Авто-переключений</span><strong>{{ failover ? String(failover.switches) : '—' }}</strong></div>
      </div>

      <div v-if="incidentLines.length" class="auto-incidents">
        <div v-for="(l, i) in incidentLines" :key="i" class="auto-incident" :class="l.level">
          <span class="ai-dot"></span>
          <span class="ai-text">{{ l.text }}</span>
        </div>
      </div>

      <button class="btn auto-more" @click="$emit('navigate', 'autoprotect')">Подробнее об автозащите ›</button>
    </section>

    <!-- ================= СОБЫТИЯ ================= -->
    <section class="events-section card">
      <div class="events-header">
        <h2>История событий</h2>
        <button class="show-more-button" @click="$emit('navigate', 'logs')">Показать больше →</button>
      </div>
      <p class="events-note">
        События ведутся в журнале приложения: запуск, проверки пути, авто-переключения,
        починка прокси. Раздел «Логи» показывает их с фильтрами по уровню.
      </p>
      <div v-if="logLines.length" class="events-list">
        <div v-for="(l, i) in logLines.slice(-4).reverse()" :key="i" class="event-line" :class="l.level">
          <span class="event-time mono">{{ fmtDateTime(l.t) }}</span>
          <span class="event-text">{{ l.text }}</span>
        </div>
      </div>
      <div v-else class="events-empty">Событий пока нет — журнал наполнится при первом действии.</div>
    </section>

    <!-- ================= НИЖНЯЯ СЕТКА ================= -->
    <section class="bottom-grid">
      <!-- Статистика -->
      <article class="statistics-card card" @click="$emit('navigate', 'statistics')">
        <div class="card-header">
          <h2>Статистика</h2>
          <span class="stat-link">открыть ›</span>
        </div>
        <div class="statistics-metrics">
          <div class="statistics-metric">
            <strong :class="isRunning ? 'green-value' : ''">{{ isRunning ? 'активно' : 'простой' }}</strong>
            <span>состояние</span>
          </div>
          <div class="statistics-metric">
            <strong>{{ channels?.length ?? '—' }}</strong>
            <span>каналов</span>
          </div>
          <div class="statistics-metric">
            <strong>{{ probeSteps.length || '—' }}</strong>
            <span>шагов probe</span>
          </div>
          <div class="statistics-metric">
            <strong :class="state?.probe?.ok ? 'green-value' : ''">{{ state?.probe ? (state.probe.ok ? '✓ ок' : '✗ провал') : '—' }}</strong>
            <span>последний probe</span>
          </div>
        </div>
        <p class="stat-note">Счётчики трафика — из фактов ОС на странице «Статистика».</p>
      </article>

      <!-- Сетевая информация -->
      <article class="network-information-card card">
        <div class="card-header"><h2>Сетевая информация</h2></div>
        <div class="network-list">
          <div class="network-row">
            <span class="net-ico"><Icon name="pulse" :size="12" /></span>
            <span>Режим</span>
            <strong>{{ state?.proxyMode === 'tun' ? 'TUN' : state?.proxyMode === 'socks' ? 'SOCKS' : '—' }}</strong>
          </div>
          <div class="network-row">
            <span class="net-ico"><Icon name="server" :size="12" /></span>
            <span>Прокси</span>
            <strong>{{ netGuard?.proxy ? `${netGuard.proxy.server}:${netGuard.proxy.port}` : '—' }}</strong>
          </div>
          <div class="network-row">
            <span class="net-ico"><Icon name="key" :size="12" /></span>
            <span>Провайдер</span>
            <strong>{{ isRunning ? 'Скрыт' : 'Нет данных' }}</strong>
          </div>
          <div class="network-row">
            <span class="net-ico"><Icon name="shield" :size="12" /></span>
            <span>Страна (через VPN)</span>
            <strong>{{ isRunning ? 'Скрыта туннелем' : '—' }}</strong>
          </div>
          <div class="network-row">
            <span class="net-ico"><Icon name="pulse" :size="12" /></span>
            <span>SOCKS :1080</span>
            <strong :class="netGuard?.socksAlive ? 'green-value' : ''">{{ netGuard ? (netGuard.socksAlive ? '● слушается' : '○ не слушается') : '—' }}</strong>
          </div>
          <div class="network-row">
            <span class="net-ico"><Icon name="pulse" :size="12" /></span>
            <span>DNS (через VPN)</span>
            <strong>{{ state?.probe ? (state.probe.ok ? 'через туннель' : 'проверить') : '—' }}</strong>
          </div>
        </div>
      </article>

      <!-- Полезные инструменты -->
      <article class="tools-card card">
        <div class="card-header"><h2>Полезные инструменты</h2></div>
        <div class="tools-list">
          <button class="tool-item" @click="runProbe">
            <span class="tool-icon"><Icon name="pulse" :size="14" /></span>
            <span class="tool-content">
              <strong>Проверить подключение</strong>
              <small>Диагностика текущего канала</small>
            </span>
            <span class="tool-arrow">›</span>
          </button>
          <button class="tool-item" @click="$emit('navigate', 'logs')">
            <span class="tool-icon"><Icon name="logs" :size="14" /></span>
            <span class="tool-content">
              <strong>Открыть логи</strong>
              <small>Просмотр событий</small>
            </span>
            <span class="tool-arrow">›</span>
          </button>
          <button class="tool-item" @click="$emit('navigate', 'tests')">
            <span class="tool-icon"><Icon name="flask" :size="14" /></span>
            <span class="tool-content">
              <strong>Тестирование</strong>
              <small>Проверки окружения</small>
            </span>
            <span class="tool-arrow">›</span>
          </button>
          <button class="tool-item" @click="$emit('navigate', 'settings')">
            <span class="tool-icon"><Icon name="key" :size="14" /></span>
            <span class="tool-content">
              <strong>Экспорт конфигурации</strong>
              <small>Секреты и профиль доставки</small>
            </span>
            <span class="tool-arrow">›</span>
          </button>
        </div>
      </article>

      <!-- Системная информация -->
      <article class="system-information-card card">
        <div class="card-header"><h2>Системная информация</h2></div>
        <div class="system-list">
          <div class="system-row"><span>Версия</span><strong>v{{ APP_VERSION }}</strong></div>
          <div class="system-row"><span>Ядро</span><strong>sing-box (pinned)</strong></div>
          <div class="system-row"><span>Режим</span><strong>{{ state?.proxyMode === 'tun' ? 'TUN' : state?.proxyMode === 'socks' ? 'SOCKS' : '—' }}</strong></div>
          <div class="system-row">
            <span>Ядро подключено</span>
            <strong>{{ coreReadyLabel }}</strong>
          </div>
        </div>

        <div class="quick-actions-title">Быстрые действия</div>
        <div class="quick-actions">
          <button @click="runProbe">Проверить сейчас</button>
          <button @click="$emit('navigate', 'tests')">Проверить сеть</button>
          <button @click="$emit('navigate', 'logs')">Открыть логи</button>
        </div>

        <div class="system-note">
          Достоверный источник трафика используется только при наличии соответствующих метрик.
        </div>
      </article>
    </section>

    <!-- ================= О ЗАЩИТЕ ================= -->
    <section class="about-protection-section card">
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
  </div>
</template>

<script lang="ts">
// emit navigate к разделам (навигация из карточек)
export default {
  emits: ['navigate'],
}
</script>

<style scoped>
.dashboard {
  display: grid;
  grid-template-columns: 1fr 390px;
  gap: 10px;
}

.dashboard > .protection-hero {
  grid-column: 1;
}
.dashboard > .quick-information {
  grid-column: 1;
}
.dashboard > .security-check-card,
.dashboard > .auto-protection-section {
  grid-column: 2;
}
.dashboard > .security-check-card {
  align-self: start;
}
.dashboard > .channels-section {
  grid-column: 1;
}
.dashboard > .events-section {
  grid-column: 2;
}
.dashboard > .bottom-grid {
  grid-column: 1 / -1;
}
.dashboard > .about-protection-section {
  grid-column: 1 / -1;
}

/* ---------- HERO ---------- */
.protection-hero {
  position: relative;
  min-height: 300px;
  display: grid;
  grid-template-columns: 38% 62%;
  overflow: hidden;
}
.protection-hero.running .protection-control::before {
  background: radial-gradient(circle at 50% 55%, rgba(24, 229, 161, 0.15), transparent 42%);
}
.protection-hero.error .protection-control::before {
  background: radial-gradient(circle at 50% 55%, rgba(255, 79, 89, 0.12), transparent 42%);
}
.protection-hero.stopped .protection-control::before,
.protection-hero.busy .protection-control::before {
  background: radial-gradient(circle at 50% 55%, rgba(20, 140, 255, 0.1), transparent 42%);
}
.protection-control {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0;
  padding: 20px;
}
.protection-control::before {
  content: '';
  position: absolute;
  inset: 14px 20px;
  pointer-events: none;
}

.protection-button {
  position: relative;
  width: 150px;
  height: 150px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  border: 5px solid #35e6a5;
  background: radial-gradient(circle, rgba(24, 216, 150, 0.17), rgba(7, 28, 38, 0.95) 67%);
  color: var(--green-bright);
  box-shadow:
    0 0 0 7px rgba(37, 227, 160, 0.09),
    0 0 28px rgba(37, 227, 160, 0.35),
    inset 0 0 30px rgba(37, 227, 160, 0.17);
  z-index: 2;
}
.protection-button.stopped,
.protection-button.busy {
  border-color: #2b5c8a;
  color: #7fb4e0;
  background: radial-gradient(circle, rgba(20, 140, 255, 0.12), rgba(7, 22, 36, 0.95) 67%);
  box-shadow:
    0 0 0 7px rgba(20, 140, 255, 0.07),
    0 0 24px rgba(20, 140, 255, 0.25),
    inset 0 0 30px rgba(20, 140, 255, 0.12);
}
.protection-button.error {
  border-color: #ff5f68;
  color: #ff8a90;
  background: radial-gradient(circle, rgba(255, 79, 89, 0.14), rgba(38, 8, 12, 0.95) 67%);
  box-shadow:
    0 0 0 7px rgba(255, 79, 89, 0.08),
    0 0 24px rgba(255, 79, 89, 0.3),
    inset 0 0 30px rgba(255, 79, 89, 0.12);
}
.protection-button:disabled {
  opacity: 0.6;
  cursor: default;
}
.power-symbol {
  font-size: 58px;
  font-weight: 200;
  line-height: 1;
}

.protection-state {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 12px;
  font-size: 9px;
  letter-spacing: 0.7px;
  color: var(--green);
}
.protection-state.stopped,
.protection-state.busy {
  color: #7fb4e0;
}
.protection-state.error {
  color: #ff8a90;
}
.state-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}

.disconnect-button {
  width: 170px;
  height: 34px;
  margin-top: 14px;
  border: 1px solid #25e3a0;
  border-radius: 7px;
  color: #f2f7f9;
  background: #071b28;
  font-size: 13px;
  font-weight: 650;
  z-index: 2;
}
.disconnect-button:hover:not(:disabled) {
  background: #0a2637;
  box-shadow: 0 0 14px rgba(37, 227, 160, 0.12);
}
.disconnect-button:disabled {
  opacity: 0.55;
  cursor: default;
}

.protection-information {
  position: relative;
  padding: 20px 18px 14px 4px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.protection-heading {
  color: var(--green-bright);
  font-size: 20px;
  font-weight: 650;
  letter-spacing: -0.2px;
}
.protection-heading.stopped,
.protection-heading.busy {
  color: #7fb4e0;
}
.protection-heading.error {
  color: #ff8a90;
}
.protection-description {
  color: #91a7ba;
  font-size: 11px;
}

/* Текущий канал */
.current-channel-card {
  border: 1px solid #203e57;
  border-radius: 11px;
  background: linear-gradient(145deg, rgba(8, 23, 37, 0.98), rgba(7, 19, 31, 0.98));
  box-shadow: 0 14px 28px rgba(0, 0, 0, 0.2), inset 0 1px rgba(255, 255, 255, 0.025);
  padding: 13px 14px 11px;
  z-index: 8;
  max-width: 330px;
}
.current-channel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
.current-channel-header h2 {
  margin: 0;
  color: #edf3f7;
  font-size: 12px;
  font-weight: 600;
}
.current-channel-name {
  height: 34px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid #1b3449;
}
.current-channel-name span:first-child {
  color: #f0f5f8;
  font-size: 16px;
  font-weight: 700;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.channel-arrow {
  color: #8ca5ba;
  font-size: 21px;
}
.current-channel-details {
  margin-top: 2px;
}
.channel-detail {
  min-height: 26px;
  display: grid;
  grid-template-columns: 80px 1fr;
  align-items: center;
  border-bottom: 1px solid #152d40;
  font-size: 10px;
}
.detail-label {
  color: #8fa6ba;
}
.channel-detail strong {
  color: #e6edf2;
  font-weight: 500;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.current-channel-actions {
  display: grid;
  grid-template-columns: 1.08fr 0.82fr;
  gap: 8px;
  margin-top: 9px;
}
.current-channel-actions button {
  height: 30px;
  border-radius: 6px;
  font-size: 10px;
}
.change-channel-button,
.channel-details-button {
  border: 1px solid #294a65;
  color: #e5eef4;
  background: linear-gradient(180deg, #102a40, #0a1e30);
}
.change-channel-button:hover,
.channel-details-button:hover {
  border-color: #367091;
  background: #10283b;
}
.channel-details-button {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
}

/* Статусы каналов */
.channel-status {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  width: fit-content;
  border-radius: 5px;
  padding: 4px 7px;
  font-size: 9px;
  line-height: 1;
  white-space: nowrap;
}
.channel-status .status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
}
.channel-status.live {
  color: #42e8ad;
  border: 1px solid var(--green-border);
  background: var(--green-soft);
}
.channel-status.live .status-dot {
  background: var(--green);
}
.channel-status.configured {
  color: #51b8ff;
  border: 1px solid var(--blue-border);
  background: var(--blue-soft);
}
.channel-status.configured .status-dot {
  background: var(--blue);
}
.channel-status.degraded {
  color: #ffc643;
  border: 1px solid var(--yellow-border);
  background: var(--yellow-soft);
}
.channel-status.degraded .status-dot {
  background: var(--yellow);
}
.channel-status.blocked {
  color: #ff6870;
  border: 1px solid var(--red-border);
  background: var(--red-soft);
}
.channel-status.blocked .status-dot {
  background: var(--red);
}
.channel-status.neutral {
  color: #a1b2c0;
  border: 1px solid var(--gray-border);
  background: var(--gray-soft);
}
.channel-status.neutral .status-dot {
  background: #71869a;
}

/* Карта маршрута */
.route-visualization {
  position: relative;
  height: 130px;
  margin-top: 4px;
  pointer-events: none;
}
.map-background {
  position: absolute;
  inset: 0;
  overflow: hidden;
  opacity: 0.8;
}
.map-dots {
  position: absolute;
  inset: 4px;
  background: radial-gradient(circle, rgba(25, 119, 180, 0.7) 0 1px, transparent 1.6px);
  background-size: 6px 6px;
  mask-image: radial-gradient(ellipse 80% 62% at 55% 50%, #000 35%, transparent 78%);
  opacity: 0.55;
}
.route-line {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  overflow: visible;
}
.route-line path {
  fill: none;
  stroke: rgba(34, 223, 161, 0.25);
  stroke-width: 2;
}
.route-visualization.active .route-line path {
  stroke: #22dfa1;
  filter: drop-shadow(0 0 5px rgba(37, 227, 160, 0.55));
}
.route-point {
  position: absolute;
  display: flex;
  align-items: center;
  gap: 8px;
}
.route-point .point {
  width: 12px;
  height: 12px;
  border: 2px solid #071a28;
  border-radius: 50%;
  background: #45aaff;
  box-shadow: 0 0 9px rgba(69, 170, 255, 0.8);
}
.route-point.server .point {
  background: var(--green);
  box-shadow: 0 0 9px rgba(37, 227, 160, 0.8);
}
.route-point.ghost .point {
  background: #35506a;
  box-shadow: none;
}
.route-point.device {
  left: 6%;
  top: 55%;
}
.route-point.server {
  right: 8%;
  top: 16%;
}
.route-location {
  display: flex;
  flex-direction: column;
  color: #dce8ef;
  font-size: 10px;
}
.route-location strong {
  font-size: 11px;
  font-weight: 500;
}
.route-location small {
  margin-top: 1px;
  color: #7e95a9;
  font-size: 9px;
}

/* Сводка защиты */
.connection-summary {
  border: 1px solid #1d3a50;
  border-radius: 8px;
  background: rgba(7, 22, 35, 0.91);
  padding: 8px 10px;
}
.connection-summary-item {
  display: flex;
  align-items: center;
  gap: 7px;
  min-height: 20px;
  color: #a4b6c5;
  font-size: 9.5px;
}
.connection-summary-item.ok {
  color: #46e7ae;
}
.connection-summary-item.bad {
  color: #ff8a90;
}
.summary-icon {
  width: 14px;
  display: inline-flex;
  color: #d2e0e9;
}

/* ---------- Быстрые факты ---------- */
.quick-information {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 9px;
  align-content: start;
}
.information-card {
  height: 71px;
  display: flex;
  align-items: center;
  padding: 10px 13px;
}
.information-icon {
  width: 34px;
  margin-right: 10px;
  color: var(--green);
  display: inline-flex;
}
.information-icon.blue {
  color: var(--blue-bright);
}
.information-content {
  min-width: 0;
}
.information-label {
  color: #8198ab;
  font-size: 9px;
}
.information-value {
  margin-top: 3px;
  color: #edf3f7;
  font-size: 12px;
  font-weight: 650;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.information-description {
  margin-top: 3px;
  color: #8399ac;
  font-size: 9px;
}
.small-status {
  display: inline-flex;
  padding: 3px 7px;
  border-radius: 5px;
  color: #a1b2c0;
  border: 1px solid #253e53;
  background: rgba(113, 132, 154, 0.1);
  font-size: 9px;
}

/* ---------- Проверка пути ---------- */
.security-check-card {
  padding: 13px;
}
.status-badge {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  border-radius: 5px;
  padding: 4px 8px;
  font-size: 9px;
  white-space: nowrap;
  margin-left: auto;
}
.status-badge .status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}
.status-badge.success {
  color: #42e8ad;
  border: 1px solid var(--green-border);
  background: var(--green-soft);
}
.status-badge.warning {
  color: #ffc643;
  border: 1px solid var(--yellow-border);
  background: var(--yellow-soft);
}
.status-badge.danger {
  color: #ff6870;
  border: 1px solid var(--red-border);
  background: var(--red-soft);
}
.status-badge.neutral {
  color: #a1b2c0;
  border: 1px solid var(--gray-border);
  background: var(--gray-soft);
}

.verification-progress {
  display: flex;
  gap: 6px;
  margin-top: 14px;
}
.verification-step {
  flex: 1;
  height: 11px;
  border-radius: 3px;
  background: #173247;
}
.verification-step.completed {
  background: linear-gradient(90deg, #21d795, #35edaa);
  box-shadow: 0 0 7px rgba(37, 227, 160, 0.2);
}
.verification-step.failed {
  background: linear-gradient(90deg, #d72130, #ed3546);
  box-shadow: 0 0 7px rgba(255, 79, 89, 0.2);
}

.verification-columns {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 5px 20px;
  margin-top: 10px;
}
.verification-column {
  display: flex;
  flex-direction: column;
}
.verification-label {
  color: #8096aa;
  font-size: 9px;
}
.verification-column strong {
  margin-top: 2px;
  color: #e7eef3;
  font-size: 11px;
  font-weight: 500;
}
.verification-column:nth-child(2) strong {
  font-size: 15px;
  font-weight: 650;
}

.secondary-button {
  height: 29px;
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 10px;
  margin-left: auto;
  padding: 0 11px;
  border: 1px solid #28455d;
  border-radius: 6px;
  color: #dce7ee;
  background: #0a1d2d;
  font-size: 10px;
}
.secondary-button:hover:not(:disabled) {
  border-color: #367091;
}

/* ---------- Каналы ---------- */
.channels-section {
  padding: 12px 11px;
  min-height: 200px;
}
.channels-header {
  min-height: 31px;
  display: flex;
  align-items: center;
}
.channels-count {
  min-width: 26px;
  height: 26px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  color: #b8c8d4;
  background: #172d41;
  font-size: 11px;
  padding: 0 7px;
}
.channels-controls {
  display: flex;
  align-items: center;
  gap: 9px;
  margin-left: auto;
}
.add-channel-button {
  height: 30px;
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 0 13px;
  border: 1px solid #178cff;
  border-radius: 6px;
  color: #fff;
  background: linear-gradient(180deg, #168fff, #0d72d6);
  box-shadow: 0 5px 14px rgba(13, 114, 214, 0.18);
  font-size: 11px;
}
.add-channel-button:hover {
  filter: brightness(1.1);
}
.channels-empty {
  margin-top: 12px;
  padding: 12px;
  border: 1px dashed var(--border);
  border-radius: 6px;
  color: #8ea4b7;
  font-size: 11px;
}
.channels-table-wrapper {
  margin-top: 8px;
  overflow: auto;
  border-radius: 5px;
  max-height: 240px;
}
.channels-table {
  width: 100%;
  border-collapse: collapse;
  table-layout: fixed;
  font-size: 10px;
}
.channels-table th {
  height: 29px;
  padding: 0 8px;
  text-align: left;
  color: #8298ab;
  font-weight: 500;
  background: #0c2031;
  border-bottom: 1px solid #1a344a;
  position: sticky;
  top: 0;
  z-index: 2;
}
.channels-table td {
  height: 32px;
  padding: 0 8px;
  color: #c6d3dd;
  border-bottom: 1px solid #152d40;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.channels-table tr:hover {
  background: rgba(20, 64, 94, 0.16);
}
.channels-table tr.selected-channel {
  background: linear-gradient(90deg, rgba(18, 99, 150, 0.13), rgba(15, 53, 80, 0.08));
}
.channels-table td strong {
  color: #e3ebf1;
  font-weight: 600;
}
.favorite {
  color: #8196aa;
  font-size: 14px;
}
.favorite.active {
  color: #ffc43c;
  text-shadow: 0 0 5px rgba(255, 196, 60, 0.25);
}
.table-action {
  min-width: 96px;
  height: 26px;
  padding: 0 10px;
  border: 1px solid #27435b;
  border-radius: 5px;
  color: #dce7ee;
  background: #0b2031;
  font-size: 9px;
}
.table-action:hover:not(:disabled) {
  background: #102a3f;
}
.table-action.disabled,
.table-action:disabled {
  color: #61778a;
  border-color: #1a3042;
  background: #091724;
  cursor: default;
}

/* ---------- Автозащита ---------- */
.auto-protection-section {
  padding: 13px;
}
.auto-protection-header {
  display: flex;
  align-items: center;
  gap: 8px;
}
.auto-protection-header h2 {
  margin: 0;
  font-size: 14px;
}
.auto-protection-header .status-badge {
  margin-left: 0;
}
.large-toggle {
  position: relative;
  width: 29px;
  height: 17px;
  border-radius: 12px;
  background: #193247;
  margin-left: auto;
  flex: 0 0 auto;
}
.large-toggle::after {
  content: '';
  position: absolute;
  left: 2px;
  top: 2px;
  width: 13px;
  height: 13px;
  border-radius: 50%;
  background: #8299ad;
}
.large-toggle.active {
  background: #15c98c;
}
.large-toggle.active::after {
  left: 14px;
  background: #effff9;
}

.watcher-status {
  min-height: 56px;
  display: flex;
  align-items: flex-start;
  gap: 10px;
  margin-top: 10px;
  padding: 9px;
  border: 1px solid #19364d;
  border-radius: 7px;
  background: rgba(7, 22, 35, 0.7);
}
.watcher-icon {
  width: 28px;
  height: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 auto;
  border-radius: 7px;
  color: var(--green);
  background: rgba(37, 227, 160, 0.12);
}
.watcher-status strong {
  color: #38e7a8;
  font-size: 11px;
}
.watcher-status p {
  margin: 3px 0 0;
  color: #8299ad;
  font-size: 9.5px;
  line-height: 1.45;
}
.auto-information {
  margin-top: 8px;
}
.auto-row {
  height: 26px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid #142d40;
  color: #8298ab;
  font-size: 10px;
}
.auto-row strong {
  color: #dbe6ed;
  font-weight: 500;
}
.auto-incidents {
  margin-top: 8px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.auto-incident {
  display: flex;
  align-items: baseline;
  gap: 7px;
  font-size: 9px;
}
.ai-dot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: #42b3ff;
  align-self: center;
  flex: 0 0 auto;
}
.auto-incident.warn .ai-dot {
  background: var(--yellow);
}
.auto-incident.error .ai-dot {
  background: var(--red);
}
.ai-text {
  color: #8ea3b5;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.egress-fact {
  margin-top: 6px;
  color: #6f8699;
  font-size: 8.5px;
}
.auto-more {
  margin-top: 10px;
  width: 100%;
  justify-content: center;
}

/* ---------- События ---------- */
.events-section {
  padding: 13px;
}
.events-list {
  margin-top: 9px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.event-line {
  display: flex;
  gap: 8px;
  align-items: baseline;
  font-size: 9.5px;
}
.event-time {
  color: #6d8497;
  flex: 0 0 auto;
}
.event-text {
  color: #a4b6c5;
  word-break: break-word;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.event-line.warn .event-text {
  color: #ffd479;
}
.event-line.error .event-text {
  color: #ff8a90;
}
.events-empty {
  margin-top: 10px;
  padding: 9px 11px;
  border: 1px dashed var(--border);
  border-radius: 6px;
  color: #70889d;
  font-size: 10px;
}
.events-header {
  display: flex;
  align-items: center;
  gap: 7px;
}
.events-header h2 {
  margin: 0;
  font-size: 14px;
}
.show-more-button {
  height: 29px;
  margin-left: auto;
  border: 1px solid #263f55;
  border-radius: 6px;
  color: #a9bac7;
  background: #0a1c2b;
  font-size: 9px;
  padding: 0 10px;
}
.show-more-button:hover {
  border-color: #367091;
}
.events-note {
  margin: 10px 0 0;
  color: #72899c;
  font-size: 10px;
  line-height: 1.55;
}

/* ---------- Нижняя сетка ---------- */
.bottom-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 10px;
}
.statistics-card,
.network-information-card,
.tools-card,
.system-information-card {
  min-height: 210px;
  padding: 13px;
}
.statistics-card {
  cursor: pointer;
}
.statistics-card:hover {
  border-color: var(--border-light);
}
.stat-link {
  margin-left: auto;
  color: #6d8497;
  font-size: 9.5px;
}
.statistics-metrics {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 10px;
  margin-top: 13px;
}
.statistics-metric {
  min-width: 0;
}
.statistics-metric strong {
  display: block;
  color: #e4edf2;
  font-size: 13px;
  font-weight: 650;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.statistics-metric span {
  display: block;
  margin-top: 2px;
  color: #7b92a5;
  font-size: 8px;
}
.stat-note {
  margin: 12px 0 0;
  color: #6f8699;
  font-size: 8.5px;
  line-height: 1.4;
}

.network-list {
  margin-top: 8px;
}
.network-row {
  min-height: 25px;
  display: flex;
  align-items: center;
  gap: 8px;
  border-bottom: 1px solid #142d40;
  font-size: 9.5px;
}
.network-row .net-ico {
  width: 16px;
  color: #d2e0e9;
  display: inline-flex;
}
.network-row > span:nth-child(2) {
  flex: 1;
  color: #8399ad;
}
.network-row strong {
  color: #dce7ee;
  font-weight: 500;
  white-space: nowrap;
}

.tools-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-top: 9px;
}
.tool-item {
  min-height: 40px;
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 6px 9px;
  border: 1px solid #1c3a50;
  border-radius: 6px;
  color: #dce7ee;
  background: #091c2b;
  text-align: left;
}
.tool-item:hover {
  border-color: #2c516c;
  background: #0d2436;
}
.tool-icon {
  width: 20px;
  color: #d9e7ef;
  display: inline-flex;
}
.tool-content {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
}
.tool-content strong {
  color: #e3edf2;
  font-size: 9.5px;
  font-weight: 600;
}
.tool-content small {
  margin-top: 1px;
  color: #71889c;
  font-size: 8px;
}
.tool-arrow {
  color: #9bb0c0;
  font-size: 14px;
}

.system-list {
  margin-top: 8px;
}
.system-row {
  min-height: 27px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  border-bottom: 1px solid #142d40;
  font-size: 9.5px;
}
.system-row span {
  color: #8399ad;
}
.system-row strong {
  color: #dce7ee;
  font-weight: 500;
}
.quick-actions-title {
  margin-top: 12px;
  margin-bottom: 6px;
  color: #e4edf2;
  font-size: 10px;
  font-weight: 650;
}
.quick-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
}
.quick-actions button {
  height: 27px;
  padding: 0 8px;
  border: 1px solid #263f55;
  border-radius: 5px;
  color: #b9c9d4;
  background: #091b2a;
  font-size: 8.5px;
}
.quick-actions button:hover {
  border-color: #367091;
}
.system-note {
  margin-top: 9px;
  color: #6f8699;
  font-size: 8px;
  line-height: 1.35;
}

/* ---------- О защите ---------- */
.about-protection-section {
  padding: 13px;
}
.about-grid {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr;
  gap: 20px;
  margin-top: 10px;
}
.about-grid h3 {
  margin: 0 0 6px;
  color: #e2ecf2;
  font-size: 11px;
}
.about-grid p,
.about-grid li {
  color: #8499ab;
  font-size: 9.5px;
  line-height: 1.5;
}
.about-grid p {
  margin: 0 0 6px;
}
.about-grid ul {
  margin: 0;
  padding-left: 15px;
}

/* ---------- Адаптив ---------- */
@media (max-width: 1180px) {
  .dashboard {
    grid-template-columns: 1fr;
  }
  .dashboard > * {
    grid-column: 1 !important;
  }
  .bottom-grid {
    grid-template-columns: 1fr 1fr;
  }
}
</style>
