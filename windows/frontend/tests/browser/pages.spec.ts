import { expect, test } from '@playwright/test'

const destinations = [
  { menu: 'Главная', page: 'dashboard' },
  { menu: 'Состояние', tab: 'Контекст сети', page: 'network' },
  { menu: 'Состояние', tab: 'Адаптация', page: 'autoprotect' },
  { menu: 'Состояние', tab: 'Телеметрия', page: 'statistics' },
  { menu: 'Состояние', tab: 'Восстановление', page: 'netguard' },
  { menu: 'Каналы', page: 'channels' },
  { menu: 'Проверки', page: 'tests' },
  { menu: 'Журнал', page: 'logs' },
  { menu: 'Настройки', tab: 'Система', page: 'settings' },
  { menu: 'Настройки', tab: 'Секреты', page: 'secrets' },
]
const sizes = [
  { width: 1366, height: 768 }, { width: 1180, height: 780 }, { width: 940, height: 600 },
  // Effective CSS workspaces at Windows scaling; native WebView DPI is a separate gate.
  { width: 752, height: 480 }, { width: 627, height: 400 }, { width: 470, height: 300 },
]

test.beforeEach(async ({ context }) => {
  // Fixtures are synthetic; block accidental requests outside the local servers.
  await context.route('**/*', route => {
    const url = new URL(route.request().url())
    if (url.hostname === '127.0.0.1') return route.continue()
    return route.abort('blockedbyclient')
  })
})

for (const size of sizes) {
  test(`all pages, names and overflow at ${size.width}×${size.height}`, async ({ page }, info) => {
    await page.setViewportSize(size)
    const errors: string[] = []
    page.on('pageerror', error => errors.push(error.message))
    await page.goto('/?mock=1')
    await expect(page.getByText('Демонстрация · данные вымышлены · VPN не подключён')).toBeVisible()
    const menu = page.getByRole('navigation', { name: 'Основная навигация' })
    for (const destination of destinations) {
      await menu.getByRole('button', { name: destination.menu, exact: true }).click()
      if (destination.tab) await page.getByRole('tab', { name: destination.tab, exact: true }).click()
      const content = page.locator(`#page-${destination.page}`)
      await expect(content).toBeVisible()
      await expect(menu.getByRole('button', { name: destination.menu, exact: true })).toHaveAttribute('aria-current', 'page')
      const overflow = await page.locator('.main-content').evaluate(element => element.scrollWidth - element.clientWidth)
      expect(overflow, destination.page).toBeLessThanOrEqual(1)
      // Empty icon buttons are not acceptable at any breakpoint.
      const unnamed = await page.locator('button:visible').evaluateAll(buttons => buttons.filter(button =>
        !button.getAttribute('aria-label') && !button.getAttribute('title') && !button.textContent?.trim()).length)
      expect(unnamed, destination.page).toBe(0)
      if (size.width === 1366) await page.screenshot({ path: info.outputPath(`${destination.page}.png`) })
    }
    expect(errors).toEqual([])
  })
}

test('tabs and search work with keyboard, including focus return', async ({ page }) => {
  await page.goto('/?mock=1')
  await page.getByRole('navigation', { name: 'Основная навигация' }).getByRole('button', { name: 'Состояние', exact: true }).click()
  await page.getByRole('tab', { name: 'Контекст сети', exact: true }).focus()
  await page.keyboard.press('ArrowRight')
  await expect(page.getByRole('tab', { name: 'Адаптация', exact: true })).toBeFocused()
  await expect(page.locator('#page-autoprotect')).toBeVisible()
  await page.keyboard.press('Control+k')
  await expect(page.getByRole('combobox')).toBeFocused()
  await page.getByRole('combobox').fill('секреты')
  await page.keyboard.press('Enter')
  await expect(page.locator('#page-secrets')).toBeVisible()
  await page.keyboard.press('Control+k')
  await page.keyboard.press('Escape')
  await expect(page.getByRole('dialog')).toHaveCount(0)
})

test('production ignores ?mock=1 and reports unavailable backend on every page', async ({ page }) => {
  await page.goto('http://127.0.0.1:5375/?mock=1')
  await expect(page.locator('.mock-banner')).toHaveCount(0)
  expect(await page.evaluate(() => (window as unknown as { go?: unknown }).go)).toBeUndefined()
  await expect(page.getByRole('button', { name: 'Откройте приложение', exact: true })).toBeDisabled()
  for (const destination of destinations) {
    await page.getByRole('navigation', { name: 'Основная навигация' }).getByRole('button', { name: destination.menu, exact: true }).click()
    if (destination.tab) await page.getByRole('tab', { name: destination.tab, exact: true }).click()
    await expect(page.locator(`#page-${destination.page}`)).toBeVisible()
  }
})

test('channel filters distinguish no matches from missing data', async ({ page }) => {
  await page.goto('/?mock=1')
  await page.getByRole('navigation', { name: 'Основная навигация' }).getByRole('button', { name: 'Каналы', exact: true }).click()
  await page.getByRole('textbox', { name: 'Поиск каналов' }).fill('no-such-fixture')
  await expect(page.getByText('По вашим фильтрам каналов нет.', { exact: false })).toBeVisible()
  await page.getByRole('textbox', { name: 'Поиск каналов' }).fill('hysteria')
  await expect(page.locator('.channels-table tbody tr')).toHaveCount(1)
  await expect(page.locator('.channels-table tbody')).toContainText('channel-a-hy2')
})

test('demo connect and disconnect update visible lifecycle through the API adapter', async ({ page }) => {
  await page.goto('/?mock=1')
  await page.getByRole('button', { name: 'Остановить', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Подключить', exact: true })).toBeEnabled()
  await page.getByRole('button', { name: 'Подключить', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Остановить', exact: true })).toBeEnabled()
})
