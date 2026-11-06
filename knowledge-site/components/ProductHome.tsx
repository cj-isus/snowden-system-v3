import { QuartzComponent, QuartzComponentConstructor } from "./types"
import { pathToRoot } from "../util/path"
import ReleaseDownloads from "./ReleaseDownloads"

export default (() => {
  const Downloads = ReleaseDownloads({ compact: true })
  const Home: QuartzComponent = props => {
    const root = pathToRoot(props.fileData.slug!)
    return <div class="product-home">
      <section class="product-hero" aria-labelledby="product-title">
        <div class="hero-copy">
          <p class="product-eyebrow"><span aria-hidden="true" />Snowden System / VPN</p>
          <h1 id="product-title">Связь начинается<br />с доверия.</h1>
          <p class="hero-description">Создаём адаптивный VPN для Windows и Android. С открытой базой знаний, понятными обновлениями и вниманием к надёжности.</p>
          <div class="product-actions"><a class="product-button primary" href={`${root}/downloads`}>Загрузки и обновления <span aria-hidden="true">→</span></a><a class="product-button secondary" href={`${root}/graph`}>Исследовать теорию</a></div>
          <p class="hero-stage">Готовимся к первому публичному выпуску</p>
        </div>
        <div class="hero-visual" aria-hidden="true">
          <svg viewBox="0 0 360 330" fill="none"><circle cx="180" cy="158" r="132" /><circle cx="180" cy="158" r="94" /><path class="visual-path" d="M48 158H119M241 158H312M180 26V97M180 219V290" /><path class="visual-shield" d="M180 89 234 111V161C234 199 201 225 180 237 159 225 126 199 126 161V111L180 89Z" /><path class="visual-check" d="m158 161 15 15 30-32" /><circle class="visual-node" cx="48" cy="158" r="6" /><circle class="visual-node" cx="312" cy="158" r="6" /><circle class="visual-node" cx="180" cy="26" r="6" /><circle class="visual-node" cx="180" cy="290" r="6" /></svg>
          <span class="visual-caption">Одна система. Две платформы.</span>
        </div>
      </section>
      <section class="product-section" aria-labelledby="apps-title">
        <div class="section-heading"><div><p class="product-eyebrow">01 / Приложения</p><h2 id="apps-title">На вашем устройстве</h2></div><a href={`${root}/downloads`}>Центр загрузок <span aria-hidden="true">→</span></a></div>
        <Downloads {...props} />
      </section>
      <section class="knowledge-entry product-section" aria-labelledby="knowledge-title">
        <div><p class="product-eyebrow">02 / Открытая база знаний</p><h2 id="knowledge-title">За каждым решением —<br />объяснение.</h2><p>Теория сетей, исследования, первоисточники и планы разработки. Читайте по темам или находите связи на интерактивном графе.</p><div class="product-actions"><a class="product-button secondary" href={`${root}/graph`}>Открыть граф <span aria-hidden="true">→</span></a><a href={`${root}/theory/READING-GUIDE`}>С чего начать чтение</a></div></div>
        <div class="knowledge-paths"><a href={`${root}/theory`}>Теория и источники <span>01 →</span></a><a href={`${root}/planning/knowledge-application/PC-PLAN`}>План Windows <span>02 →</span></a><a href={`${root}/planning/knowledge-application/ANDROID-PLAN`}>План Android <span>03 →</span></a><a href={`${root}/release-guide`}>Установка и обновление <span>04 →</span></a></div>
      </section>
      <section class="release-footer-note"><span>Развиваем продукт открыто.</span><p>Готовность приложения, доступность сервера и работа VPN на устройстве проверяются отдельно. Сведения о выпуске будут появляться здесь по мере готовности.</p></section>
    </div>
  }
  return Home
}) satisfies QuartzComponentConstructor
