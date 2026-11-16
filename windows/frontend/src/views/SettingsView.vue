<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import PageHeading from '../components/PageHeading.vue'
import ActionButton from '../components/ActionButton.vue'
import ProfileTransfer from '../components/ProfileTransfer.vue'
import ApplicationUpdate from '../components/ApplicationUpdate.vue'
import AutostartSetting from '../components/AutostartSetting.vue'
import ProcessModeSetting from '../components/ProcessModeSetting.vue'
import HintBox from '../components/HintBox.vue'
import { api } from '../api/backend'
import type { DeliveryProfile, RuntimeInfo } from '../api/contract'
import { fmtDateTime } from '../api/labels'
import { useNav } from '../composables/nav'
import { useBackendState, initBackendState } from '../composables/backendState'
import { createPoller } from '../composables/polling'
import { useRemovedControlFocus } from '../composables/removedControlFocus'

const { isRunning, isBusy, switching, stopPending, canStop, stopVpn } = useBackendState()
const { navTo } = useNav()
const page = ref<HTMLElement | null>(null)
const { focusBeforeRemoval } = useRemovedControlFocus(page, '#settings-action-heading')
function onStop(): void {
  if (!canStop.value) return
  focusBeforeRemoval()
  void stopVpn()
}
const profile = ref<DeliveryProfile | null>(null)
const runtimeInfo = ref<RuntimeInfo | null>(null)
const profileLoading = ref(false)
const runtimeLoading = ref(false)
const profileError = ref('')
const runtimeError = ref('')
const profileReceivedAt = ref<string | null>(null)
const runtimeReceivedAt = ref<string | null>(null)
const autostartRefreshVersion = ref(0)
const transferOpen = ref(false)
const updateOpen = ref(false)
let initialImportSuggested = false
let active = true
const profileReader = createPoller(api.getDeliveryProfile, delivery => {
  profile.value = delivery
  profileReceivedAt.value = delivery === null ? null : new Date().toISOString()
  profileError.value = delivery === null ? 'Сведения о профиле не получены. Повторите загрузку.' : ''
  if (!initialImportSuggested && delivery?.state === 'embedded') { initialImportSuggested = true; transferOpen.value = true }
}, null, () => {
  clearProfile()
  profileError.value = 'Сведения о профиле не получены. Повторите загрузку.'
}, pending => {
  profileLoading.value = pending
  if (pending) clearProfile()
})
const runtimeReader = createPoller(api.getRuntimeInfo, runtime => {
  runtimeInfo.value = runtime
  runtimeReceivedAt.value = runtime === null ? null : new Date().toISOString()
  runtimeError.value = runtime === null ? 'Сведения о текущем процессе недоступны.' : ''
}, null, () => {
  clearRuntime()
  runtimeError.value = 'Сведения о текущем процессе недоступны.'
}, pending => {
  runtimeLoading.value = pending
  if (pending) clearRuntime()
})
onBeforeUnmount(() => { active = false; profileReader.stop(); runtimeReader.stop() })
function clearProfile(): void {
  profile.value = null
  profileReceivedAt.value = null
  profileError.value = ''
}
function clearRuntime(): void {
  runtimeInfo.value = null
  runtimeReceivedAt.value = null
  runtimeError.value = ''
}
function refreshAll(): void {
  if (!active) return
  autostartRefreshVersion.value++
  refreshSummary()
}
function refreshSummary(invalidate = false): void {
  if (!active) return
  if (invalidate) {
    clearProfile()
    clearRuntime()
    void profileReader.invalidate()
    void runtimeReader.invalidate()
  } else {
    void profileReader.refresh()
    void runtimeReader.refresh()
  }
}
function refreshProfile(): void {
  if (!active || profileLoading.value) return
  void profileReader.refresh()
}
function refreshRuntime(): void {
  if (!active || runtimeLoading.value) return
  void runtimeReader.refresh()
}
onMounted(() => { void initBackendState(); profileReader.start(); runtimeReader.start() })
function disclosure(event: Event, section: 'transfer' | 'update'): void {
  const open = (event.target as HTMLDetailsElement).open
  if (section === 'transfer') transferOpen.value = open
  else updateOpen.value = open
}
const profileStatus = computed(() => {
  const value = profile.value
  if (profileLoading.value) return 'Сведения обновляются'
  if (!value || value.factsSchema !== 1) return 'Принятие не подтверждено'
  if (value.storageState === 'recovery_required') return 'Хранилище требует восстановления'
  return ({ accepted: 'Приложение сообщает принятую версию', restricted: 'Нет разрешённых каналов', policy_pending: 'Ожидается применение ограничений', legacy_unanchored: 'Принятие не зафиксировано', embedded: 'Внешний профиль не принят', unavailable: 'Сведения недоступны', unknown: 'Состояние неизвестно' })[value.state ?? 'unknown']
})
const policyStatus = computed(() => profile.value?.pinPolicyState === 'applied' ? `Сохранена версия ${profile.value.pinPolicyVersion || 'не сообщена'}` : profile.value?.pinPolicyState === 'pending' ? 'Ожидает применения' : profile.value?.pinPolicyState === 'not_observed' ? 'Получение не зарегистрировано' : 'Нет данных')
const storageStatus = computed(() => profile.value?.storageState === 'recovery_required' ? 'Требуется восстановление' : profile.value?.storageState === 'cleanup_pending' ? 'Очистка отложена' : profile.value?.storageState === 'not_observed' ? 'Ограничение не зарегистрировано' : 'Нет данных')
</script>

