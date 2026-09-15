<script setup lang="ts">
/**
 * Главная (UI-план v3): hero с power-кнопкой, текущий канал, карта маршрута,
 * проверка пути с инлайн-шагами, факты сети, режим SOCKS/TUN, сводка каналов
 * и автозащиты со ссылками (не дубли страниц — v2 тащил целые копии разделов).
 * Все данные — факты из backend; пусто = «нет данных». Навигация — useNav().
 */
import { computed, onMounted, onUnmounted, ref, watchEffect } from 'vue'
import Icon from '../components/Icon.vue'
import type { IconName } from '../components/Icon.vue'
import { useBackendState, initBackendState } from '../composables/backendState'
import { useLifecycle } from '../composables/lifecycle'
import { useInfoFacts } from '../composables/infoFacts'
import { useNav } from '../composables/nav'
import { validationStatusLabels, lifecycleLabels, blockedReasonLabels, fmtDateTime } from '../api/labels'
import { netGuardStatus } from '../api/backend'
import type { NetGuardStatus } from '../api/contract'
import { uiLogLines } from '../composables/uiLog'

const { state, stateLoaded, isRunning, isBusy, primaryAction, channels, tunMode, tunBusy, setTUNMode } = useBackendState()
const { start, stop, runProbe } = useLifecycle()
const { facts, failover } = useInfoFacts()
const { navTo } = useNav()
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

// BLOCKED расшифровывается машинной причиной из контракта (v2 прятал её).
const blockedLine = computed(() => {
  const r = state.value?.blockedReason
  return r ? `BLOCKED — ${blockedReasonLabels[r] ?? r}` : null
})

const heroDescription = computed(() => {
  if (blockedLine.value) return `${blockedLine.value}: запуск остаётся закрытым (fail-closed).`
  if (isRunning.value) return 'Трафик проходит через защищённый канал.'
  if (state.value?.error) return state.value.error
  return 'Соединение не установлено. Запуск — строго после успешного protected-probe.'
})

// ---------- fail-closed баннер (V2-053) ----------
// Два фактических пути к fail-closed (оба — факты из контракта, не догадки):
//  1) state='error' после исчерпания failover-on-start (failCore:
//     «все каналы не прошли probe … fail-closed (FR-001)»);
//  2) state='stopped' + blockedReason='all_channels_failed' после исчерпания
//     сторожа (V2-037 watchdog BLOCKED).
const failClosed = computed(() => {
  const s = state.value
  if (!s) return null
  const watchdogBlocked = s.blockedReason === 'all_channels_failed'
  const startExhausted = s.state === 'error' && (s.error ?? '').includes('fail-closed')
  if (!watchdogBlocked && !startExhausted) return null
  return {
    detail: watchdogBlocked
      ? 'Сторож исчерпал все validated каналы: защищённый путь недоступен ни на одном из них — туннель остановлен (BLOCKED).'
      : 'Failover-on-start перебрал все validated каналы — ни один не прошёл protected-probe (FR-001).',
    hint: 'Запуск закрыт честно: без успешного probe туннель не включается. Когда сеть восстановит путь — повторная попытка безопасна.',
  }
})

// Авто-повтор (V2-053): backend сам запланировал следующую попытку с растущим
// backoff (30с→1м→2м→4м→8м→10м); здесь — честный отсчёт до неё. Тикаем каждую
// секунду, пока баннер виден; отрицательная дельта (due давно, попытка вот-вот)
// показывается как «сейчас». Пустой nextRetryAt — авто-повтор не запланирован
// (старая сборка backend или ручное действие отменило расписание).
const nowTick = ref(Date.now())
let retryTicker: ReturnType<typeof setInterval> | null = null
watchEffect(() => {
  const active = !!failClosed.value && !isBusy.value && !!state.value?.nextRetryAt
  if (active && retryTicker === null) {
    nowTick.value = Date.now()
    retryTicker = setInterval(() => (nowTick.value = Date.now()), 1000)
  } else if (!active && retryTicker !== null) {
    clearInterval(retryTicker)
    retryTicker = null
  }
})
onUnmounted(() => {
  if (retryTicker !== null) clearInterval(retryTicker)
})

