# AGENTS.md — @gospeak/web

## Overview

GOSpeak 前端客户端：React 19 + TypeScript 的单页应用，提供语音房间、Domain 工作区、文字频道、访客访问与管理后台。HTTP 走 `/api/v1`，信令走 `/ws` 原生 WebSocket，媒体会话由 `@gospeak/sfu-client` 接入 LiveKit。

> 前端已从 SolidJS 全量迁移到 React 19，迁移暂存目录 `src/_solid-legacy/` 已随迁移完成清空删除，仓库内不再保留旧代码。整体仓库说明见根 `AGENTS.md`。

## Tech Stack

- **Framework**: React 19 + react-dom 19
- **Routing**: `@tanstack/react-router`（文件路由 `src/pages`，`routeTree.gen.ts` 由 Vite 插件自动生成）
- **State**: zustand（`useXxxStore` selector hook + `xxxStore` 命令式门面，见 `src/stores`）
- **Data Fetching**: `@tanstack/react-query` + axios（`src/api/apiClient.ts` 统一拦截、错误 toast、会话续期）
- **Styling**: Tailwind CSS v4 + `@gospeak/components` 主题 token（`packages/components/src/theme.css`）；类名合并用 `cn()`（clsx + tailwind-merge）
- **UI Library**: workspace 包 `@gospeak/components`（Radix 无头组件：Button / Card / Dialog / Input / Avatar / Toaster）
- **Icons / Toast**: `lucide-react` / sonner（品牌图标用 `components/oauth/ProviderIcon.tsx`）
- **Forms**: react-hook-form + zod
- **Realtime**: 原生 WebSocket + `@gospeak/sfu-client` 多 Provider 抽象
- **Lint/Format**: Biome（tab 缩进、LF、双引号）
- **Test**: vitest 4 + `@testing-library/react`（jsdom）
- **Build**: rolldown-vite 7 + `@vitejs/plugin-react` + `@tanstack/router-plugin/vite` + `@tailwindcss/vite`

## Commands

```bash
pnpm dev          # 开发服务器 (port 3000, proxy :8998)
pnpm build        # vite build + tsc 类型检查
pnpm test         # vitest run
pnpm format       # biome format
pnpm lint         # biome lint
pnpm check        # biome check (format + lint)
```

本机 jsdom 环境初始化较慢，跑全量测试建议加 `--no-file-parallelism --maxWorkers=1`，避免 forks worker 启动超时。

## Path Aliases

- `@/` → `src/`
- `#/` → `types/`

## Project Structure

```
src/
├── pages/          # 文件路由（(app)/ 登录路由组、manage/ 管理区；平铺 .tsx 为主）
├── layouts/        # 应用外壳：layout.tsx + common/{header,main,sidebar,domainWorkspace}
├── components/     # 业务组件，按 room / textRoom / chat / domain / discover / manage / modal / form / home / oauth / profile / dashboard … 分组
│   └── common/     # 通用组件，含错误边界 errorBoundary.tsx（ErrorBoundary / PageErrorBoundary / RouteErrorComponent / withErrorBoundary）/ errorFallback.tsx
├── stores/         # zustand store → 详见 docs/design/AGENTS-stores.md
├── hooks/          # React hooks → 详见 docs/design/AGENTS-hooks.md
├── api/            # HTTP 客户端封装（axios）与按领域拆分的 API 模块
├── socket/         # WS 客户端：wsClient / events / socketEvents / roomState / providerReload / tabLock
├── handler_audio/  # 音频处理（AudioContext、按成员音量控制）
├── protocol/       # WS 协议类型
├── types/ utils/ assets/
├── main.tsx        # 入口：QueryClientProvider + RouterProvider
├── styles.css      # 全局样式入口
└── routeTree.gen.ts # 自动生成，勿手改
```

## Key Patterns

### Routing
- 使用 `@tanstack/react-router` 文件路由，路由定义在 `src/pages/`，导出 `createFileRoute(...)` 的 `Route`
- `(app)/` 前缀为路由组（layout group），不生成 URL 段；`route.tsx` 是布局路由，页面多为平铺 `.tsx`
- `routeFileIgnorePrefix: "-"` 忽略 `-` 开头的文件；`routeFileIgnorePattern: "components"` 忽略 components 子目录
- 插件内联配置在 `vite.config.ts`（`target: "react"`、`autoCodeSplitting: true`），dev/build 时自动生成 `routeTree.gen.ts`，无需 `tsr watch`
- 完整 API 与示例见 `docs/lib/tanstack-router.md`

