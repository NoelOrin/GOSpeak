# TanStack Router (React) 使用指南

## 概述

本项目使用 `@tanstack/react-router` 做路由，`@tanstack/router-plugin/vite` 做文件路由自动生成。插件配置**内联在 `vite.config.ts`**（`tanstackRouter({ target: "react", ... })`），启动 dev/build 时自动维护 `src/routeTree.gen.ts`，无需手动执行命令。

> `tsr.config.json` 供路由 CLI（如手动执行 `tsr generate`）读取，`target` 与 `vite.config.ts` 一致为 `react`；日常 dev/build 走 Vite 插件内联配置。`routeTree.gen.ts` 为生成产物，不要手改，biome / tsconfig 已将其排除。

## 核心概念

### 文件路由

路由文件放在 `src/pages/`，由 TanStack Router Plugin 自动生成 `routeTree.gen.ts`。

```
src/pages/
├── __root.tsx                 # 根路由（Outlet + errorComponent）
├── login.tsx                  # → /login
├── register.tsx / guest.tsx
├── (app)/                     # 路由组（不生成 URL 段）
│   ├── route.tsx              # 布局路由（登录守卫 + Layout + Outlet）
│   ├── index.tsx              # → /（重定向到 /discover）
│   ├── discover.tsx           # → /discover
│   ├── chat.tsx / link.tsx / profile.tsx
│   ├── domain.$domainUUID.tsx # → /domain/:domainUUID
│   └── manage/
│       ├── route.tsx          # 管理区布局 + 权限守卫
│       └── users/index.tsx …  # 各管理页
```

**规则**：
- `index.tsx` 匹配空路径；`route.tsx` 定义布局（嵌套路由的父级）
- `(xxx)/` 括号前缀 = 路由组，不影响 URL
- 平铺 `.tsx` 与 `xxx/index.tsx` 都会生成路由；`$param` 表示动态段
- `-` 前缀文件被忽略（`routeFileIgnorePrefix`）
- `components/` 目录被忽略（`routeFileIgnorePattern`），页面私有组件放这里不会变成路由
- `autoCodeSplitting: true`，页面按路由自动拆包

---

## 常用 API

以下 import 一律来自 `@tanstack/react-router`。

### createFileRoute

每个路由文件的核心，定义路由配置和组件。

```tsx
import { createFileRoute } from "@tanstack/react-router";

export const Route = createFileRoute("/(app)/discover")({
  component: RouteComponent,
  // 可选配置
  staticData: { title: "发现域" },
  beforeLoad: async () => { /* 路由守卫 */ },
  loader: async () => { /* 数据预加载 */ },
  errorComponent: RouteErrorComponent,
  pendingComponent: LoadingComponent,
});

function RouteComponent() {
  return <div>页面内容</div>;
}
```

### Outlet

在布局路由中渲染子路由。

```tsx
import { Outlet, createFileRoute } from "@tanstack/react-router";

function LayoutComponent() {
  return (
    <div>
      <Sidebar />
      <Outlet />  {/* 子路由渲染位置 */}
    </div>
  );
}
```

### useNavigate

编程式导航。

```tsx
import { useNavigate } from "@tanstack/react-router";

function Component() {
  const navigate = useNavigate();

  // 跳转（动态路由必须带 params）
  navigate({ to: "/domain/$domainUUID", params: { domainUUID: "abc" } });

  // 带搜索参数
  navigate({
    to: "/domain/$domainUUID",
    params: { domainUUID: "abc" },
    search: { id: "123" },
  });

  // 替换历史记录
  navigate({ to: "/login", replace: true });
}
```

### useSearch

获取当前路由的搜索参数。

```tsx
import { useSearch } from "@tanstack/react-router";

export const Route = createFileRoute("/(app)/domain/$domainUUID")({
  validateSearch: (search) => ({
    id: z.string().optional().parse(search.id),
  }),
  component: RouteComponent,
});

function RouteComponent() {
  const search = useSearch({ from: "/(app)/domain/$domainUUID" });
  // search.id → string | undefined
}
```

### useParams

获取动态路由参数。

```tsx
// 文件: pages/(app)/domain.$domainUUID.tsx
import { useParams } from "@tanstack/react-router";

function Component() {
  const { domainUUID } = useParams({ from: "/(app)/domain/$domainUUID" });
}
```

### useLocation

获取当前路由信息。

```tsx
import { useLocation } from "@tanstack/react-router";

function Component() {
  const location = useLocation();
  // location.pathname → '/discover'
  // location.search → { id: '123' }
}
```

### redirect

在 `beforeLoad` 或 `loader` 中重定向。

```tsx
import { createFileRoute, redirect } from "@tanstack/react-router";

export const Route = createFileRoute("/(app)")({
  beforeLoad: async () => {
    const ok = await useUserStore.getState().ensureSession();
    if (!ok) throw redirect({ to: "/login" });
  },
});
```

---

## 路由配置

### Vite 插件配置

```ts
// vite.config.ts
import { tanstackRouter } from "@tanstack/router-plugin/vite";

tanstackRouter({
  target: "react",
  autoCodeSplitting: true,
  routesDirectory: "./src/pages",
  generatedRouteTree: "./src/routeTree.gen.ts",
  quoteStyle: "single",
  semicolons: false,
  routeFileIgnorePrefix: "-",
  routeFileIgnorePattern: "components",
});
```

### beforeLoad 路由守卫

在路由加载前执行，适合做登录检查。本项目 `(app)/route.tsx` 用 `ensureSession()` 先尝试 refresh_token 无感续期，失败才跳 `/login`；`manage/route.tsx` 再叠加 `hasManageAccess()` 权限守卫。

