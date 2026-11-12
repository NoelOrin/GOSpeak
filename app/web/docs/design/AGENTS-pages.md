# AGENTS.md — pages

## Routing

使用 **@tanstack/react-router** 文件路由，路由目录为 `src/pages/`。`routeTree.gen.ts` 由 `vite.config.ts` 内联的 `tanstackRouter` 插件自动生成（`target: "react"`、`autoCodeSplitting: true`、`routeFileIgnorePrefix: "-"`、`routeFileIgnorePattern: "components"`），**禁止手改**。`createRouter` 在 `src/main.tsx` 中通过 `RouterProvider` 挂载，并注入 `context: { queryClient }`（`@tanstack/react-query`）。

## Structure

`(app)/` 下的页面现为平铺 `.tsx` 文件，管理页组仍按子目录组织：

```
pages/
├── __root.tsx                     # 根路由（createRootRouteWithContext，全局 context + errorComponent）
├── login.tsx                      # 登录页（平铺文件）
├── register.tsx                   # 注册页
├── guest.tsx                      # 访客页
├── (app)/                         # 需要登录的路由组（不生成 URL 段）
│   ├── route.tsx                  # 布局路由：beforeLoad 守卫 + Layout + 页面级 PageErrorBoundary
│   ├── index.tsx                  # 首页（redirect 到 /discover）
│   ├── discover.tsx               # 发现页
│   ├── chat.tsx                   # 聊天页
│   ├── domain.$domainUUID.tsx     # 域工作区页（动态段 $domainUUID）
│   ├── link.tsx                   # 分享链接页
│   ├── profile.tsx                # 个人资料页
│   └── manage/                    # 管理页组
│       ├── route.tsx              # 管理布局路由（hasManageAccess 守卫）
│       ├── sfu/index.tsx          # 各管理子页（平铺 index.tsx）
│       ├── oauth/index.tsx
│       ├── domains/index.tsx (+ $domainUUID/index.tsx)
│       ├── users/、roles/、ban/、mute/、monitor/、storage/、email/ …
│       └── */components/          # 页面私有组件（被 routeFileIgnorePattern 忽略）
└── login/components/              # 登录页私有弹窗（ForgotPasswordModal、ForcePasswordChangeModal…）
```

`routeTree.gen.ts` 生成在 `src/` 根下，不在 `pages/` 内。已无 `domain/$domainUUID/index.tsx`、`index/index.tsx`、`link/index.tsx` 这类嵌套结构，`(app)/` 一级路由改为平铺文件。

## Route Groups

- `(app)/` — 路由组前缀，不生成 URL 段。如 `/(app)/domain/$domainUUID` 实际访问 `/domain/:domainUUID`。
- `route.tsx` 在路由组中定义 layout（`Layout` + `Outlet`）和 `beforeLoad` 守卫；`(app)/route.tsx` 先 `useUserStore.getState().ensureSession()` 无感续期，失败 `throw redirect({ to: "/login" })` 并记录 `gospeak_redirect`。
- `manage/` 为嵌套布局组，`manage/route.tsx` 的 `beforeLoad` 用 `hasManageAccess()` 校验，失败 `throw redirect({ to: "/discover" })`。
- 动态段用点号文件名（`domain.$domainUUID.tsx`）表达，字符串前缀 `-` 的文件与 `components/` 目录不参与路由生成。

## Conventions

- 每个路由文件 `export const Route = createFileRoute("/(app)/xxx")({ ... })`；根路由用 `createRootRouteWithContext<RouterContext>()`。
- `staticData: { title }` 用于标题 / 导航展示（`icon` 已不再由 `staticData` 提供，侧边栏菜单由布局统一渲染）。
- 路由守卫在 `beforeLoad` 中以 `throw redirect(...)` 实现。
- 页面组件尽量轻量，逻辑下沉到 `components/`、`hooks/`、`stores/`；服务端数据用 `@tanstack/react-query` 的 `useQuery`（context 中的 `queryClient`）。
- 复杂 UI 下沉到同级 `components/`（不会进入路由）；跨路由复用再提升到 `src/components/`。
- 目标：管理类页面编排层通常 < 350 行。

## Error Handling

错误组件集中在 `components/common/errorBoundary.tsx` / `errorFallback.tsx`，按三层兜底：

- **router 级**：`src/main.tsx` 的 `createRouter` 设置 `defaultErrorComponent`，渲染 `RouteErrorComponent` 并注入 `onRetry={() => void router.invalidate()}`——loader 抛错时单靠 reset 不会重新请求，必须 invalidate；另有 `defaultNotFoundComponent`。
- **根路由级**：`__root.tsx` 的 `errorComponent: RouteErrorComponent` 兜住根路由自身错误（子路由错误由 router 的 `defaultErrorComponent` 处理）。`RouteErrorComponent` 覆盖渲染错误、loader 错误与 `beforeLoad` 错误，并负责上报。
- **页面级**：`(app)/route.tsx` 用 `<PageErrorBoundary resetKeys={[location.pathname]}>` 包住 `Layout + Outlet`，应用外壳崩溃时整页兜底，切换路由（`resetKeys` 变化）自动恢复。局部高风险模块用组件级 `<ErrorBoundary>`（默认 `level="component"`，可选 `level="page"`，`props` 含 `name?` / `fallback?` / `resetKeys?` / `onReset?` / `onError?` / `report?`）或 `withErrorBoundary()`；组件级边界已接入 `components/common/dynamicRender.tsx`、`layouts/common/domainWorkspace.tsx`、`layouts/layout.tsx`，各包一层。

`resetKeys` 传路由参数、房间 id、tab 等依赖，变化时边界自动 `reset()`；`onReset` 用于清 react-query 缓存或重置 store，`onError` / `report` 控制上报。`ErrorFallback` 接 `ErrorLevel = "component" | "page"`，区分 `component`（局部虚线框）与 `page`（全屏）两级。`ErrorBoundary` props 含 `name?`、`level?:"component"|"page"`（默认 `component`）、`fallback?`、`title?`、`description?`、`resetKeys?`、`onReset?`、`onError?`、`report?`。`main.tsx` 在入口处 `installGlobalErrorHandlers()` 注册 `window.error` + `unhandledrejection` 兜底，会触发 `router.invalidate()` 重试。异步错误（事件回调 / Promise）用 `hooks/useErrorHandler.ts`（返回 `(error: unknown) => void`）抛回渲染阶段交由最近边界，或交由全局监听上报。

## 页面与私有组件

- 页面文件只保留 Route 定义、数据加载、权限与提交编排。
- 复杂 UI 下沉到同级 `components/`（不会进入路由）。
- 跨路由复用再提升到 `src/components/`。
- 目标：管理类页面编排层通常 < 350 行。
