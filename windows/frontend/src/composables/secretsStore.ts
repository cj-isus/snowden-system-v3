/**
 * Секреты-хранилище (список + операции поверх api.*). Значения не держатся
 * в сторе: из backend приходят только метаданные (fingerprint, статусы).
 */
import { ref } from 'vue'
import type { SecretMeta, SecretTestReport, SecretVerifyResult } from '@/api/contract'
import { api } from '@/api/backend'
import { toast } from './toast'

const items = ref<SecretMeta[] | null>(null) // null = источник недоступен
const loading = ref(false)

async function refresh(): Promise<void> {
  loading.value = true
  try {
    items.value = await api.listSecrets()
  } catch (e) {
    toast('error', e instanceof Error ? e.message : String(e))
  } finally {
    loading.value = false
  }
}

/** Создать/обновить секрет по типу (используется формой «Новый секрет»). */
async function save(kind: string, title: string, value: string, hint: string): Promise<boolean> {
  try {
    await api.addSecret(kind, title, value, hint)
    toast('ok', 'Секрет сохранён локально (значение не отображается)')
    await refresh()
    return true
  } catch (e) {
    toast('error', e instanceof Error ? e.message : String(e))
    return false
  }
}

/** Задать значение существующего слота (для предзаполненных карточек). */
async function setValue(id: string, value: string): Promise<boolean> {
  try {
    await api.saveSecret(id, value)
    toast('ok', 'Значение сохранено (шифрование DPAPI, статус проверки сброшен)')
    await refresh()
    return true
  } catch (e) {
    toast('error', e instanceof Error ? e.message : String(e))
    return false
  }
}

async function remove(id: string, title: string): Promise<boolean> {
  try {
    await api.deleteSecret(id)
    toast('info', `Секрет «${title}» удалён из локального хранилища`)
    await refresh()
    return true
  } catch (e) {
    toast('error', e instanceof Error ? e.message : String(e))
    return false
  }
}

async function verify(id: string): Promise<SecretVerifyResult | null> {
  try {
    const res = await api.verifySecret(id)
    await refresh()
    return res
  } catch (e) {
    // Ошибка валидации — нормальный исход: backend вернул обновлённую карточку.
    try {
      await refresh()
    } catch {
      /* список важнее тоста */
    }
    toast('error', e instanceof Error ? e.message : String(e))
    return null
  }
}

/** Живой тест значения секрета (сеть/сервер). Ошибку возвращает вызывающему
 * код карточки: отчёт с шагами показывается прямо в карточке. */
async function runTest(id: string): Promise<SecretTestReport> {
  return api.testSecret(id)
}

async function reveal(id: string): Promise<string | null> {
  try {
    const res = await api.revealSecret(id)
    if (res.value === null) {
      toast('warn', res.error || 'Раскрытие недоступно')
    }
    return res.value
  } catch (e) {
    toast('error', e instanceof Error ? e.message : String(e))
    return null
  }
}

export function useSecretsStore() {
  return { items, loading, refresh, save, setValue, remove, verify, reveal, runTest }
}
