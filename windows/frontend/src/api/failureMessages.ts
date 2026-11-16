/**
 * Display text chosen by the local action, never by an exception payload.
 * Rejected commands can have committed side effects; no rollback or completed
 * stop is inferred here. Do not append raw errors, paths, URLs or transport data.
 */
export const uiFailure = Object.freeze({
  lifecycleUnavailable: 'Упорядоченная команда недоступна или её ответ не подтверждён. Обновите состояние и используйте совместимый комплект приложения Windows и интерфейса.',
  lifecyclePendingState: 'Команда вернула ответ. Обновлённое состояние ещё не получено; применение и защита этим ответом не подтверждены.',
  load: 'Сведения не получены. Повторите обновление; причина отказа не установлена.',
  start: 'Результат запуска не подтверждён. Сверьте состояние подключения и журнал.',
  stop: 'Остановка не подтверждена. Сверьте состояние подключения и журнал.',
  switchChannel: 'Результат переключения не подтверждён. Сверьте текущий канал и состояние подключения.',
  enableTUN: 'Смена режима на TUN не подтверждена. Перечитайте сведения о текущем процессе.',
  disableTUN: 'Смена режима на прокси не подтверждена. Перечитайте сведения о текущем процессе.',
  probe: 'Результат команды проверки не подтверждён. Сверьте текущий отчёт перед повтором.',
  openLogs: 'Открытие папки журнала не подтверждено. Повторите действие из раздела «Журнал».',
  profilePreview: 'Превью публичного профиля не подтверждено. Проверьте источник, маршрут и данные независимого подтверждения, затем создайте новое превью.',
  profileApply: 'Результат применения профиля не подтверждён. Изменения уже могли быть сохранены. Сначала обновите сведения о профиле и подключении.',
  profileExport: 'Создание переноса не подтверждено. Получите новое превью и повторно задайте пароль.',
  profileExportMismatch: 'Перенос не совпадает с подтверждённым профилем. Получите новое превью; этот результат не используется.',
  profileSave: 'Сохранение файла не подтверждено. Файл уже мог появиться или измениться. Проверьте выбранное место перед новой попыткой.',
  transferPreview: 'Проверка переноса не подтверждена. Сверьте источник, режим, срок и фразу перед новой попыткой.',
  transferApply: 'Результат импорта не подтверждён. Изменения уже могли сохраниться. Перечитайте принятую версию и ограничения перед новой попыткой.',
  pinPreview: 'Превью списка отзывов не подтверждено. Проверьте источник, файл и выбранный маршрут, затем создайте новое превью.',
  pinApply: 'Результат применения списка отзывов не подтверждён. Изменения уже могли быть сохранены. Сначала обновите сведения о профиле; прежнее подключение не считается восстановленным.',
} as const)

export type ProfileActionKind = 'profile_preview' | 'profile_apply' | 'policy_preview' | 'policy_apply' | 'transfer_preview' | 'transfer_apply' | 'transfer_export' | 'transfer_save'
export type ProfileActionOutcome = 'not_sent' | 'cancelled' | 'received' | 'unread' | 'unconfirmed'

const actionTitles: Readonly<Record<ProfileActionKind, string>> = Object.freeze({
  profile_preview: 'Проверка публичного профиля', profile_apply: 'Применение профиля',
  policy_preview: 'Проверка списка отзывов', policy_apply: 'Применение списка отзывов',
  transfer_preview: 'Проверка переноса', transfer_apply: 'Импорт переноса',
  transfer_export: 'Создание переноса', transfer_save: 'Сохранение переноса',
})
const actionFailures: Readonly<Record<ProfileActionKind, string>> = Object.freeze({
  profile_preview: uiFailure.profilePreview, profile_apply: uiFailure.profileApply,
  policy_preview: uiFailure.pinPreview, policy_apply: uiFailure.pinApply,
  transfer_preview: uiFailure.transferPreview, transfer_apply: uiFailure.transferApply,
  transfer_export: uiFailure.profileExport, transfer_save: uiFailure.profileSave,
})

// Only a local pre-invocation guard can create this evidence. Never classify a
// Wails error, cancellation string, exception message or failed DTO as not sent.
// It concerns that target action; earlier stages of a macro may have run.
const localRefusals = new WeakMap<object, ProfileActionKind>()
export function profileActionNotSent(action: ProfileActionKind): Error {
  const error = new Error(`${actionTitles[action]}: запрос на это действие не передан приложению. Нужна совместимая версия Windows-приложения и интерфейса.`)
  localRefusals.set(error, action)
  return error
}
export function profileActionTitle(action: ProfileActionKind): string { return actionTitles[action] }
export function profileActionFailure(action: ProfileActionKind, error: unknown): { outcome: 'not_sent' | 'unconfirmed'; message: string } {
  const notSent = typeof error === 'object' && error !== null && localRefusals.get(error) === action
  return { outcome: notSent ? 'not_sent' : 'unconfirmed',
    message: notSent ? `${actionTitles[action]}: запрос на это действие не отправлен. Обновите совместимый комплект приложения и интерфейса.` : actionFailures[action] }
}
