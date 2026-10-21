/**
 * Инфо-факты (сеть, сторож автозащиты): общий поллинг-синглтон поверх
 * биндингов GetNetworkFacts / GetFailoverStatus. null = источник недоступен
 * (честное «нет данных», AGENTS.md §3.6). Один таймер на всех потребителей:
 * PowerShell-опрос дорогой, дублировать его для каждой страницы нельзя.
 */
import { onUnmounted, ref } from 'vue'
import { api, isBuildPhase } from '@/api/backend'
import type { FailoverStatus, NetworkFacts } from '@/api/contract'
import { createPoller } from './polling'

const facts = ref<NetworkFacts | null>(null)
const failover = ref<FailoverStatus | null>(null)

let users = 0
const poller = createPoller(async () => {
  if (isBuildPhase()) {
    return { facts: null, failover: null }
  }
  const [network, watcher] = await Promise.allSettled([api.getNetworkFacts(), api.getFailoverStatus()])
  return {
    facts: network.status === 'fulfilled' ? network.value : null,
    failover: watcher.status === 'fulfilled' ? watcher.value : null,
  }
}, value => {
  facts.value = value.facts
  failover.value = value.failover
}, 10000)

function acquire(): void {
  users++
  if (users === 1) {
    poller.start()
  }
}

function release(): void {
  users = Math.max(0, users - 1)
  if (users === 0) poller.stop()
}

/** Использовать внутри setup(): счётчик потребителей снимает поллинг при размонтировании. */
export function useInfoFacts() {
  acquire()
  onUnmounted(release)
  return { facts, failover, refresh: poller.refresh }
}
