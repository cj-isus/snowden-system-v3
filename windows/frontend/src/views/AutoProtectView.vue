<script setup lang="ts">
import PageHeading from '../components/PageHeading.vue'
/**
 * Автозащита (concept): сторож защищённого пути — состояние, правила, инциденты.
 * Состояние сторожа — факт из failoverPolicy (GetFailoverStatus); правила —
 * описание реального кода (failover.go), не выдумка. Инциденты — из журнала.
 */
import { computed, onMounted, ref } from 'vue'
import Icon from '../components/Icon.vue'
import HintBox from '../components/HintBox.vue'
import { useBackendState, initBackendState } from '../composables/backendState'
import { api } from '../api/backend'
import { usePolling } from '../composables/polling'
import { useInfoFacts } from '../composables/infoFacts'
import type { ChannelHealthEntry } from '../api/contract'
import { uiLogLines } from '../composables/uiLog'
import { fmtDateTime } from '../api/labels'

const { isRunning, state } = useBackendState()
const logLines = uiLogLines()

const { failover: status, refresh: refreshStatus } = useInfoFacts()
const { data: chanHealth, error: chanHealthError, refresh: refreshHealth } = usePolling(api.getChannelHealth, 10000)
const { data: incidentsClusters, refresh: refreshIncidents } = usePolling(api.getIncidents, 10000)
const { data: independence, error: independenceError, refresh: refreshIndependence } = usePolling(api.getChannelIndependence, 10000)
const loadError = computed(() => status.value ? '' : 'Состояние адаптации недоступно.')
async function refresh() { await Promise.all([refreshStatus(), refreshHealth(), refreshIncidents(), refreshIndependence()]) }
onMounted(() => { void initBackendState() })

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

// ---------- Здоровье каналов (PLAN-V3 A0: ChannelHealth по эпохе) ----------

// Человеческие подписи кодов failclass.go. Отсутствие кода в карте — не ошибка:
// показываем сам код (новые классы THEORY появляются раньше UI).
const FAIL_CLASS_LABELS: Record<string, string> = {
  reset_injected: 'RST-инъекция',
  port_blackholed: 'чёрная дыра порта',
  silent_freeze: 'тихая заморозка',
  tls_policing_443: 'полиция TLS:443',
  whitelist_mode: 'белый список',
  protected_channel_unavailable: 'канал недоступен',
  dns_path_failure: 'отказ DNS-пути',
  egress_divergence: 'подмена пути (egress)',
  direct_leak_confirmed: 'подтверждённая утечка',
  server_unreachable: 'сервер недоступен',
  degraded: 'деградация',
  unknown: 'причина неизвестна',
}

function classLabel(code: string): string {
  return FAIL_CLASS_LABELS[code] ?? code
}

function verdictLabel(v: string): string {
  if (v === 'healthy') return 'здоров'
  if (v === 'failing') return 'сбоит'
  return 'нет данных'
}

function verdictClass(v: string): string {
  if (v === 'healthy') return 'success'
  if (v === 'failing') return 'error'
  return 'neutral'
}

// --- D2 (V2-064): состояние восстановления канала (quarantine.go) ---

// D2_STATE_LABELS — RU-подписи состояний D2. Неизвестное состояние (новое в
// backend) показывается как есть — UI не врёт и не ломается (прецедент C1).
const D2_STATE_LABELS: Record<string, string> = {
  HEALTHY: 'здоров',
  PROBATION: 'под наблюдением',
  RECOVERING: 'восстанавливается',
  QUARANTINED: 'карантин',
}

function d2Label(s: string): string {
  return D2_STATE_LABELS[s] ?? s
}

function d2Class(s: string): string {
  switch (s) {
    case 'HEALTHY':
      return 'success'
    case 'PROBATION':
    case 'RECOVERING':
      return 'info'
    case 'QUARANTINED':
      return 'danger'
    default:
      return 'neutral'
  }
}

// freshness — свежесть последнего факта (EvidenceAge ≤2 мин живёт в Go;
// здесь только показ). Протухший факт уже честно стал no_data на бэкенде.
function freshness(c: ChannelHealthEntry): string {
  const iso = c.verdict === 'failing' ? c.lastFailAt : c.lastOkAt
  if (!iso) return 'probe ещё не был'
  const t = new Date(iso).getTime()
  if (!Number.isFinite(t)) return ''
  const age = Math.max(0, Math.round((Date.now() - t) / 1000))
  if (age < 60) return `${age} с назад`
  return `${Math.round(age / 60)} мин назад`
}

const epochShort = computed(() => {
  const h = chanHealth.value
  if (!h || h.epoch <= 0) return ''
  return `E${h.epoch}`
})

