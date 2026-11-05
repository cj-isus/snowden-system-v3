import { QuartzComponent, QuartzComponentConstructor } from "./types"

export default (() => {
  const SourceLink: QuartzComponent = ({ fileData }) => {
    const source = fileData.frontmatter?.sourceUrl
    if (typeof source !== "string" || !source.startsWith("https://github.com/cj-isus/snowden-system-v3/blob/")) return null
    return <div class="note-origin"><a href={source} target="_blank" rel="noopener noreferrer">Исходный Markdown ↗</a></div>
  }
  return SourceLink
}) satisfies QuartzComponentConstructor
