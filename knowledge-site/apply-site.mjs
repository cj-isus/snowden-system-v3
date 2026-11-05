import fs from "node:fs"
import path from "node:path"
import { createRequire } from "node:module"
import { pathToFileURL, fileURLToPath } from "node:url"

const site = path.dirname(fileURLToPath(import.meta.url))
const args = new Map()
for (let i = 2; i < process.argv.length; i += 2) args.set(process.argv[i], process.argv[i + 1])
if (!args.has("--quartz")) throw new Error("Explicit isolated Quartz directory required")
const quartz = path.resolve(args.get("--quartz")), pin = JSON.parse(fs.readFileSync(path.join(site, "upstream.json")))
const upstreamPackage = JSON.parse(fs.readFileSync(path.join(quartz, "package.json")))
if (upstreamPackage.version !== pin.version) throw new Error("Quartz version differs from the reviewed pin")
const require = createRequire(path.join(quartz, "package.json"))
const { default: sharp } = await import(pathToFileURL(require.resolve("sharp")).href)
const esbuild = await import(pathToFileURL(require.resolve("esbuild")).href)
const baseUrl = args.get("--base-url") ?? pin.siteUrl.replace(/^https:\/\//, "").replace(/\/$/, "")
const basePath = new URL("https://" + baseUrl).pathname.replace(/\/$/, "")
for (const file of ["quartz.config.ts", "quartz.layout.ts"]) fs.copyFileSync(path.join(site, file), path.join(quartz, file))
const configPath = path.join(quartz, "quartz.config.ts")
fs.writeFileSync(configPath, fs.readFileSync(configPath, "utf8").replace('baseUrl: "cj-isus.github.io/snowden-system-v3"', "baseUrl: " + JSON.stringify(baseUrl)))
fs.copyFileSync(path.join(site, "styles/custom.scss"), path.join(quartz, "quartz/styles/custom.scss"))
for (const file of fs.readdirSync(path.join(site, "components"))) fs.copyFileSync(path.join(site, "components", file), path.join(quartz, "quartz/components", file))

function patch(file, before, after) {
  const absolute = path.join(quartz, file), text = fs.readFileSync(absolute, "utf8")
  if (!text.includes(before) || text.indexOf(before) !== text.lastIndexOf(before)) throw new Error(`Upstream seam changed: ${file}`)
  fs.writeFileSync(absolute, text.replace(before, after))
}
patch("quartz/components/scripts/darkmode.inline.ts", 'localStorage.getItem("theme") ?? userPref', 'localStorage.getItem("theme") ?? "dark"')
patch("quartz/components/Graph.tsx", 'aria-label="Global Graph"', 'type="button" aria-label="Развернуть общий граф"')
patch("quartz/components/Explorer.tsx", 'class="explorer-toggle mobile-explorer hide-until-loaded"', 'class="explorer-toggle mobile-explorer hide-until-loaded" aria-label="Открыть или закрыть дерево материалов"')
patch("quartz/components/Head.tsx", '<link rel="preconnect" href="https://cdnjs.cloudflare.com" crossOrigin="anonymous" />', '')
// Fit the full corpus once at startup; keep Quartz zoom/drag and local graphs intact.
patch("quartz/components/scripts/graph.inline.ts", '  if (enableZoom) {\n    select<HTMLCanvasElement, NodeData>(app.canvas).call(\n      zoom<HTMLCanvasElement, NodeData>()', '  if (depth === -1) simulation.tick(120)\n  if (enableZoom) {\n    const graphZoom = zoom<HTMLCanvasElement, NodeData>()')
patch("quartz/components/scripts/graph.inline.ts", '        }),\n    )\n  }\n\n  let stopAnimation = false', `        })
    const canvas = select<HTMLCanvasElement, NodeData>(app.canvas).call(graphZoom)
    if (depth === -1 && graphData.nodes.length > 0) {
      const xs = graphData.nodes.map(n => n.x ?? 0), ys = graphData.nodes.map(n => n.y ?? 0)
      const minX = Math.min(...xs), maxX = Math.max(...xs), minY = Math.min(...ys), maxY = Math.max(...ys)
      const fit = Math.max(0.25, Math.min(1, (width - 60) / (maxX - minX + 30), (height - 60) / (maxY - minY + 30)))
      canvas.call(graphZoom.transform, zoomIdentity.translate(width / 2 - fit * (width / 2 + (minX + maxX) / 2), height / 2 - fit * (height / 2 + (minY + maxY) / 2)).scale(fit))
    }
  }

  let stopAnimation = false`)
patch("quartz/components/scripts/mermaid.inline.ts", '"https://cdnjs.cloudflare.com/ajax/libs/mermaid/11.4.0/mermaid.esm.min.mjs"', `new URL(${JSON.stringify(basePath + "/static/mermaid/mermaid.js")}, window.location.origin).href`)

const runtimePackage = path.join(site, "runtime/node_modules/mermaid/dist/mermaid.esm.mjs")
if (!fs.existsSync(runtimePackage)) throw new Error("Install the pinned knowledge-site runtime lockfile first")
await esbuild.build({ entryPoints: [runtimePackage], outdir: path.join(quartz, "quartz/static/mermaid"), bundle: true, splitting: true, format: "esm", platform: "browser", target: "es2022", minify: true, entryNames: "mermaid", chunkNames: "chunks/[name]-[hash]", logLevel: "warning" })
const icon = fs.readFileSync(path.join(site, "static/icon.svg"))
await sharp(icon).resize(128, 128).png().toFile(path.join(quartz, "quartz/static/icon.png"))
fs.copyFileSync(path.join(site, "static/icon.svg"), path.join(quartz, "quartz/static/icon.svg"))
fs.mkdirSync(path.join(quartz, "quartz/static/licenses"), { recursive: true })
for (const name of ["LICENSE-Quartz.txt", "LICENSE-Mermaid.txt"]) fs.copyFileSync(path.join(site, name), path.join(quartz, "quartz/static/licenses", name))
const social = `<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="630"><rect width="1200" height="630" fill="#18191f"/><g stroke="#75658e" stroke-width="3"><path d="M810 140 950 300 1110 170M950 300 835 465M950 300 1100 470"/></g><g fill="#b8a1ed"><circle cx="810" cy="140" r="15"/><circle cx="1110" cy="170" r="12"/><circle cx="835" cy="465" r="14"/><circle cx="1100" cy="470" r="11"/><circle cx="950" cy="300" r="28"/></g><text x="80" y="290" font-family="sans-serif" font-size="78" font-weight="700" fill="#eef0f8">SNOWDEN</text><text x="83" y="365" font-family="sans-serif" font-size="34" fill="#b8a1ed">База знаний</text></svg>`
await sharp(Buffer.from(social)).png().toFile(path.join(quartz, "quartz/static/og-image.png"))
console.log(JSON.stringify({ quartz: pin.version, commit: pin.commit, baseUrl, runtime: "self-hosted Mermaid; MathJax SVG; bundled Quartz graph" }))
