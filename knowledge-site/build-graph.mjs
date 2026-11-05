import fs from "node:fs"
import path from "node:path"
import { createRequire } from "node:module"
import { pathToFileURL } from "node:url"

// Quartz already pins D3. Solve the layout once during publication, never on a reader's phone.
export async function buildNavigation(quartz, content) {
  const require = createRequire(path.join(quartz, "package.json"))
  const { forceSimulation, forceLink, forceManyBody, forceCollide, forceX, forceY } = await import(pathToFileURL(require.resolve("d3")).href)
  const simple = slug => slug === "index" ? "" : slug.replace(/\/index$/, "")
  const nodes = Object.keys(content).map(slug => ({ id: simple(slug), slug, degree: 0 }))
  const byId = new Map(nodes.map(node => [node.id, node]))
  const pairs = new Map()
  for (const node of nodes) {
    for (const targetSlug of content[node.slug].links ?? []) {
      const target = simple(targetSlug)
      if (target === node.id || !byId.has(target)) continue
      const key = JSON.stringify([node.id, target].sort())
      if (!pairs.has(key)) pairs.set(key, { source: node.id, target })
    }
  }
  const links = [...pairs.values()]
  for (const link of links) { byId.get(link.source).degree++; byId.get(link.target).degree++ }
  const radius = node => 2 + Math.sqrt(node.degree)
  const layout = forceSimulation(nodes).stop()
    .force("charge", forceManyBody().strength(-85))
    .force("link", forceLink(links).id(node => node.id).distance(65))
    .force("collide", forceCollide(node => radius(node) + 3).iterations(2))
    .force("x", forceX(0).strength(.08))
    .force("y", forceY(0).strength(.08))
  layout.tick(300)
  layout.stop()
  const navigation = Object.fromEntries(nodes.map(node => {
    const { content: fullText, ...metadata } = content[node.slug]
    return [node.slug, { ...metadata, content: "", x: +node.x.toFixed(2), y: +node.y.toFixed(2), degree: node.degree }]
  }))
  const text = JSON.stringify(navigation)
  fs.writeFileSync(path.join(quartz, "public/static/navigationIndex.json"), text)
  return { bytes: Buffer.byteLength(text), nodes: nodes.length, undirectedLinks: links.length }
}
