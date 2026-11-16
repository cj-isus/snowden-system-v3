/**
 * Инфо-факты (сеть, сторож автозащиты): общий поллинг-синглтон поверх
 * биндингов GetNetworkFacts / GetFailoverStatus. null = источник недоступен
 * (честное «нет данных», AGENTS.md §3.6). По одному исходному запросу/таймеру
 * на источник для всех потребителей; дорогой OS-опрос не задерживает статус.
 */
import { onUnmounted, ref } from 'vue'
import { api, isBuildPhase } from '@/api/backend'
import type { FailoverStatus, NetworkFacts } from '@/api/contract'
import { createPoller, observePageVisibility, pageObservationVisible } from './polling'

const facts = ref<NetworkFacts | null>(null)
const failover = ref<FailoverStatus | null>(null)
const loading = ref(false)
const factsLoading = ref(false)
const failoverLoading = ref(false)
const factsError = ref('')
const failoverError = ref('')
const factsReceivedAt = ref<string | null>(null)
const failoverReceivedAt = ref<string | null>(null)

let users = 0
let networkUsers = 0
let failoverUsers = 0
let stopVisibility = () => {}
const networkPoller = createPoller(async request => {
  if (!request.current() || !hasNetworkInterest()) throw new Error('Observation interest ended')
  return isBuildPhase() ? null : api.getNetworkFacts()
}, value => {
  if (!hasNetworkInterest()) { synchronize(); return }
  factsReceivedAt.value = value === null ? null : new Date().toISOString()
  factsError.value = value !== null ? '' : isBuildPhase() ? 'Наблюдения доступны в приложении Windows.' : 'Сетевой снимок не получен. Обновите наблюдения.'
  facts.value = value
}, 10000, () => {
  if (!hasNetworkInterest()) { synchronize(); return }
  facts.value = null
  factsReceivedAt.value = null
  factsError.value = 'Сетевой снимок не получен. Повторите обновление.'
}, pending => {
  factsLoading.value = pending
  loading.value = pending || failoverLoading.value
})
const failoverPoller = createPoller(async request => {
  if (!request.current() || !hasFailoverInterest()) throw new Error('Observation interest ended')
  return isBuildPhase() ? null : api.getFailoverStatus()
}, value => {
  if (!hasFailoverInterest()) { synchronize(); return }
  failoverReceivedAt.value = value === null ? null : new Date().toISOString()
  failoverError.value = value !== null ? '' : isBuildPhase() ? 'Состояние адаптации доступно в приложении Windows.' : 'Снимок адаптации не получен. Обновите наблюдения.'
  failover.value = value
}, 10000, () => {
  if (!hasFailoverInterest()) { synchronize(); return }
  failover.value = null
  failoverReceivedAt.value = null
  failoverError.value = 'Снимок адаптации не получен. Повторите обновление.'
}, pending => {
  failoverLoading.value = pending
  loading.value = factsLoading.value || pending
})

// Legacy aggregate completion is retained; publication/loading/receipt are
// independent. A page needing one source uses its specific refresh below.
function refresh(network: boolean, adaptation: boolean): Promise<void> {
  return Promise.all([network ? refreshFacts() : Promise.resolve(), adaptation ? refreshFailover() : Promise.resolve()]).then(() => {})
}

function hasNetworkInterest(): boolean { return networkUsers > 0 && pageObservationVisible() }
function hasFailoverInterest(): boolean { return failoverUsers > 0 && pageObservationVisible() }
function refreshFacts(): Promise<void> {
  return hasNetworkInterest() ? networkPoller.refresh() : Promise.resolve()
}
function refreshFailover(): Promise<void> {
  return hasFailoverInterest() ? failoverPoller.refresh() : Promise.resolve()
}

function synchronize(): void {
  // Stop/clear only a source whose last consumer or visible interest ended.
  // Its original pending RPC remains owned. Unrelated source interest is stable.
  if (hasNetworkInterest()) {
    if (facts.value === null && !factsError.value) factsLoading.value = true
    networkPoller.start()
  } else {
    networkPoller.stop()
    facts.value = null
    factsReceivedAt.value = null
    factsError.value = ''
    factsLoading.value = false
  }
  if (hasFailoverInterest()) {
    if (failover.value === null && !failoverError.value) failoverLoading.value = true
    failoverPoller.start()
  } else {
    failoverPoller.stop()
    failover.value = null
    failoverReceivedAt.value = null
    failoverError.value = ''
    failoverLoading.value = false
  }
  loading.value = factsLoading.value || failoverLoading.value
}

function acquire(network: boolean, adaptation: boolean): void {
  if (!network && !adaptation) return
  users++
  if (network) networkUsers++
  if (adaptation) failoverUsers++
  if (users === 1) {
    stopVisibility = observePageVisibility(synchronize)
  }
  synchronize() // current visibility, not a claim about the page's load history
}

function release(network: boolean, adaptation: boolean): void {
  if (!network && !adaptation) return
  users = Math.max(0, users - 1)
  if (network) networkUsers = Math.max(0, networkUsers - 1)
  if (adaptation) failoverUsers = Math.max(0, failoverUsers - 1)
  if (users === 0) {
    stopVisibility()
    stopVisibility = () => {}
  }
  synchronize()
}

type InfoFactsSources = { facts?: boolean; failover?: boolean }

/** Inside setup(). The optional source mask is captured once; default keeps both. */
export function useInfoFacts(sources: InfoFactsSources = { facts: true, failover: true }) {
  const network = sources.facts === true, adaptation = sources.failover === true
  let released = false
  acquire(network, adaptation)
  onUnmounted(() => { if (!released) { released = true; release(network, adaptation) } })
  const interested = (read: () => Promise<void>, requested: boolean) => () => released || !requested ? Promise.resolve() : read()
  return { facts, failover, loading, factsLoading, failoverLoading, factsError, failoverError, factsReceivedAt, failoverReceivedAt,
    refresh: interested(() => refresh(network, adaptation), network || adaptation),
    refreshFacts: interested(refreshFacts, network), refreshFailover: interested(refreshFailover, adaptation) }
}