const retrySchedule = computed(() => {
  const s = state.value
  if (!failClosed.value || isBusy.value || !s?.nextRetryAt) return null
  const due = new Date(s.nextRetryAt)
  if (Number.isNaN(due.getTime())) return null // честно: битное время не выдумываем
  const leftMs = due.getTime() - nowTick.value
  const total = Math.max(0, Math.floor(leftMs / 1000))
  const mm = Math.floor(total / 60)
  const ss = total % 60
  const left = leftMs <= 0 ? 'сейчас' : mm > 0 ? `${mm}:${String(ss).padStart(2, '0')}` : `0:${String(ss).padStart(2, '0')}`
  return {
    attempt: s.retryAttempt,
    left,
    dueText: fmtTime(due),
  }
})

function fmtTime(d: Date): string {
  return d.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

const retrying = ref(false)

/** Повторный запуск из fail-closed баннера — тот же гейт, что и hero-кнопка. */
async function onRetryStart(): Promise<void> {
  if (retrying.value) return
  retrying.value = true
  try {
    await start()
  } finally {
    retrying.value = false
  }
}

// ---------- текущий канал ----------

const activeChannel = computed(() => state.value?.activeChannel ?? null)
const channelStatusPill = computed(() => {
  const v = activeChannel.value?.validation
  if (!v) return null
  const cls =
    v === 'live-verified' ? 'live' : v === 'degraded' ? 'degraded' : v === 'blocked' ? 'blocked' : v === 'retired' || v === 'planned' ? 'neutral' : 'configured'
  return { cls, text: validationStatusLabels[v] ?? v }
})

// ---------- проверка пути: шаги с человеческими статусами ----------

const probeSteps = computed(() => state.value?.probe?.steps ?? [])
const probePassed = computed(() => probeSteps.value.filter((s) => s.status === 'pass').length)

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
  items.push({ icon: 'warn', text: 'Утечек прямого трафика нет', ok: leak === 'pass' ? true : leak === 'fail' ? false : null })
  return items
})

// Egress-IP: сначала шаг «Expected egress» (его detail содержит факт),
// резерв — любой egress-шаг с распознаваемым IP (v2 брал первый попавшийся
// egress-шаг и показывал «обе цели видят один egress» вместо адреса).
const egressFact = computed(() => {
  const steps = probeSteps.value
  const pick = (needle: string) => steps.find((s) => s.name.toLowerCase().includes(needle))
  const st = pick('expected egress') ?? pick('egress')
  if (!st) return '—'
  const m = /egress IP: ([0-9a-fA-F.:]+)/.exec(st.detail) ?? /egress ([0-9a-fA-F.:]+),/.exec(st.detail)
  if (m) return m[1]
  return /([0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3})/.exec(st.detail)?.[1] ?? '—'
})

// ---------- карта маршрута ----------

const routeActive = computed(() => isRunning.value && !!activeChannel.value)
const routeServerLabel = computed(() => activeChannel.value?.id ?? '')
const routeServerHint = computed(() => activeChannel.value?.server ?? '')
const localIP = computed(() => facts.value?.localIP || null)
const directEgress = computed(() => facts.value?.publicIP || null)

// ---------- режим маршрутизации: факт + честное переключение ----------

const routingMode = computed(() => {
  if (state.value?.proxyMode === 'tun') return 'TUN (вся система)'
  if (state.value?.proxyMode === 'socks') return 'SOCKS 127.0.0.1:1080'
  return null
})

const tunSwitchLabel = computed(() => {
  if (tunBusy.value) return 'Перезапуск…'
  return tunMode.value ? 'Вернуться в SOCKS' : 'Включить TUN'
})

function onToggleTUN(): void {
  void setTUNMode(!tunMode.value)
}

// ---------- сводка каналов (вместо копии таблицы v2) ----------

const statusCounts = computed(() => {
  const arr = channels.value ?? []
  const m = new Map<string, number>()
  for (const c of arr) m.set(c.validation, (m.get(c.validation) ?? 0) + 1)
  return [...m.entries()]
})

function statusClass(v: string): string {
  if (v === 'live-verified') return 'live'
  if (v === 'degraded') return 'degraded'
  if (v === 'blocked') return 'blocked'
  if (v === 'retired' || v === 'planned') return 'neutral'
  return 'configured'
}

// ---------- автозащита (ссылка, не копия раздела) ----------

const watchdogLabel = computed(() => {
  const f = failover.value
  if (!f) return 'нет данных сторожа'
  if (!f.enabled) return 'сторож запустится вместе с VPN'
  if (f.state === 'closed') return 'сторож активен'
  if (f.state === 'half-open') return 'идёт автопопытка'
  if (f.state === 'open') return 'cooldown после сбоя'
  return f.state
})

const failoverBadgeClass = computed(() => {
  const f = failover.value
  if (!f || !f.enabled) return 'neutral'
  if (f.state === 'closed') return 'success'
  if (f.state === 'open') return 'danger'
  return 'warning'
})

