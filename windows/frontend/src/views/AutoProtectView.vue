<script setup lang="ts">
/**
 * Автозащита (concept): сторож защищённого пути — состояние, правила, инциденты.
 * Состояние сторожа — факт из failoverPolicy (GetFailoverStatus); правила —
 * описание реального кода (failover.go), не выдумка. Инциденты — из журнала.
 */
import { computed, onMounted, onUnmounted, ref } from 'vue'
import Icon from '../components/Icon.vue'
import HintBox from '../components/HintBox.vue'
import { useBackendState, initBackendState } from '../composables/backendState'
import { api, isBuildPhase } from '../api/backend'
import type { FailoverStatus } from '../api/contract'
import { uiLogLines } from '../composables/uiLog'
import { fmtDateTime } from '../api/labels'

const { isRunning, state } = useBackendState()
const logLines = uiLogLines()

const status = ref<FailoverStatus | null>(null)
const loadError = ref('')
let timer: number | undefined

async function refresh(): Promise<void> {
  if (isBuildPhase()) {
    status.value = null
    loadError.value = 'Биндинги недоступны: приложение запущено вне Wails (предпросмотр).'
    return
  }
  try {
    status.value = (await api.getFailoverStatus()) as FailoverStatus
    loadError.value = ''
  } catch (e) {
    loadError.value = e instanceof Error ? e.message : String(e)
  }
}

onMounted(() => {
  void initBackendState()
  void refresh()
  timer = window.setInterval(() => void refresh(), 10000)
})
onUnmounted(() => window.clearInterval(timer))

const tab = ref<'state' | 'rules' | 'incidents'>('state')

const stateLabel = computed(() => {
  const s = status.value
  if (!s) return 'Нет данных'
  if (!s.enabled) return 'Сторож не запущен'
  switch (s.state) {
    case 'closed':
      return 'Сторож активен'
    case 'open':
      return 'Cooldown после сбоя'
    case 'half-open':
      return 'Идёт автопопытка'
    default:
      return s.state
  }
})

const stateClass = computed(() => {
  const s = status.value
  if (!s?.enabled) return 'neutral'
  if (s.state === 'closed') return 'success'
  if (s.state === 'half-open') return 'warning'
  return 'warning'
})

const nextCheckLabel = computed(() => {
  const s = status.value
  if (!s) return '—'
  if (!s.enabled) return 'запустится вместе с VPN'
  if (s.nextCheckIn <= 0) return 'сейчас'
  return `≈ через ${s.nextCheckIn} с`
})

// Инциденты — факты из журнала UI/backend (строки про failover/переключение).
const incidents = computed(() =>
  logLines.value
    .filter((l) => /failover|переключ|probe|BLOCKED|heal/i.test(l.text))
    .slice(-12)
    .reverse(),
)

function incidentClass(level: string): string {
  if (level === 'error') return 'error'
  if (level === 'warn') return 'warning'
  return 'info'
}
</script>

