import { QuartzComponent, QuartzComponentConstructor } from "./types"
import { pathToRoot } from "../util/path"
import catalogue from "./releases.json"

type Release = {
  platform: string
  version: string
  channel: "stable" | "preview"
  publishedAt: string
  url: string
  notesUrl: string
  sha256: string
  sizeBytes: number
  requirements: string
}

export default ((options?: { compact?: boolean }) => {
  const Downloads: QuartzComponent = ({ fileData }) => {
    const root = pathToRoot(fileData.slug!)
    const releases = catalogue.releases as Release[]
    const PlatformTitle = options?.compact ? "h3" : "h2"
    return <div class={`release-downloads${options?.compact ? " compact" : ""}`}>
      <div class="platform-downloads">{catalogue.platforms.map(platform => {
        const release = releases.find(item => item.platform === platform.id)
        return <section class="platform-download" id={platform.id} aria-labelledby={`${platform.id}-title`}>
          <div class="platform-heading"><span class="platform-mark" aria-hidden="true">{platform.id === "windows" ? "⊞" : "▱"}</span><span class="platform-format">{platform.format}</span></div>
          <PlatformTitle id={`${platform.id}-title`}>{platform.name}</PlatformTitle>
          <span class={`release-status ${platform.state}`}>{release ? (release.channel === "stable" ? "Стабильный выпуск" : "Предварительный выпуск") : platform.label}</span>
          <p>{release ? "Описание изменений, требования и файл для выбранной платформы — в опубликованном выпуске." : platform.summary}</p>
          {release ? <div class="published-release">
            <p class="release-version">Версия {release.version} · {new Intl.DateTimeFormat("ru-RU", { timeZone: "UTC" }).format(new Date(release.publishedAt))}</p>
            <a class="product-button primary" href={release.url} data-no-popover="true">Скачать для {platform.name} <span aria-hidden="true">↓</span></a>
            <p class="release-requirements">{release.requirements} · {(release.sizeBytes / 1024 / 1024).toFixed(1)} МБ</p>
            <a href={release.notesUrl}>Что нового в этой версии →</a>
            {!options?.compact && <details><summary>Контрольная сумма SHA-256</summary><code class="release-hash">{release.sha256}</code><p>Сверьте сумму скачанного файла со значением для этого выпуска.</p></details>}
          </div> : <div class="release-unavailable"><span>Публичной загрузки пока нет</span><a href={`${root}/release-guide`}>Как будет устроен выпуск <span aria-hidden="true">→</span></a></div>}
          {!options?.compact && !release && <p class="platform-next"><strong>Следующий шаг</strong>{platform.next}</p>}
        </section>
      })}</div>
      {!options?.compact && <aside class="release-note"><span class="note-index">i</span><div><strong>{releases.length ? "Проверяйте версию и целостность файла" : "Сначала проверенный выпуск — затем загрузка"}</strong><p>{releases.length ? "Требования и контрольная сумма относятся к конкретному файлу. Перед обновлением прочитайте изменения и ограничения совместимости." : "Здесь появятся версия, требования к системе, описание изменений и контрольная сумма каждого файла. Сейчас публичные сборки не опубликованы."}</p><a href="https://github.com/cj-isus/snowden-system-v3/releases">Открыть GitHub Releases →</a><span class="release-updated">Статус на {new Intl.DateTimeFormat("ru-RU", { timeZone: "UTC" }).format(new Date(catalogue.updatedAt))}</span></div></aside>}
      {!options?.compact && releases.length > 0 && <section class="release-history"><h2>История выпусков</h2><ul>{releases.map(release => <li><a href={release.notesUrl}>{catalogue.platforms.find(p => p.id === release.platform)?.name} {release.version}</a><span>{release.channel === "stable" ? "Стабильный" : "Предварительный"} · {new Intl.DateTimeFormat("ru-RU", { timeZone: "UTC" }).format(new Date(release.publishedAt))}</span></li>)}</ul></section>}
    </div>
  }
  return Downloads
}) satisfies QuartzComponentConstructor