### staticData

给路由附加静态数据，顶栏等组件通过 `useMatches({ select })` 读取最近的 `staticData.title`。当前只使用 `title`，不要再设置 `icon`（侧边栏图标由布局统一渲染）。

```tsx
export const Route = createFileRoute("/(app)/discover")({
  staticData: { title: "发现域" },
});
```

### loader 数据预加载

```tsx
export const Route = createFileRoute("/(app)/domain/$domainUUID")({
  loader: async ({ context }) => {
    const rooms = await fetchRooms();
    return { rooms };
  },
  component: RouteComponent,
});

function RouteComponent() {
  const data = Route.useLoaderData();
}
```

多数页面直接用 `@tanstack/react-query` 的 `useQuery` 取数，`context.queryClient` 由 `main.tsx` 注入。

---

## 数据集成

### 配合 TanStack Query

```tsx
import { useQuery } from "@tanstack/react-query";

function Component() {
  const { data, isLoading } = useQuery({
    queryKey: ["public-domains"],
    queryFn: listPublicDomains,
  });

  if (isLoading) return <Loading />;
  return <RoomList rooms={data} />;
}
```

### 配合 zustand

路由组件内用 `useXxxStore(selector)` 订阅，命令式场景用 `xxxStore` 门面；局部状态用 React `useState` / `useEffect` / `useMemo`。

```tsx
import { useState } from "react";
import { useUserStore } from "@/stores/userStore";

function Component() {
  const user = useUserStore((s) => s.user);
  const [keyword, setKeyword] = useState("");
  return <div>{user?.username}</div>;
}
```

---

## 错误处理约定

错误组件集中在 `components/common/errorBoundary.tsx`，统一上报入口在 `utils/errorReport.ts`：

| 层级 | 配置位置 | 组件 |
|---|---|---|
| 组件级 | 业务组件局部 | `<ErrorBoundary>`（默认 `level="component"`，可选 `level="page"` 或直接用 `PageErrorBoundary`；props：`name?`、`level?:"component"\|"page"`、`fallback?: ReactNode \| (({error,reset,errorInfo})=>ReactNode)`、`title?`、`description?`、`resetKeys?`、`onReset?`、`onError?`、`report?`），或 HOC `withErrorBoundary()` |
| 页面级 | `src/pages/(app)/route.tsx` | `<PageErrorBoundary resetKeys={[location.pathname]}>` 包住 `Layout` + `Outlet`（`resetKeys` 变化即路由切换自动 `reset()`） |
| 路由级 | `src/main.tsx` 注册 `defaultErrorComponent` + `installGlobalErrorHandlers`、`src/pages/__root.tsx` 的 `errorComponent` | `RouteErrorComponent`（渲染错误 / loader 错误 / beforeLoad 错误） |

`errorReport.ts` 导出：`normalizeError`（归一化任意错误）、`registerErrorReporter`（可接 Sentry 等外部日志服务）、`reportError`、`errorSummary`、`installGlobalErrorHandlers`（在 `main.tsx` 注册 `window.error` + `unhandledrejection` 兜底，会触发 `router.invalidate()` 重试），`ErrorScope = "component" | "page" | "route" | "global"`。`ErrorFallback` 按 `ErrorLevel = "component" | "page"` 区分局部虚线框与全屏兜底。

loader 抛错时，重试需要 `router.invalidate()` 才会重新发起请求，因此 `main.tsx` 给 `RouteErrorComponent` 注入了 `onRetry`。事件回调和 Promise 里的异步错误不会被 React 边界捕获，用 `useErrorHandler()`（返回 `(error: unknown) => void` 的函数）重抛到渲染阶段；`installGlobalErrorHandlers()` 负责 `window.error` / `unhandledrejection` 兜底上报。

---

## 开发工具

### DevTools

`vite.config.ts` 通过 `@tanstack/devtools-vite` 启用了开发面板（端口 42069）。如需路由面板，可按需引入 `@tanstack/react-router-devtools` 的 `TanStackRouterDevtools`，仅在 `import.meta.env.DEV` 下渲染。

### 路由代码生成

`dev` / `build` 启动时由 Vite 插件自动生成并监听 `src/pages/` 变化，**不需要** `npx tsr generate` / `npx tsr watch`。`src/routeTree.gen.ts` 已加入 biome 与 linter 排除列表。

---

## 本项目中的用法示例

```tsx
// pages/(app)/route.tsx — 布局路由 + 登录守卫 + 页面级错误边界
import { Outlet, createFileRoute, redirect, useLocation } from "@tanstack/react-router";
import { PageErrorBoundary } from "@/components/common/errorBoundary";
import Layout from "@/layouts/layout";
import { useUserStore } from "@/stores/userStore";

export const Route = createFileRoute("/(app)")({
  beforeLoad: async () => {
    if (!(await useUserStore.getState().ensureSession())) {
      throw redirect({ to: "/login" });
    }
  },
  component: RouteComponent,
});

function RouteComponent() {
  const location = useLocation();
  return (
    <PageErrorBoundary resetKeys={[location.pathname]}>
      <Layout>
        <Outlet />
      </Layout>
    </PageErrorBoundary>
  );
}
```

```tsx
// pages/(app)/domain.$domainUUID.tsx — 带动态参数的页面路由
export const Route = createFileRoute("/(app)/domain/$domainUUID")({
  component: RouteComponent,
  staticData: { title: "语音域" },
});

function RouteComponent() {
  return (
    <div className="flex h-full">
      <RoomDetail />
    </div>
  );
}
```
