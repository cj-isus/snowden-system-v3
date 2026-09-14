<script setup lang="ts">
/**
 * Каналы активного конфига. Список приходит только из backend (FR-002):
 * страны/серверы не хардкодятся в UI. null = источник недоступен.
 */
import { computed } from 'vue'
import Icon from '../components/Icon.vue'
import HintBox from '../components/HintBox.vue'
import { useBackendState } from '../composables/backendState'
import { validationStatusLabels } from '../api/labels'

const { channels, state, selectChannel, switching, isRunning, isBusy } = useBackendState()

const list = computed(() => channels.value)
const activeId = computed(() => state.value?.activeChannelId ?? null)

/** Переключение доступно только на запущенном VPN (Reload — операция running). */
const canSwitch = computed(() => isRunning.value && !isBusy.value && !switching.value)
</script>

<template>
  <div class="view">
    <header class="head">
      <h1>Каналы</h1>
      <p class="sub">
        Только каналы активного конфига с их фактическим статусом валидации. В selector
        попадают исключительно validated каналы (fail-closed).
      </p>
    </header>

    <div v-if="list === null" class="empty-block">
      <Icon name="warn" :size="18" />
      <p>
        Список каналов недоступен: backend bindings не сгенерированы. UI не показывает
        вымышленные серверы и страны.
      </p>
    </div>

    <div v-else-if="list.length === 0" class="empty-block">
      <Icon name="info" :size="18" />
      <p>В активном конфиге нет каналов. Protected selector пуст — запуск останется заблокированным (BLOCKED).</p>
    </div>

    <div v-else class="cards">
      <section
        v-for="c in list"
        :key="c.id"
        class="chan"
        :class="{ active: c.id === activeId }"
      >
        <div class="row1">
          <Icon name="server" :size="15" />
          <strong class="mono">{{ c.id }}</strong>
          <span v-if="c.id === activeId" class="pill active-pill">активный</span>
          <span v-if="!c.enabled" class="pill off">выключен</span>
        </div>
        <div class="row2">
          <span class="meta mono">{{ c.transport }} · {{ c.server }}:{{ c.port }}</span>
          <span class="pill" :class="c.validation">{{ validationStatusLabels[c.validation] }}</span>
        </div>
        <!-- A1.4: переключение через Reload + обязательный probe (fail-closed). -->
        <button
          v-if="c.id !== activeId && c.enabled"
          class="btn-switch"
          :disabled="!canSwitch"
          :title="canSwitch
            ? 'Переключиться на этот канал: новый конфиг пройдёт проверку защищённым probe до активации'
            : 'Переключение доступно при запущенном VPN'")
          @click="selectChannel(c.id)"
        >
          <Icon name="refresh" :size="13" />
          {{ switching ? 'Переключение…' : 'Переключиться' }}
        </button>
      </section>
    </div>

    <HintBox kind="info" title="Откуда берётся этот список">
      Дескрипторы каналов описаны в конфиге и манифесте (FR-002) и попадают сюда через backend.
      Страны, серверы и outbound-теги не зашиваются в интерфейс — только данные активного конфига.
    </HintBox>

    <HintBox kind="warn" title="Статусы — это уровни доказательства">
      «настроен» ≠ «проверен live». Канал включается в protected selector только после
      live-проверки (probe + egress). Статусы planned/configured/locally-tested/live-verified —
      из контракта PLAN.md FR-002.
    </HintBox>
  </div>
</template>

<style scoped>
.view {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.head h1 {
  font-size: 20px;
}
.sub {
  margin: 4px 0 0;
  color: var(--text-dim);
  font-size: 12.5px;
  max-width: 640px;
}

.empty-block {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 14px 16px;
  border: 1px dashed var(--border);
  border-radius: 12px;
  color: var(--text-dim);
  font-size: 12.5px;
}

.cards {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.chan {
  padding: 14px 16px;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--bg-panel);
}
.chan.active {
  border-color: #5b8cff55;
  background: linear-gradient(180deg, #5b8cff0f, transparent 70%), var(--bg-panel);
}

.btn-switch {
  margin-top: 10px;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--bg-hover);
  color: var(--text);
  font-size: 12px;
  cursor: pointer;
}
.btn-switch:hover:not(:disabled) {
  border-color: #5b8cff66;
  background: var(--bg-active);
}
.btn-switch:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.row1 {
  display: flex;
  align-items: center;
  gap: 9px;
}
.row1 strong {
  font-size: 13.5px;
}

.row2 {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 8px;
  flex-wrap: wrap;
}
.meta {
  font-size: 12px;
  color: var(--text-dim);
}

.pill {
  font-size: 11.5px;
  padding: 2px 9px;
  border-radius: 999px;
  background: var(--bg-hover);
  color: var(--text-dim);
}
.pill.active-pill {
  color: var(--accent);
  background: var(--accent-soft);
}
.pill.off {
  color: var(--text-faint);
}
.pill.live-verified {
  color: var(--ok);
  background: var(--ok-soft);
}
.pill.locally-tested {
  color: var(--accent);
  background: var(--accent-soft);
}
.pill.degraded {
  color: var(--warn);
  background: var(--warn-soft);
}
.pill.blocked,
.pill.retired {
  color: var(--danger);
  background: var(--danger-soft);
}
</style>
