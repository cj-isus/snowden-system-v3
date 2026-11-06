import { QuartzComponent, QuartzComponentConstructor } from "./types"
import { pathToRoot } from "../util/path"

export default (() => {
  const Navigation: QuartzComponent = ({ fileData }) => {
    const root = pathToRoot(fileData.slug!)
    const slug = fileData.slug ?? "index"
    const items = [
      ["index", "Продукт", root],
      ["downloads", "Загрузки и обновления", `${root}/downloads`],
      ["graph", "Граф знаний", `${root}/graph`],
      ["theory/READING-GUIDE", "Читать теорию", `${root}/theory/READING-GUIDE`],
    ]
    return <nav class="product-nav" aria-label="Разделы Snowden">{items.map(([id, label, href]) =>
      <a href={href} aria-current={slug === id ? "page" : undefined}>{label}<span aria-hidden="true">→</span></a>,
    )}</nav>
  }
  return Navigation
}) satisfies QuartzComponentConstructor
