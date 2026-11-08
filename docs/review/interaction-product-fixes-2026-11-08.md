# 全局交互 + 产品思维审查：修复实施记录

- 日期：2026-11-08
- 输入报告：`docs/review/interaction-product-review-2026-11-08.md`
- 用户指令：「进行全局的交互+产品思维审查」→「全部修复」→「继续」→「全部都实现」
- 改动范围：`app/web/src/**`、`app/server/internal/signal/**`、`app/server/internal/model/permission.go`、`packages/sfu-client/src/livekit-client.ts`、`packages/desktop/src/capabilities/notification.ts`；暗色配色另见 `docs/design/dark-theme-palette.html`
- 第二阶段（§8）把第一阶段 §4 里所有「需服务端协议 / 产品决策」的待办全部落地，不再保留未实现项。

## 0. 结论

| 层级 | 结果 |
|---|---|
| P0（4 条功能断裂） | 4/4 已修 |
| P1（8 个小节） | 主体已修，含报告里点名的危险操作、失败反馈、三态、键盘、权限提示 |
| P2（打磨机会） | 6 项首轮落地；原 7 项待办在「全部都实现」阶段 **7/7 全部实现**，见 §8 |
| 校验 | `pnpm build:web`（含 tsc）、`biome check`、`lint:ui`、`lint:radius`、Go 全测、Vitest 全量回归全通过；两阶段结果见 §5 与 §8 |

---

## 1. P0：功能断裂

| 编号 | 修复 | 关键文件 |
|---|---|---|
| P0-1 邀请链接 404 | 新增 `/invite/d/$code` 落地页；未登录/访客跳 `/guest?code=...`；登录后经 `sessionStorage.gospeak_redirect` 回跳；邀请码失效给中文提示。访客加入成功后直接进 `/domain/$domainUUID`，不再被送回 `/discover`。**补充修复**：`apiClient` 的 401 拦截器原先无条件 `window.location.href = "/login"`，会把 `beforeLoad` 里带邀请码去 `/guest` 的 redirect 整页吞掉 —— 现在公开入口（`/guest`、`/invite/*`）与登录/注册页跳过硬跳，其余 401 会先记录原路径到 `gospeak_redirect` 供登录后回跳 | `pages/invite/d/$code.tsx`（新）、`pages/guest.tsx`、`api/apiClient.ts`、`utils/domainInvite.ts` |
| P0-2 域内退不出域 | 新建域级菜单挂到房间列表头部：邀请码复制/分享、域管理、离开域、删除域（输入域名才能确认）。原先被 layout 隐藏的 `domain.$domainUUID.tsx` 改为纯路由壳，只在 `beforeLoad` 激活域；访客能力同步迁到 `domainStore.activateDomain`，移动端也生效 | `components/domain/DomainMenu.tsx`（新）、`components/room/roomList.tsx`、`pages/(app)/domain.$domainUUID.tsx`、`stores/domainStore.ts` |
| P0-3 信令抖动销毁语音 | 拆出 `resetTransientState()`：断线只清房间列表/发言态，保留 `currentRoom` 与 `selectedRoomInfo`；重连后补注册语音房信令（`pendingVoiceRejoin`），并让 `useVoiceSession` 在重连且会话为空/失败时 refetch token 做完整 re-join | `stores/socketStore.ts`、`components/room/hooks/useVoiceSession.ts` |
| P0-4 私密文字房读不到历史 | `listMessages` / `searchMessages` 带 password 并 `skipErrorToast`；`joinTextRoom` 的 `room:join` 改走 `socketStore.joinRoom()` ack 路径（fire-and-forget 会吞掉密码错/房间满/被踢）；密码错提供「重新输入密码」入口；socket 未连接时直接给明确失败态，不白等 8s 握手 | `api/message.ts`、`stores/chatStore.ts`、`components/textRoom/MessageList.tsx`、`components/room/roomDetail.tsx` |

---

## 2. P1：交互一致性

### 2.1 进入手势
域空态的频道直达按钮与左侧列表行为对齐，带密码的房间提示「请在左侧列表双击进入」，空态引导文案区分「点下面直接进入」与「双击左侧频道」（`layouts/common/domainWorkspace.tsx`）。

### 2.2 危险操作确认
- `ConfirmModal` 重写：支持 `confirmPhrase`（输入名称才可确认）、每次打开清空输入、Enter 提交、`DialogTitle` 无障碍名称。
- 全仓 `window.confirm` 清零，统一到 `ConfirmModal`。
- **本轮补齐的缺口**：平台封禁、平台禁言（两个页面）、域内删除消息、语音房踢人、批量清理不活跃访客、域内删除角色、删除用户、删除 OAuth 提供商、切换 SFU 提供商、删除域、退出登录。删除用户/删除域要求输入名称。
- 封禁/禁言按钮由 `outline` 改 `destructive`，确认文案带上对象名与时长/影响面。

