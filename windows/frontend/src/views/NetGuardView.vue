<script setup lang="ts">
import { computed } from 'vue'
import PageHeading from '../components/PageHeading.vue'
import HintBox from '../components/HintBox.vue'
import Icon from '../components/Icon.vue'
import ProxyRecoveryPanel from '../components/ProxyRecoveryPanel.vue'
import { netGuardStatus } from '../api/backend'
import { fmtDateTime, fmtLoopbackObservation } from '../api/labels'
import type { NetGuardTaskState } from '../api/contract'
import { usePolling } from '../composables/polling'
import { useNav } from '../composables/nav'

const { data: status, loading, error: loadError, receivedAt, refresh } = usePolling(netGuardStatus, 15000, { pauseWhenHidden: true })
const { navTo } = useNav()
const hero = computed(() => {
  const s = status.value
  if (!s) return {
    tone: 'unknown',
    title: loading.value ? 'Получаем локальные наблюдения…' : 'Состояние неизвестно',
    detail: loading.value ? 'Ожидается ответ приложения Windows.' : loadError.value || 'Данные недоступны. Откройте приложение Windows и повторите загрузку.',
  }
  if (s.proxy.stale) return { tone: 'warning', title: 'Сообщён сбой прокси', detail: 'Для исправления нужно подтверждение владения настройками. Ответ порта такого права не даёт.' }
  if (!s.tasks.length) return { tone: 'unknown', title: 'Нет сведений о задачах', detail: 'Список задач пуст. Регистрация, выполнение и состояние восстановления не подтверждены.' }
  if (s.tasks.every(task => task.status === 'not-found')) return { tone: 'unknown', title: 'Задачи NetGuard не найдены', detail: 'Автоматическое восстановление через эти задачи не подтверждено.' }
  if (s.tasks.some(task => task.status === 'no-access' || task.status === 'not-found' || task.status.startsWith('unknown'))) {
    return { tone: 'unknown', title: 'Наблюдение ограничено', detail: 'Часть задач не найдена, скрыта правами или имеет неизвестный статус. Это не доказывает наличие или отсутствие защиты.' }
  }
  if (s.tasks.some(task => task.status === 'disabled')) return { tone: 'warning', title: 'Есть отключённые задачи', detail: 'Изучите состояние ниже. Не включайте старые скрипты сброса без проверки их полномочий.' }
  return { tone: 'observed', title: 'Локальные сведения получены', detail: 'Статусы планировщика и ответы портов не подтверждают защиту трафика, исправность DNS или результат восстановления.' }
})
const taskLabels: Record<string, string> = {
  ready: 'Готова к запуску', running: 'Выполняется', disabled: 'Отключена', queued: 'В очереди',
  'no-access': 'Недостаточно прав для наблюдения', 'not-found': 'Не найдена', unknown: 'Статус неизвестен',
}
function taskLabel(state: NetGuardTaskState): string { return taskLabels[state] ?? `Статус неизвестен (${state})` }
const eventLabels: Record<string, string> = { HEALED: 'Сообщение о восстановлении', HEALING: 'Попытка восстановления', PROBLEM: 'Сообщение о проблеме', WEEKLY: 'Плановое событие' }
const logScope = computed(() => {
  const log = status.value?.log
  if (!log) return 'Границы чтения журнала не сообщены этой версией приложения. Полнота истории неизвестна.'
  if (log.state === 'missing') return 'Файл журнала не найден. Это не устанавливает состояние установки или восстановления.'
  if (log.state === 'busy') return 'Предыдущее чтение журнала ещё выполняется. Новое чтение не начато.'
  if (log.state === 'unavailable') return 'Журнал прочитать не удалось либо закрытие предыдущего чтения не подтверждено. Остальные локальные наблюдения показаны отдельно.'
  const omitted = log.truncated ? ' Более ранние записи не вошли в этот фрагмент.' : ''
  const incomplete = log.incomplete ? ' Незавершённые, обрезанные краем фрагмента или слишком длинные записи исключены.' : ''
  return `Прочитано ${log.bytesRead} байт, обработано строк: ${log.rows}. Лимит одного чтения — 64 КиБ фрагмента и 256 байт для определения кодировки; не более 256 полных строк.${omitted}${incomplete}`
})
</script>

