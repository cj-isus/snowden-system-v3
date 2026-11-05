import { QuartzComponent, QuartzComponentConstructor } from "./types"
import Graph from "./Graph"

export default (() => {
  const WholeGraph = Graph({
    localGraph: { depth: -1, drag: true, zoom: true, scale: 0.68, repelForce: 0.7, centerForce: 0.12, linkDistance: 48, fontSize: 0.72, showTags: false, focusOnHover: true, enableRadial: false },
    globalGraph: { depth: -1, drag: true, zoom: true, scale: 0.8, repelForce: 0.7, centerForce: 0.12, linkDistance: 48, fontSize: 0.8, showTags: false, focusOnHover: true, enableRadial: false },
  })
  const KnowledgeHome: QuartzComponent = props => (
    <section class="knowledge-home" aria-label="Интерактивная карта базы знаний">
      <p class="graph-intro">Идеи, источники и инженерные решения — в одной карте.</p>
      <div class="knowledge-graph"><WholeGraph {...props} /></div>
      <div class="graph-caption">
        <span>Узел — материал. Линия — ссылка между материалами.</span>
        <span>Нажмите на узел, чтобы читать · перетаскивайте · меняйте масштаб</span>
      </div>
      <p class="graph-keyboard">Общий граф: <kbd>Ctrl</kbd> + <kbd>G</kbd> · поиск: <kbd>Ctrl</kbd> + <kbd>K</kbd>. Все материалы также доступны в дереве слева.</p>
    </section>
  )
  KnowledgeHome.css = WholeGraph.css
  KnowledgeHome.afterDOMLoaded = WholeGraph.afterDOMLoaded
  return KnowledgeHome
}) satisfies QuartzComponentConstructor