### 2.3 连接状态
顶栏新增 `ConnectionIndicator`（已连接 / 连接中 / 已断开，离线可点击重连），管理页、资料页、文字房里也常驻可见。

### 2.4 失败反馈
- 新增 `utils/errorMessages.ts`：后端英文 msg → 中文短句映射，另含 `isRoomPasswordError` / `isForbiddenError`。
- 可预期业务错误走 `skipErrorToast` + 页面就地中文提示：`register`、`previewDomainInvite`、`joinDomain`、`listMessages`、`searchMessages`。
- 修掉重复请求/双提示：存储页、邮箱页、SFU 刷新、Discover 预览与加入。
- **本轮补齐**：`setAudioOutputDevice` 的 `setSinkId` 失败给一次性 toast（按设备切换去重）；域详情页访客配置/封禁列表加载失败给中文错误提示；资料保存失败内联错误条。

### 2.5 消息发送三态
`MessageDTO` 增加 `_status: pending | failed` / `_error`；占位消息带作者信息不再显示 `?`；失败消息**保留**在列表里并提供重试/删除，移除 15 秒静默删除；新增 `retryMessage(uuid)`。

### 2.6 加载/空/失败三态
新增 `components/common/AsyncState.tsx`（loading / error / empty / content + 重试），迁移域列表、会话列表、聊天窗口、消息列表、我的域、角色面板等；`allSettled` 聚合区分「部分失败 / 全部失败」；成员侧栏断线时显示「正在连接服务器，成员列表加载中…」而不是「暂无成员」。

### 2.7 键盘与可达性
- 新增 `useGlobalShortcuts()`：语音房内按 `M` 静音/取消静音，避开输入控件与组合键、尊重禁言与访客禁说。
- 新增 `PasswordInput`（显示/隐藏切换），**全站 12 处密码框全部迁移**；管理后台自定义样式表单用 `native` 变体，避免叠出双层边框。
- 加入域预览弹窗改 Radix `Dialog`；5 处 `<h3>` 改 `DialogTitle`；Esc 清空搜索；`Form.Field` 的 `onBlur` 接线。
- **本轮补齐 Enter 提交**：新建域、新建角色、资料页昵称、OAuth 提供商表单（改为 `<form onSubmit>` 而非在 div 上挂 keydown，避免抢 Switch/按钮的键盘语义）。

### 2.8 权限提示
新增 `utils/accessDenied.ts#denyAccess()`：先 toast 说明原因再 redirect，管理后台壳层与 10 个子页全部替换原来的静默跳转。

---

## 3. P2 已落地

| 机会 | 落地方式 |
|---|---|
| 语音掉线自动回房 | 见 P0-3 |
| @提及高亮 | `MarkdownText` 在消毒后的 HTML 里处理文本节点，`@name` 高亮、`@自己` 用 primary 样式，跳过 `code/pre/a` |
| 麦克风权限被拒的降级 | `livekit-client.ts` 开麦失败只 warn，允许「仅收听」进房；`useRoomAudioBridge` 在真正开麦失败时提示去浏览器授权 |
| 密码错后重新输入 | 语音房失败覆盖层、私密文字房消息列表各提供重输入口 |
| 输入框 onBlur 未接线 | `components/form/index.tsx` textarea/number/default 三处补齐 |
| `emitSpeaking` 死代码 | 全仓无调用点（LiveKit 走原生发言事件），已删除函数与导出 |
| 设备切换反馈 | `setSinkId` 失败给一次性 toast；输入设备切换失败已有 toast |

---

## 4. 首轮暂缓项（已在第二阶段全部实现）

首轮基于「需要服务端协议 / 产品决策」或「收益风险比」暂缓了 7 项。用户后续明确要求「全部都实现」，这些项**已全部落地**，逐项实现与验证见 **§8**。此处保留原始理由以便追溯：

