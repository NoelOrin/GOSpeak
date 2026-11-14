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
      text: 下载客户端
      link: '#download'
    - theme: alt
      text: 五分钟跑起来
      link: /guide/quickstart
    - theme: alt
      text: 部署到服务器
      link: /deploy/
---

<p class="landing-platforms">浏览器直接访问 · macOS · Windows · Linux 桌面端</p>

## 界面预览

一分钟看明白。

<div class="screenshot-hero">
  <img src="/screenshots/login.png" alt="登录页：GOSpeak 品牌区与登录表单" />
  <p>登录页 — 独立部署，无第三方鉴权依赖</p>
</div>

<div class="screenshot-pair">
  <figure>
    <img src="/screenshots/domains.png" alt="发现域" />
    <figcaption>发现域 — 公开域检索、邀请码加入与已加入域快捷入口</figcaption>
  </figure>
  <figure>
    <img src="/screenshots/room.png" alt="语音房间" />
    <figcaption>语音房间 — 频道树、实时发言检测、成员面板与逐人音量</figcaption>
  </figure>
</div>

<div class="screenshot-pair">
  <figure>
    <img src="/screenshots/chat.png" alt="聊天" />
    <figcaption>聊天 — 频道与私聊共用一栏，右侧常驻成员列表</figcaption>
  </figure>
  <figure>
    <img src="/screenshots/invite.png" alt="邀请码" />
    <figcaption>邀请码 — 按域创建、禁用与查看使用记录</figcaption>
  </figure>
</div>

## 为什么自建

大多数语音平台把你的数据放在别人的服务器上。GOSpeak 把服务器还给你。

<div class="why-grid">
  <div class="why-card">
    <span class="why-num">01</span>
    <h3>数据主权</h3>
    <p>语音与消息数据不经过第三方，全部落在自己的服务器与数据库里。</p>
  </div>
  <div class="why-card">
    <span class="why-num">02</span>
    <h3>策略自由</h3>
    <p>语音路由、权限、禁言与访客能力都按自己的社群规则来配置。</p>
  </div>
  <div class="why-card">
    <span class="why-num">03</span>
    <h3>没有上限</h3>
    <p>用户数、房间数、并发由你的硬件决定，不为席位付费。</p>
  </div>
</div>

## 对比

把语音交给第三方，还是自己托管？

| 维度 | 托管语音服务 | 自托管 GOSpeak |
| ---- | ------------ | -------------- |
| 数据归属 | 存在第三方服务器 | 全部落在自有数据库 |
| 用户与房间数 | 平台或套餐限制 | 由部署硬件决定 |
| 语音引擎 | 不可控、不可换 | LiveKit SFU，自建或云用 |
| 品牌与集成 | 平台约束 | 自有域名，域级角色与权限 |
| 退出成本 | 数据锁定 | 标准 API 与 SQL，随时迁出 |

## 轻量、快、开箱即用

<div class="stat-grid">
  <div class="stat-card">
    <strong>单二进制</strong>
    <span>前端通过 go:embed 内嵌，下载即用。</span>
  </div>
  <div class="stat-card">
    <strong>SQLite 开箱</strong>
    <span>零外部依赖即可启动，并发读可开 WAL。</span>
  </div>
  <div class="stat-card">
    <strong>内嵌事件总线</strong>
    <span>NATS 状态共享与任务队列随进程启动。</span>
  </div>
  <div class="stat-card">
    <strong>渐进式数据库</strong>
    <span>PostgreSQL、MySQL、Turso 平滑升级。</span>
  </div>
</div>

## 功能

该有的都有，没有的多余。

<div class="feature-grid">
  <div class="feature-card">
    <span class="feature-icon">🎙️</span>
    <h3>语音房间</h3>
    <p>创建与加入语音频道，支持密码保护与人数上限。实时发言检测与逐人音量控制。</p>
  </div>
  <div class="feature-card">
    <span class="feature-icon">💬</span>
    <h3>文字消息</h3>
    <p>频道消息与私聊会话，支持提及、表情回应与已读回执。</p>
  </div>
  <div class="feature-card">
    <span class="feature-icon">🖥️</span>
    <h3>桌面客户端</h3>
    <p>关窗进托盘、开机自启、全局快捷键与游戏内成员浮窗，与网页版共用同一套前端。</p>
  </div>
  <div class="feature-card">
    <span class="feature-icon">🚀</span>
    <h3>单二进制部署</h3>
    <p>裸二进制或 Docker Compose，五分钟上线。</p>
  </div>
</div>

<p class="feature-chips">
  <span>同源 LiveKit 反代</span><span>域多租户</span><span>访客访问体系</span><span>RBAC 权限</span><span>OAuth2 登录</span><span>邮件验证</span><span>对象存储</span><span>审计日志</span>
</p>

<ClientDownload />

