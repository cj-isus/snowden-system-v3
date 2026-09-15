<script setup lang="ts">
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

const { tests, runningId, runTest } = useDiagnostics()
const { state } = useBackendState()

const anyRunning = computed(() => runningId.value !== null)

const canRunAll = computed(() => !anyRunning.value && Array.isArray(tests.value) && tests.value.length > 0)

function onRunAll(): void {
  if (!Array.isArray(tests.value)) return
  void (async () => {
    for (const t of tests.value as NonNullable<typeof tests.value>) {
      await runTest(t.id)
    }
  })()
}

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
    <header class="head">
      <h2>Тестирование</h2>
      <p class="sub">
        Каждая проверка возвращает факт («пройдено/не пройдено/нет данных») — без «зелёных галочек наугад».
      </p>
    </header>

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
        Список тестов недоступен: backend bindings ещё не сгенерированы. Это честное «нет данных» —
        fake-результаты UI не показывает.
      </p>
    </div>

    <div v-else-if="tests.length === 0" class="empty-block">
      <Icon name="info" :size="18" />
      <p>Backend не вернул ни одного теста для текущего конфига.</p>
    </div>

    <div v-else class="cards">
      <TestCard
        v-for="t in tests"
        :key="t.id"
        :test="t"
        :running="runningId === t.id"
        @run="(id) => void runTest(id)"
      />
    </div>

    <HintBox kind="info" title="Что проверяет probe (и почему этого мало)">
      Probe подтверждает защищённый путь: DNS через туннель, две HTTPS-цели с маркером egress,
      совпадение с ожидаемым IP и отсутствие прямых утечек. Результат «пройдено» означает ровно
      это подтверждение — не «обходит любые блокировки».
    </HintBox>

    <HintBox kind="warn" title="HY2/UDP в сети агента">
      UDP в этом окружении фильтруется (QUIC к 8.8.8.8:443 молчит), поэтому тест HY2 здесь
      закономерно не пройдёт. Проверяйте его из сети без UDP-фильтра (домашний Wi-Fi) — это
      ограничение среды, а не дефект канала.
    </HintBox>
    <HintBox kind="warn" title="TUN требует прав администратора">
      TUN-режим поднимает виртуальный интерфейс и запускается только из повышенного процесса.
      Из не-админ сессии ожидаемо отклоняется — это контракт, а не сбой.
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
  gap: 12px;
}
</style>
