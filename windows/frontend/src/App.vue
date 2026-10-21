<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import TopBar from './components/TopBar.vue'
import NavRail from './components/NavRail.vue'
import SectionHeader from './components/SectionHeader.vue'
import ContextTabs from './components/ContextTabs.vue'
import ToastHost from './components/ToastHost.vue'
import CommandPalette from './components/CommandPalette.vue'
import NotificationPanel from './components/NotificationPanel.vue'
import { MOCK_ACTIVE } from './api/preview'
import { provideNav } from './composables/nav'
import { useNavigation } from './composables/navigation'
import { disposeBackendState, initBackendState } from './composables/backendState'
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
const searchOpen = ref(false)
const notificationsOpen = ref(false)
function openSearch() { notificationsOpen.value = false; searchOpen.value = true }
function openNotifications() { searchOpen.value = false; notificationsOpen.value = true }
function onKey(event: KeyboardEvent) {
  if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') {
    event.preventDefault()
    if (searchOpen.value) searchOpen.value = false
    else openSearch()
  }
}
onMounted(() => { void initBackendState(); window.addEventListener('keydown', onKey) })
onUnmounted(() => { disposeBackendState(); window.removeEventListener('keydown', onKey) })
</script>
<template>
  <div class="app">
    <a href="#workspace" class="skip-link">К содержимому</a>
    <div v-if="MOCK_ACTIVE" class="mock-banner" role="status">Демонстрация · данные вымышлены · VPN не подключён</div>
    <TopBar @open-search="openSearch" @open-notifications="openNotifications" />
    <div class="app-body">
      <NavRail :active="section" @navigate="openSection" />
      <main id="workspace" class="main-content" tabindex="-1">
        <SectionHeader :section="section" />
        <ContextTabs :active="active" :section="section" @navigate="navigate" />
        <div :id="'page-' + active" class="page-content" :role="['network', 'settings'].includes(section) ? 'tabpanel' : undefined" :aria-labelledby="['network', 'settings'].includes(section) ? 'tab-' + active : undefined">
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
        </div>
      </main>
    </div>
    <CommandPalette v-if="searchOpen" @close="searchOpen = false" @navigate="navigate" />
    <NotificationPanel v-if="notificationsOpen" @close="notificationsOpen = false" />
    <ToastHost />
  </div>
</template>
<style scoped>
.app { height: 100dvh; display: flex; flex-direction: column; overflow: hidden; background: var(--bg); }
.app-body { flex: 1; display: flex; min-height: 0; }
.main-content { flex: 1; min-width: 0; overflow: auto; padding: 28px 32px 48px; background: radial-gradient(ellipse at 90% 0, #ff622608, transparent 45%); }
.page-content { width: min(1280px, 100%); margin: auto; min-width: 0; }
.mock-banner { flex-shrink: 0; padding: 4px 12px; text-align: center; background: #312419; color: #ffcb9e; font-size: 11px; border-bottom: 1px solid #66432c; }
.skip-link { position: absolute; left: 12px; top: -80px; z-index: 200; padding: 12px; background: var(--panel-2); }
.skip-link:focus { top: 12px; }
@media(max-width: 1180px) { .main-content { padding: 20px 20px 36px; } }
@media(max-width: 700px) { .app-body { flex-direction: column; } .main-content { padding: 16px 12px 30px; } }
</style>