const nextCheckLabel = computed(() => {
  const f = failover.value
  if (!f || !f.enabled) return '—'
  return f.nextCheckIn <= 0 ? 'сейчас' : `≈ через ${f.nextCheckIn} с`
})

// ---------- события ----------

const netGuard = ref<NetGuardStatus | null>(null)
onMounted(async () => {
  try {
    netGuard.value = await netGuardStatus()
  } catch {
    netGuard.value = null
  }
})

const incidentLines = computed(() =>
  logLines.value
    .filter((l) => /failover|переключ|BLOCKED|heal|probe/i.test(l.text))
    .slice(-3)
    .reverse(),
)
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
          <span class="power-symbol">⏻</span>
        </button>

        <div class="protection-state" :class="heroTone">
          <span class="state-dot"></span>
          <span>{{ heroHeading.toUpperCase() }}</span>
        </div>

        <button class="disconnect-button" :disabled="primaryAction.disabled" @click="onPrimary">
          {{ primaryAction.label }}
        </button>
      </div>

      <div class="protection-information">
        <div class="protection-heading" :class="heroTone">{{ heroHeading }}</div>
        <div class="protection-description">{{ heroDescription }}</div>

        <!-- Fail-closed: все каналы исчерпаны (failover-on-start или сторож) -->
        <div v-if="failClosed" class="failclosed-banner" role="alert">
          <div class="fc-head">
            <span class="fc-badge">FAIL-CLOSED</span>
            <strong>Все каналы не прошли проверку защищённого пути</strong>
          </div>
          <p class="fc-detail">{{ failClosed.detail }}</p>
          <p class="fc-hint">{{ failClosed.hint }}</p>
          <p v-if="retrySchedule" class="fc-next">
            ⟳ Авто-повтор запуска<span v-if="retrySchedule.attempt > 1"> (попытка №{{ retrySchedule.attempt }})</span>:
            через <strong>{{ retrySchedule.left }}</strong> — в {{ retrySchedule.dueText }}.
            Каждая попытка снова проверяет все каналы probe'ом.
          </p>
          <div class="fc-actions">
            <button class="fc-retry" :disabled="retrying || isBusy" @click="onRetryStart">
              {{ retrying ? 'Повторный запуск…' : '⟳ Повторить запуск' }}
            </button>
            <button class="fc-channels" @click="navTo('channels')">Посмотреть каналы</button>
          </div>
        </div>

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
            <div class="channel-detail">
              <span class="detail-label">Транспорт</span>
              <strong>{{ activeChannel.transport || '—' }}</strong>
            </div>
            <div class="channel-detail">
              <span class="detail-label">Порт</span>
              <strong>{{ activeChannel.port || '—' }}</strong>
            </div>
            <div class="channel-detail">
              <span class="detail-label">Сервер</span>
              <strong>{{ activeChannel.server || '—' }}</strong>
            </div>
          </div>

          <div class="current-channel-actions">
            <button class="change-channel-button" @click="navTo('channels')">
              <span>⇄</span>
              <span>Сменить канал</span>
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

    <!-- ================= ПРОВЕРКА ПУТИ (инлайн-отчёт) ================= -->
    <aside class="security-check-card card">
      <div class="card-header">
        <h2>Проверка защищённого пути</h2>
        <span v-if="state?.probeRunning" class="status-badge warning"><span class="status-dot"></span>Выполняется…</span>
        <span v-else-if="state?.probe?.ok" class="status-badge success"><span class="status-dot"></span>Успешно</span>
        <span v-else-if="state?.probe" class="status-badge danger"><span class="status-dot"></span>Не пройдено</span>
        <span v-else class="status-badge neutral"><span class="status-dot"></span>Нет данных</span>
      </div>

      <ol v-if="probeSteps.length" class="probe-steps">
        <li v-for="(s, i) in probeSteps" :key="i" class="probe-step" :class="s.status" :title="s.detail">
          <span class="ps-name">{{ s.name }}</span>
          <span class="ps-status">{{
            s.status === 'pass' ? 'пройдено' : s.status === 'fail' ? 'провалено' : s.status === 'running' ? 'выполняется' : s.status === 'skipped' ? 'пропущено' : 'ожидает'
          }}</span>
        </li>
      </ol>
      <p v-else class="probe-empty">Probe ещё не запускался в этой сессии.</p>

      <div class="verification-columns">
        <div class="verification-column">
          <div class="verification-label">Последняя проверка</div>
          <strong>{{ state?.probeLastAt ? fmtDateTime(state.probeLastAt) : '—' }}</strong>
        </div>
        <div class="verification-column">
          <strong>{{ probeSteps.length ? `${probePassed} / ${probeSteps.length}` : '—' }}</strong>
          <div class="verification-label">шагов пройдено</div>
        </div>
        <div class="verification-column">
          <div class="verification-label">Egress</div>
          <strong class="mono">{{ egressFact }}</strong>
        </div>
      </div>

      <button class="secondary-button" :disabled="state?.probeRunning || isBusy" @click="runProbe">
        {{ state?.probeRunning ? 'Проверка выполняется…' : 'Проверить сейчас' }}
      </button>
    </aside>

    <!-- ================= БЫСТРЫЕ ФАКТЫ ================= -->
    <section class="quick-information">
      <article class="information-card card">
        <div class="information-icon"><Icon name="dashboard" :size="20" /></div>
        <div class="information-content">
          <div class="information-label">Режим маршрутизации</div>
          <div class="information-value">{{ routingMode ?? 'Нет данных' }}</div>
          <div class="information-description">
            <button class="mode-switch" :disabled="tunBusy || !stateLoaded" :title="tunMode ? 'Перезапуск приложения в SOCKS-режиме (без UAC)' : 'Перезапуск приложения в TUN-режиме: Wintun требует права администратора (UAC). VPN будет остановлен и запущен заново.'" @click="onToggleTUN">
              {{ tunSwitchLabel }}
            </button>
          </div>
        </div>
      </article>

      <article class="information-card card">
        <div class="information-icon blue"><Icon name="pulse" :size="20" /></div>
        <div class="information-content">
          <div class="information-label">Системный прокси</div>
          <div class="information-value">{{ netGuard ? (netGuard.proxy.enabled ? `${netGuard.proxy.server}:${netGuard.proxy.port}` : 'выключен') : 'Нет данных' }}</div>
          <div class="information-description">
            {{ netGuard ? (netGuard.proxy.listenerAlive ? 'листенер жив' : 'листенер не слушается') : 'нет данных NetGuard' }}
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

    <!-- ================= КАНАЛЫ + АВТОЗАЩИТА (сводки со ссылками) ================= -->
    <section class="digest-grid">
      <section class="card digest-card">
        <div class="card-header">
          <h2>Каналы</h2>
          <span class="count-badge">{{ channels ? channels.length : '—' }}</span>
          <button class="digest-link" @click="navTo('channels')">Все каналы ›</button>
        </div>
        <div v-if="channels === null" class="digest-empty">Список каналов недоступен: backend не отвечает.</div>
        <div v-else-if="channels.length === 0" class="digest-empty">В конфиге нет каналов — запуск останется заблокированным (fail-closed).</div>
        <div v-else class="digest-chips">
          <span v-for="[v, n] in statusCounts" :key="v" class="channel-status" :class="statusClass(v)">
            <span class="status-dot"></span>{{ validationStatusLabels[v as keyof typeof validationStatusLabels] ?? v }}: {{ n }}
          </span>
        </div>
        <p class="digest-note">
          Активный: <strong>{{ state?.activeChannelId ?? '—' }}</strong>. В protected selector попадают только каналы
          «проверен live»; переключение проходит обязательный probe (fail-closed).
        </p>
      </section>

      <section class="card digest-card">
        <div class="card-header">
          <h2>Автозащита</h2>
          <span class="status-badge" :class="failoverBadgeClass"><span class="status-dot"></span>{{ watchdogLabel }}</span>
          <button class="digest-link" @click="navTo('autoprotect')">Подробнее ›</button>
        </div>
        <div class="digest-rows">
          <div class="digest-row"><span>Следующая проверка</span><strong>{{ nextCheckLabel }}</strong></div>
          <div class="digest-row"><span>Последний probe</span><strong>{{ state?.probeLastAt ? fmtDateTime(state.probeLastAt) : '—' }}</strong></div>
          <div class="digest-row"><span>Авто-переключений</span><strong>{{ failover ? String(failover.switches) : '—' }}</strong></div>
        </div>
        <div v-if="incidentLines.length" class="auto-incidents">
          <div v-for="(l, i) in incidentLines" :key="i" class="auto-incident" :class="l.level">
            <span class="ai-dot"></span>
            <span class="ai-text">{{ l.text }}</span>
          </div>
        </div>
      </section>
    </section>

    <!-- ================= СОБЫТИЯ ================= -->
    <section class="events-section card">
      <div class="events-header">
        <h2>История событий</h2>
        <button class="show-more-button" @click="navTo('logs')">Показать больше →</button>
      </div>
      <div v-if="logLines.length" class="events-list">
        <div v-for="(l, i) in logLines.slice(-4).reverse()" :key="i" class="event-line" :class="l.level">
          <span class="event-time mono">{{ fmtDateTime(l.t) }}</span>
          <span class="event-text">{{ l.text }}</span>
        </div>
      </div>
      <div v-else class="events-empty">Событий пока нет — журнал наполнится при первом действии.</div>
    </section>
  </div>
