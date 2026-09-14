/**
 * Диагностика для раздела «Тестирование»: список тестов и их запуск.
 * Результаты приходят только из backend (null = источник недоступен),
 * UI не выдумывает статусы (AGENTS.md §3.6).
 */
import { ref } from 'vue'
import type { TestResult } from '@/api/contract'
import { api } from '@/api/backend'
import { toast } from './toast'
import { uiLog } from './uiLog'

const tests = ref<TestResult[] | null>(null) // null = источник недоступен
const runningId = ref<string | null>(null)

/** Перезагрузка списка тестов из backend. */
export async function refreshTests(): Promise<void> {
  try {
    tests.value = await api.listTests()
  } catch (e) {
    toast('error', e instanceof Error ? e.message : String(e))
  }
}

/** Запуск одного теста по id. Повторный запуск заблокирован до завершения. */
export async function runTest(id: string): Promise<void> {
  if (runningId.value) return
  runningId.value = id
  uiLog('info', `Тест: запуск ${id}`)
  try {
    const res = await api.runTest(id)
    if (res.ok === true) {
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
    runningId.value = null
    await refreshTests()
  }
}

export function useDiagnostics() {
  return { tests, runningId, runTest, refreshTests }
}
