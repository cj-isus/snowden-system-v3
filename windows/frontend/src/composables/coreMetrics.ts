import { computed, ref, watch } from 'vue'
import { api } from '../api/backend'
import { diagnosticContextKey } from '../api/labels'
import { refreshState, useBackendState } from './backendState'
import { usePolling } from './polling'
import { createCoreTrafficHistory, type CoreTrafficInterval } from './trafficHistory'

/** Component-owned telemetry. Public context only denies reuse, never grants traffic. */
export function useCoreMetrics() {
  const root = useBackendState()
  const scope = computed(() => {
    const state = root.state.value
    const context = state?.diagnosticContext
    if (!root.stateLoaded.value || !context || !context.intentCurrent || context.closing || !context.ready ||
        state?.state !== 'running' || context.lifecycle !== 'running' || context.channel !== state.activeChannelId || context.mode !== state.proxyMode ||
        (context.path !== '0' && !context.pathCurrent) || root.isBusy.value || root.switching.value || root.tunBusy.value) return null
    return diagnosticContextKey(context)
  })
  const polling = usePolling(async request => {
    const original = scope.value
    if (original === null) return null
    const value = await api.getMetrics()
    if (!request.current() || original !== scope.value) return null
    if (value?.available && diagnosticContextKey(value.context) !== original) {
      // Reuse the root's single state request; never retag this metrics reply.
      void refreshState()
      return { ...value, available: false, note: 'Контекст снимка изменился. Ожидаем новые данные.',
        sessionUp: null, sessionDown: null, rateUp: null, rateDown: null, connections: [], at: '' }
    }
    return value
  }, 2000, { scope: () => scope.value, pauseWhenHidden: true })

  const history = createCoreTrafficHistory()
  const interval = ref<CoreTrafficInterval | null>(null)
  watch(polling.data, value => { interval.value = history.observe(value) }, { flush: 'sync' })
  const note = computed(() => scope.value === null ?
    'Счётчики ядра доступны после запуска в текущем контексте. Во время смены состояния прежний снимок скрыт.' :
    polling.error.value || 'Снимок ядра не получен. Повторите обновление.')
  return { ...polling, interval, note }
}
