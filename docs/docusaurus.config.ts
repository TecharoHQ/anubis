import { themes as prismThemes } from 'prism-react-renderer';
import type { Config } from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

// This runs in Node.js - Don't use client-side code here (browser APIs, JSX...)

const config: Config = {
  title: 'Anubis',
  tagline: 'Weigh the soul of incoming HTTP requests to protect your website!',
  favicon: 'img/favicon.ico',

  // Set the production url of your site here
  url: 'https://anubis.techaro.lol',
  // Set the /<baseUrl>/ pathname under which your site is served
  // For GitHub pages deployment, it is often '/<projectName>/'
  baseUrl: '/',

  // GitHub pages deployment config.
  // If you aren't using GitHub pages, you don't need these.
  organizationName: 'TecharoHQ', // Usually your GitHub org/user name.
  projectName: 'anubis', // Usually your repo name.

  onBrokenLinks: 'throw',
  onBrokenMarkdownLinks: 'warn',

  // Even if you don't use internationalization, you can use this field to set
  // useful metadata like html lang. For example, if your site is Chinese, you
  // may want to replace "en" with "zh-Hans".
  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  markdown: {
    mermaid: true,
  },
  themes: ['@docusaurus/theme-mermaid'],

  presets: [
    [
      'classic',
      {
        blog: {
          showReadingTime: true,
          feedOptions: {
            type: ['rss', 'atom', "json"],
            xslt: true,
          },
          editUrl: 'https://github.com/TecharoHQ/anubis/tree/main/docs/',
          onInlineTags: 'warn',
          onInlineAuthors: 'warn',
          onUntruncatedBlogPosts: 'throw',
        },
        docs: {
          sidebarPath: './sidebars.ts',
          editUrl: 'https://github.com/TecharoHQ/anubis/tree/main/docs/',
        },
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  themeConfig: {
    colorMode: {
      respectPrefersColorScheme: true,
    },
    // Replace with your project's social card
    image: 'img/social-card.jpg',
    navbar: {
      title: 'Anubis',
      logo: {
        alt: 'A happy jackal woman with brown hair and red eyes',
        src: 'img/favicon.webp',
      },
      items: [
        { to: '/blog', label: 'Blog', position: 'left' },
        {
          type: 'docSidebar',
          sidebarId: 'tutorialSidebar',
          position: 'left',
          label: 'Docs',
        },
        {
          to: '/docs/admin/botstopper',
          label: "Unbranded Version",
          position: "left"
        },
        {
          href: 'https://github.com/TecharoHQ/anubis',
          label: 'GitHub',
          position: 'right',
        },
        {
          href: 'https://github.com/sponsors/Xe',
          label: "Sponsor the Project",
          position: 'right'
        },
      ],
    },
    footer: {
      style: 'dark',
      links: [
        {
          title: 'Docs',
          items: [
            {
              label: 'Intro',
              to: '/docs/',
            },
            {
              label: "Installation",
              to: "/docs/admin/installation",
            },
          ],
        },
        {
          title: 'Community',
          items: [
            {
              label: 'GitHub Discussions',
              href: 'https://github.com/TecharoHQ/anubis/discussions',
            },
            {
              label: 'Bluesky',
              href: 'https://bsky.app/profile/techaro.lol',
            },
          ],
        },
        {
          title: 'More',
          items: [
            {
              label: 'Blog',
              to: '/blog',
            },
            {
              label: 'GitHub',
              href: 'https://github.com/TecharoHQ/anubis',
            },
          ],
        },
      ],
      copyright: `Copyright © ${new Date().getFullYear()} Techaro. Made with ❤️ in 🇨🇦.`,
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
      magicComments: [
        {
          className: 'code-block-diff-add-line',
          line: 'diff-add'
        },
        {
          className: 'code-block-diff-remove-line',
          line: 'diff-remove'
        }
      ],
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
