<script setup lang="ts">
import PageHeading from '../components/PageHeading.vue'
import ActionButton from '../components/ActionButton.vue'
/**
 * Ручная диагностика: пакет запускает проверки по очереди. Остановка пакета
 * запрещает следующие запуски, но не объявляет текущий вызов завершённым.
 * Результаты приходят из backend; null означает недоступные сведения.
 */
import { computed } from 'vue'
import TestCard from '../components/TestCard.vue'
import HintBox from '../components/HintBox.vue'
import Icon from '../components/Icon.vue'
import { useDiagnostics } from '../composables/diagnostics'
import { useBackendState } from '../composables/backendState'
import { fmtDateTime } from '../api/labels'

const { tests, runningId, busy: anyRunning, batchRunning, batchStopRequested, loading, loaded, loadError,
  lastOutcome, receivedAt, canRun, resultContext, runTest, runAll, refreshTests, stopBatch } = useDiagnostics()
const { state } = useBackendState()


const canRunAll = computed(() => canRun.value && !state.value?.probeRunning)

function onRunAll(): void {
  if (batchRunning.value) { if (!batchStopRequested.value) stopBatch() }
  else if (canRunAll.value) void runAll()
}

function onRefresh(): void { if (!anyRunning.value && !loading.value) void refreshTests() }
</script>

<template>
  <div class="view">
    <PageHeading title="Диагностика пути" description="Запустите нужную проверку и изучите её результат. Отсутствие результата не означает успех." ></PageHeading>

    <div class="actions-row">
      <ActionButton class="primary" :disabled="batchRunning ? batchStopRequested : !canRunAll" :busy="batchStopRequested" @click="onRunAll" :title="batchRunning ? 'Остановить следующие запуски; текущий вызов ожидается до ответа' : 'Выполнить проверки по очереди'">
        <Icon :name="batchRunning ? (batchStopRequested ? 'refresh' : 'cross') : 'play'" :size="14" />
        {{ batchRunning ? (batchStopRequested ? 'Ожидание ответа…' : 'Не запускать следующие') : 'Запустить все' }}
      </ActionButton>
      <ActionButton :disabled="anyRunning || loading" :busy="loading" @click="onRefresh">
        <Icon name="refresh" :size="14" />
        {{ loading ? 'Загрузка…' : 'Обновить список' }}
      </ActionButton>
      <span v-if="state?.probeRunning" class="probe-flag">probe выполняется…</span>
    </div>

    <p class="scope-note">Результаты относятся к моменту запуска. Смена известного контекста приложения отзывает их актуальность и останавливает следующие запуски пакета. Проверки автоматически не запускаются.</p>
    <p class="scope-note">Сведения получены: {{ fmtDateTime(receivedAt) }}. Это время чтения списка, не нового измерения.</p>
    <p v-if="lastOutcome" class="scope-note">{{ lastOutcome }}</p>
    <p v-if="loadError && tests !== null" class="scope-note">{{ loadError }}</p>

    <div v-if="loading && (!loaded || tests === null)" class="empty-block" role="status">Загружаем список проверок…</div>

    <div v-else-if="tests === null" class="empty-block" role="status">
      <Icon name="warn" :size="18" />
      <p>
        {{ loadError || 'Данные ещё не получены. Обновите список проверок.' }}
      </p>
    </div>

    <div v-else-if="tests.length === 0" class="empty-block" role="status">
      <Icon name="info" :size="18" />
      <p>Backend не вернул ни одного теста для текущего конфига.</p>
    </div>

    <div v-else class="cards test-list" :aria-busy="anyRunning || loading">
      <TestCard
        v-for="t in tests"
        :key="t.id"
        :test="t"
        :running="runningId === t.id"
        :disabled="!canRun"
        :context="resultContext(t.id)"
        @run="(id) => void runTest(id)"
      />
    </div>

    <details class="check-limits">
      <summary>Область измерения и ограничения</summary>
      <HintBox kind="info" title="Что проверяет probe">
        Probe проверяет ответы конкретных HTTPS-целей, DoH через выбранный канал и соответствие
        сообщённого выходного IP профилю. Совпадение или различие IP не устанавливает локальный
        маршрут. Отсутствие утечек native DNS, IPv6 и других приложений этим probe не измеряется.
      </HintBox>

      <HintBox kind="info" title="HY2/UDP зависит от текущей сети">
        Доступность UDP и работа HY2 требуют отдельных результатов для текущих канала, сети
        и приложения. Старый live-результат или работа другого VPN не подтверждают этот запуск.
      </HintBox>
      <HintBox kind="info" title="Допуск TUN требует отдельного подтверждения">
        Общий допуск TUN пока закрыт. Original-helper/WFP протокол реализуется отдельно;
        наличие фильтров, запущенного процесса или совпавшего IP не означает готовый защищённый путь.
      </HintBox>
    </details>
  </div>
</template>

<style scoped>
.view {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.probe-flag {
  color: var(--accent);
  font-size: 12.5px;
}

.scope-note { margin: 0; color: var(--muted); line-height: 1.6; }
.check-limits summary { cursor: pointer; padding: 8px 0; color: var(--text-soft); }
.check-limits :deep(.hint) { margin-top: 10px; }
.view, .test-list { min-width: 0; }

.cards {
  display: flex;
  flex-direction: column;
  gap: 0;
  border: 1px solid var(--border);
  border-radius: 8px;
  overflow: hidden;
}
.test-list :deep(.card) { border: 0; border-bottom: 1px solid var(--border); border-radius: 0; box-shadow: none; }
.test-list :deep(.card:last-child) { border-bottom: 0; }
</style>
