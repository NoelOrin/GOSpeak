import { defineConfig } from 'vitepress'

const isProd = process.env.NODE_ENV === 'production'
const base = isProd ? '/GOSpeak/' : '/'

export default defineConfig({
  title: 'GOSpeak',
  description: '自部署的游戏语音平台 — 文档',
  lang: 'zh-CN',

  base,

  head: [
    ['link', { rel: 'icon', href: `${base}favicon.png` }],
  ],

  themeConfig: {
    logo: '/logo.png',

    nav: [
      { text: '指南', link: '/guide/quickstart', activeMatch: '/guide/' },
      { text: '部署', link: '/deploy/', activeMatch: '/deploy/' },
      { text: '语音', link: '/sfu/', activeMatch: '/sfu/' },
      { text: '参考', link: '/reference/api', activeMatch: '/reference/' },
      { text: '架构', link: '/architecture/', activeMatch: '/architecture/' },
    ],

    sidebar: {
      '/guide/': [
        {
          text: '开始',
          items: [
            { text: '五分钟跑起来', link: '/guide/quickstart' },
            { text: '界面导览', link: '/guide/tour' },
            { text: '桌面客户端', link: '/guide/desktop' },
          ],
        },
        {
          text: '配置',
          items: [
            { text: '配置项', link: '/guide/configuration' },
            { text: '域与频道', link: '/guide/domains' },
            { text: '权限', link: '/guide/permissions' },
            { text: '文件存储', link: '/guide/storage' },
            { text: '邮件', link: '/guide/email' },
            { text: '第三方登录', link: '/guide/oauth' },
          ],
        },
        {
          text: '其他',
          items: [
            { text: '常见问题', link: '/guide/faq' },
          ],
        },
      ],

      '/deploy/': [
        {
          text: '部署',
          items: [
            { text: '部署到服务器', link: '/deploy/' },
            { text: 'Docker Compose', link: '/deploy/compose' },
            { text: '单容器', link: '/deploy/docker' },
            { text: '单二进制', link: '/deploy/binary' },
          ],
        },
        {
          text: '运维',
          items: [
            { text: '反代与 HTTPS', link: '/deploy/nginx' },
            { text: '数据库', link: '/deploy/database' },
            { text: '监控', link: '/deploy/monitoring' },
            { text: '故障排查', link: '/deploy/troubleshooting' },
          ],
        },
      ],

      '/sfu/': [
        {
          text: '语音服务',
          items: [
            { text: '概述', link: '/sfu/' },
            { text: '自建 LiveKit', link: '/sfu/selfhost' },
            { text: 'LiveKit Cloud', link: '/sfu/cloud' },
          ],
        },
      ],

      '/reference/': [
        {
          text: '参考',
          items: [
            { text: 'API', link: '/reference/api' },
            { text: '信令事件', link: '/reference/events' },
            { text: '错误码', link: '/reference/errors' },
          ],
        },
      ],

      '/architecture/': [
        {
          text: '架构',
          items: [
            { text: '整体设计', link: '/architecture/' },
          ],
        },
      ],
    },

    socialLinks: [
      { icon: 'github', link: 'https://github.com/NoelOrin/GOSpeak' },
    ],

    footer: {
      message: '基于 Apache 2.0 许可开源',
      copyright: 'Copyright © 2026 GOSpeak',
    },

    search: {
      provider: 'local',
    },

    outline: {
      level: [2, 3],
      label: '本页目录',
    },

    docFooter: {
      prev: '上一篇',
      next: '下一篇',
    },

    lastUpdated: {
      text: '最后更新',
    },

    returnToTopLabel: '回到顶部',
    sidebarMenuLabel: '目录',
    darkModeSwitchLabel: '主题',
    lightModeSwitchTitle: '切到浅色',
    darkModeSwitchTitle: '切到深色',
  },
})
