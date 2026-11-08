# @gospeak/web — GOSpeak 前端

基于 React 19 的自托管游戏语音平台 Web 客户端。Vite SPA，通过 `/api/v1` HTTP 接口与 `/ws` 原生 WebSocket 连接 Go 后端，媒体会话由 `@gospeak/sfu-client` 接入 LiveKit。

> 前端已从 SolidJS 全量迁移到 React 19，迁移期间暂存的 `src/_solid-legacy/` 已随迁移完成删除。

## 技术栈

| 领域 | 选型 |
|---|---|
| 框架 | React 19 + react-dom 19 + TypeScript |
| 路由 | `@tanstack/react-router`（文件路由 `src/pages`，`routeTree.gen.ts` 自动生成） |
| 数据 | `@tanstack/react-query` + axios（`src/api/apiClient.ts` 统一拦截与会话续期） |
| 状态 | zustand（`useXxxStore` selector + `xxxStore` 命令式门面） |
| UI | workspace 包 `@gospeak/components`（Radix 无头组件 + 自有主题 token）|
| 样式 | Tailwind CSS 4（`@tailwindcss/vite`），主题变量见 `packages/components/src/theme.css` |
| 图标 / 通知 | `lucide-react` / sonner |
| 表单 | react-hook-form + zod |
| 构建 | rolldown-vite 7 + `@vitejs/plugin-react` + `@tanstack/router-plugin/vite` + `@tanstack/devtools-vite` |
| 测试 | vitest 4 + `@testing-library/react` |
| 规范 | Biome（tab 缩进、LF、双引号） |

## 命令

```bash
pnpm dev          # 开发服务器，端口 3000（代理 /api/v1、/ws 到 :8998）
pnpm build        # vite build + tsc 类型检查
pnpm test         # vitest run
pnpm lint         # biome lint
pnpm format       # biome format
pnpm check        # biome check（format + lint）
```

在仓库根目录也可用 `pnpm dev:web` / `pnpm build:web` / `pnpm typecheck`。

**测试环境注意**：jsdom 环境初始化较慢，本机跑全量测试时建议加 `--no-file-parallelism --maxWorkers=1`，否则 forks worker 可能启动超时：

```bash
pnpm --filter @gospeak/web exec vitest run --no-file-parallelism --maxWorkers=1
```

## 目录结构

```
src/
├── pages/          # 文件路由（(app)/ 为需要登录的路由组，不生成 URL 段；manage/ 为管理区）
├── layouts/        # 应用外壳（layout.tsx / common/domainWorkspace.tsx / common/dynamicRender.tsx 各包一层组件级 ErrorBoundary）
├── components/     # 业务组件，按 room / textRoom / chat / domain / discover / manage / modal / form / home / oauth / profile / dashboard … 分组
│   └── common/     # 通用组件，含 ErrorBoundary / PageErrorBoundary / RouteErrorComponent / withErrorBoundary
├── stores/         # zustand store，详见 docs/design/AGENTS-stores.md
├── hooks/          # React hooks（media / useAuthParallax / useUpload / useErrorHandler）
├── api/            # HTTP 客户端与按领域拆分的 API 模块
├── socket/         # WebSocket 客户端（重连、ACK、房间状态、SFU 热切换）
├── handler_audio/  # AudioContext、按成员音量控制
├── protocol/       # WS 协议类型
├── types/ utils/ assets/
├── main.tsx        # 入口：QueryClient + RouterProvider
├── styles.css      # 全局样式入口
└── routeTree.gen.ts # 自动生成，勿手改
```

路径别名：`@/` → `src/`，`#/` → `types/`。

## 路由与错误边界

路由规则、插件参数、常用 API 与示例见 [TanStack Router 使用指南](docs/lib/tanstack-router.md)（`vite.config.ts` 内联 `tanstackRouter({ target: "react" })`，构建时自动生成路由树）。

三级错误边界，共用一个统一上报入口 `src/utils/errorReport.ts`（导出 `normalizeError` / `registerErrorReporter` / `reportError` / `errorSummary` / `installGlobalErrorHandlers`，`ErrorScope = "component" | "page" | "route" | "global"`）：

- **组件级**：`<ErrorBoundary>`（默认 `level="component"`，可选 `level="page"` 或直接用 `PageErrorBoundary` 做整页兜底），props 含 `name?`、`level?:"component"|"page"`、`fallback?`、`title?`、`description?`、`resetKeys?`、`onReset?`、`onError?`、`report?`；也可包一层 HOC `withErrorBoundary()`；只兜住局部子树，例如左侧动态面板、语音舞台、文字频道、成员列表。组件级边界已接入 `components/common/dynamicRender.tsx`、`layouts/common/domainWorkspace.tsx`、`layouts/layout.tsx`，各自包一层。
- **页面级**：`src/pages/(app)/route.tsx` 用 `<PageErrorBoundary resetKeys={[pathname]}>` 包住 `Layout` + `Outlet`，路由切换（`resetKeys` 变化）自动 `reset()`。
- **路由级**：`src/main.tsx` 的 `defaultErrorComponent` 与 `src/pages/__root.tsx` 的 `errorComponent` 均指向 `RouteErrorComponent`（接 router `errorComponent`）。

异步错误用 `useErrorHandler()`（返回 `(error: unknown) => void` 的函数）在 Promise.catch / 事件回调中重抛到最近的边界；`window.error` / `unhandledrejection` 由 `installGlobalErrorHandlers()` 兜底上报，会触发 `router.invalidate()` 重试。生产环境的错误详情不暴露堆栈。

## 开发约定

- 组件采用 React 函数组件 + hooks；条件渲染用 `&&` / 三元，列表用 `.map()` + `key`
- 通用 UI 优先复用 `@gospeak/components`，图标一律 `lucide-react`，品牌图标走 `components/oauth/ProviderIcon.tsx`
- 类名用 `cn()`（clsx + tailwind-merge）合并；Biome 开启 `noUnusedImports: error`
- 页面编排层保持轻量，复杂 UI 下沉到同级 `components/`（该目录不会被生成成路由）

更多设计约定见 `docs/AGENTS.md`、`docs/design/` 与 `docs/lib/`。后端与整体架构见仓库根 `AGENTS.md`、`ARCHITECTURE.md`，运维与部署文档见 `app/docs/` 与 `docs/`。
