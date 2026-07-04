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
│   ├── searchModal.tsx   # 搜索弹窗
│   └── settting/         # 设置弹窗（子目录）
├── form/           # 表单组件
├── home/           # 首页组件
├── funcButton.tsx  # 功能按钮（固定在主区域右下角）
├── svgIcon.tsx     # SVG 图标组件
└── userBar.tsx     # 底部用户栏（头像/状态/音量控制）
```

## Key Components

### room/roomDetail.tsx
房间主容器。选中房间后自动获取 token → 创建 LiveKit Room → 加入。包含房间头部栏（名称、在线人数、离开按钮）和 VoiceChat 面板。

### room/voiceChat.tsx
语音聊天面板，展示成员卡片网格。自适应列数（ResizeObserver），每个成员卡片显示头像首字母、名称、音量控制。成员数据来自 `socketStore.members()`。

### room/roomList.tsx
左侧房间列表，管理 Socket.IO 连接生命周期。选中房间触发 `socketStore.setSelectedRoom()`。

### userBar.tsx
底部用户信息栏，显示当前用户头像、名称，以及音频设备控制。

## Patterns

- 组件接收 `ref` prop 用于外部引用（`{ ref?: HTMLDivElement }`）
- 条件渲染统一使用 SolidJS `<Show>` 组件
- 列表渲染使用 `<For each={}>` 组件
- 下拉菜单使用 DaisyUI `dropdown` 类名
- 弹窗使用 DaisyUI modal 或自定义 `commonModal`
