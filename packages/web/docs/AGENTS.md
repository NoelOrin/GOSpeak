# AGENTS.md — @go-rtc/web

## Overview

GOSpeak 前端客户端，基于 SolidJS 构建的实时语音聊天应用。

## Tech Stack

- **Framework**: SolidJS 1.9+
- **Routing**: TanStack Solid Router（文件路由，目录 `src/pages`）
- **State**: SolidJS Signals + `createRoot` 单例 stores（`src/stores`）
- **Data Fetching**: TanStack Solid Query
- **Styling**: Tailwind CSS v4 + DaisyUI 5 + `tailwind-merge` / `clsx`
- **UI Library**: cui-solid
- **Realtime**: Socket.IO client 2.5 + LiveKit client 2.16
- **Lint/Format**: Biome（tab 缩进，LF 换行）
- **Build**: Vite (rolldown-vite) + vite-plugin-solid

## Commands

```bash
pnpm dev          # 开发服务器 (port 3000, proxy :8998)
pnpm build        # 生产构建 + tsc 类型检查
pnpm test         # vitest run
pnpm format       # biome format
pnpm lint         # biome lint
pnpm check        # biome check (format + lint)
```

## Path Aliases

- `@/` → `src/`
- `#/` → `types/`

## Project Structure

```
src/
├── pages/          # 文件路由（TanStack Router）
│   ├── (app)/      # 需要登录的路由组
│   │   ├── channel/  # 频道页（VoiceChat 主界面）
│   │   ├── index/    # 首页
│   │   └── link/     # 链接页
│   └── login/      # 登录页
├── components/     # UI 组件 → 详见 docs/design/AGENTS-components.md
├── stores/         # 全局状态（Signal 单例）→ 详见 docs/design/AGENTS-stores.md
├── hooks/          # 自定义 hooks（含 LiveKit 封装）→ 详见 docs/design/AGENTS-hooks.md
├── layouts/        # 布局组件（Header, Sidebar, Main）
├── api/            # HTTP 客户端封装（axios）
├── handler_audio/  # 音频处理（AudioContext, 音量控制）
├── assets/         # 静态资源
└── styles.css      # 全局样式
```

## Key Patterns

### Routing
- 使用 TanStack Router 文件路由，路由定义在 `src/pages/`
- `(app)/` 前缀为路由组（layout group），不生成 URL 段
- `routeFileIgnorePrefix: "-"` 忽略 `-` 开头的文件
- `routeFileIgnorePattern: "components"` 忽略 components 子目录
- 路由自动生成到 `routeTree.gen.ts`，开发时 `tsr watch` 实时更新

### State Management
- 全局 store 使用 `createRoot` + `createSignal` 单例模式（非 Context）
- 每个 store 文件导出一个单例对象，直接 import 使用
- 组件内局部状态用 `createSignal` / `createMemo`

### Styling
- Tailwind CSS v4（@tailwindcss/vite 插件）
- DaisyUI 5 提供组件类名（btn, modal, dropdown 等）
- 深色模式：`class` 策略（`darkMode: 'class'`）
- 合并类名用 `cn()` 工具（clsx + tailwind-merge）

### Realtime Architecture
- **Socket.IO**: 房间管理（创建/加入/离开/列表）+ 成员同步
- **LiveKit**: WebRTC 音频传输，token 认证
- socketStore 管理房间状态，LiveKit hooks 管理音轨订阅

## Conventions

- 组件文件名：camelCase（如 `voiceChat.tsx`）
- 事件常量集中定义在 `socketStore.ts` 的 `EVENTS` 对象
- TypeScript 严格模式，`noExplicitAny: off`（Biome 配置）
- 未使用导入报错（`noUnusedImports: error`）
- 生产构建自动移除 `console.log` / `debugger`

## Proxy

开发模式下 Vite 代理：
- `/api/v1` → `http://localhost:8998`
- `/socket.io` → `http://localhost:8998`（含 WebSocket）

## Docs

```
docs/
├── AGENTS.md                   # 本文件（全局概览）
├── design/                     # 架构与设计文档
│   ├── AGENTS-components.md    # 组件结构与模式
│   ├── AGENTS-pages.md         # 路由与页面结构
│   ├── AGENTS-stores.md        # 状态管理模式
│   └── AGENTS-hooks.md         # Hooks 与 LiveKit 架构
├── lib/                        # 第三方库参考文档
│   └── tanstack-router.md      # TanStack Router 使用指南与 API
└── plan/                       # 规划与方案文档
```
