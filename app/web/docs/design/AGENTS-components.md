# AGENTS.md — components

## Structure

按业务域分目录，节选实际文件如下：

```
components/
├── common/         # 通用基础组件
│   ├── avatar.tsx           # 全局头像：有 src 展示图片，无 src 或加载失败(onError)时回退为「用户名首字 + 主题渐变」占位
│   ├── ConfirmModal.tsx     # 确认弹窗（基于 @gospeak/components 的 Dialog）
│   ├── divider.tsx          # 分割线
│   ├── dynamicRender.tsx    # 动态渲染（侧边栏内容切换）
│   ├── optionSquare.tsx     # 方形选项按钮
│   ├── errorBoundary.tsx    # 错误边界：ErrorBoundary（组件级）/ PageErrorBoundary（页面级）/ RouteErrorComponent（路由级）/ withErrorBoundary（HOC）
│   └── errorFallback.tsx    # 错误兜底 UI（component / page 两级）
├── room/           # 房间与语音会话
│   ├── VoiceSessionProvider.tsx  # 语音会话常驻宿主（React Context）
│   ├── VoiceStatusBar.tsx        # 语音状态条
│   ├── roomDetail.tsx            # 语音频道视图（成员网格 + 悬浮控制条）
│   ├── roomList.tsx              # 房间列表
│   ├── roomListUtils.ts          # 列表纯逻辑
│   ├── components/               # 房间私有 UI（roomItem、memberSidebar、userInfoPopover、passwordModal…）
│   ├── hooks/                    # useVoiceSession / useRoomAudioBridge / useRoomSounds
│   ├── session/                  # providers / runVoiceJoin / voiceSessionTypes / voiceSessionUi
│   └── services/                 # loadSfuClient（provider chunk 预热 + 动态加载）
├── textRoom/       # 文字房间（TextRoomPanel、MessageList、MessageItem、MessageInput、ReactionBar、MarkdownText）
├── chat/           # 聊天（conversationList、chatWindow、audioControl、micControl、speakerControl、memberSidebar）
├── discover/       # 发现页（DiscoverDomainGrid / Card / JoinPanel、DiscoverMyDomains）
├── domain/         # 域管理 UI（DomainRoomTable、DomainMemberTable、GuestManageTable、CreateDomainModal、InviteShareModal、DomainIcon…）
├── manage/         # 管理壳复用件（ManageShell、manageNav、UserSearchSelect、MuteDurationPicker）
├── modal/          # 弹窗（createRoomModal、editRoomModal、userCard、settingModal/）
├── form/           # 表单（PasswordChangeForm、index.tsx）
├── home/           # 首页（homePage）
├── oauth/          # ProviderIcon（跨登录与 OAuth 管理复用）
├── profile/        # profileForm、UserGroupPanel、presetAvatars、AvatarCropperModal（react-easy-crop 圆形头像裁切：拖拽/缩放/旋转，cropImage.ts 导出 512px 圆形 PNG）
├── dashboard/      # quick-actions
└── userBar.tsx     # 底部用户栏（头像/状态/音频设备控制）
```

通用 UI 基础件（Button / Card / Dialog / Input / Avatar / Toaster）来自 workspace 包 `@gospeak/components`，主题 token 定义在 `packages/components/src/theme.css`；本目录只放业务组件。

## Key Components

### room/VoiceSessionProvider.tsx
语音会话的常驻宿主。用 React Context 持有 SFU 会话、音频桥与提示音，挂在不会被主舞台切换卸载的层级；视图组件通过 `useVoiceSessionContext()` 消费，切到文字频道不会中断通话。

### room/roomDetail.tsx
Discord 风格语音频道视图：参与者卡片网格 + 底部悬浮控制条。语音会话由常驻 Provider 持有，成员/音量数据来自 `useSocketStore` / `socketStore` 门面与 `handler_audio`。

### room/roomList.tsx
房间列表。用 `@tanstack/react-query` 的 `useQueryClient` 刷新、`@gospeak/components` 的 Dialog 系列承载创建/编辑/确认弹窗，房间选择走 `socketStore` 门面。

### common/ConfirmModal.tsx
基于 `@gospeak/components` 的 `Dialog` / `DialogContent`（`open` + `onOpenChange` 受控）封装的确认弹窗，是新增弹窗的参考样本。

### common/errorBoundary.tsx / errorFallback.tsx
错误边界与兜底 UI，供组件级、页面级与路由级错误复用（见 Patterns 与 AGENTS-pages.md）。

### userBar.tsx / textRoom/ / chat/
底部用户信息栏、文字房间消息链路、聊天面板与控制条，按域组织、互不交叉引用。

## Patterns

