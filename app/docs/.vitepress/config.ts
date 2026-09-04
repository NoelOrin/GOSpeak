import { defineConfig } from 'vitepress'

const isProd = process.env.NODE_ENV === 'production'
const base = isProd ? '/GOSpeak/' : '/'

export default defineConfig({
  title: 'GOSpeak',
  description: '自部署游戏语音平台 — 文档',
  lang: 'zh-CN',

  base,

  head: [
    ['link', { rel: 'icon', href: `${base}favicon.png` }],
  ],

  themeConfig: {
    logo: '/logo.png',

    nav: [
      { text: '首页', link: '/' },
      { text: '快速开始', link: '/guide/getting-started' },
      { text: 'SFU 配置', link: '/sfu/' },
      { text: '部署指南', link: '/deployment/' },
      { text: '架构', link: '/architecture/' },
      { text: '权限模型', link: '/guide/permissions' },
    ],

    sidebar: {
      '/guide/': [
        {
          text: '使用指南',
          items: [
            { text: '快速开始', link: '/guide/getting-started' },
            { text: '基本使用', link: '/guide/usage' },
            { text: '环境变量', link: '/guide/configuration' },
            { text: '权限模型', link: '/guide/permissions' },
            { text: 'OAuth 登录', link: '/guide/oauth' },
            { text: '常见问题', link: '/guide/faq' },
          ],
        },
      ],

      '/sfu/': [
        {
          text: 'SFU 音视频引擎',
          items: [
            { text: '概述', link: '/sfu/' },
            { text: 'LiveKit — 自建', link: '/sfu/livekit-selfhost' },
            { text: 'LiveKit — 云服务', link: '/sfu/livekit-cloud' },
            { text: 'SRS — 自建', link: '/sfu/srs-selfhost' },
            { text: 'MediaSoup — 自建', link: '/sfu/mediasoup-selfhost' },
            { text: 'Agora — 云服务', link: '/sfu/agora-cloud' },
            { text: 'Daily — 云服务', link: '/sfu/daily-cloud' },
            { text: 'Cloudflare — 云服务', link: '/sfu/cloudflare' },
            { text: 'Provider 成熟度对比', link: '/sfu/comparison' },
          ],
        },
      ],

      '/deployment/': [
        {
          text: '部署指南',
          items: [
            { text: '概述', link: '/deployment/' },
            { text: 'Docker Compose 部署', link: '/deployment/docker-compose' },
            { text: '单容器 Docker 部署', link: '/deployment/docker' },
            { text: '生产部署', link: '/deployment/production' },
            { text: '单二进制部署', link: '/deployment/binary' },
            { text: '数据库演进', link: '/deployment/database' },
            { text: 'Nginx 配置', link: '/deployment/nginx' },
          ],
        },
      ],

      '/architecture/': [
        {
          text: '架构文档',
          items: [
            { text: '项目架构', link: '/architecture/' },
            { text: 'API 参考', link: '/architecture/api' },
            { text: '数据结构', link: '/architecture/models' },
          ],
        },
      ],
    },

    socialLinks: [
      { icon: 'github', link: 'https://github.com/noelorin/GOSpeak' },
    ],

    footer: {
      message: '基于 Apache 2.0 许可开源',
      copyright: 'Copyright © 2026 GOSpeak',
    },

    search: {
      provider: 'local',
    },
  },
})