<template>
  <div ref="page" class="view">
    <PageHeading id="settings-action-heading" tabindex="-1" title="Настройки системы" description="Профиль, запуск, перенос и обслуживание. Ответ на команду не равен готовому защищённому подключению.">
      <ActionButton @click="refreshAll">Обновить сведения</ActionButton>
      <ActionButton v-if="isRunning || isBusy || switching" :disabled="!canStop" :busy="stopPending" @click="onStop">{{ stopPending ? 'Ожидание остановки…' : 'Остановить' }}</ActionButton>
    </PageHeading>
    <section class="profile-summary" :aria-busy="profileLoading">
      <div class="section-heading"><h2>Профиль доставки</h2><span class="status-label" role="status">{{ profileStatus }}</span><ActionButton :disabled="profileLoading" :busy="profileLoading" @click="refreshProfile">Обновить сведения о профиле</ActionButton></div>
      <p class="source-note">Проверено приложением: {{ fmtDateTime(profile?.checkedAt) }} · Получено экраном: {{ fmtDateTime(profileReceivedAt) }}. Срок действия профиля и состояние подключения — разные сведения.</p>
      <p v-if="profileError" class="error" role="status">{{ profileError }}</p>
      <dl v-if="profile" class="profile-facts">
        <div><dt>Принятая версия</dt><dd>{{ profile.profileIdentity || 'Не подтверждена' }}</dd></div>
        <div><dt>Удержанная версия</dt><dd>{{ profile.retainedIdentity || 'Не сообщена' }}</dd></div>
        <div><dt>Издатель</dt><dd class="mono">{{ profile.keyId || 'Не подтверждён' }}</dd></div>
        <div><dt>Срок профиля</dt><dd>{{ fmtDateTime(profile.expiresAt) }}</dd></div>
        <div><dt>Ключи профиля</dt><dd>{{ profile.activeProfileKeys ?? 'Нет данных' }}</dd></div>
        <div><dt>Объявленные / разрешённые каналы</dt><dd>{{ profile.channelsDeclared ?? '—' }} / {{ profile.channelsEligible ?? '—' }}</dd></div>
        <div><dt>Ограниченные / выведенные каналы</dt><dd>{{ profile.channelsRestricted ?? '—' }} / {{ profile.channelsRetired ?? '—' }}</dd></div>
        <div><dt>Отозванные сертификаты</dt><dd>{{ profile.certificatesRevoked ?? 'Нет данных' }}</dd></div>
        <div><dt>Список отзывов</dt><dd>{{ policyStatus }}</dd></div>
        <div><dt>Ресурсы хранилища</dt><dd>{{ storageStatus }}</dd></div>
      </dl>
      <p v-if="profile?.error || profile?.storageWarning" class="error">Приложение сообщает ограничение профиля или хранилища. Сверьте состояние выше перед новым импортом; исходный текст ошибки здесь не публикуется.</p>
      <p class="source-note">Разрешённый канал и принятая версия не подтверждают native применение, доступность сервера или отсутствие утечек. Отсутствие зарегистрированного сбоя хранилища не является проверкой диска.</p>
      <button class="btn" @click="navTo('channels')">Состав каналов и наблюдения</button>
    </section>

    <section class="system-settings">
      <h2>Запуск и режим</h2>
      <AutostartSetting :refresh-version="autostartRefreshVersion" />
      <ProcessModeSetting id="settings-process-mode" />
      <div class="actions"><button class="btn" @click="navTo('netguard')">Локальное восстановление</button><button class="btn" @click="navTo('secrets')">Локальные секреты</button></div>
    </section>

    <details class="maintenance" :open="transferOpen" @toggle="disclosure($event, 'transfer')"><summary>Профиль, доверие и перенос на устройство</summary><ProfileTransfer v-if="transferOpen" @changed="refreshSummary(true)" /></details>
    <details class="maintenance" :open="updateOpen" @toggle="disclosure($event, 'update')"><summary>Проверка и установка обновления</summary><ApplicationUpdate v-if="updateOpen" @changed="refreshSummary(true)" /></details>
    <details class="runtime" :aria-busy="runtimeLoading"><summary>Сведения о текущем процессе</summary>
      <p class="source-note">{{ runtimeLoading ? 'Сведения о процессе обновляются.' : runtimeInfo ? 'Сведения о процессе получены.' : 'Сведения о процессе не получены.' }} Получено экраном: {{ fmtDateTime(runtimeReceivedAt) }}.</p>
      <p v-if="runtimeError" class="error">{{ runtimeError }}</p>
      <dl class="profile-facts"><div><dt>Версия EXE</dt><dd>{{ runtimeInfo?.version || 'Не сообщена' }}</dd></div><div><dt>Ядро</dt><dd>{{ runtimeInfo?.coreVersion || 'Не сообщено' }}</dd></div><div><dt>Wails</dt><dd>{{ runtimeInfo?.shellVersion || 'Не сообщён' }}</dd></div><div><dt>Процесс</dt><dd>{{ runtimeInfo ? `${runtimeInfo.os}/${runtimeInfo.arch}` : 'Нет данных' }} · PID {{ runtimeInfo && runtimeInfo.pid > 0 ? runtimeInfo.pid : 'не сообщён' }}</dd></div></dl>
      <ActionButton :disabled="runtimeLoading" :busy="runtimeLoading" @click="refreshRuntime">Обновить сведения о процессе</ActionButton>
      <button class="btn" @click="navTo('logs')">Журнал и сводка для поддержки</button>
    </details>
    <HintBox kind="info">После импорта или команды перезапуска перечитайте факты. Проверка пакета, сохранение профиля и готовность текущего процесса не заменяют результата нового подключения.</HintBox>
  </div>
