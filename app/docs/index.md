---
layout: home

title: GOSpeak
titleTemplate: 自部署的游戏语音平台

hero:
  name: GOSpeak
  text: 你自己的游戏语音服务器
  tagline: 一台服务器、一条命令就能跑起来。语音、文字、权限、桌面客户端全都自带，数据留在你自己手里。
  image:
    src: /logo.png
    alt: GOSpeak
  actions:
    - theme: brand
      text: 五分钟跑起来
      link: /guide/quickstart
    - theme: alt
      text: 我要部署到服务器
      link: /deploy/
    - theme: alt
      text: 下载桌面客户端
      link: /guide/desktop

features:
  - icon: 🎙️
    title: 语音和文字，一个地方
    details: 语音频道、文字频道、私聊都在同一个域里。进频道就说话，切频道不断线。
  - icon: 🖥️
    title: 有桌面客户端
    details: 关窗后留在托盘、开机自启、全局快捷键，还有能浮在游戏画面上的成员浮窗。
  - icon: 🗄️
    title: 数据库按需升级
    details: 起步一个 SQLite 文件，人多了换成 PostgreSQL 或 MySQL。不用一开始就搭一堆东西。
  - icon: 🐳
    title: 部署不折腾
    details: 一条 docker compose 命令拉起整套服务，也可以只跑一个二进制文件，前端已经打进去了。
  - icon: 🔐
    title: 权限分得清
    details: 域管理员、频道权限、访客开关各管各的。谁能听、谁能说、谁能踢人，都由你定。
  - icon: 📦
    title: 数据是你的
    details: 语音走你自己的 LiveKit，文件存在你自己的磁盘或对象存储，不经过任何第三方。
---
