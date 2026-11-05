import { QuartzComponent, QuartzComponentConstructor } from "./types"
import Graph from "./Graph"

export default (() => {
  const WholeGraph = Graph({
    localGraph: { depth: -1 },
    globalGraph: { depth: -1 },
  })
  const KnowledgeHome: QuartzComponent = props => (
    <section class="knowledge-home" aria-label="Интерактивная карта базы знаний">
      <p class="graph-intro">Идеи, источники и инженерные решения — в одной карте.</p>
      <div class="knowledge-graph"><WholeGraph {...props} /></div>
      <div class="graph-caption">
        <span>Узел — материал. Линия — ссылка между материалами.</span>
        <span>Выберите узел · двигайте карту · приближайте для подробных подписей</span>
      </div>
      <p class="graph-keyboard">Общий граф: <kbd>Ctrl</kbd> + <kbd>G</kbd> · поиск: <kbd>Ctrl</kbd> + <kbd>K</kbd>. Все материалы также доступны в дереве слева.</p>
    </section>
  )
  KnowledgeHome.css = WholeGraph.css
  KnowledgeHome.afterDOMLoaded = WholeGraph.afterDOMLoaded
  return KnowledgeHome
}) satisfies QuartzComponentConstructor
