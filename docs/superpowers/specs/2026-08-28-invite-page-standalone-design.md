# 邀请页独立化与视觉重构设计

日期：2026-08-28

## 背景与问题

`/invite/d/$code` 目前挂在 `(app)` 布局组内，桌面端呈现为：

- 左栏（Split prev）：Sidebar 图标栏 + 首页仪表盘（「快捷入口」面板）——与加入域毫无关系；
- 右栏（Main）：一张 max-w-md 卡片垂直居中悬浮在大片空旷的 base-200 背景上。

问题：页面空旷、视觉断裂、信息层级弱；移动端则被困在「Header + 底部导航」的壳里，也不是邀请确认页应有的聚焦形态。

## 方案对比

| 方案 | 说明 | 取舍 |
|------|------|------|
| A. 独立全屏页（推荐） | 路由迁出 `(app)` 组到 `pages/invite/d/$code/`，自建会话守卫，全屏居中卡片 | URL 不变（`(app)` 是 pathless 组）；邀请链接获得 Discord 式聚焦体验；桌面/移动端形态统一 |
| B. 留在壳内重做卡片 | 保留布局，仅加宽卡片、加信息 | 左栏仪表盘依旧违和，空旷问题不解决 |
| C. 壳内全屏遮罩 | 路由留在 `(app)`，页面用 fixed 全屏背景盖住壳 | Hack，壳仍挂载在背后，维护成本高 |

选择 A。

## 设计

### 路由与守卫

- 新文件 `app/web/src/pages/invite/d/$code/index.tsx`，路由 id `/invite/d/$code/`，删除旧文件 `app/web/src/pages/(app)/invite/d/$code/index.tsx`。
- `beforeLoad` 复制 `(app)/route.tsx` 的会话守卫：`userStore.ensureSession()` 失败时写 `sessionStorage["gospeak_redirect"]` 并 `redirect({ to: "/login" })`，登录成功后由 login 页回跳，现有登录→回跳链路不变。
- `staticData: { title: "邀请加入" }`，页面内调用 `useTitle()`（standalone 页不经过 Layout，需自行设置标题）。
- 不再有 `default export`（autoCodeSplitting 约束）。

### 页面结构（视觉语言对齐新版 login 页）

```
grid 纹理背景（静态，radial mask，无视差）
├─ 品牌头：favicon + GO/Speak 字标
├─ 居中卡片（max-w-sm, border-base-content/10, bg-base-100, shadow-xl）
│   ├─ ready：eyebrow「Invite」→「你被邀请加入语音服务器」→ 64px 域头像
│   │        （icon_url 或首字母 fallback）→ 域名 → 描述 → 公开/私有 badge
│   │        → 加入失败 alert → 已加入提示 → 主 CTA（确认加入/进入域/加入中…）
│   ├─ loading：skeleton（头像 + 两行文本 + 按钮）
│   └─ invalid/error：LinkOff 图标 +「邀请链接无效或已失效」+ 返回首页
└─ 页脚：GOSpeak · 自托管游戏语音平台
```

### 逻辑复用与边界

- 不改 `DomainInvitePreview` 组件（discover 页仍在用）。新页面复用它导出的纯函数
  `getDomainInvitePreviewStatus` / `getDomainInviteAction`，展示层自行实现。
- 加入/跳转逻辑（`previewDomainInvite`、`joinDomain`、`domainStore.addDomain`、
  `setCurrentDomain`、navigate 到 `/domain/$domainUUID`）与原页面保持一致。
- 无入场动画（遵循 routeFlicker 回归测试对 opacity:0 起帧的约束，页面保持静态克制）。

### 成员判定与错误健壮性（验证阶段补充）

- standalone 页没有应用壳代为加载 `domainStore`，挂载时自行调用 `loadMyDomains()`；
  成员判定就绪前 CTA 用 skeleton 占位，避免「确认加入」闪跳成「进入域」。
- resource fetcher 失败后再读 `domain()` 会重新抛错（Solid 行为，会把页面炸进
  路由错误边界），所有读取走 `safeDomain()`（先查 `domain.error`）。
- `joinDomain` 返回 `3002 ALREADY_EXISTS`（already a member）时：刷新 myDomains、
  清空错误，按钮翻转为「进入域」，不弹红色报错。

### 牵连改动

- `src/routes/__tests__/routeFlicker.test.ts` 中硬编码的旧路径更新为新路径。
- `routeTree.gen.ts` 由 vite 插件在 build/dev 时重新生成。

### 不做的事（YAGNI）

- 不引入视差/入场动画；不展示邀请人信息（后端 preview 未返回）；不加成员数
  （preview 返回的是裸 Domain 模型）；不动 `DomainIcon`/`OptionSquare`（固定 size-12
  且带点击/tooltip 语义，不适合 hero 头像，页面内联渲染头像）。
