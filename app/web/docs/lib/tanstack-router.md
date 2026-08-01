# TanStack Router (Solid) 使用指南

## 概述

本项目使用 `@tanstack/solid-router` 做路由，`@tanstack/router-plugin/vite` 做文件路由自动生成。

## 核心概念

### 文件路由

路由文件放在 `src/pages/` 目录，由 TanStack Router Plugin 自动生成 `routeTree.gen.ts`。

```
src/pages/
├── __root.tsx              # 根路由
├── (app)/                  # 路由组（不生成 URL 段）
│   ├── route.tsx           # 布局路由（Outlet）
│   ├── domain/$domainUUID/index.tsx   # → /domain/$domainUUID
│   └── index/index.tsx     # → /
└── login/index.tsx         # → /login
```

**规则**：
- `index.tsx` 匹配空路径
- `route.tsx` 定义布局（嵌套路由的父级）
- `(xxx)/` 括号前缀 = 路由组，不影响 URL
- `-` 前缀文件被忽略（`routeFileIgnorePrefix`）
- `components/` 目录被忽略（`routeFileIgnorePattern`）

---

## 常用 API

### createFileRoute

每个路由文件的核心，定义路由配置和组件。

```tsx
import { createFileRoute } from '@tanstack/solid-router'

export const Route = createFileRoute('/(app)/domain/$domainUUID/')({
  component: RouteComponent,
  // 可选配置
  staticData: { title: '语音域', icon: 'icon-channel' },
  beforeLoad: async () => { /* 路由守卫 */ },
  loader: async () => { /* 数据预加载 */ },
  errorComponent: ErrorComponent,
  pendingComponent: LoadingComponent,
})

function RouteComponent() {
  return <div>页面内容</div>
}
```

### Outlet

在布局路由中渲染子路由。

```tsx
import { Outlet } from '@tanstack/solid-router'

function LayoutComponent() {
  return (
    <div>
      <Sidebar />
      <Outlet />  {/* 子路由渲染位置 */}
    </div>
  )
}
```

### useNavigate

编程式导航。

```tsx
import { useNavigate } from '@tanstack/solid-router'

function Component() {
  const navigate = useNavigate()
  
  // 跳转（动态路由必须带 params）
  navigate({ to: '/domain/$domainUUID', params: { domainUUID: 'abc' } })
  
  // 带搜索参数
  navigate({ to: '/domain/$domainUUID', params: { domainUUID: 'abc' }, search: { id: '123' } })
  
  // 替换历史记录
  navigate({ to: '/login', replace: true })
}
```

### useSearch

获取当前路由的搜索参数。

```tsx
import { useSearch } from '@tanstack/solid-router'

// 配合类型安全的 search params
export const Route = createFileRoute('/(app)/domain/$domainUUID/')({
  validateSearch: (search) => ({
    id: z.string().optional().parse(search.id),
  }),
  component: RouteComponent,
})

function RouteComponent() {
  const search = useSearch({ from: '/(app)/domain/$domainUUID/' })
  // search.id → string | undefined
}
```

### useParams

获取动态路由参数。

```tsx
// 文件: pages/(app)/room/$roomId.tsx
import { useParams } from '@tanstack/solid-router'

function Component() {
  const { roomId } = useParams({ from: '/(app)/room/$roomId' })
}
```

### useLocation

获取当前路由信息。

```tsx
import { useLocation } from '@tanstack/solid-router'

function Component() {
  const location = useLocation()
  // location.pathname → '/domain/$domainUUID'
  // location.search → { id: '123' }
}
```

### redirect

在 `beforeLoad` 或 `loader` 中重定向。

```tsx
import { redirect } from '@tanstack/solid-router'

export const Route = createFileRoute('/(app)')({
  beforeLoad: () => {
    if (!isLoggedIn()) {
      throw redirect({ to: '/login' })
    }
  },
})
```

---

## 路由配置

### Vite 插件配置

```ts
// vite.config.ts
import { tanstackRouter } from '@tanstack/router-plugin/vite'

tanstackRouter({
  target: 'solid',
  autoCodeSplitting: true,
  routesDirectory: './src/pages',
  generatedRouteTree: './src/routeTree.gen.ts',
  quoteStyle: 'single',
  semicolons: false,
  routeFileIgnorePrefix: '-',
  routeFileIgnorePattern: 'components',
})
```

### beforeLoad 路由守卫

在路由加载前执行，适合做登录检查。

```tsx
export const Route = createFileRoute('/(app)')({
  beforeLoad: () => {
    if (!userStore.isLoggedIn()) {
      throw redirect({ to: '/login' })
    }
  },
})
```

### staticData

给路由附加静态数据，侧边栏等组件可读取。

```tsx
export const Route = createFileRoute('/(app)/domain/$domainUUID/')({
  staticData: {
    title: '语音域',
    icon: 'icon-channel',
  },
})
```

### loader 数据预加载

路由匹配后、组件渲染前加载数据。

```tsx
export const Route = createFileRoute('/(app)/domain/$domainUUID/')({
  loader: async ({ context }) => {
    // 数据会在组件渲染前完成
    const rooms = await fetchRooms()
    return { rooms }
  },
  component: RouteComponent,
})

function RouteComponent() {
  // 通过 useLoaderData 获取
  const data = Route.useLoaderData()
}
```

---

## 数据集成

### 配合 TanStack Query

```tsx
import { useQuery } from '@tanstack/solid-query'

function Component() {
  const query = useQuery(() => ({
    queryKey: ['rooms'],
    queryFn: fetchRooms,
  }))

  return (
    <Show when={query.isSuccess} fallback={<Loading />}>
      <RoomList rooms={query.data} />
    </Show>
  )
}
```

### 配合 SolidJS Signals

路由组件内直接使用 SolidJS 响应式系统。

```tsx
import { createSignal, createEffect } from 'solid-js'

function Component() {
  const [count, setCount] = createSignal(0)
  
  createEffect(() => {
    // 响应 count 变化
    console.log(count())
  })
}
```

---

## 开发工具

### TanStack Router DevTools

```tsx
import { SolidRouterDevtools } from '@tanstack/solid-router-devtools'

// 仅在开发环境渲染
{import.meta.env.DEV && <SolidRouterDevtools />}
```

### 路由代码生成

```bash
# 手动生成路由树
npx tsr generate

# 开发时自动监听（vite.config.ts 中已配置）
npx tsr watch
```

---

## 本项目中的用法示例

```tsx
// pages/(app)/route.tsx — 布局路由 + 登录守卫
export const Route = createFileRoute('/(app)')({
  beforeLoad: () => {
    if (!userStore.isLoggedIn()) {
      throw redirect({ to: '/login' })
    }
  },
  component: RouteComponent,
})

function RouteComponent() {
  return (
    <Layout>
      <Outlet />
    </Layout>
  )
}
```

```tsx
// pages/(app)/domain/$domainUUID/index.tsx — 页面路由
export const Route = createFileRoute('/(app)/domain/$domainUUID/')({
  component: RouteComponent,
  staticData: { title: '语音域', icon: 'icon-channel' },
})

function RouteComponent() {
  return (
    <div class="flex h-full">
      <RoomDetail />
    </div>
  )
}
```
