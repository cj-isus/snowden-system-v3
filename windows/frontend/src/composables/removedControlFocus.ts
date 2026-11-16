import { onBeforeUnmount, onBeforeUpdate, onUpdated, type Ref } from 'vue'

/** Recover only a removed control from this mounted page. The selector names
 * an existing, persistent focus target inside the same root, not another page. */
export function useRemovedControlFocus(root: Readonly<Ref<HTMLElement | null>>, selector: string) {
  let previous: HTMLElement | null = null
  function focusFallback(): void {
    const owner = root.value
    if (!owner?.isConnected || document.querySelector('dialog[open]')) return
    const target = owner.querySelector<HTMLElement>(selector)
    if (target?.isConnected && owner.contains(target)) target.focus({ preventScroll: true })
  }
  onBeforeUpdate(() => {
    const current = document.activeElement
    previous = current instanceof HTMLElement && root.value?.contains(current) ? current : null
  })
  onUpdated(() => {
    const departed = previous
    previous = null
    if (departed && !departed.isConnected && document.activeElement === document.body) focusFallback()
  })
  onBeforeUnmount(() => { previous = null })
  // Slot updates need not run this parent's hooks. Use explicitly before an
  // owned action removes its focused conditional control, such as Settings Stop.
  function focusBeforeRemoval(): void {
    if (root.value?.contains(document.activeElement)) focusFallback()
  }
  return { focusBeforeRemoval }
}
