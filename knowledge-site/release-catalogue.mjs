import fs from "node:fs"
import path from "node:path"

// Curated public release metadata only. Never discover local executables, profiles or credentials.
export function readReleases(site) {
  const data = JSON.parse(fs.readFileSync(path.join(site, "releases.json"), "utf8"))
  const validDate = value => typeof value === "string" && /^\d{4}-\d{2}-\d{2}$/.test(value) && Number.isFinite(Date.parse(value))
  const text = value => typeof value === "string" && value.trim().length > 0
  if (!validDate(data.updatedAt) || !Array.isArray(data.platforms) || !Array.isArray(data.releases)) throw new Error("Invalid public release catalogue")
  if (data.platforms.length !== 2 || new Set(data.platforms.map(p => p.id)).size !== 2) throw new Error("Expected distinct Windows and Android entries")
  for (const platform of data.platforms) {
    if (!["windows", "android"].includes(platform.id) || !["development", "integration", "released"].includes(platform.state) || ![platform.name, platform.format, platform.label, platform.summary, platform.next].every(text)) throw new Error("Invalid platform metadata")
  }
  const identities = new Set()
  for (const release of data.releases) {
    const key = `${release.platform}:${release.version}`
    if (identities.has(key)) throw new Error("Duplicate release version")
    identities.add(key)
    const url = new URL(release.url), notes = new URL(release.notesUrl)
    const prefix = "/cj-isus/snowden-system-v3/releases/"
    if (![url, notes].every(u => u.protocol === "https:" && u.hostname === "github.com" && !u.username && !u.password && !u.port && !u.search && !u.hash && u.pathname.startsWith(prefix))) throw new Error("Release must point to this repository's public GitHub Releases")
    if (!url.pathname.startsWith(prefix + "download/") || !notes.pathname.startsWith(prefix + "tag/") || url.pathname.split("/")[5] !== notes.pathname.split("/")[5]) throw new Error("Release asset and notes tags must agree")
    const extension = release.platform === "windows" ? ".exe" : ".apk"
    if (!["windows", "android"].includes(release.platform) || !url.pathname.toLowerCase().endsWith(extension) || !["stable", "preview"].includes(release.channel) || !validDate(release.publishedAt) || ![release.version, release.requirements].every(text) || !/^[a-f0-9]{64}$/.test(release.sha256) || !Number.isSafeInteger(release.sizeBytes) || release.sizeBytes <= 0) throw new Error("Release needs platform, version, date, channel, requirements, exact size and SHA-256")
  }
  // The first entry per platform is the selected release; retain the rest as history.
  return data
}
