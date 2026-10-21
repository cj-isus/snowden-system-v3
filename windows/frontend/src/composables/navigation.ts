import { computed, ref } from 'vue'
import type { IconName } from '../components/Icon.vue'

/** Menu sections own pages. Sidebar, tabs and search read this same registry. */
export type PageId = 'dashboard' | 'network' | 'autoprotect' | 'statistics' | 'netguard'
  | 'channels' | 'tests' | 'logs' | 'settings' | 'secrets'
export type SectionId = 'dashboard' | 'network' | 'channels' | 'tests' | 'logs' | 'settings'
export type Page = { id: PageId; label: string; caption: string; icon: IconName }
export type Section = { id: SectionId; label: string; icon: IconName; hint: string; pages: Page[] }

export const sections: Section[] = [
  { id: 'dashboard', label: 'Главная', icon: 'dashboard', hint: 'Состояние защиты и текущий маршрут', pages: [
    { id: 'dashboard', label: 'Обзор защиты', caption: 'Подключение, маршрут и последняя проверка', icon: 'dashboard' },
  ] },
  { id: 'network', label: 'Состояние', icon: 'pulse', hint: 'Сеть и адаптивная защита', pages: [
    { id: 'network', label: 'Контекст сети', caption: 'Подключение и DNS', icon: 'pulse' },
    { id: 'autoprotect', label: 'Адаптация', caption: 'Здоровье и отказоустойчивость', icon: 'shield' },
    { id: 'statistics', label: 'Телеметрия', caption: 'Трафик и измерения', icon: 'stats' },
    { id: 'netguard', label: 'Восстановление', caption: 'Прокси и NetGuard', icon: 'refresh' },
  ] },
  { id: 'channels', label: 'Каналы', icon: 'server', hint: 'Маршруты и статусы', pages: [
    { id: 'channels', label: 'Реестр маршрутов', caption: 'Поиск и переключение каналов', icon: 'server' },
  ] },
  { id: 'tests', label: 'Проверки', icon: 'shield', hint: 'Диагностика защищённого пути', pages: [
    { id: 'tests', label: 'Диагностика пути', caption: 'Проверки окружения и соединения', icon: 'flask' },
  ] },
  { id: 'logs', label: 'Журнал', icon: 'logs', hint: 'События приложения и ядра', pages: [
    { id: 'logs', label: 'События', caption: 'Поиск по журналу приложения', icon: 'logs' },
  ] },
  { id: 'settings', label: 'Настройки', icon: 'key', hint: 'Система, профиль и секреты', pages: [
    { id: 'settings', label: 'Система', caption: 'Запуск, профиль и обновления', icon: 'key' },
    { id: 'secrets', label: 'Секреты', caption: 'Локальное хранилище', icon: 'key' },
  ] },
]

export const pages = sections.flatMap(section => section.pages)
export function sectionForPage(id: PageId): Section {
  const section = sections.find(section => section.pages.some(page => page.id === id))
  if (!section) throw new Error(`Unknown page: ${id}`)
  return section
}

export function useNavigation() {
  const active = ref<PageId>('dashboard')
  const section = computed(() => sectionForPage(active.value).id)
  const remembered = new Map<SectionId, PageId>()
  function navigate(id: PageId) {
    active.value = id
    remembered.set(sectionForPage(id).id, id)
  }
  function openSection(id: SectionId) {
    navigate(remembered.get(id) ?? sections.find(section => section.id === id)!.pages[0].id)
  }
  return { active, section, navigate, openSection }
}
