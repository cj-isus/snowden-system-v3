/**
 * Единая навигация между разделами (UI-план v3).
 *
 * Дефект v2: Dashboard/Network/Settings эмитили 'navigate', но App.vue не
 * слушал — кнопки не работали. Теперь: App.vue вызывает provideNav(navTo);
 * любой компонент берёт navTo() из inject — без цепочек событий и без
 * возможности «потерять» обработчик.
 */
import { inject, provide } from 'vue'
import type { NavId } from '../components/NavRail.vue'

const NAV_KEY: unique symbol = Symbol('nav')

export type NavTo = (id: NavId) => void

/** Вызывает App.vue один раз: регистрирует реализацию перехода. */
export function provideNav(navTo: NavTo): void {
  provide(NAV_KEY, navTo)
}

/** Для любых компонентов: переход в раздел. */
export function useNav(): { navTo: NavTo } {
  const navTo = inject<NavTo>(NAV_KEY)
  if (!navTo) {
    // Провайдер не установлен — ошибка интеграции, а не молчаливая смерть кнопки.
    throw new Error('nav: App.vue не установил provideNav — навигация недоступна')
  }
  return { navTo }
}