</template>

<style scoped>
.view { display: flex; flex-direction: column; gap: 20px; min-width: 0; overflow-wrap: anywhere; }
h2 { margin: 0; font-size: 17px; }
h3 { margin: 0 0 8px; font-size: 14px; }
.profile-summary, .system-settings { padding: 20px; border: 1px solid var(--border); border-radius: var(--radius); background: var(--panel); min-width: 0; }
.section-heading, .actions { display: flex; flex-wrap: wrap; gap: 10px 18px; align-items: baseline; }
.section-heading { justify-content: space-between; }
.status-label, .source-note { color: var(--muted); font-size: 12.5px; line-height: 1.65; }
.profile-facts { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px 24px; margin: 20px 0; }
dt { color: var(--muted); font-size: 12px; }
dd { margin: 6px 0 0; color: var(--text-soft); font-size: 13px; }
.setting-row { display: flex; flex-wrap: wrap; justify-content: space-between; gap: 14px 20px; padding: 18px 0; border-bottom: 1px solid var(--border); }
.setting-row > div { min-width: 0; flex: 1 1 320px; }
.setting-row > .btn { align-self: center; }
.actions { margin-top: 16px; }
summary { padding: 12px 0; color: var(--text-soft); font-size: 14px; cursor: pointer; }
.maintenance, .runtime { min-width: 0; border-top: 1px solid var(--border); }
.btn { min-height: 40px; white-space: normal; }
.btn[aria-disabled='true'] { color: var(--muted); cursor: not-allowed; }
.error { color: var(--yellow); font-size: 13px; line-height: 1.6; }
@media (max-width: 760px) { .profile-summary, .system-settings { padding: 16px; } .profile-facts { grid-template-columns: minmax(0, 1fr); } }
</style>
