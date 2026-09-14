<script setup lang="ts">
/**
 * Инлайн-SVG иконки (никаких icon-зависимостей). Токены — в IconName.
 */
const props = defineProps<{
  name: IconName
  size?: number
}>()

export type IconName =
  | 'dashboard'
  | 'flask'
  | 'key'
  | 'server'
  | 'logs'
  | 'shield'
  | 'shield-warn'
  | 'play'
  | 'stop'
  | 'refresh'
  | 'copy'
  | 'check'
  | 'cross'
  | 'info'
  | 'warn'
  | 'error'
  | 'lock'
  | 'chevron'
  | 'external'
  | 'eye'
  | 'trash'
  | 'pulse'

const paths: Record<IconName, string> = {
  dashboard: 'M3 3h8v10H3zM13 3h8v6h-8zM13 11h8v10h-8zM3 15h8v6H3z',
  flask: 'M9 3h6M10 3v5l-5.5 9A2 2 0 0 0 6.2 20h11.6a2 2 0 0 0 1.7-3L14 8V3M8 14h8',
  key: 'M14 10a4 4 0 1 0-8 0 4 4 0 0 0 8 0zM14 10l6 6M17 16h3v3h-3z',
  server: 'M4 4h16v6H4zM4 14h16v6H4zM8 7h.01M8 17h.01',
  logs: 'M6 3h9l5 5v13H6zM15 3v5h5M9 12h6M9 16h6',
  shield: 'M12 3l7 3v5c0 5-3 8.2-7 10-4-1.8-7-5-7-10V6l7-3z',
  'shield-warn': 'M12 3l7 3v5c0 5-3 8.2-7 10-4-1.8-7-5-7-10V6l7-3zM12 8v4M12 15h.01',
  play: 'M7 5l12 7-12 7z',
  stop: 'M6 6h12v12H6z',
  refresh: 'M20 11A8 8 0 0 0 6.3 6.3L4 8.5M4 13a8 8 0 0 0 13.7 4.7L20 15.5M4 4v4.5H8.5M20 20v-4.5H15.5',
  copy: 'M9 9h10v10a2 2 0 0 1-2 2H9a2 2 0 0 1-2-2V9zM9 9V7a2 2 0 0 1 2-2h6a2 2 0 0 1 2 2v2',
  check: 'M5 13l4 4L19 7',
  cross: 'M6 6l12 12M18 6L6 18',
  info: 'M12 11v5M12 8h.01M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0z',
  warn: 'M12 4l9 15H3l9-15zM12 11v3M12 17h.01',
  error: 'M12 22a10 10 0 1 1 0-20 10 10 0 0 1 0 20zM12 7v6M12 16h.01',
  lock: 'M6 11V8a6 6 0 1 1 12 0v3M5 11h14v9H5zM12 15v2',
  chevron: 'M9 6l6 6-6 6',
  external: 'M14 4h6v6M20 4l-8 8M18 13v6H5V6h6',
  eye: 'M2 12s3.5-6 10-6 10 6 10 6-3.5 6-10 6-10-6-10-6zM12 12h.01M9 12a3 3 0 1 0 6 0 3 3 0 0 0-6 0z',
  trash: 'M4 7h16M9 7V5a2 2 0 0 1 2-2h2a2 2 0 0 1 2 2v2M7 7l1 13h8l1-13M10 11v5M14 11v5',
  pulse: 'M3 12h4l2-7 4 14 2-7h6',
}

const sizeNum = (): number => props.size ?? 16
</script>

<template>
  <svg
    :width="sizeNum()"
    :height="sizeNum()"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="1.7"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <path :d="paths[props.name]" />
  </svg>
</template>
