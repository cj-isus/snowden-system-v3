<script setup lang="ts">
import { computed } from 'vue'
import { lifecycleLabels } from '../api/labels'
import { useBackendState } from '../composables/backendState'
import type { NavId } from './NavRail.vue'

const props = defineProps<{ section: NavId }>()
const { lifecycle, tunMode } = useBackendState()

const copy: Partial<Record<NavId, { eyebrow: string; title: string; subtitle: string }>> = {
  dashboard: { eyebrow: 'COMMAND CENTER', title: 'Центр защиты', subtitle: 'Главное состояние, активный маршрут и последнее доказательство защищённого пути' },
  network: { eyebrow: 'SYSTEM STATUS', title: 'Состояние', subtitle: 'Контекст сети, здоровье каналов и работа адаптивной защиты' },
  channels: { eyebrow: 'ROUTING', title: 'Каналы', subtitle: 'Доступные маршруты, уровень их проверки и безопасное переключение' },
  tests: { eyebrow: 'VERIFICATION', title: 'Проверки', subtitle: 'Диагностика DNS, HTTPS, egress и отсутствия прямых утечек' },
  logs: { eyebrow: 'EVIDENCE', title: 'Журнал', subtitle: 'Фактические события приложения, ядра и защищённых проверок' },
  settings: { eyebrow: 'CONFIGURATION', title: 'Настройки', subtitle: 'Системное поведение, профиль доставки и локальные секреты' },
}

const current = computed(() => copy[props.section] ?? copy.dashboard!)
const stateLabel = computed(() => lifecycle.value ? lifecycleLabels[lifecycle.value] : 'Нет данных')
const stateTone = computed(() => lifecycle.value === 'running' ? 'running' : lifecycle.value === 'error' ? 'error' : 'idle')
</script>

<template>
  <header class="section-header">
    <div class="section-copy">
      <span class="eyebrow">{{ current.eyebrow }}</span>
      <h1>{{ current.title }}</h1>
      <p>{{ current.subtitle }}</p>
    </div>
    <div class="section-state" :class="stateTone">
      <span class="signal"></span>
      <span>
        <strong>{{ stateLabel }}</strong>
        <small>{{ lifecycle === 'running' ? (tunMode ? 'TUN · путь проверен' : 'SOCKS · приложения с прокси') : 'защищённый путь не подтверждён' }}</small>
      </span>
    </div>
  </header>
</template>

<style scoped>
.section-header {
  width: min(1280px, 100%);
  min-height: 96px;
  margin: 0 auto;
  padding: 18px 22px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  position: relative;
  border: 1px solid #1d2d39;
  border-bottom-color: rgba(255, 98, 38, .44);
  background:
    linear-gradient(90deg, rgba(6, 14, 20, .96), rgba(8, 16, 23, .86)),
    repeating-linear-gradient(135deg, transparent 0 28px, rgba(255, 98, 38, .025) 29px 30px);
  clip-path: polygon(0 9px, 9px 0, 100% 0, 100% calc(100% - 9px), calc(100% - 9px) 100%, 0 100%);
}
.section-header::after {
  content: '';
  position: absolute;
  right: 7%;
  top: 0;
  width: 180px;
  height: 1px;
  background: linear-gradient(90deg, transparent, #ff6226, transparent);
  box-shadow: 0 0 12px rgba(255, 98, 38, .45);
}
.section-copy { min-width: 0; }
.eyebrow { color: #ff7541; font-family: var(--mono); font-size: 12px; letter-spacing: .22em; }
h1 { margin: 5px 0 4px; font-size: clamp(24px, 2.2vw, 30px); letter-spacing: -.025em; }
p { margin: 0; color: #8394a6; font-size: 12.5px; }
.section-state {
  min-width: 200px;
  display: flex;
  align-items: center;
  gap: 13px;
  padding: 10px 15px;
  border-left: 2px solid #405062;
}
.signal { width: 13px; height: 13px; border-radius: 50%; background: #637386; box-shadow: 0 0 12px rgba(99,115,134,.35); }
.section-state > span:last-child { display: flex; flex-direction: column; }
.section-state strong { font-size: 11.5px; font-weight: 600; }
.section-state small { color: var(--muted); font-size: 11px; margin-top: 2px; }
.section-state.running { border-left-color: #ff6226; }
.section-state.running .signal { background: #ff6226; box-shadow: 0 0 14px rgba(255,98,38,.75); }
.section-state.error { border-left-color: var(--red); }
.section-state.error .signal { background: var(--red); }

@media (max-width: 900px) {
  .section-header { min-height: 86px; padding: 15px 17px; }
  .section-copy p, .section-state small, .eyebrow { display: none; }
  .section-state { min-width: 0; padding-right: 0; }
  h1 { font-size: 24px; }
}

@media (max-width: 560px) {
  .section-header { min-height: 72px; padding: 12px 14px; }
  .section-state { padding: 8px 0 8px 11px; }
  .section-state strong { font-size: 12px; }
  h1 { font-size: 21px; }
}
</style>
