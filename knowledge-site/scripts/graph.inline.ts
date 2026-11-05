import { getFullSlug, resolveRelative, simplifySlug, type FullSlug, type SimpleSlug } from "../../util/path"

interface GraphNode { id: SimpleSlug; title: string; x: number; y: number; degree: number }
interface GraphData { nodes: GraphNode[]; edges: [GraphNode, GraphNode][]; neighbours: Map<string, Set<string>> }
type NavigationEntry = { title: string; links?: string[]; x: number; y: number; degree: number }
const simple = (slug: string) => simplifySlug(slug as FullSlug)
let graphPromise: Promise<GraphData> | undefined
function graphData() {
  return graphPromise ??= fetchData.then(data => {
    const nodes = Object.entries(data as Record<string, NavigationEntry>).map(([id, entry]) => ({ id: simple(id), title: entry.title, x: entry.x, y: entry.y, degree: entry.degree }))
    const byId = new Map(nodes.map(node => [node.id, node]))
    const neighbours = new Map(nodes.map(node => [node.id as string, new Set<string>()]))
    const edges: GraphData["edges"] = [], seen = new Set<string>()
    for (const [slug, entry] of Object.entries(data as Record<string, NavigationEntry>)) {
      const source = byId.get(simple(slug))!
      for (const link of entry.links ?? []) {
        const target = byId.get(simple(link))
        if (!target || source === target) continue
        const key = JSON.stringify([source.id, target.id].sort())
        if (seen.has(key)) continue
        seen.add(key); edges.push([source, target])
        neighbours.get(source.id)!.add(target.id); neighbours.get(target.id)!.add(source.id)
      }
    }
    return { nodes, edges, neighbours }
  })
}

function button(label: string, text: string, run: () => void) {
  const element = document.createElement("button")
  element.type = "button"; element.setAttribute("aria-label", label); element.title = label
  element.textContent = text; element.addEventListener("click", run)
  return element
}

