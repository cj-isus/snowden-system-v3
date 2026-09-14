/**
 * Словарь фактических состояний (RU). Никаких синонимов «подключено/соединено»:
 * слова должны совпадать со states контракта 1:1 (AGENTS.md §3.6).
 */
import type { BlockedReason, LifecycleState, ProbeStepStatus, SecretKind, SecretBackend, ValidationStatus } from './contract'

export const lifecycleLabels: Record<LifecycleState, string> = {
  stopped: 'Остановлен',
  starting: 'Запускается…',
  running: 'Работает',
  stopping: 'Останавливается…',
  reloading: 'Переключение канала…',
  error: 'Ошибка',
}

export const blockedReasonLabels: Record<BlockedReason, string> = {
  no_validated_channel: 'нет проверенного канала',
  probe_failed: 'probe не пройден',
  config_invalid: 'конфиг невалиден',
  all_channels_failed: 'все каналы недоступны (failover исчерпан)',
}

export const probeStepStatusLabels: Record<ProbeStepStatus, string> = {
  pending: 'ожидает',
  running: 'выполняется',
  pass: 'пройдено',
  fail: 'провалено',
  skipped: 'пропущено',
}

export const validationStatusLabels: Record<ValidationStatus, string> = {
  planned: 'запланирован',
  configured: 'настроен',
  'locally-tested': 'проверен локально',
  'live-verified': 'проверен live',
  degraded: 'деградирован',
  blocked: 'заблокирован',
  retired: 'выведен',
}

export const secretKindLabels: Record<SecretKind, string> = {
  'vless-uuid': 'UUID VLESS',
  'hy2-password': 'Пароль HY2',
  'hy2-obfs-password': 'Пароль obfs (HY2)',
  'vps-ssh-key': 'SSH-ключ VPS',
  'cf-api-token': 'API-токен Cloudflare',
  custom: 'Своё значение',
}

export const secretKindHints: Record<SecretKind, string> = {
  'vless-uuid':
    'Идентификатор пользователя VLESS на сервере. Значение знает только сервер и клиент; в чат и скриншоты не вставлять.',
  'hy2-password':
    'Пароль аутентификации Hysteria2 (UDP-канал). Сверка с сервером — по SHA256-хешу, не по значению.',
  'hy2-obfs-password':
    'Пароль обфускации salamander для HY2. Должен совпадать с серверным; иначе UDP-пакеты не распознаются.',
  'vps-ssh-key':
    'Приватный SSH-ключ доступа к VPS. Значение никогда не попадает в логи и diagnostics.',
  'cf-api-token':
    'API-токен Cloudflare. Утечка в git-историю = компрометация: токен нужно отозвать и перевыпустить.',
  custom: 'Произвольный секрет для тестов. Хранится локально, в git не попадает.',
}

export const secretBackendLabels: Record<SecretBackend, string> = {
  file: 'Файл (secrets/)',
  dpapi: 'DPAPI (шифрование Windows)',
  keyring: 'Системное хранилище',
}

export const proxyModeLabels: Record<'socks' | 'tun', string> = {
  socks: 'SOCKS :1080',
  tun: 'TUN (нужен admin)',
}

/** Дата/время для журналов. Возвращает '—' вместо пустой строки (unavailable как unavailable). */
export function fmtTime(iso: string | null | undefined): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return d.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

export function fmtDateTime(iso: string | null | undefined): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return d.toLocaleString('ru-RU', {
    day: '2-digit',
    month: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}
