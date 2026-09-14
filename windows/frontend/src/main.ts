import { createApp } from 'vue'
import App from './App.vue'
import './style.css'

// Ошибка верхней точки отказа: честный экран, а не белый лист.
window.addEventListener('unhandledrejection', (e) => {
  console.error('[app] unhandled rejection:', e.reason)
})

createApp(App).mount('#app')
