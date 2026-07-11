---
layout: home

title: GOSpeak
titleTemplate: 自部署游戏语音平台

hero:
  name: GOSpeak
  text: 自部署 Discord 语音平替
  tagline: 开箱即用的游戏语音平台。基于 WebRTC，支持多 SFU 后端运行时切换，渐进式数据库，自托管语音数据。
  image:
    src: /logo.svg
    alt: GOSpeak
  actions:
    - theme: brand
      text: 快速开始
      link: /guide/getting-started
    - theme: alt
      text: SFU 配置
      link: /sfu/
    - theme: alt
      text: Docker Compose 部署
      link: /deployment/

features:
  - icon: 🎮
    title: 游戏语音频道
    details: 创建/加入语音房间，类Discord频道体验。支持密码保护、创建者/管理员踢人、角色权限体系。
  - icon: 🔄
    title: 多 SFU 切换
    details: 运行时切换 LiveKit / SRS / MediaSoup / Agora / Daily，不修改代码。自建或云服务任选。
  - icon: 🗄️
    title: 渐进式数据库
    details: SQLite 开箱即用零配置，可按需升级到 PostgreSQL + Redis，无供应商锁定。
  - icon: 🔐
    title: 多认证方式
    details: JWT + OAuth2 三端登录（GitHub / Google / QQ）。内置 RBAC 权限控制。
  - icon: 🔊
    title: 实时语音控制
    details: 发言检测、成员独立音量控制、一键静音。所有设置持久化到 IndexedDB。
  - icon: 🐳
    title: 一键 Docker 部署
    details: 完整 docker-compose 编排，profile 按需选择组件。从本地开发到生产一键升级。
---
