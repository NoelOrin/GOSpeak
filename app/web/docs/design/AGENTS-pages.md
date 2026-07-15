# AGENTS.md — pages

## Routing

使用 TanStack Solid Router 文件路由，路由目录为 `src/pages/`。

## Structure

```
pages/
├── __root.tsx          # 根路由（全局 Provider）
├── (app)/              # 需要登录的路由组
│   ├── route.tsx       # 布局路由（登录守卫 + Layout）
│   ├── channel/
│   │   └── index.tsx   # 频道页（RoomDetail 容器）
│   ├── index/
│   │   └── index.tsx   # 首页
│   └── link/
│       └── index.tsx   # 链接页
└── login/
    └── ...             # 登录页
```

## Route Groups

- `(app)/` — 路由组前缀，不生成 URL 段。`/(app)/channel/` 实际访问 `/channel`
- `route.tsx` 在路由组中定义 layout 和 `beforeLoad` 守卫

## Conventions

- 每个路由文件导出 `Route`（`createFileRoute`）和 `RouteComponent`
- `staticData` 中定义 `title` 和 `icon` 用于侧边栏展示
- 路由守卫在 `beforeLoad` 中实现（未登录跳转 `/login`）
- 页面组件尽量轻量，逻辑下沉到 `components/` 和 `hooks/`

## 页面与私有组件

- 页面文件只保留 Route 定义、数据加载、权限与提交编排。
- 复杂 UI 下沉到同级 `components/`（不会进入路由）。
- 跨路由复用再提升到 `src/components/`。
- 目标：管理类页面编排层通常 < 350 行。