// --- C1: машина состояний адаптивного цикла (V2-060/062) ---

// SM_STATE_LABELS — RU-подписи состояний controller.go. Неизвестное состояние
// (новое в backend) показывается как есть — UI не врёт и не ломается.
const SM_STATE_LABELS: Record<string, string> = {
  HEALTHY: 'здоров',
  SUSPECTED: 'подозрение',
  CLASSIFYING: 'классификация',
  DEGRADED: 'деградация',
  SWITCHING: 'переключение',
  BLOCKED: 'заблокирован',
  RECOVERING: 'восстановление',
}

const smState = computed(() => status.value?.smState ?? '')

const smStateLabel = computed(() => {
  const s = smState.value
  if (!s) return 'цикл не активен'
  return SM_STATE_LABELS[s] ?? s
})

const smStateClass = computed(() => {
  switch (smState.value) {
    case 'HEALTHY':
      return 'success'
    case 'SUSPECTED':
    case 'CLASSIFYING':
    case 'RECOVERING':
      return 'info'
    case 'DEGRADED':
    case 'SWITCHING':
      return 'warning'
    case 'BLOCKED':
      return 'danger'
    default:
      return 'neutral' // "" = цикл не активен (нет туннеля / ручной Stop)
  }
})

const smStateHint = computed(() => {
  switch (smState.value) {
    case 'HEALTHY':
      return 'последний probe прошёл — путь жив'
    case 'SUSPECTED':
      return 'одиночный провал — подтверждение серией быстрых репроб'
    case 'CLASSIFYING':
      return 'серия подтверждена — классификатор определяет характер отказа'
    case 'DEGRADED':
      return 'канал деградировал — готовится автопереключение'
    case 'SWITCHING':
      return 'идёт попытка переключения с обязательным probe нового канала'
    case 'BLOCKED':
      return 'кандидаты исчерпаны — туннель остановлен (fail-closed, прямой трафик не включается)'
    case 'RECOVERING':
      return 'туннель остановлен — автоповтор запуска с растущей паузой'
    default:
      return 'цикл не активен: нет туннеля (ручной Stop или до первого запуска)'
  }
})

// smSinceLabel — давность текущего состояния (обновляется тикером refresh).
const smSinceLabel = computed(() => {
  const iso = status.value?.smSince
  if (!iso) return ''
  const t = new Date(iso).getTime()
  if (!Number.isFinite(t)) return ''
  const age = Math.max(0, Math.round((Date.now() - t) / 1000))
  if (age < 60) return `${age} с в состоянии`
  if (age < 3600) return `${Math.round(age / 60)} мин в состоянии`
  return `${Math.floor(age / 3600)} ч ${Math.round((age % 3600) / 60)} мин в состоянии`
})
</script>

