# AGENTS.md — components

## Structure

```
components/
├── common/         # 通用基础组件
│   ├── avatar.tsx        # 头像组件
│   ├── commonModal.tsx   # 通用弹窗
│   ├── divider.tsx       # 分割线
│   ├── dynamicRender.tsx # 动态渲染（侧边栏内容切换）
│   ├── optionSquare.tsx  # 方形选项按钮
│   └── visible.tsx       # 条件渲染包装器
├── room/           # 房间相关组件
│   ├── roomDetail.tsx    # 房间详情（加入/离开/主容器）
│   ├── roomList.tsx      # 房间列表
│   ├── roomMemberInfo.tsx# 成员信息展示
│   └── voiceChat.tsx     # 语音聊天面板（成员卡片网格）
├── chat/           # 聊天组件
│   ├── input.tsx         # 消息输入
│   └── output.tsx        # 消息展示
├── modal/          # 弹窗组件
│   └── settting/         # 设置弹窗（子目录）
├── form/           # 表单组件
├── home/           # 首页组件
├── svgIcon.tsx     # SVG 图标组件
└── userBar.tsx     # 底部用户栏（头像/状态/音量控制）
```

## Key Components

### room/roomDetail.tsx
房间主容器。选中房间后自动获取 token → 创建 LiveKit Room → 加入。包含房间头部栏（名称、在线人数、离开按钮）和 VoiceChat 面板。

### room/voiceChat.tsx
语音聊天面板，展示成员卡片网格。自适应列数（ResizeObserver），每个成员卡片显示头像首字母、名称、音量控制。成员数据来自 `socketStore.members()`。

### room/roomList.tsx
左侧房间列表，管理 WebSocket 连接生命周期。选中房间触发 `socketStore.setSelectedRoom()`。

### userBar.tsx
底部用户信息栏，显示当前用户头像、名称，以及音频设备控制。

## Patterns

- 组件接收 `ref` prop 用于外部引用（`{ ref?: HTMLDivElement }`）
- 条件渲染统一使用 SolidJS `<Show>` 组件
- 列表渲染使用 `<For each={}>` 组件
- 下拉菜单使用 DaisyUI `dropdown` 类名
- 弹窗使用 DaisyUI modal 或自定义 `commonModal`

## 组件抽离粒度

目标：**按变更边界拆，不按行数机械拆**。页面保持编排层，可复用 UI/规则下沉。

### 放置规则

| 类型 | 位置 | 例子 |
|------|------|------|
| 路由编排 | `pages/**/index.tsx` | 权限守卫、`createResource`、提交动作 |
| 页面私有 UI | `pages/**/components/*` | 某管理页的表格/表单/卡片 |
| 跨页面复用 UI | `components/**` | `FormField`、`ProviderIcon`、`Users` 无关的通用件 |
| 领域逻辑 | `hooks/` / `stores/` / `api/` | 会话、状态、HTTP |

`routeFileIgnorePattern: "components"` 已忽略路由下 `components/` 目录，页面私有组件放这里不会生成路由。

### 何时拆

- 单文件混有 **2 个以上独立 UI 区块**（列表 + 表单 + 弹窗）
- 含可单测的纯逻辑（校验、preset、字段映射）
- 同构表单字段重复（抽 `FormField` / 配置表单）
- 行数 > 300 且可读性下降

### 何时不拆 / 应合并

- 纯 re-export 或 1:1 换名包装（如 `useRoomJoinSession`）
- 只有一行 JSX 的透传组件（除非作为布局槽位语义）
- 拆完后 props 超过 ~15 个且无复用——优先留在页面或改用局部 store/memo
- DaisyUI 单类名包装（`divider` 等）除非多处定制

### 管理页约定

```
pages/(app)/manage/<feature>/
├── index.tsx              # Route + 数据/动作编排（建议 < 350 行）
└── components/
    ├── *Table.tsx         # 列表展示
    ├── *Form.tsx          # 创建/编辑表单
    ├── constants.ts       # 预设/枚举
    └── validation.ts      # 纯校验（可选）
```

已按此模式拆分：`sfu`、`oauth`、`permission`、`users`；登录页弹窗在 `pages/login/components/`。

### 参考良好样本

- `components/dashboard/*`：首页区块拆分清晰
- `components/room/{components,hooks,session}`：UI / 会话 / 适配分层
- `components/oauth/ProviderIcon.tsx`：跨登录与 OAuth 管理复用
