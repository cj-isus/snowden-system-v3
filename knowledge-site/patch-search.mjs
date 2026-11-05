// Narrow, guarded adaptations of the pinned Quartz search. Full text is never indexed on page load.
export function applySearchPatches(patch, basePath) {
  const file = "quartz/components/scripts/search.inline.ts"
  patch(file, '\nconst p = new DOMParser()', `
let fullSearchPromise: Promise<void> | undefined
function prepareFullSearch(data: ContentIndex) {
  return fullSearchPromise ??= fetch(new URL(${JSON.stringify(basePath + "/static/contentIndex.json")}, location.origin))
    .then(response => { if (!response.ok) throw new Error("Search unavailable"); return response.json() })
    .then(async (full: ContentIndex) => {
      for (const [slug, entry] of Object.entries(full)) if (data[slug]) Object.assign(data[slug], entry)
      await fillDocument(data)
    })
    .catch(error => { fullSearchPromise = undefined; throw error })
}
const p = new DOMParser()`)
  patch(file, '  function showSearch(searchTypeNew: SearchType) {', `  function showSearch(searchTypeNew: SearchType) {
    searchBar.placeholder = "Загрузка поиска…"
    void prepareFullSearch(data).then(() => { searchBar.placeholder = "Найти что-нибудь" }).catch(() => { searchBar.placeholder = "Не удалось загрузить поиск. Повторите открытие." })`)
  patch(file, '    currentSearchTerm = (e.target as HTMLInputElement).value', `    const requestedTerm = (e.target as HTMLInputElement).value
    try { await prepareFullSearch(data) } catch { return }
    if (searchBar.value !== requestedTerm || !container.classList.contains("active")) return
    currentSearchTerm = requestedTerm`)
  patch(file, '  await fillDocument(data)\n}', '}')
}