<template>
  <div class="view">
    <PageHeading title="Адаптация" description="Как система выбирает маршрут и реагирует на сбои в текущей сети." ><span class="pill" :class="stateClass">{{ stateLabel }}</span><button class="btn" @click="refresh">Обновить</button></PageHeading>

    <div class="tabs card">
      <button class="auto-tab" :class="{ active: tab === 'state' }" @click="tab = 'state'">Обзор</button>
      <button class="auto-tab" :class="{ active: tab === 'rules' }" @click="tab = 'rules'">Политика</button>
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

          <!-- C1 (V2-060/062): машина состояний адаптивного цикла — формализация
               того, что сторож уже делает; решения принимает policy, карточка —
               честная проекция фактов см. controller.go. -->
          <div v-if="status" class="sm-box">
            <div class="sm-head">
              <span class="sm-title">Адаптивный цикл</span>
              <span class="pill" :class="smStateClass"><span class="dot"></span>{{ smStateLabel }}</span>
            </div>
            <p class="sm-hint">{{ smStateHint }}</p>
            <p v-if="smSinceLabel" class="sm-since">{{ smSinceLabel }}<template v-if="status?.smClass"> · последний отказ: {{ classLabel(status.smClass) }}</template></p>
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

      <section class="card health">
        <div class="card-header">
          <h2>Здоровье каналов</h2>
          <span v-if="epochShort" class="pill neutral"><span class="dot"></span>эпоха {{ epochShort }}<template v-if="chanHealth?.netClass"> · {{ chanHealth.netClass }}</template></span>
        </div>
        <p class="health-sub">
          Факты probe внутри текущей сетевой эпохи: смена сети обнуляет реестр — факт «канал жил в
          прошлой эпохе» ничего не обещает сейчас. «Здоров» = свежий успешный probe (≤2 мин),
          а не гарантия будущего.
        </p>
        <p v-if="chanHealthError" class="load-error">Нет данных: {{ chanHealthError }}</p>
        <div v-else-if="!chanHealth" class="empty">Нет данных.</div>
        <template v-else>
          <div v-if="chanHealth.channels.length === 0" class="empty">Каналов нет.</div>
          <div v-for="c in chanHealth.channels" :key="c.id" class="chan-row">
            <div class="chan-id">
              <strong>{{ c.id }}</strong>
              <span v-if="c.transport" class="chan-transport">{{ c.transport }}</span>
            </div>
            <span class="pill" :class="verdictClass(c.verdict)"><span class="dot"></span>{{ verdictLabel(c.verdict) }}</span>
            <div class="chan-stats">
              <span class="chan-counts">ok {{ c.ok }} / fail {{ c.fail }}</span>
              <span v-if="c.lastClass" class="chan-class">{{ classLabel(c.lastClass) }}</span>
              <span class="chan-fresh">{{ freshness(c) }}</span>
            </div>
            <div v-if="c.d2State && c.d2State !== 'HEALTHY'" class="chan-d2" :title="c.d2Note">
              <span class="pill" :class="d2Class(c.d2State)"><span class="dot"></span>{{ d2Label(c.d2State) }}</span>
              <span class="chan-d2-note">{{ c.d2Note }}</span>
            </div>
          </div>
        </template>
      </section>

      <!-- V2-074 (аудит C5/D16): независимость по failure domains. Это policy-модель
           (дескрипторы + известный ASN), не измерение магистрали: показываем число,
           общие домены и честное «ниже порога». -->
      <section class="card health">
        <div class="card-header">
          <h2>Независимость каналов</h2>
          <span v-if="independence && independence.length" class="pill neutral">
            <span class="dot"></span>{{ independence.length }} пар
          </span>
        </div>
        <p class="health-sub">
          Оценка по пересечению failure domains (origin, ASN, carrier, транспорт, класс порта,
          слой, TLS-донор): чем ниже, тем сильнее каналы зависят от одной инфраструктуры.
          Порог независимости — 0.35; ниже порога пара не является безопасной заменой друг друга.
          Это policy-модель, а не измерение сети.
        </p>
        <p v-if="independenceError" class="load-error">Нет данных: {{ independenceError }}</p>
        <div v-else-if="!independence" class="empty">Нет данных.</div>
        <template v-else>
          <div v-for="row in independence" :key="row.a + '|' + row.b" class="indep-row" :class="{ 'indep-low': row.belowLimit }">
            <div class="indep-pair">
              <strong>{{ row.a }}</strong>
              <span class="indep-vs">↔</span>
              <strong>{{ row.b }}</strong>
            </div>
            <span class="indep-score mono">{{ row.score.toFixed(2) }}</span>
            <span class="pill" :class="row.belowLimit ? 'warning' : 'success'">
              <span class="dot"></span>{{ row.belowLimit ? 'ниже порога' : 'независимы' }}
            </span>
            <span class="indep-shared">{{ row.sharedFdoms || 'общих известных доменов нет' }}</span>
          </div>
        </template>
      </section>
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
        <div class="card-header">
          <h2>Волны отказов (эпоха {{ epochShort || '—' }})</h2>
        </div>
        <p class="health-sub">
          Подтверждённые отказы каналов, сгруппированные по времени (окно 90 с, центр ±60 с):
          отказы одной волны — это реакция сети на несколько каналов сразу, а не совпадение.
          Смена сети (новая эпоха) обнуляет картину.
        </p>
        <div v-if="incidentsClusters === null" class="empty">Нет данных: эпоха сети ещё не открыта (наблюдений отказов не было).</div>
        <div v-else-if="incidentsClusters.length === 0" class="empty">Подтверждённых отказов в этой эпохе не было.</div>
        <div v-for="c in incidentsClusters" :key="c.clusterId" class="incident-cluster">
          <span class="pill" :class="c.isWave ? 'warning' : 'neutral'"><span class="dot"></span>{{ c.isWave ? 'волна' : 'одиночный' }}</span>
          <div class="incident-cluster-body">
            <div class="incident-cluster-head">
              <strong>{{ c.channels.join(' · ') }}</strong>
              <span class="incident-cluster-count">{{ c.count }} отказ(ов), {{ fmtDateTime(c.since) }} — {{ fmtDateTime(c.until) }}</span>
            </div>
            <div v-if="c.classes.length" class="incident-cluster-classes">
              {{ c.classes.map((code) => classLabel(code)).join(' · ') }}
            </div>
            <div v-if="c.netClasses.length || c.isps.length" class="incident-cluster-classes">
              Сеть: {{ [...c.netClasses, ...c.isps].join(' · ') }}
            </div>
          </div>
        </div>
        <div class="card-header" style="margin-top: 10px"><h2>Строки журнала</h2></div>
        <div v-if="incidents.length === 0" class="empty">Пока пусто — это норма до первого инцидента.</div>
        <div v-for="(l, i) in incidents" :key="'log' + i" class="incident" :class="incidentClass(l.level)">
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
  background: var(--accent-soft);
}