| 项 | 首轮暂缓理由 | 现状 |
|---|---|---|
| 正在输入（typing）提示 | 需要服务端新增 WS 事件与广播；全仓原无 typing 事件 | ✅ 已实现前后端协议，见 §8.1 |
| 文字频道未读 badge | 需要服务端已读游标 / 未读计数协议 | ✅ 已用前端 `MESSAGE_CREATED` + 本地持久化实现，见 §8.2 |
| 私聊消息列表虚拟化 | 收益只在超长会话出现 | ✅ 已接入 `@tanstack/react-virtual`，见 §8.3 |
| 踢人 / 改角色乐观更新 | 需要快照 / 回滚改造 | ✅ 已抽出通用乐观执行器并覆盖 11 类操作，见 §8.4 |
| 产品层三个判断 | 属于产品决策，需先确认方向 | ✅ 首屏分流、域概念解释、权限表收紧全部落地，见 §8.5 |
| `hasManageAccess()` 兜底权限表过宽 | 需产品确认 | ✅ 前后端同步收紧普通 `user` 角色，见 §8.5 |
| 后台静默 catch | 担心启动探测误报骚扰 | ✅ 去重式后台失败上报 + 启动探测继续静默，见 §8.6 |

---

## 5. 验证

| 命令 | 结果 |
|---|---|
| `pnpm build:web`（vite build + tsc） | 通过 |
| `npx biome check src` | 233 文件，无 error / 无 warning |
| `pnpm -s lint:ui` / `pnpm -s lint:radius` | `ui token check ok` / `radius token check ok` |
| Vitest 全量 | 38 文件 / 216 用例全部通过，`EXIT=0` |
| Playwright 页面冒烟 | 见 §7 |

回归中同步修正了 3 个测试文件，使断言与新行为一致（不是实现回滚）：

- `stores/chatStore.domain.test.ts`：`room:join` 改走 `socketStore.joinRoom` ack 后更新断言，并新增 3 个用例——socket 离线给明确失败态且不等待 ack、非密码类 join 失败不拉历史、`retryTextRoomWithPassword` 带新密码重新注册并拉历史。
- `api/auth.spec.ts`、`api/domainApi.spec.ts`：`register` / `previewDomainInvite` / `joinDomain` 的断言补上 `skipErrorToast: true`。

> 排查记录：早先「全量 Vitest 挂起」并非死循环，而是 jsdom 环境启动开销大（约 5 分钟，environment 累计 500s+）叠加 `| tail` 缓冲导致看不到进度。改为日志落盘 + `--maxWorkers=2` 后可见完整进度。

---

## 6. 仍需人工 / 真机验证

构建与单测覆盖不到真实链路，建议在接了后端的环境里过一遍：

1. ~~`/invite/d/<code>` 不再 404；未登录跳 `/guest?code=...`~~ —— 已用最新构建产物 + 无头浏览器验证，见 §7；**访客加入后直达目标域**、登录后回跳仍需接后端确认。
2. 域内 `DomainMenu` 可见，离开域 / 删除域（输入域名）流程可用。
3. 私密文字房：非创建者输对密码能读到历史；密码错可重输。
4. 发送失败的消息保留在列表里，可重试 / 删除。
5. WS 断线重连时语音不中断，重连后自动回到原房间。
6. 顶栏连接指示在 `/manage`、`/profile`、文字房内都可见。
7. 麦克风权限拒绝后能「仅收听」进房。
8. **第二阶段新增**：登录后直接落进上次的域（换设备/退出域后自动回退）；普通用户侧栏看不到「管理」入口；文字房里别人打字出现「正在输入」，离开文字房时其它频道的未读 badge 正确累加、进频道清零；踢人/改角色/删角色立即生效、失败自动回滚并弹中文原因；管理操作断网时乐观行回滚。

---

## 7. 回归记录与页面冒烟

**测试**：第一轮全量暴露 4 个断言过期用例（2 个文件），修正断言后 37 文件 / 213 用例全通过；补入 `api/redirectToLogin.test.ts` 后最终全量 **38 文件 / 216 用例全部通过**（`EXIT=0`）。

**页面冒烟**（`vite preview` + 最新 `dist` + 无头浏览器）：

| 路径 | 修复前 | 修复后 |
|---|---|---|
| `/invite/d/TESTCODE`（未登录） | `/login`，邀请码丢失 | `/guest?code=TESTCODE`，访客加入表单正常 |
| `/guest?code=TESTCODE` | 正常 | 正常 |
| `/discover`（未登录） | `/login` | `/login`（受保护页面行为未变） |

三个页面均无 `pageerror`。这一步才发现 P0-1 的第二层根因：`apiClient` 401 拦截器整页硬跳登录，吞掉了路由层带邀请码的 redirect（已修，见 §1）。

**环境提示**：本机 4173 端口走代理会返回 502，`curl` 需 `--noproxy '*'`，浏览器需 `--no-proxy-server`。

---

## 8. 第二阶段：「全部都实现」

用户指令：「全部都实现」——§4 里的 7 个暂缓项不再保留，全部落地。