<template>
  <div class="view">
    <header class="head">
      <div class="section-title-group">
        <h2>Автоматическая защита</h2>
        <span class="pill" :class="stateClass"><span class="dot"></span>{{ stateLabel }}</span>
      </div>
      <p class="sub">
        Сторож периодически проверяет защищённый путь и автоматически переключает канал при
        подтверждённой деградации. Решение принимает policy (failover.go), применение — через
        тот же Reload + probe, что и ручное переключение.
      </p>
    </header>

    <div class="tabs card">
      <button class="auto-tab" :class="{ active: tab === 'state' }" @click="tab = 'state'">Состояние</button>
      <button class="auto-tab" :class="{ active: tab === 'rules' }" @click="tab = 'rules'">Правила</button>
      <button class="auto-tab" :class="{ active: tab === 'incidents' }" @click="tab = 'incidents'">Инциденты</button>
    </div>

    <template v-if="tab === 'state'">
      <div class="state-grid">
        <section class="card watcher">
          <div class="watcher-status">
            <div class="watcher-icon"><Icon name="shield" :size="18" /></div>
            <div>
              <strong>{{ stateLabel }}</strong>
              <p>
                {{ isRunning
                  ? 'VPN запущен: сторож тикает каждые 30 секунд (лёгкий probe через SOCKS-инбаунд).'
                  : 'Сторож стартует вместе с VPN. Сейчас защита не запущена.' }}
              </p>
            </div>
          </div>

          <div class="rows">
            <div class="row"><span>Состояние policy</span><strong>{{ status?.state ?? '—' }}</strong></div>
            <div class="row"><span>Сторож запущен</span><strong :class="status?.enabled ? 'green-value' : ''">{{ status ? (status.enabled ? 'да' : 'нет') : '—' }}</strong></div>
            <div class="row"><span>Следующая проверка</span><strong>{{ nextCheckLabel }}</strong></div>
            <div class="row"><span>Авто-переключений за инцидент</span><strong>{{ status?.switches ?? '—' }}</strong></div>
            <div class="row"><span>Последняя причина сбоя</span><strong>{{ status?.lastError || '—' }}</strong></div>
          </div>

          <p v-if="loadError" class="load-error">Нет данных: {{ loadError }}</p>
        </section>

        <section class="card side">
          <div class="card-header"><h2>Почему это безопасно</h2></div>
          <ul class="facts">
            <li>Переключение выполняется ТОЛЬКО после успешного protected-probe нового канала.</li>
            <li>Кандидаты — только enabled live-verified каналы (HY2-configured не автопереключается).</li>
            <li>2 подтверждённых провала → cooldown 30/60/120 с (circuit breaker).</li>
            <li>Кандидаты исчерпаны → BLOCKED: туннель закрывается, прямой трафик не включается.</li>
          </ul>
          <p v-if="state?.probeLastAt" class="side-note">Последний probe: {{ fmtDateTime(state.probeLastAt) }}</p>
        </section>
      </div>
    </template>

    <template v-else-if="tab === 'rules'">
      <section class="card rules">
        <div class="card-header"><h2>Правила автозащиты (фактический код)</h2></div>
        <div class="rule">
          <strong>1. Проверка защищённого пути</strong>
          <p>Каждые 30 секунд при работающем VPN выполняется лёгкий probe: DNS через туннель, две HTTPS-цели с маркером egress, expected egress, отсутствие утечек.</p>
        </div>
        <div class="rule">
          <strong>2. Подтверждение деградации</strong>
          <p>Один краткий сбой не роняет канал. Автопереключение стартует после 2 подряд подтверждённых провалов защищённого пути.</p>
        </div>
        <div class="rule">
          <strong>3. Кандидаты</strong>
          <p>Порядок попыток: прочие каналы живого селектора, затем остальные enabled live-verified. Текущий канал исключается; каналы вне селектора требуют рестарта и автопереключением не являются.</p>
        </div>
        <div class="rule">
          <strong>4. Применение</strong>
          <p>Переключение идёт через Reload с двойной валидацией конфига и обязательным probe. Неудача = fail-closed (движок останавливается, прокси восстанавливается).</p>
        </div>
        <div class="rule">
          <strong>5. Circuit breaker</strong>
          <p>После исчерпания попыток (2 авто-переключения за инцидент) — BLOCKED и cooldown 30→60→120 с. Новый пользовательский Start сбрасывает счётчики инцидента.</p>
        </div>
      </section>
    </template>

    <template v-else>
      <section class="card incidents">
        <div class="card-header"><h2>Инциденты (из журнала)</h2></div>
        <div v-if="incidents.length === 0" class="empty">Пока пусто — это норма до первого инцидента.</div>
        <div v-for="(l, i) in incidents" :key="i" class="incident" :class="incidentClass(l.level)">
          <span class="incident-dot"></span>
          <time class="mono">{{ fmtDateTime(l.t) }}</time>
          <span class="incident-text">{{ l.text }}</span>
        </div>
      </section>
    </template>

    <HintBox kind="info" title="Откуда берутся данные">
      Состояние — биндинг GetFailoverStatus (снимок failoverPolicy), инциденты — строки журнала
      приложения. Никаких выдуманных «аптаймов» и «процентов защиты».
    </HintBox>
  </div>
</template>

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

.tabs {
  display: flex;
  padding: 4px 6px;
  gap: 4px;
}
.auto-tab {
  min-width: 110px;
  height: 30px;
  border: 0;
  border-radius: 5px;
  color: #8096aa;
  background: transparent;
  font-size: 11px;
}
.auto-tab.active {
  color: #e2edf4;
  background: #102d47;
}

.state-grid {
  display: grid;
  grid-template-columns: 1.2fr 1fr;
  gap: 10px;
}

.watcher,
.side,
.rules,
.incidents {
  padding: 13px;
}

.watcher-status {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 10px;
  border: 1px solid #19364d;
  border-radius: 7px;
  background: rgba(7, 22, 35, 0.7);
}
.watcher-icon {
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 auto;
  border-radius: 8px;
  color: var(--green);
  background: rgba(37, 227, 160, 0.12);
}
.watcher-status strong {
  color: #38e7a8;
  font-size: 12px;
}
.watcher-status p {
  margin: 4px 0 0;
  color: #8299ad;
  font-size: 10px;
  line-height: 1.5;
}

.rows {
  margin-top: 10px;
}
.row {
  min-height: 28px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  border-bottom: 1px solid #142d40;
  color: #8298ab;
  font-size: 10.5px;
}
.row strong {
  color: #dbe6ed;
  font-weight: 500;
  text-align: right;
}
.load-error {
  margin: 10px 0 0;
  color: #ff8a90;
  font-size: 10.5px;
}

.side .facts {
  margin: 10px 0 0;
  padding-left: 16px;
  color: #8499ab;
  font-size: 10.5px;
  line-height: 1.6;
}
.side-note {
  margin: 10px 0 0;
  color: #6f8699;
  font-size: 10px;
}

.rule {
  padding: 10px 0;
  border-bottom: 1px solid #142d40;
}
.rule:last-child {
  border-bottom: 0;
}
.rule strong {
  color: #e2ecf2;
  font-size: 11px;
}
.rule p {
  margin: 4px 0 0;
  color: #8499ab;
  font-size: 10.5px;
  line-height: 1.55;
}

.empty {
  padding: 12px;
  color: #70889d;
  font-size: 11px;
}
.incident {
  display: grid;
  grid-template-columns: 12px 110px 1fr;
  gap: 8px;
  align-items: baseline;
  padding: 7px 0;
  border-bottom: 1px solid #142d40;
  font-size: 10.5px;
}
.incident-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #42b3ff;
  align-self: center;
}
.incident.warning .incident-dot {
  background: var(--yellow);
}
.incident.error .incident-dot {
  background: var(--red);
}
.incident time {
  color: #73899d;
  font-size: 9.5px;
}
.incident-text {
  color: #a4b6c5;
  word-break: break-word;
}

@media (max-width: 1000px) {
  .state-grid {
    grid-template-columns: 1fr;
  }
}
</style>