// Canvas draws only after an interaction, a resize or a visibility/theme change. No idle loop.
function mountGraph(container: HTMLElement, slug: FullSlug) {
  const abort = new AbortController(), signal = abort.signal
  let disposed = false, visible = false, frame = 0, dirty = true, initialized = false, initializing = false
  let width = 0, height = 0, fitScale = 1, scale = 1, offsetX = 0, offsetY = 0
  let data: GraphData, canvas: HTMLCanvasElement, context: CanvasRenderingContext2D
  let selected: GraphNode | undefined, hover: GraphNode | undefined
  let resize: ResizeObserver | undefined
  const coarse = matchMedia("(pointer: coarse)").matches
  const current = simple(slug)
  const depth = (JSON.parse(container.dataset.cfg ?? "{}") as {depth?: number}).depth ?? 1
  const loading = document.createElement("p")
  loading.className = "graph-loading"; loading.textContent = "Загрузка карты…"; container.replaceChildren(loading)
  const controls = document.createElement("div"), selection = document.createElement("div")
  controls.className = "graph-controls"; controls.setAttribute("aria-label", "Масштаб графа")
  selection.className = "graph-selection"; selection.hidden = true
  const selectionTitle = document.createElement("span"), selectionLink = document.createElement("a")
  selectionLink.textContent = "Открыть →"; selectionLink.className = "internal"
  selection.append(selectionTitle, selectionLink)

  function requestDraw() {
    dirty = true
    if (frame || !initialized || !visible || document.hidden || disposed) return
    frame = requestAnimationFrame(() => { frame = 0; if (!disposed && visible && !document.hidden && dirty) { dirty = false; draw() } })
  }
  function screen(node: GraphNode) { return { x: node.x * scale + offsetX, y: node.y * scale + offsetY } }
  function fit() {
    if (!data?.nodes.length || !width || !height) return
    const xs = data.nodes.map(n => n.x), ys = data.nodes.map(n => n.y)
    const minX = Math.min(...xs), maxX = Math.max(...xs), minY = Math.min(...ys), maxY = Math.max(...ys)
    fitScale = Math.min((width - 52) / Math.max(50, maxX - minX + 30), (height - 68) / Math.max(50, maxY - minY + 30))
    scale = fitScale; offsetX = width / 2 - (minX + maxX) / 2 * scale; offsetY = height / 2 - (minY + maxY) / 2 * scale
    requestDraw()
  }
  function zoom(factor: number, x = width / 2, y = height / 2) {
    const next = Math.max(fitScale * .35, Math.min(fitScale * 30, scale * factor)), ratio = next / scale
    offsetX = x - (x - offsetX) * ratio; offsetY = y - (y - offsetY) * ratio; scale = next
    requestDraw()
  }
  function select(node?: GraphNode) {
    selected = node; selection.hidden = !node
    if (node) { selectionTitle.textContent = node.title; selectionLink.href = resolveRelative(slug, node.id) }
    requestDraw()
  }
  function open(node: GraphNode) { window.spaNavigate(new URL(resolveRelative(slug, node.id), location.href)) }
  function hit(x: number, y: number) {
    let closest: GraphNode | undefined, distance = coarse ? 18 : 9
    for (const node of data.nodes) {
      const p = screen(node), d = Math.hypot(p.x - x, p.y - y)
      const radius = Math.max(distance, Math.min(13, (2 + Math.sqrt(node.degree)) * scale))
      if (d <= radius && (!closest || d < distance)) { closest = node; distance = d }
    }
    return closest
  }

  function draw() {
    canvas.dataset.renderCount = String(Number(canvas.dataset.renderCount ?? 0) + 1)
    const style = getComputedStyle(document.documentElement)
    const color = (name: string) => style.getPropertyValue(name).trim()
    const ratio = Math.min(devicePixelRatio || 1, coarse ? 1.5 : 2)
    context.setTransform(ratio, 0, 0, ratio, 0, 0); context.clearRect(0, 0, width, height)
    const active = hover ?? selected, near = active ? data.neighbours.get(active.id)! : undefined
    const activeNode = (node: GraphNode) => node === active || near?.has(node.id)
    // Batch paths instead of rebuilding a GPU object for every edge on every animation frame.
    for (const highlight of [false, true]) {
      if (highlight && !active) continue
      context.beginPath()
      for (const [a, b] of data.edges) {
        if (!!(active && (a === active || b === active)) !== highlight) continue
        const p = screen(a), q = screen(b)
        if ((p.x < 0 && q.x < 0) || (p.x > width && q.x > width) || (p.y < 0 && q.y < 0) || (p.y > height && q.y > height)) continue
        context.moveTo(p.x, p.y); context.lineTo(q.x, q.y)
      }
      context.strokeStyle = color(highlight ? "--secondary" : "--gray")
      context.globalAlpha = highlight ? .65 : active ? .07 : .21
      context.lineWidth = highlight ? 1.2 : .65; context.stroke()
    }
    const shown = data.nodes.filter(node => { const p = screen(node); return p.x > -20 && p.y > -20 && p.x < width + 20 && p.y < height + 20 })
    for (const node of shown) {
      const p = screen(node), radius = Math.max(1.6, Math.min(13, (2 + Math.sqrt(node.degree)) * scale))
      context.globalAlpha = active && !activeNode(node) ? .2 : 1
      context.fillStyle = color(node.id === current || node === active ? "--secondary" : activeNode(node) ? "--tertiary" : "--gray")
      context.beginPath(); context.arc(p.x, p.y, radius, 0, Math.PI * 2); context.fill()
    }
    context.globalAlpha = 1
    // Labels grow with zoom, appear progressively, and compete for screen space.
    const zoomRatio = scale / fitScale
    const fontSize = Math.max(8, Math.min(26, 11 * Math.sqrt(zoomRatio)))
    canvas.dataset.labelFontSize = fontSize.toFixed(2)
    canvas.dataset.zoomRatio = zoomRatio.toFixed(2)
    const candidates = [...shown].sort((a, b) => Number(b === active) - Number(a === active) || Number(b.id === current) - Number(a.id === current) || Number(!!activeNode(b)) - Number(!!activeNode(a)) || b.degree - a.degree)
    const boxes: {left: number; top: number; right: number; bottom: number}[] = []
    const budget = coarse ? Math.min(16, Math.floor(3 + zoomRatio * 3)) : Math.min(42, Math.floor(3 + zoomRatio * 4))
    let count = 0
    for (const node of candidates) {
      if (count >= budget) break
      const important = node === active || node.id === current
      if (!important && zoomRatio < 1.3 && node.degree < 10) continue
      const p = screen(node), font = important ? Math.max(12, fontSize + 1) : fontSize
      const limit = important ? 46 : zoomRatio > 3 ? 35 : 25
      const text = node.title.length > limit ? node.title.slice(0, limit - 1).trimEnd() + "…" : node.title
      context.font = `${important ? "600" : "400"} ${font}px ${style.getPropertyValue("--bodyFont")}`
      const textWidth = context.measureText(text).width, y = p.y - Math.max(6, (2 + Math.sqrt(node.degree)) * scale) - 7
      const box = { left: p.x - textWidth / 2 - 4, right: p.x + textWidth / 2 + 4, top: y - font - 3, bottom: y + 4 }
      if (box.left < 4 || box.right > width - 4 || box.top < 4 || box.bottom > height - 44) continue
      if (!important && boxes.some(b => box.left < b.right && box.right > b.left && box.top < b.bottom && box.bottom > b.top)) continue
      boxes.push(box); count++
      context.textAlign = "center"; context.lineJoin = "round"; context.lineWidth = 4
      context.strokeStyle = color("--light"); context.strokeText(text, p.x, y)
      context.fillStyle = color(important ? "--dark" : "--darkgray"); context.fillText(text, p.x, y)
    }
    canvas.dataset.labelCount = String(count)
  }

  async function initialize() {
    if (initialized || initializing || disposed) return
    initializing = true
    try {
      const all = await graphData()
      if (disposed) return
      let included = new Set(all.nodes.map(n => n.id as string))
      if (depth >= 0) {
        included = new Set([current]); let frontier = new Set([current as string])
        for (let d = 0; d < depth; d++) {
          const next = new Set<string>()
          for (const id of frontier) for (const neighbour of all.neighbours.get(id) ?? []) if (!included.has(neighbour)) next.add(neighbour)
          for (const id of next) included.add(id)
          frontier = next
        }
      }
      const nodes = all.nodes.filter(n => included.has(n.id)).map(n => ({...n}))
      const byId = new Map(nodes.map(n => [n.id, n]))
      data = {nodes, edges: all.edges.filter(([a,b]) => included.has(a.id) && included.has(b.id)).map(([a,b]): [GraphNode, GraphNode] => [byId.get(a.id)!,byId.get(b.id)!]), neighbours: all.neighbours}
      canvas = document.createElement("canvas"); canvas.className = "knowledge-canvas"; canvas.tabIndex = 0
      canvas.setAttribute("role", "img"); canvas.setAttribute("aria-label", "Карта связей. Плюс и минус — масштаб, стрелки — перемещение, 0 — вписать. Все материалы доступны в дереве и поиске.")
      const ctx = canvas.getContext("2d")
      if (!ctx) throw new Error("Canvas unavailable")
      context = ctx; initialized = true
      controls.append(button("Отдалить", "−", () => zoom(1 / 1.4)), button("Вписать граф", "⤢", fit), button("Приблизить", "+", () => zoom(1.4)))
      container.replaceChildren(canvas, controls, selection)
      const updateSize = () => {
        const w = container.clientWidth, h = container.clientHeight
        if (!w || !h || (w === width && h === height)) return
        width = w; height = h
        const dpr = Math.min(devicePixelRatio || 1, coarse ? 1.5 : 2)
        canvas.width = Math.round(width * dpr); canvas.height = Math.round(height * dpr)
        fit()
      }
      resize = new ResizeObserver(updateSize); resize.observe(container); updateSize()

      const pointers = new Map<number, {x: number; y: number}>()
      let pressed: GraphNode | undefined, start = {x: 0, y: 0}, moved = false, multiple = false
      const point = (event: PointerEvent | WheelEvent) => { const r = canvas.getBoundingClientRect(); return {x: event.clientX - r.left, y: event.clientY - r.top} }
      canvas.addEventListener("pointerdown", event => {
        if (event.button !== 0) return
        const p = point(event); pointers.set(event.pointerId, p); canvas.setPointerCapture(event.pointerId)
        if (pointers.size === 1) { pressed = hit(p.x, p.y); start = p; moved = false; multiple = false }
        else { multiple = true; moved = true }
        canvas.dataset.pointedNode = pressed?.title ?? ""
      }, {signal})
      canvas.addEventListener("pointermove", event => {
        const p = point(event), previous = pointers.get(event.pointerId)
        if (!previous) { const next = hit(p.x, p.y); if (next !== hover) { hover = next; canvas.style.cursor = next ? "pointer" : "grab"; requestDraw() } return }
        if (Math.hypot(p.x - start.x, p.y - start.y) > 5) moved = true
        if (pointers.size === 2) {
          const other = [...pointers.entries()].find(([id]) => id !== event.pointerId)![1]
          const oldDistance = Math.hypot(previous.x - other.x, previous.y - other.y)
          const newDistance = Math.hypot(p.x - other.x, p.y - other.y)
          if (oldDistance > 1) zoom(newDistance / oldDistance, (previous.x + other.x) / 2, (previous.y + other.y) / 2)
          offsetX += (p.x - previous.x) / 2; offsetY += (p.y - previous.y) / 2
        } else if (moved) {
          if (pressed && !multiple && event.pointerType !== "touch") { pressed.x += (p.x - previous.x) / scale; pressed.y += (p.y - previous.y) / scale }
          else { offsetX += p.x - previous.x; offsetY += p.y - previous.y }
        }
        pointers.set(event.pointerId, p); requestDraw()
      }, {signal})
      const release = (event: PointerEvent) => {
        if (!pointers.has(event.pointerId)) return
        pointers.delete(event.pointerId)
        if (event.type === "pointerup" && !moved && !multiple) {
          if (pressed) { if (event.pointerType === "touch" && selected !== pressed) select(pressed); else open(pressed) }
          else select(undefined)
        }
        if (canvas.hasPointerCapture(event.pointerId)) canvas.releasePointerCapture(event.pointerId)
        if (!pointers.size) pressed = undefined
      }
      canvas.addEventListener("pointerup", release, {signal}); canvas.addEventListener("pointercancel", release, {signal})
      canvas.addEventListener("pointerleave", () => { if (!pointers.size) { hover = undefined; requestDraw() } }, {signal})
      canvas.addEventListener("wheel", event => { event.preventDefault(); const p = point(event); zoom(Math.exp(-Math.max(-120, Math.min(120, event.deltaY)) * .003), p.x, p.y) }, {passive: false, signal})
      canvas.addEventListener("keydown", event => {
        if (event.ctrlKey || event.metaKey || event.altKey) return
        if (["+", "=", "-", "0", "ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown", "Enter"].includes(event.key)) event.preventDefault()
        if (event.key === "+" || event.key === "=") zoom(1.4)
        else if (event.key === "-") zoom(1 / 1.4)
        else if (event.key === "0") fit()
        else if (event.key === "Enter" && selected) open(selected)
        else { if (event.key === "ArrowLeft") offsetX += 40; if (event.key === "ArrowRight") offsetX -= 40; if (event.key === "ArrowUp") offsetY += 40; if (event.key === "ArrowDown") offsetY -= 40; requestDraw() }
      }, {signal})
      requestDraw()
    } catch {
      if (!disposed) { loading.textContent = "Карта не загрузилась. Материалы доступны в дереве и поиске."; container.replaceChildren(loading) }
    }
  }
  const observer = new IntersectionObserver(entries => {
    visible = entries[0].isIntersecting
    if (visible) { void initialize(); requestDraw() }
    else if (frame) { cancelAnimationFrame(frame); frame = 0 }
  })
  observer.observe(container)
  document.addEventListener("visibilitychange", () => { if (document.hidden && frame) { cancelAnimationFrame(frame); frame = 0 } else requestDraw() }, {signal})
  document.addEventListener("themechange", requestDraw, {signal})
  return () => { disposed = true; abort.abort(); observer.disconnect(); resize?.disconnect(); if (frame) cancelAnimationFrame(frame); container.replaceChildren() }
}

