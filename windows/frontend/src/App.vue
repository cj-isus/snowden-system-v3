<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import TopBar from './components/TopBar.vue'
import NavRail from './components/NavRail.vue'
import SectionHeader from './components/SectionHeader.vue'
import ContextTabs from './components/ContextTabs.vue'
import ToastHost from './components/ToastHost.vue'
import CommandPalette from './components/CommandPalette.vue'
import NotificationPanel from './components/NotificationPanel.vue'
import { MOCK_ACTIVE } from './api/preview'
import { api } from './api/backend'
import { provideNav } from './composables/nav'
import { sectionForPage, useNavigation } from './composables/navigation'
import type { PageId } from './composables/navigation'
import { disposeBackendState, initBackendState } from './composables/backendState'
import { activateApplicationUpdate, disposeApplicationUpdate } from './composables/applicationUpdate'
import { activateProfileActions, disposeProfileActions } from './composables/profileActions'
import { activateClipboard, disposeClipboard } from './composables/clipboard'
import DashboardView from './views/DashboardView.vue'
import ChannelsPageView from './views/ChannelsPageView.vue'
import AutoProtectView from './views/AutoProtectView.vue'
import NetworkView from './views/NetworkView.vue'
import StatisticsView from './views/StatisticsView.vue'
import TestsView from './views/TestsView.vue'
import SecretsView from './views/SecretsView.vue'
import NetGuardView from './views/NetGuardView.vue'
import LogsView from './views/LogsView.vue'
import SettingsView from './views/SettingsView.vue'