<template>
  <div class="view">
    <PageHeading title="Локальное восстановление" description="Настройки прокси, задачи и история. Сначала — границы владения и доступное действие.">
      <button class="btn" :disabled="loading" @click="refresh">{{ loading ? 'Обновление…' : 'Обновить наблюдения' }}</button>
    </PageHeading>

    <section class="observation" :class="hero.tone">
      <div class="observation-heading" role="status" aria-live="polite" aria-atomic="true">
        <Icon name="info" :size="22" />
        <div><h2>{{ hero.title }}</h2><p>{{ hero.detail }}</p></div>
      </div>
      <p class="receipt-time">Получено экраном: {{ fmtDateTime(receivedAt) }}. Это время ответа интерфейсу, а не время измерения Windows.</p>
      <p v-if="status && loading" class="receipt-time">Наблюдения обновляются; показан предыдущий ответ.</p>
    </section>

    <section class="recovery-boundary">
      <h2>Что можно восстановить</h2>
      <dl class="recovery-facts">
        <div><dt>Изменения текущей сессии</dt><dd>При обычной остановке backend может восстановить только собственные подтверждённые изменения. Конфликт с другой программой не даёт права перезаписать её настройки.</dd></div>
        <div><dt>После аварии или нового запуска</dt><dd>Для записанных изменений Snowden можно подготовить отдельное подтверждение. При отсутствии проверенного журнала или неизвестном результате предыдущей операции восстановление недоступно.</dd></div>
        <div><dt>Результат восстановления</dt><dd>Наблюдение порта, статус задачи и запись старого события не являются результатом текущей операции восстановления.</dd></div>
      </dl>
      <div class="actions-row">
        <button class="btn" @click="navTo('dashboard')">Перейти к остановке VPN</button>
        <button class="btn" @click="navTo('logs')">Изучить журнал</button>
      </div>
      <HintBox kind="info">Если настройки заняты другой программой, изучите причину конфликта. Не сбрасывайте proxy, DNS или системные правила по одному признаку недоступного порта.</HintBox>
      <ProxyRecoveryPanel />
    </section>

    <template v-if="status">
      <div class="observed-grid" :aria-busy="loading">
        <section class="observed-section">
          <h2>Системный прокси</h2>
          <dl class="proxy-facts">
            <div><dt>Настройка Windows</dt><dd>{{ status.proxy.enabled ? 'Включена' : 'Выключена' }}</dd></div>
            <div><dt>Сообщённый адрес</dt><dd class="mono">{{ status.proxy.server || 'Не задан' }}</dd></div>
            <div><dt>Порт из настройки</dt><dd>{{ status.proxy.port > 0 ? fmtLoopbackObservation(status.loopbacks, status.proxy.port) : 'Локальный порт не определён' }}</dd></div>
            <div><dt>Порт 127.0.0.1:1080</dt><dd>{{ fmtLoopbackObservation(status.loopbacks, 1080) }}</dd></div>
          </dl>
          <p class="scope-note">Наблюдается только IPv4/TCP на локальном порту; повторяющийся порт проверяется один раз. Ответ не устанавливает владельца процесса, исправность прокси-протокола или доступность VPN-сервера. Отказ, истечение времени и неизвестный исход различаются. Время отдельных измерений не передано.</p>
        </section>

        <section class="observed-section">
          <h2>Задачи планировщика</h2>
          <p v-if="!status.tasks.length" class="scope-note">Список пуст: состояние задач неизвестно.</p>
          <ul v-else class="task-list">
            <li v-for="task in status.tasks" :key="task.name">
              <div class="task-title"><strong class="mono">{{ task.name }}</strong><span :class="{ warning: task.status === 'disabled' }">{{ taskLabel(task.status) }}</span></div>
              <dl v-if="task.status !== 'not-found' && task.status !== 'no-access'" class="task-dates">
                <div><dt>Последний запуск</dt><dd>{{ fmtDateTime(task.lastRun) }}</dd></div>
                <div><dt>Следующий запуск</dt><dd>{{ fmtDateTime(task.nextRun) }}</dd></div>
                <div><dt>Код прошлого запуска</dt><dd>{{ task.lastCode || 'Не сообщён' }}</dd></div>
              </dl>
            </li>
          </ul>
          <p class="scope-note">«Готова» и код 0 описывают планировщик, а не результат восстановления или защиту трафика.</p>
        </section>
      </div>

      <details class="event-history">
        <summary>Сообщения журнала<span v-if="status.log?.state === 'observed' || status.events.length"> · {{ status.events.length }}</span></summary>
        <p class="scope-note">{{ logScope }}</p>
        <p v-if="!status.events.length" class="scope-note">Значимые сообщения не получены. Это не доказывает отсутствие сбоев.</p>
        <ol v-else class="event-list">
          <li v-for="(event, index) in status.events" :key="index">
            <span class="event-time mono">{{ event.time }}</span>
            <span class="event-kind">{{ eventLabels[event.kind] ?? 'Событие неизвестного типа' }}</span>
            <p>{{ event.text }}</p>
          </li>
        </ol>
        <p v-if="status.lastRun" class="scope-note">Последняя запись о запуске в полученном фрагменте: {{ status.lastRun }}</p>
        <p class="scope-note">Исторические сообщения не подтверждают состояние текущей сессии.</p>
      </details>
    </template>
  </div>