<style>
:root:root {
  --vp-home-hero-name-color: transparent;
  --vp-home-hero-name-background: linear-gradient(120deg, #4d7c0f 10%, #84cc16 55%, #a3e635);
  --vp-home-hero-image-background-image: linear-gradient(-45deg, rgba(132, 204, 22, 0.32) 40%, rgba(96, 165, 250, 0.28) 50%);
  --vp-home-hero-image-filter: blur(48px);
}

.dark.dark {
  --vp-home-hero-name-color: transparent;
  --vp-home-hero-name-background: linear-gradient(120deg, #a3e635 10%, #bef264 55%, #d9f99d);
  --vp-home-hero-image-background-image: linear-gradient(-45deg, rgba(132, 204, 22, 0.26) 40%, rgba(59, 130, 246, 0.22) 50%);
}

.VPHome {
  position: relative;
}

.VPHome::before {
  content: '';
  position: absolute;
  top: -140px;
  left: 50%;
  width: min(960px, 100vw);
  height: 620px;
  transform: translateX(-50%);
  background:
    radial-gradient(40% 52% at 30% 36%, rgba(132, 204, 22, 0.14), transparent 70%),
    radial-gradient(36% 46% at 72% 28%, rgba(96, 165, 250, 0.1), transparent 70%);
  filter: blur(28px);
  pointer-events: none;
}

.landing-platforms {
  width: fit-content;
  margin: -6px auto 48px;
  padding: 6px 18px;
  font-size: 13px;
  color: var(--vp-c-text-2);
  background: var(--vp-c-bg-soft);
  border: 1px solid var(--vp-c-divider);
  border-radius: 999px;
}

.screenshot-hero { margin: 24px 0 32px; }
.screenshot-hero img {
  width: 100%;
  border-radius: 14px;
  border: 1px solid var(--vp-c-divider);
  box-shadow: 0 24px 56px -24px rgba(0, 0, 0, 0.28);
}
.dark .screenshot-hero img { box-shadow: 0 24px 56px -20px rgba(0, 0, 0, 0.6); }
.screenshot-hero p { margin-top: 10px; text-align: center; color: var(--vp-c-text-2); font-size: 14px; }
.screenshot-pair { display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 16px; margin-bottom: 16px; }
.screenshot-pair figure { margin: 0; }
.screenshot-pair img {
  width: 100%;
  border-radius: 12px;
  border: 1px solid var(--vp-c-divider);
  transition: border-color 0.25s ease, box-shadow 0.25s ease;
}
.screenshot-pair img:hover {
  border-color: var(--vp-c-brand-2);
  box-shadow: 0 14px 36px -18px rgba(0, 0, 0, 0.3);
}
.screenshot-pair figcaption { margin-top: 8px; text-align: center; color: var(--vp-c-text-2); font-size: 13px; }

.why-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 16px;
  margin: 24px 0 8px;
}

.why-card,
.stat-card,
.feature-card {
  background: linear-gradient(180deg, var(--vp-c-bg-soft), var(--vp-c-bg));
  border: 1px solid var(--vp-c-divider);
  border-radius: 14px;
  transition: transform 0.25s ease, box-shadow 0.25s ease, border-color 0.25s ease;
}

.why-card:hover,
.stat-card:hover,
.feature-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 14px 36px -14px rgba(0, 0, 0, 0.2);
}

.dark .why-card:hover,
.dark .stat-card:hover,
.dark .feature-card:hover {
  box-shadow: 0 14px 36px -14px rgba(0, 0, 0, 0.55);
}

.why-card { padding: 20px 22px; }

.why-num {
  font-size: 26px;
  font-weight: 700;
  line-height: 1;
  background: linear-gradient(120deg, #4d7c0f, #84cc16);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
  color: transparent;
}

.dark .why-num { background: linear-gradient(120deg, #a3e635, #d9f99d); background-clip: text; -webkit-text-fill-color: transparent; }

.why-card h3 { margin: 10px 0 8px; font-size: 16px; font-weight: 600; }
.why-card p { margin: 0; font-size: 14px; line-height: 1.7; color: var(--vp-c-text-2); }

.stat-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 16px;
  margin: 24px 0 8px;
}

.stat-card {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 18px 20px;
}

.stat-card strong { font-size: 15px; font-weight: 600; }
.stat-card span { font-size: 13px; line-height: 1.7; color: var(--vp-c-text-2); }

.feature-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 16px;
  margin: 24px 0 8px;
}

.feature-card { padding: 20px 22px; }

.feature-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 44px;
  height: 44px;
  font-size: 22px;
  border-radius: 12px;
  background: linear-gradient(135deg, rgba(132, 204, 22, 0.18), rgba(132, 204, 22, 0.05));
  border: 1px solid rgba(132, 204, 22, 0.28);
}

.feature-card h3 { margin: 12px 0 6px; font-size: 16px; font-weight: 600; }
.feature-card p { margin: 0; font-size: 14px; line-height: 1.7; color: var(--vp-c-text-2); }

.feature-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin: 16px 0 8px;
}

.feature-chips span {
  padding: 4px 14px;
  font-size: 13px;
  color: var(--vp-c-text-2);
  background: var(--vp-c-bg-soft);
  border: 1px solid var(--vp-c-divider);
  border-radius: 999px;
  transition: color 0.25s ease, border-color 0.25s ease;
}

.feature-chips span:hover {
  color: var(--vp-c-brand-1);
  border-color: var(--vp-c-brand-1);
}
</style>