const { active, section, navigate, openSection } = useNavigation()
provideNav(navigate)
const workspace = ref<HTMLElement | null>(null)
const currentSection = computed(() => sectionForPage(active.value))
const hasTabs = computed(() => currentSection.value.pages.length > 1)
const searchOpen = ref(false)
const notificationsOpen = ref(false)
function openSearch() { notificationsOpen.value = false; searchOpen.value = true }
function openNotifications() { searchOpen.value = false; notificationsOpen.value = true }
let searchNavigationRevision = 0
async function navigateFromSearch(page: PageId) {
  const revision = ++searchNavigationRevision
  navigate(page)
  searchOpen.value = false
  await nextTick()
  const main = workspace.value
  if (revision !== searchNavigationRevision || active.value !== page || !main?.isConnected || searchOpen.value || notificationsOpen.value || document.querySelector('dialog[open]')) return
  main.scrollTo({ top: 0, left: 0, behavior: 'auto' })
  main.querySelector<HTMLElement>('#page-' + page)?.focus({ preventScroll: true })
}
function onKey(event: KeyboardEvent) {
  if (event.defaultPrevented || event.isComposing || event.repeat || event.altKey || event.shiftKey) return
  if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') {
    // A recovery/confirmation modal keeps its own action and dismissal scope.
    if (event.target instanceof Element && event.target.closest('dialog, [role="dialog"]') && !searchOpen.value && !notificationsOpen.value) return
    event.preventDefault()
    if (searchOpen.value) searchOpen.value = false
    else openSearch()
  }
}
// Page views still unmount on navigation. Empty hidden panel shells keep each
// tab's controls target present without retaining inactive readers or actions.
let navigationRevision = 0
watch(active, async (page, previous) => {
  const revision = ++navigationRevision
  const from = document.activeElement
  const departing = workspace.value?.querySelector('#page-' + previous)
  const restoreFocus = from instanceof HTMLElement && departing?.contains(from)
  await nextTick()
  const main = workspace.value
  if (revision !== navigationRevision || active.value !== page || !main?.isConnected) return
  main.scrollTo({ top: 0, left: 0, behavior: 'auto' })
  // Do not take focus from the rail, tabs, dialogs or a newer user interaction.
  if (restoreFocus && from instanceof HTMLElement && !from.isConnected && document.activeElement === document.body && !searchOpen.value && !notificationsOpen.value) {
    main.querySelector<HTMLElement>('#page-' + page)?.focus({ preventScroll: true })
  }
})
let releaseRendererReadiness = () => {}
onMounted(() => {
  activateClipboard()
  activateApplicationUpdate()
  activateProfileActions()
  releaseRendererReadiness = api.acknowledgeRendererMounted()
  void initBackendState()
  window.addEventListener('keydown', onKey)
})
onUnmounted(() => {
  navigationRevision++
  searchNavigationRevision++
  disposeClipboard()
  disposeProfileActions()
  disposeApplicationUpdate()
  releaseRendererReadiness()
  disposeBackendState()
  window.removeEventListener('keydown', onKey)
})
</script>
<template>
  <div class="app">
    <a href="#workspace" class="skip-link">К содержимому</a>
    <div v-if="MOCK_ACTIVE" class="mock-banner" role="status">Демонстрация · данные вымышлены · VPN не подключён</div>
    <TopBar :search-open="searchOpen" :notifications-open="notificationsOpen" @open-search="openSearch" @open-notifications="openNotifications" />
    <div class="app-body">
      <NavRail :active="section" @navigate="openSection" />
      <main id="workspace" ref="workspace" class="main-content" tabindex="-1" :aria-labelledby="'section-' + section">
        <SectionHeader :section="section" />
        <ContextTabs :active="active" :section="section" @navigate="navigate" />
        <div v-for="page in currentSection.pages" :id="'page-' + page.id" :key="page.id" class="page-content"
          :hidden="page.id !== active" :role="hasTabs ? 'tabpanel' : undefined"
          :aria-labelledby="hasTabs ? 'tab-' + page.id : 'section-' + section" :tabindex="hasTabs && page.id === active ? 0 : -1">
          <template v-if="page.id === active">
            <DashboardView v-if="active === 'dashboard'" />
            <ChannelsPageView v-else-if="active === 'channels'" />
            <AutoProtectView v-else-if="active === 'autoprotect'" />
            <NetworkView v-else-if="active === 'network'" />
            <StatisticsView v-else-if="active === 'statistics'" />
            <TestsView v-else-if="active === 'tests'" />
            <SecretsView v-else-if="active === 'secrets'" />
            <NetGuardView v-else-if="active === 'netguard'" />
            <LogsView v-else-if="active === 'logs'" />
            <SettingsView v-else />
          </template>
        </div>
      </main>
    </div>
    <CommandPalette v-if="searchOpen" @close="searchOpen = false" @navigate="navigateFromSearch" />
    <NotificationPanel v-else-if="notificationsOpen" @close="notificationsOpen = false" />
    <ToastHost />
  </div>
</template>
<style scoped>
.app { --navigation-width: 210px; height: 100dvh; display: flex; flex-direction: column; overflow: hidden; background: var(--bg); }
.app-body { flex: 1; display: flex; min-width: 0; min-height: 0; }
.main-content { flex: 1; min-width: 0; min-height: 0; overflow: auto; scrollbar-gutter: stable; overscroll-behavior: contain; padding: 28px 32px 48px; background: radial-gradient(ellipse at 90% 0, #ff622608, transparent 45%); }
.page-content { width: min(1280px, 100%); margin: auto; min-width: 0; }
.mock-banner { flex-shrink: 0; padding: 4px 12px; text-align: center; background: #312419; color: #ffcb9e; font-size: 11px; border-bottom: 1px solid #66432c; }
.skip-link { position: absolute; left: 12px; top: -80px; z-index: 200; padding: 12px; background: var(--panel-2); }
.skip-link:focus { top: 12px; }
@media(max-width: 1180px) { .app { --navigation-width: 176px; } .main-content { padding: 20px 20px 36px; } }
@media(max-width: 900px) { .app { --navigation-width: 72px; } }
@media(max-width: 700px) { .app-body { flex-direction: column; } .main-content { padding: 16px 12px 30px; } }
</style>