.state-grid {
  display: grid;
  grid-template-columns: 1.2fr 1fr;
  gap: 10px;
}

.watcher, .side, .rules { padding: 20px; }
/* D3 (V2-065): кластеры волн отказов */
.incident-cluster {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 8px 0;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
}

.incident-cluster-body {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.incident-cluster-head {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 8px;
}

.incident-cluster-count {
  font-size: 12px;
  color: #6f8699;
}

.incident-cluster-classes {
  font-size: 12px;
  color: #6f8699;
}

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
  font-size: 12px;
  line-height: 1.5;
}

.rows {
  margin-top: 10px;
}

/* C1: карточка машины состояний адаптивного цикла */
.sm-box {
  margin-top: 10px;
  padding: 10px;
  border: 1px solid #19364d;
  border-radius: 7px;
  background: rgba(7, 22, 35, 0.7);
}
.sm-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
.sm-title {
  color: #8298ab;
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.06em;
}
.sm-hint {
  margin: 6px 0 0;
  color: #8299ad;
  font-size: 12px;
  line-height: 1.5;
}
.sm-since {
  margin: 5px 0 0;
  color: #5f788c;
  font-size: 12px;
}
.row {
  min-height: 28px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  border-bottom: 1px solid #142d40;
  color: #8298ab;
  font-size: 12px;
}
.row strong {
  color: #dbe6ed;
  font-weight: 500;
  text-align: right;
}
.load-error {
  margin: 10px 0 0;
  color: #ff8a90;
  font-size: 12px;
}

.side .facts {
  margin: 10px 0 0;
  padding-left: 16px;
  color: #8499ab;
  font-size: 12px;
  line-height: 1.6;
}
.side-note {
  margin: 10px 0 0;
  color: #6f8699;
  font-size: 12px;
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
  font-size: 12px;
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
  font-size: 12px;
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
  font-size: 12px;
}
.incident-text {
  color: #a4b6c5;
  word-break: break-word;
}

.health {
  padding: 13px;
}
.health-sub {
  margin: 8px 0 0;
  color: #6f8699;
  font-size: 12px;
  line-height: 1.55;
}
.chan-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
  padding: 8px 0;
  border-bottom: 1px solid #142d40;
}
.chan-row:last-child {
  border-bottom: 0;
}
.chan-id {
  min-width: 0;
  flex-wrap: wrap;
  display: flex;
  align-items: baseline;
  gap: 8px;
}
.chan-id strong {
  color: #dbe6ed;
  font-size: 11px;
  font-weight: 500;
}
.chan-transport {
  padding: 1px 6px;
  border-radius: 4px;
  border: 1px solid #19364d;
  color: #7e96a9;
  font-size: 12px;
  background: rgba(7, 22, 35, 0.7);
}
.chan-stats {
  flex-wrap: wrap;
  margin-left: auto;
  display: flex;
  align-items: baseline;
  gap: 12px;
  font-size: 12px;
  color: #8298ab;
}
.chan-class {
  color: #ffc46b;
}
.chan-fresh {
  color: #6f8699;
}

/* D2 (V2-064): строка состояния восстановления канала */
.chan-d2 {
  grid-column: 1 / -1;
  display: flex;
  align-items: center;
  gap: 10px;
  padding-top: 2px;
}

.chan-d2-note {
  font-size: 12px;
  color: #6f8699;
}

/* V2-074: независимость каналов (failure domains) */
.indep-row {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 10px;
  padding: 7px 0;
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
  font-size: 12px;
}
.indep-row:last-child {
  border-bottom: none;
}
.indep-low {
  opacity: 0.95;
}
.indep-pair {
  display: flex;
  align-items: baseline;
  gap: 6px;
  min-width: 0;
  flex-wrap: wrap;
}
.indep-pair strong {
  font-weight: 600;
  color: #cdd9e5;
}
.indep-vs {
  color: #6f8699;
}
.indep-score {
  color: #9fb3c8;
}
.indep-shared {
  margin-left: auto;
  color: #8298ab;
  font-size: 11px;
}

@media (max-width: 1000px) {
  .state-grid {
    grid-template-columns: 1fr;
  }
}
</style>