</template>

<style scoped>
.dashboard {
  display: grid;
  grid-template-columns: 1fr 360px;
  gap: 10px;
}

.dashboard > .protection-hero { grid-column: 1; }
.dashboard > .quick-information { grid-column: 1; }
.dashboard > .security-check-card,
.dashboard > .events-section { grid-column: 2; }
.dashboard > .security-check-card { align-self: start; }
.dashboard > .digest-grid { grid-column: 1; }

/* ---------- HERO ---------- */
.protection-hero {
  position: relative;
  min-height: 300px;
  display: grid;
  grid-template-columns: 36% 64%;
  overflow: hidden;
}
.protection-hero.running .protection-control::before {
  background: radial-gradient(circle at 50% 55%, rgba(24, 229, 161, 0.15), transparent 42%);
}
.protection-hero.error .protection-control::before {
  background: radial-gradient(circle at 50% 55%, rgba(255, 79, 89, 0.12), transparent 42%);
}
.protection-control {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
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
.power-symbol {
  font-size: 56px;
  line-height: 1;
  font-weight: 300;
}
.protection-button.stopped,
.protection-button.busy {
  border-color: var(--blue);
  color: var(--blue-bright);
  background: radial-gradient(circle, rgba(20, 140, 255, 0.12), rgba(7, 24, 38, 0.95) 67%);
  box-shadow:
    0 0 0 7px rgba(20, 140, 255, 0.07),
    0 0 26px rgba(20, 140, 255, 0.25),
    inset 0 0 30px rgba(20, 140, 255, 0.13);
}
/* fail-closed баннер (V2-053) */
.failclosed-banner {
  margin-top: 14px;
  padding: 14px 16px;
  border: 1px solid #ff636355;
  border-left: 3px solid var(--red, #ff6363);
  border-radius: 10px;
  background: #ff63630d;
  display: grid;
  gap: 6px;
}
.fc-head {
  display: flex;
  align-items: center;
  gap: 10px;
  color: var(--text);
  font-size: 13.5px;
}
.fc-badge {
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.08em;
  padding: 3px 8px;
  border-radius: 6px;
  background: #ff636322;
  border: 1px solid #ff636355;
  color: #ffb9bd;
}
.fc-detail,
.fc-hint,
.fc-next {
  margin: 0;
  font-size: 12.5px;
  line-height: 1.45;
}
.fc-detail {
  color: var(--text);
}
.fc-hint {
  color: var(--text-faint);
}
.fc-next {
  color: var(--text-faint);
}
.fc-next strong {
  color: var(--text);
  font-variant-numeric: tabular-nums;
}
.fc-actions {
  display: flex;
  gap: 8px;
  margin-top: 6px;
}
.fc-retry,
.fc-channels {
  cursor: pointer;
  border-radius: 8px;
  padding: 8px 14px;
  font-size: 12.5px;
  font-weight: 600;
  transition: all 0.15s ease;
}
.fc-retry {
  border: 1px solid #ff636366;
  background: #ff63631a;
  color: #ffd2d4;
}
.fc-retry:hover:not(:disabled) {
  background: #ff63632e;
}
.fc-retry:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}
.fc-channels {
  border: 1px solid var(--border);
  background: transparent;
  color: var(--text);
}
.fc-channels:hover {
  background: var(--bg-hover);
}

.protection-button.error {
  border-color: var(--red);
  color: var(--red);
  background: radial-gradient(circle, rgba(255, 79, 89, 0.12), rgba(38, 7, 14, 0.95) 67%);
  box-shadow:
    0 0 0 7px rgba(255, 79, 89, 0.08),
    0 0 26px rgba(255, 79, 89, 0.25),
    inset 0 0 30px rgba(255, 79, 89, 0.13);
}
.protection-button:disabled {
  filter: saturate(0.4) brightness(0.75);
  cursor: not-allowed;
}
.protection-button:not(:disabled):hover {
  filter: brightness(1.12);
}

.protection-state {
  display: flex;
  align-items: center;
  gap: 7px;
  font-size: 11px;
  letter-spacing: 0.09em;
  color: var(--text-soft);
  z-index: 2;
}
.protection-state .state-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--muted-2);
}
.protection-state.running .state-dot { background: var(--green); box-shadow: 0 0 9px var(--green); }
.protection-state.error .state-dot { background: var(--red); box-shadow: 0 0 9px var(--red); }
.protection-state.busy .state-dot { background: var(--yellow); animation: pulse 1.1s infinite; }
@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.35; }
}

