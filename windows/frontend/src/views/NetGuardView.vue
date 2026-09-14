<script setup lang="ts">
/**
 * NetGuard: фактическое состояние авто-починки сети (tools/netguard).
 * Только факты из backend-биндинга: задачи планировщика (машинные статусы),
 * системный прокси, живость SOCKS:1080, последние события лога.
 * Нет данных — «нет данных». no-access — честный факт (SYSTEM-задача скрыта
 * от не-elevated процесса), а не «не найдена».
 */
import { computed, onMounted, onUnmounted, ref } from 'vue'
import Icon from '../components/Icon.vue'
import type { IconName } from '../components/Icon.vue'
import { netGuardStatus, runProxyHeal } from '../api/backend'
import type { NetGuardStatus, NetGuardTaskState } from '../api/contract'
import { toast } from '../composables/toast'

const status = ref<NetGuardStatus | null>(null)
const loading = ref(false)
const healing = ref(false)
let timer: number | undefined

async function refresh(): Promise<void> {
  loading.value = true
  try {
    status.value = await netGuardStatus()
  } catch (e) {
    status.value = null
    toast('error', `NetGuard: биндинг недоступен — ${e instanceof Error ? e.message : String(e)}`)
  } finally {
    loading.value = false
  }
}

async function healNow(): Promise<void> {
  healing.value = true
  try {
    const healed = await runProxyHeal()
    if (healed === null) {
      toast('warn', 'Heal недоступен: приложение запущено вне Wails')
    } else if (healed) {
      toast('ok', 'Зависший системный прокси отключён')
    } else {
      toast('info', 'Чинить нечего: прокси валиден или не наш формат')
    }
    await refresh()
  } catch (e) {
    toast('error', `Heal не выполнен: ${e instanceof Error ? e.message : String(e)}`)
  } finally {
    healing.value = false
  }
}

onMounted(() => {
  void refresh()
  timer = window.setInterval(() => void refresh(), 15000)
})
onUnmounted(() => window.clearInterval(timer))

const staleProxy = computed(() => status.value?.proxy.stale ?? false)
const vpnActive = computed(() => status.value?.socksAlive ?? false)

/** Честный вердикт героя — только из машинных статусов. Пока данных нет —
 * честная загрузка, не выдумка. */
const hero = computed<{ icon: IconName; cls: string; state: string; sub: string }>(() => {
  const s = status.value
  if (!s) {
    return { icon: 'refresh', cls: 'busy', state: 'Загрузка данных NetGuard…', sub: 'Биндинг опрашивается; факты появятся после ответа.' }
  }
  const tasks = s.tasks
  const anyMissing = tasks.some((t) => t.status === 'not-found')
  const anyNoAccess = tasks.some((t) => t.status === 'no-access')
  const allGone = tasks.every((t) => t.status === 'not-found')

  if (staleProxy.value) {
    return {
      icon: 'shield-warn',
      cls: 'err',
      state: 'Обнаружен зависший прокси',
      sub: 'Устраняется плановой задачей NetGuard-AutoHeal-User в течение 2 минут, при следующем запуске приложения — мгновенно. Можно устранить сейчас кнопкой ниже.',
    }
  }
  if (allGone) {
    return {
      icon: 'shield-warn',
      cls: 'idle',
      state: 'NetGuard не установлен',
      sub: 'Плановые задачи не найдены. Установите: tools/netguard/install.ps1 (запуск от администратора).',
    }
  }
  if (anyMissing || anyNoAccess) {
    // Смешанный вид: часть задач видна, часть скрыта правами/отсутствует.
    const missing = tasks.filter((t) => t.status === 'not-found').map((t) => t.name)
    const hidden = tasks.filter((t) => t.status === 'no-access').map((t) => t.name)
    const parts: string[] = []
    if (hidden.length) parts.push(`скрыта правами: ${hidden.join(', ')}`)
    if (missing.length) parts.push(`не найдена: ${missing.join(', ')}`)
    return {
      icon: 'info',
      cls: 'busy',
      state: 'Защита частично наблюдаема',
      sub: `Процесс без прав администратора видит не все задачи (${parts.join('; ')}). SYSTEM-задача продолжает работать — это ограничение просмотра, не сбой.`,
    }
  }
  const allEnabled = tasks.every((t) => t.status === 'ready' || t.status === 'running')
  if (allEnabled && tasks.length > 0) {
    return {
      icon: 'shield',
      cls: 'ok',
      state: 'Защита активна',
      sub: 'Плановые задачи auto-heal зарегистрированы и работают: зависший прокси и DNS-сбои устраняются автоматически.',
    }
  }
  const degraded = tasks.filter((t) => t.status === 'disabled').map((t) => t.name)
  return {
    icon: 'warn',
    cls: 'idle',
    state: 'Защита не в рабочем состоянии',
    sub: `Статусы задач: ${tasks.map((t) => `${t.name}=${t.status}`).join(', ')}. ${degraded.length ? 'Задачи отключены — включите в Планировщике задач или переустановите NetGuard.' : 'Проверьте планировщик задач.'}`,
  }
})

const taskStateText: Record<string, string> = {
  ready: 'готова',
  running: 'выполняется',
  disabled: 'отключена',
  queued: 'в очереди',
  'no-access': 'скрыта правами (SYSTEM)',
  'not-found': 'не найдена — установите NetGuard',
  unknown: 'статус неизвестен',
}

