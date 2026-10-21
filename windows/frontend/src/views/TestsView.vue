<script setup lang="ts">
import PageHeading from '../components/PageHeading.vue'
/**
 * Раздел «Тестирование». Кнопка «Запустить все» блокируется, пока идёт
 * хотя бы один тест. Результаты приходят только из backend (null = недоступен).
 */
import { computed, onMounted } from 'vue'
import TestCard from '../components/TestCard.vue'
import HintBox from '../components/HintBox.vue'
import Icon from '../components/Icon.vue'
import { useDiagnostics, refreshTests } from '../composables/diagnostics'
import { useBackendState } from '../composables/backendState'
import { toast } from '../composables/toast'

const { tests, runningId, busy: anyRunning, runTest, runAll } = useDiagnostics()
const { state } = useBackendState()


const canRunAll = computed(() => !anyRunning.value && Array.isArray(tests.value) && tests.value.length > 0)

function onRunAll(): void { void runAll() }

function onRefresh(): void {
  void refreshTests()
  toast('info', 'Список тестов перечитан из backend')
}

onMounted(() => {
  void refreshTests()
})
</script>

<template>
  <div class="view">
    <PageHeading title="Диагностика пути" description="Запустите нужную проверку и изучите её результат. Отсутствие результата не означает успех." ></PageHeading>

    <div class="actions-row">
      <button class="btn primary" :disabled="!canRunAll" @click="onRunAll" title="Выполнить все тесты по очереди">
        <Icon name="play" :size="14" />
        Запустить все
      </button>
      <button class="btn" :disabled="anyRunning" @click="onRefresh">
        <Icon name="refresh" :size="14" />
        Обновить
      </button>
      <span v-if="state?.probeRunning" class="probe-flag">probe выполняется…</span>
    </div>

    <div v-if="tests === null" class="empty-block">
      <Icon name="warn" :size="18" />
      <p>
        Список тестов недоступен: нет ответа от приложения Windows. Откройте приложение и повторите загрузку.
      </p>
    </div>

    <div v-else-if="tests.length === 0" class="empty-block">
      <Icon name="info" :size="18" />
      <p>Backend не вернул ни одного теста для текущего конфига.</p>
    </div>

    <div v-else class="cards test-list">
      <TestCard
        v-for="t in tests"
        :key="t.id"
        :test="t"
        :running="runningId === t.id"
        :disabled="anyRunning"
        @run="(id) => void runTest(id)"
      />
    </div>

    <HintBox kind="info" title="Что проверяет probe (и почему этого мало)">
      Probe подтверждает защищённый путь: DNS через туннель, две HTTPS-цели с маркером egress,
      совпадение с ожидаемым IP и отсутствие прямых утечек. Результат «пройдено» означает ровно
      это подтверждение — не «обходит любые блокировки».
    </HintBox>

    <HintBox kind="info" title="HY2/UDP зависит от текущей сети">
      Доступность UDP измеряется при каждом запуске. Канал HY2 ранее подтверждён live, но оператор
      может временно фильтровать QUIC/UDP; такой результат относится к текущей сетевой эпохе.
    </HintBox>
    <HintBox kind="info" title="TUN защищён WFP policy">
      Перед запуском TUN приложение включает системный WFP kill switch, проверяет фильтры,
      интерфейс snowden0 и egress внешнего процесса. Любой провал останавливает запуск fail-closed.
    </HintBox>
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
