import { onBeforeUnmount, readonly, ref } from 'vue'
import type { SecretKind, SecretMeta } from '../api/contract'
import { api } from '../api/backend'

export type SecretActionResult<T> = { ok: true; value: T; refreshed: boolean } | { ok: false; message: string }
const items = ref<SecretMeta[] | null>(null)
const loading = ref(false)
const loadError = ref('')
const receivedAt = ref<string | null>(null)
const actionBusy = ref(false)
const lastOutcome = ref('')
let users = 0
let generation = 0
let revision = 0
let pending: Promise<boolean> | null = null

function refresh(): Promise<boolean> {
  if (!users) return Promise.resolve(false)
  if (pending) return pending
  const owner = generation, readRevision = revision
  loading.value = true
  loadError.value = ''
  pending = Promise.resolve().then(() => api.listSecrets()).then(result => {
    if (!users || owner !== generation || readRevision !== revision) return false
    items.value = result
    receivedAt.value = result === null ? null : new Date().toISOString()
    if (result === null) loadError.value = 'Метаданные недоступны. Откройте приложение Windows.'
    return result !== null
  }, () => {
    if (!users || owner !== generation || readRevision !== revision) return false
    items.value = null
    receivedAt.value = null
    loadError.value = 'Не удалось получить метаданные. Повторите загрузку; пустота хранилища не подтверждена.'
    return false
  }).finally(() => {
    pending = null
    if (users && (owner !== generation || readRevision !== revision)) void refresh()
    else loading.value = false
  })
  return pending
}

/** Native operations cannot be cancelled by Vue; no raw errors or values in refs/toasts. */
async function perform<T>(operation: () => Promise<T>, success: string, failure: string, waitForList = true): Promise<SecretActionResult<T>> {
  if (!users || actionBusy.value) return { ok: false, message: 'Другая операция ещё не завершена. Дождитесь её результата.' }
  const owner = generation
  actionBusy.value = true
  lastOutcome.value = ''
  revision++
  try {
    const value = await operation()
    revision++
    let refreshed = false
    if (users) {
      if (waitForList) refreshed = await refresh()
      else void refresh()
    }
    if (users && owner === generation) lastOutcome.value = success + (refreshed ? ' Метаданные перечитаны.' : ' Актуальные метаданные ещё не подтверждены.')
    return { ok: true, value, refreshed }
  } catch {
    revision++
    if (users) void refresh()
    if (users && owner === generation) lastOutcome.value = failure
    return { ok: false, message: failure }
  } finally { actionBusy.value = false }
}

export function useSecretsStore() {
  users++
  onBeforeUnmount(() => {
    users = Math.max(0, users - 1)
    if (!users) {
      generation++
      revision++
      items.value = null
      receivedAt.value = null
      lastOutcome.value = ''
      loadError.value = ''
    }
  })
  return {
    items: readonly(items), loading: readonly(loading), loadError: readonly(loadError), receivedAt: readonly(receivedAt),
    actionBusy: readonly(actionBusy), lastOutcome: readonly(lastOutcome), refresh,
    save: (kind: SecretKind, title: string, value: string, hint: string) => perform(() => api.addSecret(kind, title, value, hint), 'Сохранение подтверждено приложением.', 'Сохранение не подтверждено. Перечитайте метаданные перед повторной попыткой.'),
    setValue: (id: string, value: string) => perform(() => api.saveSecret(id, value), 'Запись значения подтверждена приложением.', 'Запись не подтверждена. Перечитайте метаданные перед повторной попыткой.'),
    remove: (id: string) => perform(() => api.deleteSecret(id), 'Удаление подтверждено приложением.', 'Удаление не подтверждено. Перечитайте метаданные перед повторной попыткой.'),
    verify: (id: string) => perform(() => api.verifySecret(id), 'Ответ локальной проверки получен.', 'Положительный результат проверки не получен. Сверьте формат и обновлённое состояние слота.'),
    runTest: (id: string) => perform(() => api.testSecret(id), 'Отчёт проверки получен.', 'Отчёт не получен. Причина сетевого или серверного отказа не установлена.'),
    // Return plaintext to the original caller promptly; do not hold it for a list reload.
    reveal: (id: string) => perform(() => api.revealSecret(id), 'Ответ на выдачу значения получен; копирование является отдельным действием.', 'Выдача значения не подтверждена.', false),
  }
}