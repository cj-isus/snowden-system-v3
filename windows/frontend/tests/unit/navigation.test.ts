import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { pages, sections, sectionForPage, useNavigation } from '../../src/composables/navigation'
import ContextTabs from '../../src/components/ContextTabs.vue'

describe('section/page contract', () => {
  it('every page has one owner; subsection names do not duplicate menus', () => {
    expect(new Set(pages.map(page => page.id)).size).toBe(10)
    for (const page of pages) {
      expect(sections.filter(section => section.pages.includes(page))).toHaveLength(1)
      expect(sections.map(section => section.label)).not.toContain(page.label)
      expect(sectionForPage(page.id).pages).toContain(page)
    }
  })
  it('search/deep navigation selects the owning menu; switching menus remembers its tab', () => {
    const nav = useNavigation()
    nav.navigate('statistics')
    expect(nav.section.value).toBe('network')
    nav.openSection('settings')
    expect(nav.active.value).toBe('settings')
    nav.navigate('secrets')
    nav.openSection('network')
    expect(nav.active.value).toBe('statistics')
    nav.openSection('settings')
    expect(nav.active.value).toBe('secrets')
  })
  it('single-page sections have no redundant tab strip', () => {
    const wrapper = mount(ContextTabs, { props: { section: 'channels', active: 'channels' } })
    expect(wrapper.find('[role=tablist]').exists()).toBe(false)
  })
  it('tabs expose names, selected state, focus and wraparound keyboard navigation', async () => {
    const wrapper = mount(ContextTabs, { attachTo: document.body, props: { section: 'network', active: 'network' } })
    const tabs = wrapper.findAll('[role=tab]')
    expect(tabs).toHaveLength(4)
    expect(tabs[0].attributes('aria-selected')).toBe('true')
    for (const tab of tabs) expect(tab.attributes('aria-label')).toBeTruthy()
    await tabs[0].trigger('keydown', { key: 'ArrowLeft' })
    expect(wrapper.emitted('navigate')?.at(-1)).toEqual(['netguard'])
    expect(document.activeElement?.id).toBe('tab-netguard')
    await tabs[3].trigger('keydown', { key: 'Home' })
    expect(wrapper.emitted('navigate')?.at(-1)).toEqual(['network'])
    await tabs[0].trigger('keydown', { key: 'End' })
    expect(wrapper.emitted('navigate')?.at(-1)).toEqual(['netguard'])
  })
})
