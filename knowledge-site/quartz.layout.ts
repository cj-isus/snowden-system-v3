import { PageLayout, SharedLayout } from "./quartz/cfg"
import type { QuartzComponentProps } from "./quartz/components/types"
import * as Component from "./quartz/components"
import KnowledgeHome from "./quartz/components/KnowledgeHome"
import SourceLink from "./quartz/components/SourceLink"
import ProductHome from "./quartz/components/ProductHome"
import ProductNavigation from "./quartz/components/ProductNavigation"
import ReleaseDownloads from "./quartz/components/ReleaseDownloads"

const isHome = (page: QuartzComponentProps) => page.fileData.slug === "index"
const isGraph = (page: QuartzComponentProps) => page.fileData.slug === "graph"
const isProduct = (page: QuartzComponentProps) => ["index", "downloads", "release-guide"].includes(page.fileData.slug ?? "")
const graphOptions = {
  localGraph: { depth: 1 },
  globalGraph: { depth: -1 },
}
const navigation = [
  Component.PageTitle(),
  Component.MobileOnly(Component.Spacer()),
  Component.Flex({ components: [
    { Component: Component.Search(), grow: true },
    { Component: Component.Darkmode() },
    { Component: Component.ReaderMode() },
  ] }),
  ProductNavigation(),
  Component.Explorer({ title: "База знаний", folderDefaultState: "collapsed", folderClickBehavior: "link", useSavedState: true, filterFn: node => !["tags", "graph", "downloads", "release-guide"].includes(node.slug) }),
]

export const sharedPageComponents: SharedLayout = {
  head: Component.Head(), header: [], afterBody: [],
  footer: Component.Footer({ links: { "Исходники базы": "https://github.com/cj-isus/snowden-system-v3/tree/main/docs/theory", "Quartz · MIT": "https://github.com/jackyzha0/quartz" } }),
}

export const defaultContentPageLayout: PageLayout = {
  beforeBody: [
    Component.ConditionalRender({ component: Component.Breadcrumbs({ rootName: "Главная" }), condition: page => !isHome(page) }),
    Component.ConditionalRender({ component: Component.ArticleTitle(), condition: page => !isHome(page) }),
    SourceLink(),
    Component.ConditionalRender({ component: ProductHome(), condition: isHome }),
    Component.ConditionalRender({ component: ReleaseDownloads(), condition: page => page.fileData.slug === "downloads" }),
    Component.ConditionalRender({ component: KnowledgeHome(), condition: isGraph }),
  ],
  left: navigation,
  right: [
    Component.ConditionalRender({ component: Component.Graph(graphOptions), condition: page => !isHome(page) && !isGraph(page) && !isProduct(page) }),
    Component.DesktopOnly(Component.TableOfContents()),
    Component.ConditionalRender({ component: Component.Backlinks(), condition: page => !isHome(page) && !isProduct(page) && !isGraph(page) }),
  ],
}

export const defaultListPageLayout: PageLayout = {
  beforeBody: [Component.Breadcrumbs({ rootName: "Главная" }), Component.ArticleTitle()],
  left: navigation,
  right: [],
}
