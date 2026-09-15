import { createApp } from 'vue'
import App from './App.vue'
import { MOCK_ACTIVE } from './api/mock'
import './style.css'

// Ошибка верхней точки отказа: честный экран, а не белый лист.
window.addEventListener('unhandledrejection', (e) => {
  console.error('[app] unhandled rejection:', e.reason)
})

// ?mock=1 — dev-превью с мок-данными (UI-план v3). В Wails-сборке мок
// невозможен (installMockIfRequested отказывается при живом window.go).
void MOCK_ACTIVE

createApp(App).mount('#app')