### State Management
- 全局 store 用 zustand：组件内 `useXxxStore(selector)` 订阅，事件回调/异步流程用 `xxxStore` 命令式门面
- 局部状态用 React `useState` / `useEffect` / `useMemo`
- 页面数据优先 `@tanstack/react-query` 的 `useQuery`，`context.queryClient` 由 `main.tsx` 注入

### Styling
- Tailwind CSS v4（`@tailwindcss/vite`），主题 token 在 `packages/components/src/theme.css`（acid 浅色 / dark 深色）
- 通用组件从 `@gospeak/components` 引入，图标一律 `lucide-react`
- 合并类名用 `cn()`；不要引入 daisyUI 语义类

### Realtime Architecture
- **WebSocket**: 房间管理（创建/加入/离开/列表）+ 成员同步 + 事件推送；握手鉴权由 HttpOnly access cookie 自动携带
- **SFU client layer**: 统一页面侧媒体会话生命周期，按运行时 provider 动态加载对应客户端
- **Current state**: `socketStore` 管理房间状态；`VoiceSessionProvider` 常驻持有语音会话，切换文字频道不断音

### SFU Provider rules
- 页面组件避免直接依赖任一 provider SDK 类型，统一走 `@gospeak/sfu-client`
- 管理能力由服务端 capabilities 下发，UI 按 capability-based 降级
- `@gospeak/sfu-client` 语义上是「媒体会话客户端」，不要扩展成管理动作或本地播放控制抽象

### Mute semantics
- `静音` = 本地对远端音轨静音，只影响当前客户端播放
- `禁言` = 用户级仅收听限制，允许听，但禁止发布本地麦克风轨道
- 不存在房间级静音作为前端产品语义
- 不要把本地静音和用户级禁言混为一谈

### Error Handling
- 三层边界：组件级 `<ErrorBoundary>`（局部 fallback + 重试 + `resetKeys`）、页面级 `PageErrorBoundary`（`(app)/route.tsx` 以 `resetKeys={[pathname]}` 包住 Layout + Outlet，路由切换自动 `reset()`）、路由级 `RouteErrorComponent`（`main.tsx` 的 `defaultErrorComponent` 与 `__root.tsx` 的 `errorComponent`）
- loader 抛错时重试需 `router.invalidate()`，由 `main.tsx` 注入 `onRetry`
- 异步错误用 `useErrorHandler()` 重抛到最近的边界；`window.error` / `unhandledrejection` 由 `installGlobalErrorHandlers()` 兜底上报
- 统一上报入口 `src/utils/errorReport.ts`，导出 `normalizeError` / `registerErrorReporter`（可接 Sentry 等外部日志服务）/ `reportError` / `errorSummary` / `installGlobalErrorHandlers`，`ErrorScope = "component" | "page" | "route" | "global"`；生产环境错误详情不暴露堆栈

## Conventions

- 文件名沿用所在目录既有风格：页面平铺 `discover.tsx`、通用组件 `camelCase`、页面私有组件多为 `PascalCase`
- 事件名常量集中定义在 `src/socket/events.ts` 的 `EVENTS` 对象
- TypeScript 严格模式；Biome 开启 `noUnusedImports: error`、`noExplicitAny: off`
- 页面编排层保持轻量，复杂 UI 下沉到同级 `components/`（该目录不会被生成成路由）

## Proxy

开发模式下 Vite 代理：
- `/api/v1` → `http://localhost:8998`
- `/ws` → `http://localhost:8998`（含 WebSocket）

## Docs

```
docs/
├── AGENTS.md                   # 本文件（前端全局概览）
├── design/                     # 架构与设计约定
│   ├── AGENTS-components.md    # 组件结构与抽离粒度
│   ├── AGENTS-pages.md         # 路由、页面结构与错误边界
│   ├── AGENTS-stores.md        # zustand 状态管理
│   └── AGENTS-hooks.md         # hooks 与语音会话分层
└── lib/
    ├── tanstack-router.md      # TanStack Router (React) 使用指南与 API
    └── openapi.yaml            # GOSpeak API OpenAPI 3.0 规范
```
