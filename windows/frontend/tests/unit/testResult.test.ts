import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import TestCard from '../../src/components/TestCard.vue'
describe('diagnostic result presentation', () => {
  it.each([
    { ok: true, checkedAt: '2026-01-01T00:00:00Z', text: 'пройдено' },
    { ok: false, checkedAt: '2026-01-01T00:00:00Z', text: 'не пройдено' },
    { ok: null, checkedAt: null, text: 'не запускался' },
    { ok: null, checkedAt: '2026-01-01T00:00:00Z', text: 'нет результата' },
  ])('shows $text without coercing unknown to false or success', ({ ok, checkedAt, text }) => {
    const wrapper = mount(TestCard, { props: { test: { id: 'fixture', ok, checkedAt, detail: '' }, running: false } })
    expect(wrapper.get('.status').text()).toBe(text)
  })
  it('cannot launch another check while a batch owns execution', async () => {
    const wrapper = mount(TestCard, { props: { test: { id: 'fixture', ok: null, checkedAt: null, detail: '' }, running: false, disabled: true } })
    await wrapper.get('button').trigger('click')
    expect(wrapper.emitted('run')).toBeUndefined()
  })
  it('does not present an entirely skipped check as success', () => {
    const wrapper = mount(TestCard, { props: {
      test: { id: 'fixture', ok: true, checkedAt: '2026-01-01T00:00:00Z', detail: '',
        steps: [{ name: 'inapplicable transport', status: 'skipped', detail: '' }] },
      running: false,
    } })
    expect(wrapper.get('.status').text()).toBe('пропущено')
    expect(wrapper.get('.status').classes()).not.toContain('pass')
  })
  it('keeps a successful check when only an optional step was skipped', () => {
    const wrapper = mount(TestCard, { props: {
      test: { id: 'fixture', ok: true, checkedAt: '2026-01-01T00:00:00Z', detail: '',
        steps: [{ name: 'required', status: 'pass', detail: '' }, { name: 'optional', status: 'skipped', detail: '' }] },
      running: false,
    } })
    expect(wrapper.get('.status').text()).toBe('пройдено')
  })
})
