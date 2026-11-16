import { computed, onBeforeUnmount, readonly, ref, shallowRef } from 'vue'

import { profileActionFailure, profileActionTitle } from '../api/failureMessages'
import type { ProfileActionKind, ProfileActionOutcome } from '../api/failureMessages'

type OriginalAction = { owner: symbol; token: symbol; kind: ProfileActionKind }
type Settlement = Readonly<{ kind: ProfileActionKind; outcome: ProfileActionOutcome }>

// Metadata only: no passwords, transport, endpoint, preview tokens or native DTOs.
// A received reply is display metadata, never current profile/native authority.
// A component/root unmount retires interest; only the original callback's actual
// finally releases admission. This is not a backend/native retirement receipt.
const pending = shallowRef<OriginalAction | null>(null)
const selected = shallowRef(Symbol())
const changed = shallowRef(Symbol())
const active = ref(false)
const settled = shallowRef<Settlement | null>(null)
const attention = shallowRef<Settlement | null>(null)
let root = Symbol()

export function activateProfileActions(): void { active.value = true }
export function disposeProfileActions(): void {
  active.value = false
  root = Symbol()
  selected.value = Symbol()
}

const busy = computed(() => pending.value !== null)
const settlementLabel = computed(() => {
  const value = settled.value
  if (!value) return ''
  const title = profileActionTitle(value.kind)
  switch (value.outcome) {
    case 'not_sent': return `${title}: запрос на это действие не отправлен приложению.`
    case 'cancelled': return `${title}: диалог закрыт без выбора. Это не отменяет прежние операции.`
    case 'received': return `${title}: ответ получен. Текущий профиль и защиту соединения сверяйте отдельно.`
    case 'unread': return `${title}: ответ пришёл после изменения экрана или не совпал с выбранными данными. Результат не принят интерфейсом.`
    case 'unconfirmed': return `${title}: результат не подтверждён. Это не означает отмену или откат.`
  }
})
const attentionLabel = computed(() => {
  const value = attention.value
  if (!value) return ''
  const title = profileActionTitle(value.kind)
  return value.kind === 'transfer_save'
    ? `${title}: файл уже мог появиться или измениться. Проверьте выбранное место перед повтором.`
    : `${title}: ранее получен неподтверждённый результат. Профиль или история доверия уже могли измениться; перечитайте текущие сведения перед повтором.`
})
const pendingLabel = computed(() => {
  switch (pending.value?.kind) {
    case 'profile_preview': return 'Ожидается проверка публичного профиля…'
    case 'profile_apply': return 'Ожидается ответ на применение профиля…'
    case 'policy_preview': return 'Ожидается проверка списка отзывов…'
    case 'policy_apply': return 'Ожидается ответ на применение списка отзывов…'
    case 'transfer_preview': return 'Ожидается выбор или проверка переноса…'
    case 'transfer_apply': return 'Ожидается ответ на импорт переноса…'
    case 'transfer_export': return 'Ожидается создание переноса…'
    case 'transfer_save': return 'Ожидается сохранение переноса…'
    default: return ''
  }
})

export function useProfileActions() {
  const owner = Symbol(), scope = root
  let mounted = true
  const current = () => mounted && active.value && scope === root
  const ownsPending = computed(() => pending.value?.owner === owner)
  const blocked = computed(() => busy.value && !ownsPending.value)
  onBeforeUnmount(() => { mounted = false })

  function begin(kind: ProfileActionKind) {
    if (!current() || pending.value !== null) return null
    const original: OriginalAction = { owner, token: Symbol(), kind }
    pending.value = original // reserve before any callback or reactive selection invalidation
    selected.value = original.token
    settled.value = null // the separate negative reminder survives a new request
    let returned = false
    let outcome: ProfileActionOutcome = 'unconfirmed'
    return {
      token: original.token,
      current: () => current() && selected.value === original.token,
      received: (accepted: boolean) => { if (!returned) outcome = accepted ? 'received' : 'unread' },
      cancelled: () => { if (!returned) outcome = 'cancelled' },
      failed: (error: unknown) => {
        const failure = profileActionFailure(kind, error)
        if (!returned) outcome = failure.outcome
        return failure.message
      },
      finish: () => {
        if (returned) return
        returned = true
        if (pending.value === original) {
          const late = !current() || selected.value !== original.token
          const result: Settlement = { kind, outcome: late && (outcome === 'received' || outcome === 'cancelled') ? 'unread' : outcome }
          settled.value = result
          if (result.outcome === 'unconfirmed' || result.outcome === 'unread') attention.value = result
          pending.value = null
        }
        changed.value = Symbol() // only demand fresh facts; never adopt an old result
      },
    }
  }

  return { begin, current, busy, blocked, ownsPending, pendingLabel,
    settlementLabel, attentionLabel, dismissAttention: () => { if (current()) attention.value = null }, // display only; never native clearance
    revision: readonly(selected), changed: readonly(changed) }
}