### 8.1 typing 正在输入提示（前后端协议）

| 侧 | 内容 |
|---|---|
| 服务端 | 新增 `EventTyping = "typing"`（`signal/events.go`）；`hub_typing.go` 实现：身份取连接 Claims（伪造无效）、禁言 fail-closed、start 即广播 + 2s 心跳 + 30s 过期、`publishRoom` 支持多实例 eventBus、不落库 fire-and-forget。`hub.go` 注册事件与 `typingMu`/`typing` 状态 |
| 服务端测试 | `hub_typing_test.go`：start/stop/节流、心跳续期、已鉴权 identity、非成员忽略、mute fail-closed、malformed frame 忽略 |
| 前端 | `typingStore`（TTL 6s）→ `socketEvents` 绑定 → `useTypingReporter`（throttle 心跳 1200ms / debounce 停手 3000ms，卸载/切房间并发 stop）→ `TypingIndicator`（排除自己、多人聚合、每秒 tick 让过期条目消失）→ `TextRoomPanel` / `MessageInput` 接线 |

### 8.2 文字频道未读 badge

`chatStore` 新增 `unreadByRoom`（localStorage 持久化 `gospeak_text_unread`）；全局 `MESSAGE_CREATED` 经 `socketStore.setMessageCreatedHandler` 分流：当前房间走 `applyCreated`，其它频道 `incrementUnread`（过滤自己发的）；进入频道 `markRoomRead` 清零；`roomItem` 对 `type === "text"` 渲染未读数（99+ 封顶，`title` 说明）。

### 8.3 私聊消息列表虚拟化

`chatWindow.tsx` 用 `@tanstack/react-virtual` 重写：`estimateSize 64`、`overscan 10`、`measureElement` 动态测量；仅当用户贴底（`atBottomRef`）时自动滚到最新，避免阅读历史时被新消息拽走；保留 `AsyncState` 三态。文字房 `MessageList` 的虚拟化模式由此复用到私聊。

### 8.4 管理操作乐观更新

新增通用执行器 `utils/optimistic.ts`（含 7 个单测）：

- `runOptimistic({ snapshot, apply, restore, request, settle, onError, onSuccess })`：快照 → 乐观应用 → 请求 → 失败回滚 → 无论成败对账（settle）。同步快照保持同步（乐观值与点击同一 tick 落到界面），异步快照（React Query 需要 `await cancelQueries`）走 await 分支；`onError/onSuccess/settle` 自身异常都不会把成功的操作变成未处理拒绝。
- React Query 辅助：`snapshotQueries`（先 `cancelQueries` 防旧响应覆盖乐观值，再 `getQueriesData` 快照）/ `restoreQueries` / `updateQueryData`（`setQueriesData` 前缀匹配分页缓存）/ `invalidateQueries`。

覆盖的操作（11 类）：用户改角色、删除用户、禁言、解禁言（用户页+禁言页，两份缓存同步更新）、平台创建/删除角色、封禁/解封（同时更新封禁页与用户页两份缓存）、域内踢人、改成员角色、创建/修改/删除域角色、访客解封、删除房间。zustand / useState 数据源通过 `domainStore.setMemberCache` 与组件内快照闭包接入，失败回滚原数组。

顺带清理：删除死代码 `executeKickMember`、四个页面里未使用的 `useQueryClient`。

### 8.5 首屏分流、域概念解释与权限表收紧

| 项 | 实现 |
|---|---|
| 「上次所在域」持久化 | `domainStore` 新增 `gospeak_last_domain`（localStorage 只存 UUID，隐私模式静默降级）；`setCurrentDomain` 写入、移除当前域时清除、`createDomainStore` 重置时清除 |
| 首屏分流 | `utils/firstScreen.ts` 纯函数（3 个单测）：无域 → `/discover` 引导；有域 → 回上次所在域（校验仍在 `myDomainUUIDs` 内，过期记录自动回退到第一个域）；访客直接 `/discover`。`/` 的 `beforeLoad` 先 `await loadMyDomains()` 再分流 |
| 域概念解释 | 新建域弹窗顶部增加「域是一个独立的语音空间，类似一个服务器……」说明条，「公开域」下方补一句影响说明；邀请落地页对未加入者解释域是什么；访客页说明访客可用权限由域管理员设置；`DiscoverMyDomains` 空态由「暂无已加入的域」改为带行动指引的文案 |
| 权限表收紧 | 普通用户角色从 `user:read` / `role:read` 收紧（前后端同步：`utils/permissions.ts` + `model/permission.go`）。这两个码只守卫后台只读页（`/user/list`、`/role/list`），收掉后普通用户不再看到「管理」入口；新增前端 `permissions.test.ts`（6 例）与 Go 断言（user 角色不得携带平台管理权限）。`domain:create` / `room:create` 保留 |