document.addEventListener("nav", () => {
  const abort = new AbortController(), signal = abort.signal, cleanups: (() => void)[] = []
  const slug = getFullSlug(window)
  for (const container of document.querySelectorAll<HTMLElement>(".graph-container")) cleanups.push(mountGraph(container, slug))
  let returnFocus: HTMLElement | null = null, globalCleanup: (() => void) | undefined
  const overlay = document.querySelector<HTMLElement>(".global-graph-outer")
  const graph = overlay?.querySelector<HTMLElement>(".global-graph-container")
  const close = () => { overlay?.classList.remove("active"); globalCleanup?.(); globalCleanup = undefined; returnFocus?.focus() }
  const show = () => {
    if (!overlay || !graph) return
    if (overlay.classList.contains("active")) { close(); return }
    returnFocus = document.activeElement as HTMLElement
    overlay.classList.add("active"); overlay.setAttribute("role", "dialog"); overlay.setAttribute("aria-modal", "true"); overlay.setAttribute("aria-label", "Общий граф знаний")
    globalCleanup = mountGraph(graph, slug)
    overlay.querySelector(".graph-close")?.remove()
    const closeButton = button("Закрыть общий граф", "×", close)
    closeButton.className = "graph-close"; overlay.append(closeButton)
    closeButton.focus()
  }
  for (const icon of document.querySelectorAll(".global-graph-icon")) icon.addEventListener("click", show, {signal})
  overlay?.addEventListener("click", event => { if (event.target === overlay) close() }, {signal})
  document.addEventListener("keydown", event => {
    if (event.key === "g" && (event.ctrlKey || event.metaKey) && !event.shiftKey) { event.preventDefault(); show() }
    if (!overlay?.classList.contains("active")) return
    if (event.key === "Escape") { event.preventDefault(); close() }
    if (event.key === "Tab") {
      const items = [...overlay.querySelectorAll<HTMLElement>("button, canvas, a[href]")].filter(e => e.getClientRects().length)
      const first = items[0], last = items.at(-1)
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus() }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus() }
    }
  }, {signal})
  window.addCleanup(() => { abort.abort(); for (const cleanup of cleanups) cleanup(); globalCleanup?.(); overlay?.querySelector(".graph-close")?.remove(); overlay?.classList.remove("active") })
})
