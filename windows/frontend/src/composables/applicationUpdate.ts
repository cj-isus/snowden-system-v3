import { computed, onBeforeUnmount, readonly, ref, shallowRef } from 'vue'
import { api, isBuildPhase } from '../api/backend'
import type { UpdatePreview } from '../api/contract'
import { applyUpdatePackage, updateActionReason, updateCommandPending, updateMessage } from './backendState'

// One original native picker/check/apply across disclosure and root lifetimes.
// Retiring display interest never cancels Wails or frees this original slot.
const pending = shallowRef<{ owner: symbol; phase: 'picker' | 'check' | 'apply' } | null>(null)
const preview = shallowRef<UpdatePreview | null>(null)
const problem = ref('')
// Finite display metadata survives disclosure/root disposal; never retain a
// directory, approval token, package facts or exception from an old request.
const settledMessage = ref('')
const changed = shallowRef(Symbol()) // refresh demand only, not an apply/success receipt
let root = Symbol()
const active = ref(false)
const viewer = shallowRef<symbol | null>(null)

export function activateApplicationUpdate(): void { active.value = true }
export function disposeApplicationUpdate(): void {
  active.value = false
  root = Symbol()
  viewer.value = null
  preview.value = null
  problem.value = ''
}

export function useApplicationUpdate() {
  const owner = Symbol(), scope = root
  viewer.value = owner
  preview.value = null
  problem.value = ''
  const current = () => active.value && scope === root && viewer.value === owner
  onBeforeUnmount(() => {
    if (viewer.value === owner) { viewer.value = null; preview.value = null; problem.value = '' }
  })
  const busy = computed(() => pending.value !== null || updateCommandPending.value)
  const disabledReason = computed(() => !current() ? 'Панель обновления закрыта.' : isBuildPhase() ? 'Откройте приложение Windows.' : updateActionReason.value)
  const canChoose = computed(() => current() && !busy.value && disabledReason.value === '')
  const canApply = computed(() => canChoose.value && preview.value?.ok === true && preview.value.approval !== null)
  const phase = computed(() => pending.value?.phase ?? null)

  async function choose(): Promise<void> {
    if (!canChoose.value) return
    const request = { owner, phase: 'picker' as 'picker' | 'check' | 'apply' }
    pending.value = request
    preview.value = null
    problem.value = ''
    settledMessage.value = ''
    let checking = false
    try {
      const directory = await api.pickUpdateDir()
      if (!current()) { settledMessage.value = 'Выбор завершился после закрытия панели. Проверка этого пакета не запрашивалась.'; return }
      if (directory === '') { settledMessage.value = 'Каталог не выбран. Проверка и установка по этому выбору не запрашивались.'; return }
      pending.value = { ...request, phase: 'check' }
      if (!current()) { settledMessage.value = 'Проверка пакета не запрашивалась: панель уже закрыта.'; return }
      checking = true
      const result = await api.checkUpdate(directory, { current })
      settledMessage.value = current()
        ? 'Ответ на проверку пакета получен. Установка не запрашивалась; история доверия могла измениться.'
        : 'Ответ на прежнюю проверку пришёл после закрытия панели. Превью не принято; история доверия уже могла измениться.'
      if (current()) preview.value = result
    } catch {
      settledMessage.value = checking
        ? 'Проверка пакета не подтверждена. История доверия уже могла измениться; перечитайте профиль и версию перед повтором.'
        : 'Выбор каталога не подтверждён. Проверка и установка пакета этим действием не запрашивались.'
      if (current()) problem.value = 'Пакет не получил подтверждённого превью. Сверьте источник и обновите сведения о версии приложения.'
    } finally {
      if (pending.value?.owner === owner) pending.value = null
      changed.value = Symbol()
    }
  }

  async function apply(): Promise<void> {
    if (!canApply.value) return
    const approval = preview.value?.approval
    if (!approval) return
    preview.value = null // one use, including a failed or superseded request
    problem.value = ''
    settledMessage.value = ''
    pending.value = { owner, phase: 'apply' }
    try {
      const outcome = await applyUpdatePackage(approval.token, current)
      const messages = {
        not_sent: 'Запрос установки не передан приложению. Подтверждение использовано интерфейсом; выберите и проверьте пакет заново.',
        before_install: 'Приложение вернуло отказ до входа в установку. Проверенная история доставки уже могла измениться.',
        next_process_unconfirmed: 'Ответ на применение получен. Установка и запуск нового процесса ещё не подтверждены; проверьте его версию.',
        unconfirmed: 'Результат применения не подтверждён. Не считайте это отменой или откатом; проверьте сведения о процессе и восстановлении.',
      }
      settledMessage.value = messages[outcome]
    } catch {
      settledMessage.value = 'Результат применения не подтверждён. Проверьте сведения о процессе и восстановлении перед новой попыткой.'
    } finally {
      if (pending.value?.owner === owner) pending.value = null
      changed.value = Symbol()
    }
  }

  return { preview: readonly(preview), problem: readonly(problem), settledMessage: readonly(settledMessage), changed: readonly(changed), busy, phase, disabledReason,
    canChoose, canApply, message: updateMessage, choose, apply }
}
