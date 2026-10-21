import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { ref } from 'vue'
import { deferred } from '../helpers'
import ProfileTransfer from '../../src/components/ProfileTransfer.vue'
import SecretsView from '../../src/views/SecretsView.vue'
import SecretCard from '../../src/components/SecretCard.vue'

const api = vi.hoisted(() => ({ previewOnboarding: vi.fn(), applyOnboarding: vi.fn(), loadOnboardingFile: vi.fn() }))
const save = vi.hoisted(() => vi.fn())
const setValue = vi.hoisted(() => vi.fn())
vi.mock('../../src/api/backend', () => ({ api, isBuildPhase: () => false }))
vi.mock('../../src/composables/secretsStore', () => ({
  useSecretsStore: () => ({ items: ref([]), loading: ref(false), refresh: vi.fn(), save, setValue }),
}))
const preview = { channels: 2, secrets: 1, bundleVersion: 2, currentVersion: 1, wouldDowngrade: false, keys: [] }
function button(wrapper: ReturnType<typeof mount>, text: string) {
  return wrapper.findAll('button').find(button => button.text().includes(text))!
}
beforeEach(() => { Object.values(api).forEach(fn => fn.mockReset()); save.mockReset(); setValue.mockReset() })
describe('sensitive forms, synthetic values only', () => {
  it('existing empty secret slot also requires confirmation and one save at a time', async () => {
    const pending = deferred<boolean>()
    setValue.mockReturnValue(pending.promise)
    const wrapper = mount(SecretCard, { props: { secret: {
      id: 'fixture-slot', kind: 'custom', title: 'Synthetic slot', hint: '', backend: 'dpapi',
      storedValue: null, fingerprint: '', createdAt: '', updatedAt: '', lastVerifiedAt: null,
      verifyStatus: 'unverified', verifyError: '', reveals: 0,
    } } })
    await button(wrapper, 'Задать значение').trigger('click')
    const passwords = wrapper.findAll('input[type=password]')
    await passwords[0].setValue('synthetic-value')
    expect(button(wrapper, 'Сохранить').attributes('disabled')).toBeDefined()
    await passwords[1].setValue('synthetic-value')
    await button(wrapper, 'Сохранить').trigger('click')
    await button(wrapper, 'Сохранить').trigger('click')
    expect(setValue).toHaveBeenCalledExactlyOnceWith('fixture-slot', 'synthetic-value')
    pending.resolve(true)
    await flushPromises()
    expect(wrapper.find('input[type=password]').exists()).toBe(false)
  })
  it('requires matching nonempty confirmation and prevents duplicate save', async () => {
    const pending = deferred<boolean>()
    save.mockReturnValue(pending.promise)
    const wrapper = mount(SecretsView)
    await button(wrapper, 'Добавить секрет').trigger('click')
    await wrapper.get('input[type=text]').setValue('Synthetic test slot')
    const passwords = wrapper.findAll('input[type=password]')
    await passwords[0].setValue('synthetic-fixture')
    expect(button(wrapper, 'Сохранить локально').attributes('disabled')).toBeDefined()
    await passwords[1].setValue('mismatch')
    expect(button(wrapper, 'Сохранить локально').attributes('disabled')).toBeDefined()
    await passwords[1].setValue('synthetic-fixture')
    await button(wrapper, 'Сохранить локально').trigger('click')
    await button(wrapper, 'Сохранить локально').trigger('click')
    expect(save).toHaveBeenCalledTimes(1)
    pending.resolve(true)
    await flushPromises()
    expect(wrapper.find('input[type=password]').exists()).toBe(false)
  })
  it('editing approved bytes revokes the apply action', async () => {
    api.previewOnboarding.mockResolvedValue(preview)
    const wrapper = mount(ProfileTransfer)
    await wrapper.get('[aria-label="Пароль импорта"]').setValue('fixture-passphrase')
    await wrapper.get('textarea').setValue('SNOB1.synthetic-a')
    await button(wrapper, 'Проверить бандл').trigger('click')
    await flushPromises()
    expect(button(wrapper, 'Применить на этом устройстве')).toBeDefined()
    await wrapper.get('textarea').setValue('SNOB1.synthetic-b')
    expect(button(wrapper, 'Применить на этом устройстве')).toBeUndefined()
    expect(api.applyOnboarding).not.toHaveBeenCalled()
  })
  it('a late preview of old inputs cannot approve new inputs', async () => {
    const pending = deferred<typeof preview>()
    api.previewOnboarding.mockReturnValue(pending.promise)
    const wrapper = mount(ProfileTransfer)
    await wrapper.get('[aria-label="Пароль импорта"]').setValue('fixture-passphrase')
    await wrapper.get('textarea').setValue('SNOB1.synthetic-a')
    await button(wrapper, 'Проверить бандл').trigger('click')
    await wrapper.get('[aria-label="Пароль импорта"]').setValue('changed-passphrase')
    pending.resolve(preview)
    await flushPromises()
    expect(button(wrapper, 'Применить на этом устройстве')).toBeUndefined()
  })
  it('downgrade preview cannot be applied', async () => {
    api.previewOnboarding.mockResolvedValue({ ...preview, wouldDowngrade: true })
    const wrapper = mount(ProfileTransfer)
    await wrapper.get('[aria-label="Пароль импорта"]').setValue('fixture-passphrase')
    await wrapper.get('textarea').setValue('SNOB1.synthetic')
    await button(wrapper, 'Проверить бандл').trigger('click')
    await flushPromises()
    expect(button(wrapper, 'Применить на этом устройстве').attributes('disabled')).toBeDefined()
    expect(api.applyOnboarding).not.toHaveBeenCalled()
  })
})
