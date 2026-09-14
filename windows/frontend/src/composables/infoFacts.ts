/**
 * Инфо-факты (сеть, сторож автозащиты): общий поллинг-синглтон поверх
 * биндингов GetNetworkFacts / GetFailoverStatus. null = источник недоступен
 * (честное «нет данных», AGENTS.md §3.6). Один таймер на всех потребителей:
 * PowerShell-опрос дорогой, дублировать его для каждой страницы нельзя.
 */
import { onUnmounted, ref } from 'vue'
import { api, isBuildPhase } from '@/api/backend'
import type { FailoverStatus, NetworkFacts } from '@/api/contract'

const facts = ref<NetworkFacts | null>(null)
const failover = ref<FailoverStatus | null>(null)

let timer: number | undefined
let users = 0

async function refresh(): Promise<void> {
  if (isBuildPhase()) {
    facts.value = null
    failover.value = null
    return
  }
  try {
    facts.value = await api.getNetworkFacts()
  } catch {
    facts.value = null
  }
  try {
    failover.value = await api.getFailoverStatus()
  } catch {
    failover.value = null
  }
}

function acquire(): void {
  users++
  if (users === 1) {
    void refresh()
    timer = window.setInterval(() => void refresh(), 15000)
  }
}

function release(): void {
  users = Math.max(0, users - 1)
  if (users === 0 && timer !== undefined) {
    window.clearInterval(timer)
    timer = undefined
  }
}

/** Использовать внутри setup(): счётчик потребителей снимает поллинг при размонтировании. */
export function useInfoFacts() {
  acquire()
  onUnmounted(release)
  return { facts, failover, refresh }
}
