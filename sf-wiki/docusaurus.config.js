// @ts-check
import { themes as prismThemes } from "prism-react-renderer";

/** @type {import('@docusaurus/types').Config} */
const config = {
  title: "strong-fish",
  tagline: "Powerlifting programs, clubs and coaching",
  favicon: "img/favicon.ico",

  url: "https://doc.strong-fish.com",
  baseUrl: "/",

  organizationName: "cwcloud",
  projectName: "strong-fish",

  // A broken link here is a page somebody clicks and lands nowhere, so it
  // fails the build rather than the reader.
  onBrokenLinks: "throw",
  onBrokenMarkdownLinks: "warn",

  plugins: [
    [
      "@easyops-cn/docusaurus-search-local",
      {
        hashed: true,
        language: ["en", "fr"],
        docsRouteBasePath: "/docs",
        indexDocs: true,
        indexPages: false,
        highlightSearchTermsOnTargetPage: true,
        removeDefaultStopWordFilter: true,
        removeDefaultStemmer: true,
      },
    ],
  ],

  i18n: {
    defaultLocale: "en",
    locales: ["en", "fr"],
    localeConfigs: {
      en: { label: "English" },
      fr: { label: "Français" },
    },
  },

  presets: [
    [
      "classic",
      /** @type {import('@docusaurus/preset-classic').Options} */
      ({
        docs: {
          sidebarPath: require.resolve("./sidebars.js"),
          // The docs *are* the site: there is no marketing landing page to
          // put in front of them, so /docs is where the root lands.
          routeBasePath: "/docs",
        },
        blog: false,
        theme: {
          customCss: require.resolve("./src/css/custom.css"),
        },
      }),
    ],
  ],

  themeConfig:
    /** @type {import('@docusaurus/preset-classic').ThemeConfig} */
    ({
      image: "img/logo512.png",
      metadata: [
        {
          name: "description",
          content:
            "strong-fish documentation: RPE-based programs whose loads follow each athlete's own maxes, clubs, and a training feed.",
        },
        {
          name: "keywords",
          content: "strong-fish, powerlifting, RPE, 1RM, e1RM, squat, bench, deadlift, coaching",
        },
      ],
      navbar: {
        title: "strong-fish",
        logo: {
          alt: "strong-fish",
          src: "img/logo192.png",
          // The stock mark is inked in navy and all but disappears on a dark
          // background, so dark mode gets the light-inked variant - the same
          // pair of assets the web and mobile apps use.
          srcDark: "img/logo192-dark.png",
        },
        items: [
          { label: "Documentation", to: "/docs", position: "right" },
          { label: "Tutorials", to: "/docs/tutorials/signup", position: "right" },
          { label: "App", href: "https://strong-fish.com", position: "right" },
          { type: "localeDropdown", position: "right" },
        ],
      },
      footer: {
        style: "dark",
        links: [
          {
            title: "Documentation",
            items: [
              { label: "About", to: "/docs/about" },
              { label: "RPE, 1RM and e1RM", to: "/docs/vocabulary" },
              { label: "Tutorials", to: "/docs/tutorials/signup" },
            ],
          },
          {
            title: "strong-fish",
            items: [
              { label: "Open the app", href: "https://strong-fish.com" },
              { label: "Sources", href: "https://gitlab.cwcloud.tech/oss/strong-fish" },
            ],
          },
        ],
        copyright: `strong-fish is an open-source product by CWCloud. © ${new Date().getFullYear()}`,
      },
      // Light and dark, following the reader's own OS by default - the same
      // three-way behaviour the app itself has.
      colorMode: {
        defaultMode: "light",
        disableSwitch: false,
        respectPrefersColorScheme: true,
      },
      prism: {
        theme: prismThemes.github,
        darkTheme: prismThemes.dracula,
        additionalLanguages: ["bash", "json"],
      },
    }),
};

module.exports = config;
