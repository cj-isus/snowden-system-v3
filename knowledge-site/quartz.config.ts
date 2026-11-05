import { QuartzConfig } from "./quartz/cfg"
import * as Plugin from "./quartz/plugins"

const config: QuartzConfig = {
  configuration: {
    pageTitle: "Snowden / знания",
    pageTitleSuffix: " · Snowden",
    enableSPA: true,
    enablePopovers: true,
    analytics: null,
    locale: "ru-RU",
    baseUrl: "cj-isus.github.io/snowden-system-v3",
    ignorePatterns: [".obsidian", ".artifacts", "private", "templates"],
    defaultDateType: "modified",
    theme: {
      fontOrigin: "local",
      cdnCaching: false,
      typography: { header: "Segoe UI", body: "Segoe UI", code: "Consolas" },
      colors: {
        lightMode: {
          light: "#f9fafc", lightgray: "#e0e3ec", gray: "#7c8498",
          darkgray: "#41495c", dark: "#202538", secondary: "#7054b3",
          tertiary: "#237a79", highlight: "rgba(112,84,179,0.09)", textHighlight: "#eadc9e99",
        },
        darkMode: {
          light: "#18191f", lightgray: "#30323e", gray: "#8a90a5",
          darkgray: "#c1c5d4", dark: "#eef0f8", secondary: "#b8a1ed",
          tertiary: "#8ac5c1", highlight: "rgba(184,161,237,0.10)", textHighlight: "#88702c66",
        },
      },
    },
  },
  plugins: {
    transformers: [
      Plugin.FrontMatter(),
      Plugin.SyntaxHighlighting({ theme: { light: "github-light", dark: "github-dark" }, keepBackground: false }),
      Plugin.ObsidianFlavoredMarkdown({ enableInHtmlEmbed: false, enableYouTubeEmbed: false, enableVideoEmbed: false }),
      Plugin.GitHubFlavoredMarkdown(),
      Plugin.TableOfContents(),
      Plugin.CrawlLinks({ markdownLinkResolution: "relative" }),
      Plugin.Description(),
      Plugin.Latex({ renderEngine: "mathjax" }),
    ],
    filters: [],
    emitters: [
      Plugin.AliasRedirects(), Plugin.ComponentResources(), Plugin.ContentPage(),
      Plugin.FolderPage(), Plugin.TagPage(),
      Plugin.ContentIndex({ enableSiteMap: true, enableRSS: false }),
      Plugin.Assets(), Plugin.Static(), Plugin.NotFoundPage(),
    ],
  },
}

export default config
