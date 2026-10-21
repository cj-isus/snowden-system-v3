/**
 * Диагностика для раздела «Тестирование»: список тестов и их запуск.
 * Результаты приходят только из backend (null = источник недоступен),
 * UI не выдумывает статусы (AGENTS.md §3.6).
 */
import { computed, ref } from 'vue'
import type { TestResult } from '@/api/contract'
import { api } from '@/api/backend'
import { isSkippedTest } from '@/api/labels'
import { toast } from './toast'
import { uiLog } from './uiLog'

const tests = ref<TestResult[] | null>(null) // null = источник недоступен
const runningId = ref<string | null>(null)
const batchRunning = ref(false)
const busy = computed(() => batchRunning.value || runningId.value !== null)
let refreshRevision = 0

/** Перезагрузка списка тестов из backend. */
export async function refreshTests(): Promise<void> {
  const revision = ++refreshRevision
  try {
    const result = await api.listTests()
    if (revision === refreshRevision) tests.value = result
  } catch (e) {
    if (revision !== refreshRevision) return
    tests.value = null
    toast('error', e instanceof Error ? e.message : String(e))
  }
}

/** Запуск одного теста по id. Повторный запуск заблокирован до завершения. */
export async function runTest(id: string): Promise<void> {
  if (busy.value) return
  await execute(id)
}

export async function runAll(): Promise<void> {
  if (busy.value || !tests.value?.length) return
  const ids = tests.value.map(test => test.id)
  batchRunning.value = true
  try { for (const id of ids) await execute(id) }
  finally { batchRunning.value = false }
}

async function execute(id: string): Promise<void> {
  runningId.value = id
  uiLog('info', `Тест: запуск ${id}`)
  try {
    const res = await api.runTest(id)
    if (isSkippedTest(res)) {
      toast('info', `Тест «${id}»: пропущен — ${res.detail}`)
    } else if (res.ok === true) {
      toast('ok', `Тест «${id}»: пройден`)
    } else if (res.ok === false) {
      toast('error', `Тест «${id}»: не пройден — ${res.detail}`)
    } else {
      toast('warn', `Тест «${id}»: результат недоступен — ${res.detail}`)
    }
    uiLog(res.ok === true ? 'info' : 'warn', `Тест ${id}: ${res.detail}`)
  } catch (e) {
    toast('error', e instanceof Error ? e.message : String(e))
  } finally {
    await refreshTests()
    runningId.value = null
  }
}

export function useDiagnostics() {
  return { tests, runningId, busy, runTest, runAll, refreshTests }
}