### 8.6 后台静默失败可见化

原则：**启动期探测（麦克风权限申请、设备变化、localStorage）继续静默**；**用户会注意到的数据加载必须有可见反馈**。

- 新增 `utils/backgroundFailure.ts`（4 个单测）：同一 scope 一次会话只弹一次 toast、成功后 `clearBackgroundFailure` 允许再次提示、复用 `__toastShown` 与 apiClient 拦截器去重、文案走 `localizedApiMessage` 中文化。
- `domainStore`：`loadMyDomains` / `ensureDomainLoaded` / `loadMembers` / `loadMyPermissions` / `syncGuestCaps` 全部接入；新增 `myPermissionsErrors`（此前权限加载失败既不 toast 也不写 store，管理按钮会静默失效）；访客能力 fail-closed 成「全禁」时明确告知原因。域详情页头部对权限加载失败渲染可重试的内联警告条。
- `audioDeviceStore.fetchAudioDevices`：原先连 `.catch` 都没有，失败是未处理拒绝；现在写 `deviceLoadError`，音频设置页内联展示 + 重试，点「刷新设备」额外 toast（`notify` 选项区分挂载探测与主动刷新）。
- `manage/roles`：角色权限加载失败原先静默清空勾选，用户会误以为「该角色没权限」并保存，把权限真的清掉；现在内联报错并**禁用保存**，重试按钮重新拉取。
- `runVoiceJoin` background 分支：media 成功后信令失败原只把 phase 置 `failed`（`error: null`），用户看不到原因；现在 `toastActionError` 给出中文原因。
- `roomItem` 离开房间失败从静默改为 toast。

### 8.7 顺带修复的前置阻断问题（非审查范围，但挡住构建）

工作区里有一套未提交的 `@gospeak/desktop` 桌面 SDK WIP，且 `app/web/package.json` 已声明依赖但未安装链接，导致 `pnpm build:web` 与 `tsc --noEmit` 全挂。为让回归可跑，做了最小修复：

1. `packages/desktop/src/capabilities/notification.ts` 缺一个右括号（`.then(...)` 未闭合就接 `.catch`），补上并去掉未使用的 `unsub`。
2. `pnpm install` 同步 workspace 链接（lockfile 补上 `@gospeak/desktop`）。
3. `settingModal/tab_item/shared.tsx` 的 `Section` / `Field` 补 `desc?: string`（`desktop.tsx` 按 `Page` / `Toggle` 的惯例传 `desc`，两个基础组件却没有这个 prop）。
4. `overlay.tsx` 两处 `text-foreground/90` 违反 UI token 规则（只允许 foreground / muted / faint 三级），改 `text-muted-foreground`；`useEffect` 出现在提前 `return` 之后（违反 hooks 规则），拆成外层平台判断 + 内层 `OverlayHud` 组件；三处 `size={13}` 归一到 `12`。

### 8.8 第二阶段验证

| 命令 | 结果 |
|---|---|
| `cd app/server && go test ./... -count=1` | 全部包 ok（含新增 `hub_typing_test.go`、`permission_test.go` 新断言） |
| `pnpm build:web`（turbo：components + desktop + sfu-client + web，含 tsc） | 4/4 通过 |
| `npx biome check src` | 249 文件，无 error / 无 warning |
| `pnpm -s lint:ui` / `pnpm -s lint:radius` | `ui token check ok` / `radius token check ok` |
| `npx tsc --noEmit` | 通过 |
| Vitest 全量 | **43 文件 / 240 用例全部通过**（`EXIT=0`，新增 `optimistic` / `firstScreen` / `backgroundFailure` / `permissions` / `domainStore.persistence` 等测试） |
| Playwright 页面冒烟（最新 dist） | `/invite/d/TESTCODE → /guest?code=TESTCODE`、`/discover → /login`、`/ → /login`（未登录），三页均无 `pageerror` |

新增测试文件：`utils/optimistic.test.ts`、`utils/firstScreen.test.ts`、`utils/backgroundFailure.test.ts`、`utils/permissions.test.ts`、`stores/domainStore.persistence.test.ts`、`server/internal/signal/hub_typing_test.go`。

> 性能备注：`domainStore` 引入后台失败上报后，其测试文件首次 import 会把组件库 barrel 拖进来，jsdom transform 超过 10s 测试超时。三个 domainStore 测试文件统一 `vi.mock("@gospeak/components")` 截断该依赖链（生产构建不受影响）。

