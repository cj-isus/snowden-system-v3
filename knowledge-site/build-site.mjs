import fs from "node:fs"
import path from "node:path"
import crypto from "node:crypto"
import { spawnSync } from "node:child_process"

const flag = process.argv.indexOf("--quartz")
if (flag < 0 || !process.argv[flag + 1]) throw new Error("Explicit isolated Quartz directory required")
const quartz = path.resolve(process.argv[flag + 1])
const pkg = JSON.parse(fs.readFileSync(path.join(quartz, "package.json")))
if (pkg.name !== "@jackyzha0/quartz" || pkg.version !== "4.5.2") throw new Error("Not the reviewed isolated site engine")
const output = path.resolve(quartz, "public")
if (path.dirname(output) !== quartz) throw new Error("Unsafe output directory")
const exported = JSON.parse(fs.readFileSync(path.join(quartz, "knowledge-export.json")))
if (exported.materials < 200 || !fs.existsSync(path.join(quartz, "site-content/index.md"))) throw new Error("Knowledge export missing")
const build = spawnSync(process.execPath, ["quartz/bootstrap-cli.mjs", "build", "--directory", "site-content", "--output", "public", "--concurrency", "2"], { cwd: quartz, stdio: "inherit" })
if (build.status !== 0) process.exit(build.status ?? 1)
const index = fs.readFileSync(path.join(output, "index.html"), "utf8")
const content = JSON.parse(fs.readFileSync(path.join(output, "static/contentIndex.json"), "utf8"))
const missing = exported.entries.filter(entry => entry.type === "material" && !fs.existsSync(path.join(output, entry.destination.replace(/\.md$/, ".html"))))
if (missing.length || Object.keys(content).length < exported.materials || !index.includes("knowledge-home") || !index.includes("graph-container")) throw new Error(`Incomplete Pages artifact: ${missing.length} materials missing`)
const receipt = {
  sourceCommit: exported.sourceRef, sourceMaterials: exported.materials,
  indexedPages: Object.keys(content).length,
  links: Object.values(content).reduce((sum, page) => sum + (page.links?.length ?? 0), 0),
  homeSHA256: crypto.createHash("sha256").update(index).digest("hex"),
  scope: "Static site packaging receipt; no VPN or native/device/live verification.",
}
fs.writeFileSync(path.join(quartz, "site-build.json"), JSON.stringify(receipt, null, 2) + "\n")
console.log(JSON.stringify(receipt))
