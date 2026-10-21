import { createApp } from 'vue'
import App from './App.vue'
import { markMockPreview } from './api/preview'
import './style.css'

// Ошибка верхней точки отказа: честный экран, а не белый лист.
window.addEventListener('unhandledrejection', (e) => {
  console.error('[app] unhandled rejection:', e.reason)
})

// ?mock=1 — dev-превью с мок-данными (UI-план v3). В Wails-сборке мок
// невозможен (installMockIfRequested отказывается при живом window.go).
if (import.meta.env.DEV && new URLSearchParams(location.search).get('mock') === '1') {
  const { MOCK_ACTIVE } = await import('./api/mock')
  if (MOCK_ACTIVE) markMockPreview()
}

createApp(App).mount('#app')