- 条件渲染用 `&&` / 三元表达式；列表渲染用 `.map()` 并给每项稳定的 `key`。
- 需要暴露 DOM 时用 React 的 `ref`（`useRef` 创建并挂到 `ref` 属性；React 19 起 `ref` 可作为普通 prop 透传给函数组件），组件间需要 DOM 引用时通过 props 传递 ref。
- 弹窗统一用 `@gospeak/components` 的 `Dialog` / `DialogContent`（Radix 风格，`open` / `onOpenChange` 受控）；确认类复用 `common/ConfirmModal`，不自建遮罩层。
- 样式用 Tailwind 4 工具类，类名拼接用 `@gospeak/components` 导出的 `cn()`（内部 clsx + tailwind-merge，解决类名冲突）；部分存量组件仍直接用 `clsx`，新代码优先 `cn()`。
- 图标用 `lucide-react`，toast 用 `@gospeak/components` 的 `toast` / `Toaster`（底层 sonner），不要引第三方图标集或另起一套通知。
- 表单基于 react-hook-form + zod：RHF 管字段与提交、`zod` schema 管校验（纯函数，见 `utils/formSchemas.ts`、`pages/(app)/manage/sfu/components/validation.ts`），便于单测；控件用 `@gospeak/components` 的 `Input` / `Button`。
- 全局状态用 zustand：组件内 `useXxxStore((s) => s.xxx)` 订阅，非 React 侧用 `xxxStore` 门面（见 AGENTS-stores.md）；服务端数据用 `@tanstack/react-query` 的 `useQuery`。
- 错误边界使用约定：高风险局部模块（列表项、面板）用 `<ErrorBoundary name="模块名">` 包裹（默认 `level="component"`，可选 `level="page"`，只兜住子树）；整页兜底用 `PageErrorBoundary`；HOC `withErrorBoundary()` 可把边界包在任意组件外层；路由级错误由 router 的 `defaultErrorComponent` / 根路由 `errorComponent`（均指向 `RouteErrorComponent`）统一处理。组件级边界已接入 `components/common/dynamicRender.tsx`、`layouts/common/domainWorkspace.tsx`、`layouts/layout.tsx`，各包一层。依赖变化（路由参数、房间 id、tab）传 `resetKeys` 自动恢复；`onReset` 用于清 react-query 缓存或重置 store。

## 组件抽离粒度

目标：**按变更边界拆，不按行数机械拆**。页面保持编排层，可复用 UI/规则下沉。

### 放置规则

| 类型 | 位置 | 例子 |
|------|------|------|
| 路由编排 | `pages/**` 路由文件（平铺 `.tsx`；manage 子页为 `index.tsx`） | `beforeLoad` 守卫、`useQuery`、提交动作 |
| 页面私有 UI | `pages/**/components/*` | 某管理页的表格/表单/卡片 |
| 跨页面复用 UI | `components/**` | `ConfirmModal`、`ProviderIcon` 等通用件 |
| 领域逻辑 | `hooks/` / `stores/` / `api/` | 会话、状态、HTTP |
| 语音会话分层 | `components/room/{hooks,session,services}` | 编排 / provider 适配 / SFU 加载 |

`routeFileIgnorePattern: "components"` 已忽略路由下 `components/` 目录，页面私有组件放这里不会生成路由。

### 何时拆

- 单文件混有 **2 个以上独立 UI 区块**（列表 + 表单 + 弹窗）
- 含可单测的纯逻辑（校验、preset、字段映射）
- 同构表单字段重复（抽通用字段组件 / 配置表单）
- 行数 > 300 且可读性下降

### 何时不拆 / 应合并

- 纯 re-export 或 1:1 换名包装
- 只有一行 JSX 的透传组件（除非作为布局槽位语义）
- 拆完后 props 超过 ~15 个且无复用——优先留在页面或改用局部 store / `memo`
- 单一 Tailwind 类名的包装（如 `divider`）除非多处定制

### 管理页约定

```
pages/(app)/manage/<feature>/
├── index.tsx              # Route + 数据/动作编排（建议 < 350 行）
└── components/
    ├── *Table.tsx         # 列表展示
    ├── *Form.tsx          # 创建/编辑表单
    ├── constants.ts       # 预设/枚举
    └── validation.ts      # 纯校验（zod schema，可选）
```

带 `components/` 私有组件的子页：`sfu`、`oauth`、`domains/$domainUUID`；其余管理子页（users、roles、ban、mute、monitor、storage、email）为平铺 `index.tsx`。登录页弹窗在 `pages/login/components/`。

### 参考良好样本

- `components/dashboard/*`：首页区块拆分清晰
- `components/room/{components,hooks,session,services}`：UI / 编排 / provider 适配 / SFU 加载分层
- `components/oauth/ProviderIcon.tsx`：跨登录与 OAuth 管理复用