.disconnect-button {
  height: 34px;
  padding: 0 22px;
  border-radius: 8px;
  border: 1px solid var(--border-light);
  background: var(--panel-3);
  color: var(--text-soft);
  font-size: 12px;
  z-index: 2;
}
.disconnect-button:hover:not(:disabled) { background: var(--bg-hover); color: var(--text); }
.disconnect-button:disabled { opacity: 0.55; cursor: not-allowed; }

.protection-information {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 18px 20px 16px 6px;
  min-width: 0;
}
.protection-heading {
  font-size: 21px;
  font-weight: 680;
}
.protection-heading.running { color: var(--green-bright); }
.protection-heading.error { color: var(--red); }
.protection-heading.busy { color: var(--yellow); }
.protection-description {
  color: var(--muted);
  font-size: 12px;
  max-width: 560px;
}

.current-channel-card {
  border: 1px solid var(--border);
  border-radius: 10px;
  background: linear-gradient(160deg, rgba(13, 30, 47, 0.9), rgba(10, 25, 40, 0.9));
  padding: 11px 13px;
}
.current-channel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
.current-channel-header h2 {
  margin: 0;
  font-size: 10.5px;
  text-transform: uppercase;
  letter-spacing: 0.1em;
  color: var(--muted);
  font-weight: 600;
}
.current-channel-name {
  display: flex;
  align-items: baseline;
  gap: 8px;
  font-size: 16px;
  font-weight: 640;
  margin: 5px 0 8px;
}
.channel-arrow { color: var(--green); font-weight: 700; }
.current-channel-details {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
}
.channel-detail {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 7px 9px;
  border-radius: 7px;
  background: rgba(7, 21, 34, 0.7);
  border: 1px solid rgba(26, 52, 75, 0.55);
  min-width: 0;
}
.detail-label {
  font-size: 9.5px;
  text-transform: uppercase;
  letter-spacing: 0.07em;
  color: var(--muted-2);
}
.channel-detail strong {
  font-size: 11.5px;
  font-family: var(--mono);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.current-channel-actions { margin-top: 9px; }
.change-channel-button {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  height: 28px;
  padding: 0 13px;
  border-radius: 7px;
  border: 1px solid var(--green-border);
  background: var(--green-soft);
  color: var(--green-bright);
  font-size: 11.5px;
}
.change-channel-button:hover { background: rgba(37, 227, 160, 0.19); }

.route-visualization {
  position: relative;
  height: 118px;
  border-radius: 10px;
  border: 1px solid var(--border);
  background: #081827;
  overflow: hidden;
}
.map-background {
  position: absolute;
  inset: 0;
  background-image: radial-gradient(rgba(65, 110, 148, 0.16) 1px, transparent 1px);
  background-size: 16px 16px;
}
.route-line {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
}
.route-line path {
  fill: none;
  stroke: rgba(20, 140, 255, 0.28);
  stroke-width: 1.6;
  stroke-dasharray: 5 5;
}
.route-visualization.active .route-line path {
  stroke: var(--green);
  animation: dashmove 1.4s linear infinite;
}
@keyframes dashmove {
  to { stroke-dashoffset: -20; }
}
.route-point {
  position: absolute;
  display: flex;
  align-items: center;
  gap: 8px;
}
.route-point.device { left: 12px; bottom: 14px; }
.route-point.server { right: 12px; top: 14px; text-align: right; flex-direction: row-reverse; }
.route-point.server.ghost { opacity: 0.45; }
.route-point .point {
  width: 9px;
  height: 9px;
  border-radius: 50%;
  background: var(--blue);
  box-shadow: 0 0 10px var(--blue);
}
.route-visualization.active .route-point .point {
  background: var(--green);
  box-shadow: 0 0 12px var(--green);
}
.route-location { display: flex; flex-direction: column; gap: 1px; }
.route-location strong { font-size: 11.5px; font-family: var(--mono); }
.route-location small { font-size: 10px; color: var(--muted-2); }

.egress-fact {
  font-size: 10.5px;
  color: var(--muted-2);
}

.connection-summary {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 6px;
}
.connection-summary-item {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 7px 9px;
  border-radius: 7px;
  border: 1px solid var(--border);
  background: rgba(7, 21, 34, 0.65);
  font-size: 10.5px;
  color: var(--text-soft);
  min-width: 0;
}
.connection-summary-item .summary-icon { color: var(--muted-2); flex: 0 0 auto; }
.connection-summary-item.ok { border-color: var(--green-border); }
.connection-summary-item.ok .summary-icon { color: var(--green); }
.connection-summary-item.bad { border-color: var(--red-border); color: #ffb9bd; }
.connection-summary-item.bad .summary-icon { color: var(--red); }

/* ---------- ПРОВЕРКА ПУТИ ---------- */
.security-check-card { padding: 13px; display: flex; flex-direction: column; gap: 10px; }
.probe-steps {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.probe-step {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  padding: 6px 10px;
  border-radius: 7px;
  background: var(--bg-elevated);
  font-family: var(--mono);
  font-size: 11px;
  color: var(--text-dim);
}
.probe-step .ps-status { flex: 0 0 auto; }
.probe-step.pass .ps-status { color: var(--ok); }
.probe-step.fail .ps-status { color: var(--danger); }
.probe-step.running .ps-status { color: var(--accent); }
.probe-step.skipped .ps-status { color: var(--text-faint); }
.probe-empty { color: var(--text-faint); font-size: 12px; margin: 0; }

.verification-columns {
  display: grid;
  grid-template-columns: 1.3fr 0.7fr 1fr;
  gap: 8px;
}
.verification-column {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 8px 10px;
  border-radius: 7px;
  background: rgba(7, 21, 34, 0.6);
  border: 1px solid rgba(26, 52, 75, 0.5);
}
.verification-column strong { font-size: 12px; }
.verification-label {
  font-size: 9.5px;
  text-transform: uppercase;
  letter-spacing: 0.07em;
  color: var(--muted-2);
}
.secondary-button {
  height: 32px;
  border-radius: 8px;
  border: 1px solid var(--border-light);
  background: var(--panel-3);
  color: var(--text-soft);
  font-size: 12px;
}
.secondary-button:hover:not(:disabled) { background: var(--bg-hover); color: var(--text); }
.secondary-button:disabled { opacity: 0.55; cursor: not-allowed; }

/* ---------- БЫСТРЫЕ ФАКТЫ ---------- */
.quick-information {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
}
.information-card {
  display: flex;
  gap: 11px;
  padding: 13px;
  align-items: flex-start;
}
.information-icon {
  width: 34px;
  height: 34px;
  flex: 0 0 auto;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 9px;
  color: var(--green);
  background: var(--green-soft);
  border: 1px solid var(--green-border);
}
.information-icon.blue {
  color: var(--blue-bright);
  background: var(--blue-soft);
  border-color: var(--blue-border);
}
.information-content { min-width: 0; }
.information-label {
  font-size: 10px;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: var(--muted-2);
}
.information-value {
  font-size: 13.5px;
  font-weight: 640;
  margin: 2px 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.information-description {
  font-size: 10.5px;
  color: var(--muted-2);
  min-height: 24px;
  display: flex;
  align-items: center;
}
.mode-switch {
  height: 22px;
  padding: 0 10px;
  border-radius: 6px;
  border: 1px solid var(--blue-border);
  background: var(--blue-soft);
  color: var(--blue-bright);
  font-size: 10.5px;
}
.mode-switch:hover:not(:disabled) { background: rgba(20, 140, 255, 0.22); }
.mode-switch:disabled { opacity: 0.5; cursor: wait; }

/* ---------- СВОДКИ (каналы + автозащита) ---------- */
.digest-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}
.digest-card { padding: 13px; display: flex; flex-direction: column; gap: 9px; }
.card-header {
  display: flex;
  align-items: center;
  gap: 8px;
}
.card-header h2 { margin: 0; font-size: 13.5px; font-weight: 650; }
.count-badge {
  min-width: 20px;
  height: 18px;
  padding: 0 6px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 9px;
  background: var(--gray-soft);
  border: 1px solid var(--gray-border);
  font-size: 10.5px;
  color: var(--text-soft);
}
.digest-link {
  margin-left: auto;
  background: none;
  border: none;
  color: var(--blue-bright);
  font-size: 11.5px;
  padding: 2px 4px;
}
.digest-link:hover { text-decoration: underline; }
.digest-empty { color: var(--muted); font-size: 11.5px; }
.digest-chips { display: flex; flex-wrap: wrap; gap: 6px; }
.digest-note { margin: 0; font-size: 10.5px; color: var(--muted-2); }
.digest-note strong { font-family: var(--mono); color: var(--text-soft); }
.digest-rows { display: flex; flex-direction: column; gap: 5px; }
.digest-row {
  display: flex;
  justify-content: space-between;
  font-size: 11.5px;
  color: var(--muted);
}
.digest-row strong { color: var(--text-soft); font-weight: 600; }

.auto-incidents { display: flex; flex-direction: column; gap: 4px; margin-top: 2px; }
.auto-incident {
  display: flex;
  align-items: baseline;
  gap: 7px;
  font-size: 10.5px;
  padding: 5px 8px;
  border-radius: 6px;
  background: rgba(7, 21, 34, 0.6);
}
.auto-incident .ai-dot { width: 6px; height: 6px; border-radius: 50%; background: var(--muted-2); flex: 0 0 auto; }
.auto-incident.warn .ai-dot, .auto-incident.warning .ai-dot { background: var(--yellow); }
.auto-incident.error .ai-dot { background: var(--red); }
.auto-incident .ai-text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--text-soft);
}

/* ---------- СОБЫТИЯ ---------- */
.events-section { padding: 13px; display: flex; flex-direction: column; gap: 8px; }
.events-header { display: flex; align-items: center; justify-content: space-between; }
.events-header h2 { margin: 0; font-size: 13.5px; font-weight: 650; }
.show-more-button {
  background: none;
  border: none;
  color: var(--blue-bright);
  font-size: 11.5px;
}
.show-more-button:hover { text-decoration: underline; }
.events-list { display: flex; flex-direction: column; gap: 4px; }
.event-line {
  display: flex;
  gap: 8px;
  align-items: baseline;
  font-size: 10.5px;
  padding: 5px 8px;
  border-radius: 6px;
  background: rgba(7, 21, 34, 0.6);
}
.event-time { color: var(--muted-2); flex: 0 0 auto; }
.event-text { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--text-soft); }
.event-line.error .event-text { color: #ffb9bd; }
.event-line.warn .event-text { color: #ffe2a8; }
.events-empty { color: var(--muted-2); font-size: 11.5px; }

/* ---------- общие статусные чипы ---------- */
.channel-status {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 3px 9px;
  border-radius: 20px;
  font-size: 10px;
  border: 1px solid;
}
.channel-status .status-dot { width: 6px; height: 6px; border-radius: 50%; }
.channel-status.live { color: var(--green-bright); border-color: var(--green-border); background: var(--green-soft); }
.channel-status.live .status-dot { background: var(--green); }
.channel-status.configured { color: var(--blue-bright); border-color: var(--blue-border); background: var(--blue-soft); }
.channel-status.configured .status-dot { background: var(--blue); }
.channel-status.degraded { color: var(--yellow); border-color: var(--yellow-border); background: var(--yellow-soft); }
.channel-status.degraded .status-dot { background: var(--yellow); }
.channel-status.blocked { color: var(--red); border-color: var(--red-border); background: var(--red-soft); }
.channel-status.blocked .status-dot { background: var(--red); }
.channel-status.neutral { color: var(--muted); border-color: var(--gray-border); background: var(--gray-soft); }
.channel-status.neutral .status-dot { background: var(--muted-2); }

.status-badge {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 3px 9px;
  border-radius: 20px;
  font-size: 10px;
  border: 1px solid var(--gray-border);
  background: var(--gray-soft);
  color: var(--muted);
}
.status-badge .status-dot { width: 6px; height: 6px; border-radius: 50%; background: var(--muted-2); }
.status-badge.success { color: var(--green-bright); border-color: var(--green-border); background: var(--green-soft); }
.status-badge.success .status-dot { background: var(--green); }
.status-badge.warning { color: var(--yellow); border-color: var(--yellow-border); background: var(--yellow-soft); }
.status-badge.warning .status-dot { background: var(--yellow); }
.status-badge.danger { color: var(--red); border-color: var(--red-border); background: var(--red-soft); }
.status-badge.danger .status-dot { background: var(--red); }

.mono { font-family: var(--mono); }

@media (max-width: 1180px) {
  .dashboard { grid-template-columns: 1fr; }
  .dashboard > .security-check-card,
  .dashboard > .events-section { grid-column: 1; }
  .quick-information { grid-template-columns: repeat(2, 1fr); }
  .connection-summary { grid-template-columns: repeat(2, 1fr); }
}
</style>