</template>

<style scoped>
.view { display: flex; flex-direction: column; gap: 20px; min-width: 0; overflow-wrap: anywhere; }
h2 { margin: 0; font-size: 17px; }
.observation, .recovery-boundary { min-width: 0; padding: 20px; border: 1px solid var(--border); border-radius: var(--radius); background: var(--panel); }
.observation { border-left: 2px solid var(--accent); }
.observation.warning { border-left-color: var(--yellow); }
.observation-heading { display: flex; align-items: flex-start; gap: 12px; }
.observation-heading :deep(svg) { flex: 0 0 auto; color: var(--accent); margin-top: 2px; }
.observation-heading p { margin: 8px 0 0; color: var(--text-soft); line-height: 1.6; }
.receipt-time, .scope-note { margin: 12px 0 0; font-size: 12.5px; line-height: 1.6; color: var(--muted); }
.recovery-facts, .proxy-facts { margin: 12px 0 0; }
.recovery-facts > div, .proxy-facts > div { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 2fr); gap: 12px 20px; padding: 12px 0; border-bottom: 1px solid var(--border); }
dt { color: var(--muted); font-size: 12.5px; }
dd { margin: 0; color: var(--text-soft); font-size: 13px; line-height: 1.6; }
.actions-row { display: flex; flex-wrap: wrap; gap: 10px; margin: 18px 0; }
.btn { min-height: 40px; white-space: normal; }
.observed-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 24px; }
.observed-section { min-width: 0; padding: 6px 0; }
.task-list { list-style: none; padding: 0; margin: 12px 0 0; }
.task-list > li { padding: 14px 0; border-bottom: 1px solid var(--border); }
.task-title { display: flex; flex-wrap: wrap; gap: 8px 16px; justify-content: space-between; font-size: 12.5px; }
.task-title strong { font-weight: 500; color: var(--text); }
.task-title span { color: var(--muted); }
.task-title .warning { color: var(--yellow); }
.task-dates { margin: 10px 0 0; display: flex; flex-wrap: wrap; gap: 8px 20px; }
.task-dates dt, .task-dates dd { font-size: 11.5px; }
.event-history { border-top: 1px solid var(--border); padding-top: 12px; }
.event-history summary { padding: 8px 0; color: var(--text-soft); cursor: pointer; }
.event-list { list-style: none; padding: 0; margin: 8px 0; }
.event-list li { padding: 12px 0; border-bottom: 1px solid var(--border); }
.event-time, .event-kind { font-size: 12px; color: var(--muted); margin-right: 12px; }
.event-list p { color: var(--text-soft); margin: 6px 0 0; font-size: 13px; line-height: 1.6; }
@media (max-width: 1180px) { .observed-grid { grid-template-columns: minmax(0, 1fr); gap: 16px; } }
@media (max-width: 760px) { .recovery-facts > div, .proxy-facts > div { grid-template-columns: minmax(0, 1fr); gap: 6px; } .observation, .recovery-boundary { padding: 16px; } }
</style>