function stateText(s: NetGuardTaskState): string {
  return taskStateText[s] ?? s
}

function statePill(s: NetGuardTaskState): 'live-verified' | 'degraded' | 'blocked' | '' {
  if (s === 'ready' || s === 'running') return 'live-verified'
  if (s === 'no-access') return ''
  if (s === 'disabled') return 'degraded'
  if (s === 'not-found' || s.startsWith('unknown')) return 'blocked'
  return ''
}

function fmtDate(iso: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleString('ru-RU', { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' })
}

function kindClass(kind: string): string {
  return kind === 'PROBLEM' ? 'fail' : kind === 'HEALED' || kind === 'HEALING' ? 'pass' : ''
}
</script>

<template>
  <div class="view">
    <header class="head">
      <h1>NetGuard</h1>
      <p class="sub">
        Авто-починка сети (tools/netguard): устраняет зависший системный прокси после
        аварийного завершения VPN и включает DNS-over-HTTPS, если сеть блокирует порт 53.
        Данные только из backend, обновление каждые 15 секунд.
      </p>
      <div class="actions-row">
        <button class="btn" :disabled="loading" @click="refresh()">
          <Icon name="refresh" :size="15" />
          {{ loading ? 'Обновление…' : 'Обновить' }}
        </button>
        <button class="btn" :disabled="healing" @click="healNow()">
          <Icon name="pulse" :size="15" />
          {{ healing ? 'Починка…' : 'Починить прокси сейчас' }}
        </button>
      </div>
    </header>

    <div v-if="status === null && !loading" class="empty-block">
      <Icon name="warn" :size="18" />
      <p>
        Данные NetGuard недоступны: биндинг не отвечает или NetGuard не установлен
        (tools/netguard/install.ps1). Вымышленные статусы не показываются.
      </p>
    </div>

    <template v-else-if="status">
      <section class="hero" :class="hero.cls">
        <Icon :name="hero.icon" :size="30" class="hero-ico" />
        <div>
          <div class="hero-state">{{ hero.state }}</div>
          <div class="hero-sub">{{ hero.sub }}</div>
        </div>
      </section>

      <div class="grid">
        <section class="panel">
          <h2>Плановые задачи</h2>
          <div v-for="t in status.tasks" :key="t.name" class="kv task-row">
            <span class="k mono">{{ t.name }}</span>
            <span class="v">
              <span :class="{ 'dim-text': t.status === 'no-access' }">{{ stateText(t.status) }}</span>
              <span v-if="statePill(t.status)" :class="['pill', statePill(t.status)]">
                {{ t.status === 'ready' || t.status === 'running' ? 'ок' : t.status }}
              </span>
              <template v-if="t.status !== 'not-found' && t.status !== 'no-access'">
                <span v-if="t.lastRun" class="task-dates">прошлый: {{ fmtDate(t.lastRun) }}</span>
                <span v-if="t.nextRun" class="task-dates">следующий: {{ fmtDate(t.nextRun) }}</span>
                <span v-if="t.lastCode && t.lastCode !== '0'" class="pill degraded">код {{ t.lastCode }}</span>
              </template>
            </span>
          </div>
        </section>

        <section class="panel">
          <h2>Системный прокси</h2>
          <div class="kv">
            <span class="k">Состояние</span>
            <span class="v">
              {{ status.proxy.enabled ? 'включён' : 'выключен' }}
              <span v-if="status.proxy.enabled && status.proxy.listenerAlive" class="pill live-verified">листенер жив</span>
              <span v-if="staleProxy" class="pill blocked">листенер мёртв (stale)</span>
            </span>
            <span class="k">Адрес</span>
            <span class="v mono">{{ status.proxy.server || '—' }}</span>
            <span class="k">SOCKS VPN :1080</span>
            <span class="v">
              <span :class="vpnActive ? 'ok-text' : 'dim-text'">{{ vpnActive ? 'слушается (VPN активен)' : 'не слушается (VPN выключен)' }}</span>
            </span>
          </div>
        </section>
      </div>

      <section class="panel">
        <h2>Последние события</h2>
        <div v-if="status.events.length === 0" class="dim-text">
          Существенных событий нет: сбоев auto-heal не зафиксировано
          <span v-if="status.dohDisabled"> (лог NetGuard не найден)</span>.
        </div>
        <ul v-else class="steps">
          <li v-for="(e, i) in status.events" :key="i" class="step" :class="kindClass(e.kind)">
            <span class="st mono">{{ e.time }}</span>
            <span class="st badge-kind">{{ e.kind }}</span>
            <span class="step-status">{{ e.text }}</span>
          </li>
        </ul>
        <p v-if="status.lastRun" class="last-at mono">последний запуск: {{ status.lastRun }}</p>
      </section>
    </template>
  </div>
</template>

<style scoped>
.task-row {
  margin-bottom: 8px;
}
.task-dates {
  display: inline-block;
  margin-left: 10px;
  font-size: 11.5px;
  color: var(--text-faint);
}
.ok-text {
  color: var(--ok);
}
.dim-text {
  color: var(--text-dim);
  font-size: 12.5px;
}
.badge-kind {
  padding: 1px 8px;
  border-radius: 999px;
  background: var(--bg-hover);
  font-size: 10.5px;
  font-family: var(--mono);
}
.step.pass .badge-kind {
  color: var(--ok);
}
.step.fail .badge-kind {
  color: var(--danger);
}
.mono {
  font-family: var(--mono);
}
</style>
