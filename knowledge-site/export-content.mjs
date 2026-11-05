import fs from "node:fs"
import path from "node:path"
import crypto from "node:crypto"
import { spawnSync } from "node:child_process"
import { createRequire } from "node:module"
import { pathToFileURL } from "node:url"

const args = new Map()
for (let i = 2; i < process.argv.length; i += 2) args.set(process.argv[i], process.argv[i + 1])
const repository = path.resolve(args.get("--repository") ?? ".")
const quartz = path.resolve(args.get("--quartz") ?? "")
const output = path.resolve(args.get("--out") ?? "")
const ref = args.get("--ref") ?? "HEAD"
if (!args.has("--quartz") || !args.has("--out") || output === repository || output === quartz) throw new Error("Explicit isolated Quartz and output directories are required")
if (fs.existsSync(output) && fs.readdirSync(output).length) throw new Error("Use a fresh export directory; canonical files are never overwritten")
const require = createRequire(path.join(quartz, "package.json"))
const matter = require("gray-matter")
const { unified } = await import(pathToFileURL(require.resolve("unified")).href)
const { default: remarkParse } = await import(pathToFileURL(require.resolve("remark-parse")).href)
const { slug: slugHeading } = await import(pathToFileURL(require.resolve("github-slugger")).href)
const parser = unified().use(remarkParse)
const sha256 = bytes => crypto.createHash("sha256").update(bytes).digest("hex")
function git(...args) {
  const result = spawnSync("git", ["-C", repository, ...args], { maxBuffer: 32 * 1024 * 1024 })
  if (result.status) throw new Error(String(result.stderr))
  return result.stdout
}
const sourceRef = args.get("--source-ref") ?? git("rev-parse", `${ref}^{commit}`).toString().trim()
if (!/^[a-f0-9]{40}$/.test(sourceRef)) throw new Error("Expected a pinned source commit")
const allPaths = new Set(git("ls-tree", "-r", "--name-only", "-z", ref).toString().split("\0").filter(Boolean))
const sourcePaths = [...allPaths].filter(p =>
  (p.startsWith("docs/theory/") || p.startsWith("docs/planning/knowledge-application/") || p === "docs/KNOWLEDGE-BASE.md") &&
  /\.(?:md|json)$/.test(p) && !p.split("/").some(s => s.startsWith(".")),
).sort()
if (sourcePaths.length < 200) throw new Error("Incomplete knowledge corpus; refusing a partial publication")
const snapshot = spawnSync("git", ["-C", repository, "cat-file", "--batch"], {
  input: sourcePaths.map(p => `${ref}:${p}`).join("\n") + "\n", maxBuffer: 32 * 1024 * 1024,
})
if (snapshot.status) throw new Error(String(snapshot.stderr))
const sources = new Map()
let cursor = 0
for (const source of sourcePaths) {
  const end = snapshot.stdout.indexOf(10, cursor)
  const header = snapshot.stdout.subarray(cursor, end).toString()
  const match = header.match(/^[a-f0-9]{40} blob (\d+)$/)
  if (!match) throw new Error(`Missing source blob: ${source}`)
  const size = Number(match[1]), start = end + 1
  sources.set(source, snapshot.stdout.subarray(start, start + size))
  cursor = start + size + 1
}
if (cursor !== snapshot.stdout.length) throw new Error("Incomplete source stream")
const destinations = new Map(sourcePaths.map(p => [p, p.slice("docs/".length).replace(/(^|\/)README\.md$/, "$1index.md")]))
if (new Set(destinations.values()).size !== destinations.size) throw new Error("Two source files map to one page")

