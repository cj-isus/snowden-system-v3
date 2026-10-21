import { afterEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import CommandPalette from '../../src/components/CommandPalette.vue'

afterEach(() => { document.body.innerHTML = '' })
describe('command palette', () => {
  it('focuses search, traps focus and restores the triggering control', async () => {
    const trigger = document.createElement('button')
    document.body.appendChild(trigger)
    trigger.focus()
    const wrapper = mount(CommandPalette)
    await nextTick()
    const input = document.querySelector('input')!
    expect(document.activeElement).toBe(input)
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', shiftKey: true, bubbles: true, cancelable: true }))
    expect(document.activeElement?.getAttribute('role')).toBe('option')
    document.activeElement!.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true }))
    expect(document.activeElement).toBe(input)
    wrapper.unmount()
    expect(document.activeElement).toBe(trigger)
  })
  it('resets selection after filtering including a preceding empty result', async () => {
    const navigate = vi.fn()
    mount(CommandPalette, { props: { onNavigate: navigate } })
    await nextTick()
    const input = document.querySelector('input')!
    for (let i = 0; i < 8; i++) input.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }))
    input.value = 'нет такого раздела'
    input.dispatchEvent(new Event('input', { bubbles: true }))
    await nextTick()
    expect(document.querySelector('[role=status]')?.textContent).toContain('не найден')
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }))
    input.value = 'секреты'
    input.dispatchEvent(new Event('input', { bubbles: true }))
    await nextTick()
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))
    expect(navigate).toHaveBeenCalledExactlyOnceWith('secrets')
  })
  it('Escape closes without navigating', async () => {
    const close = vi.fn(), navigate = vi.fn()
    mount(CommandPalette, { props: { onClose: close, onNavigate: navigate } })
    await nextTick()
    document.querySelector('input')!.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    expect(close).toHaveBeenCalledTimes(1)
    expect(navigate).not.toHaveBeenCalled()
  })
})
