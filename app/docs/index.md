---
layout: home

title: GOSpeak
titleTemplate: 自部署的游戏语音平台

hero:
  name: GOSpeak
  text: 自托管的游戏语音平台
  tagline: 单个二进制即可运行，前端内嵌。基于 LiveKit 提供语音，数据存储于自有服务器。
  image:
    src: /logo.png
    alt: GOSpeak
  actions:
    - theme: brand
      text: 五分钟跑起来
      link: /guide/quickstart
    - theme: alt
      text: 部署到服务器
      link: /deploy/
    - theme: alt
      text: 桌面客户端
      link: /guide/desktop

features:
  - icon: 💬
    title: 文字消息
    details: 频道与私聊会话，支持提及与表情回应，与语音在同一处。
  - icon: 🎙️
    title: 语音房间
    details: 创建与加入语音频道，支持密码保护与人数上限。实时发言检测与逐人音量控制。
  - icon: 🖥️
    title: 桌面客户端
    details: 关窗进托盘、开机自启、全局快捷键与游戏内覆盖层，与网页版共用同一套前端。
  - icon: 🚀
    title: 单二进制部署
    details: 前端通过 go:embed 内嵌，下载即用，也可通过 Docker Compose 部署。
  - icon: 🔁
    title: 同源 LiveKit
    details: 内置 /livekit 反向代理，信令走应用域名，LiveKit 不需要单独的域名、证书或代理规则。
  - icon: 🗄️
    title: 渐进式数据库
    details: 支持 SQLite、PostgreSQL、MySQL，由 GORM 自动迁移，可从单文件起步平滑升级。
  - icon: 🔐
    title: 权限与认证
    details: 完整的角色权限体系，支持 JWT 与 OAuth2（GitHub、Google、QQ）。
---