const folderTitles = {
  theory: "Знания по темам", foundations: "Основы", transports: "Блокировки и транспорты",
  observability: "Измерения и диагностика", methodology: "Метод и доказательства",
  adaptation: "Адаптивный выбор", architecture: "Защита и доверие", performance: "Скорость и ресурсы",
  android: "Android", ux: "Удобство использования", roadmap: "Выводы для разработки",
  research: "Аргументы и источники", archive: "Авторские оригиналы", reviews: "Рецензии",
  cycles: "Дополнительные разборы", handoffs: "Краткие выводы", planning: "Разработка",
  "knowledge-application": "Применение знаний", sections: "По компонентам", crosschecks: "Совместимость решений",
  "02-CENSORSHIP": "02 · Блокировки", "03-ADAPTATION": "03 · Адаптация",
  "04-PERFORMANCE": "04 · Производительность", "05-TRUST-SAFETY": "05 · Доверие",
  "06-RELIABILITY-UX": "06 · Надёжность и UX", "2026-10-28-contracts": "Контракты и границы",
  C02: "C02 · Сетевой путь", C03: "C03 · Адаптация", C04: "C04 · Ресурсы",
  C05: "C05 · Доверие", C06: "C06 · Надёжность", Z01: "Z01 · ECH и DNS",
  Z02: "Z02 · Доступность и unknown", Z03: "Z03 · Учёт наблюдений", Z04: "Z04 · QUIC",
  Z05: "Z05 · Android и энергия", Z06: "Z06 · Статусы и полномочия",
  Z07: "Z07 · История доверия", Z08: "Z08 · UID и маршруты", Z09: "Z09 · Полезное восстановление",
}
const titles = new Map(), entries = [], unresolved = []
const plain = node => node.value ?? (node.children ?? []).map(plain).join("")
const decode = value => { try { return decodeURIComponent(value) } catch { return value } }
function resolveDestination(url, source, destination) {
  if (/^(?:[a-z][a-z0-9+.-]*:|\/\/)/i.test(url) || url.startsWith("#")) return { kind: "unchanged", url }
  const anchorAt = url.indexOf("#"), anchor = anchorAt < 0 ? "" : url.slice(anchorAt)
  const pathname = decode((anchorAt < 0 ? url : url.slice(0, anchorAt)).split("?")[0]).replaceAll("\\", "/")
  const target = path.posix.normalize(path.posix.join(path.posix.dirname(source), pathname))
  const mapped = destinations.get(target)
  if (mapped) {
    let relative = path.posix.relative(path.posix.dirname(destination), mapped)
    if (!relative.startsWith(".")) relative = "./" + relative
    return { kind: "site", url: relative + anchor }
  }
  if (allPaths.has(target)) return { kind: "external", url: `https://github.com/cj-isus/snowden-system-v3/blob/${sourceRef}/${encodeURI(target)}${anchor}` }
  unresolved.push({ source, target, anchor })
  return { kind: "coordinate", coordinate: target + anchor }
}
function mathDelimiters(text) {
  // Presentation-only normalization outside Markdown code. The source stays byte-identical in Git.
  const tree = parser.parse(text), ranges = []
  function visit(node) { if (["code", "inlineCode"].includes(node.type)) ranges.push([node.position.start.offset, node.position.end.offset]); else for (const child of node.children ?? []) visit(child) }
  visit(tree)
  let cursor = 0, output = ""
  const convert = part => part.replace(/\\\[([\s\S]*?)\\\]/g, (_, value) => `\n$$\n${value.trim()}\n$$\n`).replace(/\\\(([^\n]*?)\\\)/g, (_, value) => `$${value}$`)
  for (const [start, end] of ranges.sort((a, b) => a[0] - b[0])) { output += convert(text.slice(cursor, start)) + text.slice(start, end); cursor = end }
  return output + convert(text.slice(cursor))
}
function rewriteMarkdown(body, source, destination) {
  const tree = parser.parse(body), edits = [], definitions = new Map()
  for (const node of tree.children) if (node.type === "definition") definitions.set(node.identifier, resolveDestination(node.url, source, destination))
  const firstTitle = tree.children.find(n => n.type === "heading" && n.depth === 1)
  if (firstTitle) {
    const id = slugHeading(plain(firstTitle))
    const explicit = body.includes(`id="${id}"`) || body.includes(`id='${id}'`)
    edits.push({ start: firstTitle.position.start.offset, end: firstTitle.position.end.offset, text: explicit ? "" : `<a id="${id}"></a>` })
  }
  function visit(node) {
    if (["link", "image", "definition"].includes(node.type)) {
      const resolved = resolveDestination(node.url, source, destination)
      if (resolved.kind === "unchanged") return
      const start = node.position.start.offset, end = node.position.end.offset, raw = body.slice(start, end)
      if (node.type === "definition") {
        edits.push({ start, end, text: resolved.kind === "coordinate" ? `<!-- Historical source coordinate: ${resolved.coordinate.replaceAll("--", "—")} -->` : `[${node.label ?? node.identifier}]: ${resolved.url}${node.title ? " " + JSON.stringify(node.title) : ""}` })
        return
      }
      const label = node.type === "image" ? node.alt ?? "" : raw.slice(1, raw.lastIndexOf("]("))
      edits.push({ start, end, text: resolved.kind === "coordinate" ? `${label} (\`${resolved.coordinate}\`)` : `${node.type === "image" ? "!" : ""}[${label}](${resolved.url}${node.title ? " " + JSON.stringify(node.title) : ""})` })
      return
    }
    if (["linkReference", "imageReference"].includes(node.type)) {
      const resolved = definitions.get(node.identifier)
      if (resolved?.kind === "coordinate") {
        const label = node.type === "imageReference" ? node.alt ?? "" : (node.children ?? []).map(plain).join("")
        edits.push({ start: node.position.start.offset, end: node.position.end.offset, text: `${label} (\`${resolved.coordinate}\`)` })
      }
      return
    }
    for (const child of node.children ?? []) visit(child)
  }
  visit(tree)
  let last = body.length
  for (const edit of edits.sort((a, b) => b.start - a.start)) {
    if (edit.end > last) throw new Error(`Overlapping presentation edits in ${source}`)
    body = body.slice(0, edit.start) + edit.text + body.slice(edit.end)
    last = edit.start
  }
  return mathDelimiters(body)
}
function write(file, content) {
  const dest = path.resolve(output, file)
  if (!dest.startsWith(output + path.sep)) throw new Error("Export escaped its directory")
  fs.mkdirSync(path.dirname(dest), { recursive: true })
  fs.writeFileSync(dest, content)
}
fs.mkdirSync(output, { recursive: true })
for (const source of sourcePaths) {
  const bytes = sources.get(source), destination = destinations.get(source)
  if (!source.endsWith(".md")) { write(destination, bytes); entries.push({ source, destination, sourceSha256: sha256(bytes), type: "asset" }); continue }
  const raw = bytes.toString("utf8").replace(/^\uFEFF/, ""), parsed = matter(raw), tree = parser.parse(parsed.content)
  const heading = tree.children.find(n => n.type === "heading" && n.depth === 1)
  const originalTitle = typeof parsed.data.title === "string" ? parsed.data.title : heading ? plain(heading) : path.basename(source, ".md")
  const directory = path.posix.basename(path.posix.dirname(destination))
  const title = destination.endsWith("/index.md") && folderTitles[directory] ? folderTitles[directory] : originalTitle
  titles.set(destination, title)
  let body = rewriteMarkdown(parsed.content, source, destination)
  if (source.includes("/archive/")) body = "> **Авторский оригинал.** Историческая версия сохранена целиком. Актуальные выводы и поправки находятся в тематических главах и рецензиях.\n\n" + body
  const data = { ...parsed.data, title, sourcePath: source, sourceUrl: `https://github.com/cj-isus/snowden-system-v3/blob/${sourceRef}/${encodeURI(source)}` }
  delete data.draft
  write(destination, matter.stringify(body, data))
  entries.push({ source, destination, sourceSha256: sha256(bytes), type: "material" })
}
const directories = new Set()
for (const file of destinations.values()) { let directory = path.posix.dirname(file); while (directory !== ".") { directories.add(directory); directory = path.posix.dirname(directory) } }
for (const directory of [...directories].sort()) {
  const index = directory + "/index.md"
  if (titles.has(index)) continue
  const title = folderTitles[path.posix.basename(directory)] ?? path.posix.basename(directory)
  const children = []
  for (const file of [...destinations.values()]) if (file.endsWith(".md") && path.posix.dirname(file) === directory && !file.endsWith("/index.md")) children.push([titles.get(file), path.posix.basename(file)])
  for (const child of directories) if (path.posix.dirname(child) === directory) children.push([folderTitles[path.posix.basename(child)] ?? path.posix.basename(child), path.posix.basename(child) + "/index.md"])
  write(index, matter.stringify("Материалы раздела:\n\n" + children.map(([label, target]) => `- [${label}](./${target})`).join("\n") + "\n", { title }))
  titles.set(index, title)
}
const materials = entries.filter(e => e.type === "material").length
write("index.md", matter.stringify(`**${materials} материалов**: теория, аргументы, источники, рецензии и планы разработки.\n\n[Начать чтение](./theory/READING-GUIDE.md) · [Найти ответ](./theory/KNOWLEDGE-MAP.md) · [Словарь](./theory/GLOSSARY.md) · [План ПК](./planning/knowledge-application/PC-PLAN.md) · [План Android](./planning/knowledge-application/ANDROID-PLAN.md)\n\nГраф отражает реальные ссылки между заметками. Подробные источники и авторские оригиналы доступны через дерево. Научный вывод, план реализации и проверка работающего VPN имеют разные границы доказательства.\n`, { title: "Граф знаний", description: "Открытая база знаний Snowden System: теория, источники и разработка адаптивного VPN." }))
const exportedDestinations = new Set(destinations.values())
const report = { sourceRef, ref, sourceFiles: sourcePaths.length, materials, generatedDirectoryPages: [...directories].filter(d => !exportedDestinations.has(d + "/index.md")).length, entries, historicalCoordinates: unresolved }
fs.writeFileSync(path.join(path.dirname(output), "knowledge-export.json"), JSON.stringify(report, null, 2) + "\n")
console.log(JSON.stringify({ sourceRef, sourceFiles: sourcePaths.length, materials, exportedMarkdown: titles.size + 1, historicalCoordinates: unresolved.length }))
