# 全局交互 + 产品思维审查

- 日期：2026-11-08
- 范围：`app/web/src`（110 个 tsx）+ `packages/components/src` + 相关 Go 侧契约
- 方法：四条线并行取证（异步状态 / 表单与危险操作 / 实时语音链路 / 信息架构与旅程），全部结论带 `文件:行号`；**P0 四条已手工逐条复验**，其余为代码扫描结论
- 定位：这是**交互与产品**审查，不涉及视觉风格（视觉已在 `ui-review-2026-11-08.md` 与暗色配色改造中处理）

---

## 0. 结论摘要

这个产品的功能骨架是完整的：域、语音房、文字房、访客、管理后台都在，代码质量也不差（分层清晰、token 统一、错误码体系规整）。**问题不在"有没有做"，而在"做完了但用户走不过去 / 走过去了但不知道自己现在什么状态"。**

三条主线：

| # | 主线 | 一句话 | 量化 |
|---|---|---|---|
| A | **生命线断裂** | 产品最关键的两个入口——邀请链接、离开/删除域——一个 404，一个没有入口 | 2 处硬断点 |
| B | **状态不透明** | 加载中、发送失败、断线、密码错误，大量场景用户看不到任何提示，或者看到英文原文 | 20+ 处静默/半静默 |
| C | **交互不统一** | 同一个动作有 2~4 套实现：确认弹窗 3 套、错误展示 4 种位置、进入房间 2 种手势 | 见 §2 |

如果只能改三件事：**修邀请链接** → **给域加回离开/删除入口** → **把所有静默失败换成明确反馈**。

---

## 1. P0：功能断裂（已逐条复验）

### P0-1 邀请链接指向一个不存在的路由，访客体系整条断裂

`domainInviteUrl()` 生成的是 `/invite/d/<CODE>`：

```ts
// app/web/src/utils/domainInvite.ts:31-33
export function domainInviteUrl(code: string): string {
	return `${window.location.origin}/invite/d/${encodeURIComponent(code)}`;
}
```

但路由树里**没有** `invite` 这一支（`grep invite app/web/src/routeTree.gen.ts` 结果为空；全部路径只有 `/`、`/guest`、`/discover`、`/chat`、`/link`、`/domain/$domainUUID`、`/profile`、`/manage/*`）。

用户点开邀请链接 → 落到 404 页，文案是「页面不存在，或该页面尚未从 Solid 迁移到 React」（`app/web/src/main.tsx:32-44`）—— 对访客来说这句话毫无意义。

影响：`/link` 页、`InviteShareModal`、二维码编码的都是这个 URL（`app/web/src/pages/(app)/link.tsx:63-65`、`components/domain/InviteShareModal.tsx:36`），也就是**所有对外分享的链接 100% 失效**。而访客入口是自托管语音平台的主要拉新方式。

同时 `/guest` 在全仓没有任何 `Link`/`navigate` 指向它（`to="/guest"` 检索为 0 条），只能手输 URL——即使修好链接，也没有路径把访客带过去。

建议：补 `/invite/d/$code` 路由（`beforeLoad` 解析 code → 未登录跳 `/login?redirect=...` / 已登录直接进域 / 允许访客则跳 `/guest?code=`），并把 `/guest` 接进这个流程。

---

### P0-2 域内没有「离开域」「删除域」的任何入口

域页把所有操作渲染在自己的 children 里（`app/web/src/pages/(app)/domain.$domainUUID.tsx:111-228`：域名、描述、邀请码、管理按钮、**离开域**、**删除域**）。

但布局把它藏掉了：

```tsx
// app/web/src/layouts/layout.tsx:307-312（桌面）
<div style={{ display: isDomain ? "none" : undefined }}>
    {children}
</div>

// app/web/src/layouts/layout.tsx:261-263（移动）
{!isDomain && !isManage && (
    <div className="bg-muted h-full overflow-y-auto">{children}</div>
)}
```

`isDomain` 时桌面 `display:none`、移动端根本不渲染。全仓搜「离开域」「删除域」的触发点**只存在于这一个被隐藏的文件里**，没有第二个入口。

影响：
- 普通用户**加入域之后无法退出**（只能让管理员踢）
- 域主**无法删除自己创建的域**
- 连带丢失：域描述展示、邀请码复制/分享入口（`/link` 页可替代，但那是另一条路）

这是"功能存在但用户摸不到"，比没做更糟——后端接口、确认弹窗、错误处理全写好了，就差把 UI 挂出去。

建议：把域信息区做成 `DomainHeader`（域名 + 描述 + 邀请 + 设置/离开/删除的 `⋯` 菜单），挂在 `DomainWorkspace` 顶部或房间列表头部；桌面和移动共用一份。

---

### P0-3 信令 WebSocket 抖一下，语音房间就被完整踢出

`resetRoomState()` 在 WS 断开时清空 `selectedRoomInfo`：

```ts
// app/web/src/stores/socketStore.ts:221-237, 325-334
onDisconnected: resetRoomState()  // 清 currentRoom / rooms / selectedRoomInfo
```

而 `useVoiceSession` 对 `selectedRoom` 为空有强副作用：

```ts
// app/web/src/components/room/hooks/useVoiceSession.ts:140-152
useEffect(() => {
  if (!selectedRoom) {
    abortCurrentJoin();
    if (toTeardown) void teardownClient(toTeardown);   // ← 销毁 LiveKit 会话 + 音频
    cleanupAudioHandler();
    setSession(null);
    socketStore.clearCurrentRoom();
  }
}, [selectedRoom, ...]);
```

链路是：WS 抖动 → `resetRoomState` → `selectedRoomInfo = null` → `selectedRoom = null` → **LiveKit 房间被销毁、麦克风释放、UI 回到"请选择语音频道"**。

而 WS 的自动重连本身是很健康的（指数退避 + 抖动、无上限、`wsClient.ts:205-225`），重连后还会弹「已重新连接」。于是用户看到的是：**先被踢出语音房间，2 秒后又告诉你"已重新连接"**——但人已经不在房间里了，得重新双击进入。

重连成功后的恢复逻辑也只恢复房间列表和文字房（`socketStore.ts:190-213` 的 `pendingTextRejoin`），**语音房没有对应的 re-join**。

影响：任何网络抖动（切 WiFi、地铁、代理重连、后端滚动重启）都会中断语音，而且用户不知道要重新进房。语音产品的核心体验就是"连接韧性"。

建议：断开时**区分"信令断"与"离开房间"**——`resetRoomState` 不要清 `selectedRoomInfo`（清 `rooms` 可以）；给语音会话加"信令离线但媒体仍连着"的中间态；重连成功后若之前在语音房，自动 re-join（或至少给一个「重新进入」的一键按钮）。

---

### P0-4 平台级私密文字房：输对密码也读不到历史消息

前端取历史消息不带密码：

```ts
// app/web/src/api/message.ts:4-13
export async function listMessages(room_uuid: string, before?: string, limit = 100) {
	const data = await apiClient.post<MessageListResult>({
		url: "/api/v1/room/messages/list",
		data: { room_uuid, before, limit },   // ← 没有 password
	});
```

后端对**平台级**私密房强校验密码：

```go
// app/server/internal/service/message_history.go:104-120
if room.Password == "" { return nil }
if actor.UserUUID != "" && room.CreatedBy == actor.Identity { return nil }  // 创建者免密
if pkg.VerifyPassword(room.Password, password) { return nil }
return pkg.NewAppError(pkg.FORBIDDEN, "room password required")
```

密码是通过 WS `room:join` 传的（`stores/chatStore.ts:154-162`），但那条消息是 `emitFireAndForget`（无 id），服务端对无 id 请求不回包（`server/internal/ws/handler.go:65-74`）。

所以：**非创建者进入带密码的平台级文字房时，界面进去了（`chatStore` 已写入 `textRoom`），消息区却是空的，只有一条英文 toast「room password required」。输对密码也一样。**

影响：私密文字房对非创建者不可用（历史读不到，实时消息因 fanout 未 join 也收不到）。域内房间走成员校验不受影响，所以这个 bug 只在「首页/快速创建」创建的房间上出现——恰恰是最容易被用来验证功能的那条路径。

建议：`listMessages` / `searchMessages` 带上房间密码（前端把已验证的密码存到 chatStore 的会话态）；`room:join` 改用带 id 的 ack 通道，把 `room requires password` / `wrong room password` 变成可捕获的错误。

---

## 2. P1：交互一致性

### 2.1 进入动作有 2 套手势，且与提示文案不一致

| 位置 | 交互 | 证据 |
|---|---|---|
| 桌面左侧频道列表 | **单击只弹提示「双击进入」**，双击才进 | `components/room/components/roomItem.tsx:220-230` |
| 域空态里的频道按钮 | 单击直接进 | `layouts/common/domainWorkspace.tsx:285-296` |
| 移动端 | 单击直接进 | `roomItem.tsx:220-230` |

同一个"进入频道"的动作，页面上三种行为。用户第一次点房间看到的是 tooltip 而不是动作，很容易以为"点不动"。

建议：桌面双击是 Discord 的历史习惯，但它只对语音房有意义（防止误入）。统一为：**语音房双击进（保留）+ 单击选中并显示"双击进入"提示**（现状合理），**但空态按钮要改成同样的文案**，或干脆所有"进入"按钮统一单击。

---

### 2.2 危险操作确认：三套机制并存，覆盖面缺口大

| 机制 | 使用者 | 问题 |
|---|---|---|
| 自研 `ConfirmModal`（Radix） | 7 处 | 大部分不显示对象名、不说明"不可恢复" |
| 原生 `window.confirm` | 3 处（删角色、删用户、删 OAuth） | 样式与全站割裂，无法显示富文本 |
| **完全没有确认** | 10+ 处 | 见下 |

完全没有二次确认的破坏性操作（证据均带行号）：

- 语音房踢人：`components/room/roomDetail.tsx:253-265` → 点击即 `kickMember`
- 平台封禁：`pages/(app)/manage/ban/index.tsx:154-162`，且触发按钮还是 `variant="outline"`（不是 destructive）
- 禁言：`manage/users/index.tsx:386-394`、`manage/mute/index.tsx:252-261`
- **批量清理 30 天未活跃访客**：`manage/domains/$domainUUID/index.tsx:647-653` → 一键批量删除，无确认
- 域内删除自定义角色：`components/domain/DomainRolePanel.tsx:104-112`（平台侧同名操作有 `confirm`）
- 切换 SFU 提供商（影响全局语音后端）：`manage/sfu/index.tsx:403-415` → `handleSwitch` 无确认
- 删除消息：`components/textRoom/MessageItem.tsx:110-119` → 无确认、无撤销

且全仓**没有任何"输入名称才能确认"**的机制（`ConfirmModal` 的 props 只有 title/message/variant，`components/common/ConfirmModal.tsx:5-15`）。最该加的三处：删域、删用户、批量清访客。

另外 `ConfirmModal` 默认 `confirmVariant="destructive"`，导致「离开域」这种可逆操作也用了红色主按钮（`domain.$domainUUID.tsx:203-226`）—— 危险等级没有区分。

建议：统一到 `ConfirmModal`，给它加一个可选的 `confirmPhrase`（输入指定文本才可点确认）；把 `window.confirm` 三处迁过来；补齐上表 7 处。

---

### 2.3 断线与连接状态：只有房间列表看得见

WS 状态机是 `new/connecting/open/closing/closed`，**没有 reconnecting 态**（`socket/wsClient.ts:11-16`），而 `socketStore.socketState` 除了 `roomList` 的 `connecting` 之外**没有任何 UI 消费**（`stores/socketStore.ts:43,59,591`）。

顶栏只有主题切换按钮（`layouts/common/header.tsx:59-78`），没有连接指示灯。

用户能看到的只有：
- 房间列表区的「正在连接服务器…/连接失败，请检查服务器状态」（`roomList.tsx:43-67`）—— 但这个区域在进入域/管理页后被别的内容替换
- 一条 `duration: Infinity` 的 warning toast（`socketStore.ts:233-237`）

也就是说：**在 `/manage`、`/profile`、文字房里，用户完全不知道自己和服务器还连着没有。**

建议：顶栏或用户栏加一个常驻连接指示点（绿/黄/红），hover 显示「已连接 / 重连中 / 已断开」；把 toast 换成一个可恢复的常驻条。

---

### 2.4 失败反馈：静默、英文原文、双份 toast 三种病灶

**静默失败（用户得不到任何提示）：**

| 场景 | 证据 |
|---|---|
| 保存个人资料失败 | `components/profile/profileForm.tsx:145-146` 空 `catch {}` |
| 访客配置/封禁列表加载失败 | `manage/domains/$domainUUID/index.tsx:228-233` 只 `console.error` |
| 消息历史/翻页失败 | `stores/chatStore.ts:191-205,207-228` 只有 `finally`，无 `catch` |
| 域成员/权限加载失败 | `stores/domainStore.ts:116,273-274` `.catch(() => {})` |
| 用户资料拉取失败 | `stores/userStore.ts:66-77` 只标记 `userStale` |
| 输出设备 `setSinkId` 失败 | `handler_audio/index.ts:67-85` 只 `console.warn` |
| 麦克风权限申请失败 | `hooks/media.ts:43-46` 只 `console.warn`（启动即申请，失败了界面上完全看不出来） |

**英文原文直接甩给用户：** 加入房间失败会拼出「加入房间失败：wrong room password」「room is full」「duplicate connection not allowed」（`useVoiceSession.ts:400-466` + `server/internal/service/sfu_service.go:77-87`）；管理页错误条直接渲染 `error.message`（`manage/email/index.tsx:142-146`、`manage/sfu/index.tsx:254-258`）。

**同一次失败提示两遍：**

- 注册：banner + 拦截器 toast（`pages/register.tsx:346-353` + `api/auth.ts:43-55` 未设 `skipErrorToast`）
- Discover 加入：弹窗内联 + toast（`pages/(app)/discover.tsx:120-131`）
- 存储页：同一接口被 `useQuery` 和 `useEffect` 各请求一次，失败弹两条（`manage/storage/index.tsx:61-84`）

建议：立一条规矩——**拦截器只负责"意外错误"，业务可预期的错误（密码错、房间满、邀请码失效）必须走 `skipErrorToast` + 就地文案**，并把后端英文 msg 在前端映射成中文短句（已有这套表的雏形，在 `login.tsx:141-147`）。

---

### 2.5 消息发送：乐观更新做了，但失败用户看不到

发送逻辑是乐观的（`stores/chatStore.ts:230-257` 先插入 `pending` 消息），但：

- `MessageItem` 没有 pending / failed 分支（`components/textRoom/MessageItem.tsx:23-27,78-88`），占位消息 `author_id` 为空 → 渲染成 **`?` 头像 + `?` 名字**
- ack 失败或 10s 超时 → `markFailed` 后在 **15 秒**静默删除（`chatStore.ts:262-275`），无 toast、无重试
- 若发送时 socket 未连接，连发送都不会发生，占位消息**永久停在 `?` 状态**（`chatStore.ts:259-294`）
- `message:error` 的处理器存在（`chatStore.ts:124-133`），但服务端从来没有发布过这个事件（全仓 `EventMessageError` 只有定义）

用户体感：消息"发出去了"，15 秒后自己消失了，不知道发生了什么。

建议：占位消息加 3 个视觉态（发送中/失败/重试），失败保留在列表里并提供重试按钮；服务端补 `message:error` 或直接用 ack 错误。

---

### 2.6 加载态与空态：8 处数据面板加载时显示"空"

全站只有一个真骨架屏（房间列表 `roomList.tsx:28-41`），其余是转圈或什么都没有。

关键缺口：

- `manage/domains/index.tsx:15-36` 完全不读 `isLoading`，加载中直接显示「暂无域」——用户会以为自己的域丢了
- `DomainRolePanel.tsx:9` 声明了 `loading` 但组件体内从未使用（角色列整块空白）
- `MessageList.tsx:62` 读了 `loading` 但只用于滚动判断，初始加载不渲染占位，直接落到底部「暂无消息」
- `conversationList.tsx` / `chatWindow.tsx` 加载中显示「暂无会话/暂无消息」，而 `chatStore` 里的 `loadingList` / `loadingMessages` **根本没有组件消费**（`stores/chatStore.ts:497-522,528`）
- `memberSidebar` 两个版本都直接「暂无成员」
- 角色/权限矩阵直接 `map`，空数组时整块白（`manage/roles/index.tsx:185-213`、`DomainRolePanel.tsx:53-68`）
- `DiscoverMyDomains` 用 `allSettled` 只保留成功项，**全部失败时三个分支都不命中 → 整块空白**（`components/discover/DiscoverMyDomains.tsx:16-54`）

「加载中」「没有数据」「加载失败」三种状态在用户眼里长得一样，这是最伤信任的一类缺陷。

建议：抽一个 `<AsyncPanel>`（loading/empty/error 三态 + 重试），把上表 8 处迁过去；`allSettled` 的聚合列表必须区分"部分失败"。

---

### 2.7 键盘与可达性

| 项 | 现状 | 证据 |
|---|---|---|
| 全局快捷键 | **一个都没有**（无 ⌘K、无 M 静音、无 Esc 退出全屏） | 全仓 `addEventListener("keydown")` 仅 1 处，且只关右键菜单（`roomItem.tsx:129`） |
| Esc 清空搜索 | 无（只靠 X 按钮） | `channelHeader.tsx:78-97` 等 4 处 |
| 密码可见性切换 | 全站 11 个密码框**都没有**「小眼睛」 | `login.tsx:251`、`register.tsx:268,287`、`PasswordChangeForm.tsx:146,163,179` 等 |
| Enter 提交 | 包在 `<form>` 里的可以；「新建域」「OAuth 表单」「资料页昵称」「新建角色」不行 | `CreateDomainModal.tsx:95-102`、`OAuthProviderForm.tsx:259-274`、`profileForm.tsx:256-263` |
| Dialog 可访问名称 | 5 处用 `<h3>` 而非 Radix `DialogTitle`（无 `aria-labelledby`） | `ConfirmModal.tsx:36`、`CreateDomainModal.tsx:63`、`passwordModal.tsx:36`、`createRoomModal.tsx:191`、`editRoomModal.tsx:88` |
| Radix 默认焦点 | 确认弹窗 DOM 顺序是「取消」在前 → 默认聚焦**取消**（对危险操作其实是好事，但没有 `autoFocus` 是碰巧） | `ConfirmModal.tsx:38-49` |
| 自定义浮层无 Dialog 语义 | 加入域预览弹窗是手写 div，Esc 无效 | `pages/(app)/discover.tsx:263-293` |

对语音产品来说「M 静音 / 按住说话」几乎是肌肉记忆级的期待，目前一个都没有。

---

### 2.8 权限不足：静默重定向，没有 403 页

- 无管理入口权限 → 直接 `redirect({ to: "/discover" })`，无任何提示（`manage/route.tsx:10-15`，10 个子页同样是这个模式）
- 后端 403 → 通用 toast「forbidden」（`apiClient.ts:140-152` + `server/internal/pkg/errors.go:82`）
- `/manage/domains/$uuid` 没有域权限守卫，非成员进来只能看到「域加载失败：<后端消息>」（`index.tsx:745-757`）

用户点了个链接被弹回首页，不知道为什么。

另外：`hasManageAccess()` 的兜底权限表里普通 `user` 角色含 `user:read`/`role:read`（`utils/permissions.ts:45-72`），意味着**所有登录用户都会看到"管理"入口**，进去只有「用户管理/角色权限」两个只读页签 —— 需要确认这是产品意图（让普通用户能看用户列表）还是权限表写宽了。

---

## 3. P2：打磨与产品机会

按"投入小 / 体感提升大"排序：

| 机会 | 现状 | 建议 |
|---|---|---|
| **语音掉线自动回房** | 见 P0-3 | 重连后自动 re-join，是语音产品的及格线 |
| **正在输入提示** | 完全没有（全仓无 typing 事件） | WS 加一个轻量 `typing` 广播，节流 3s |
| **文字频道未读数** | 只有私聊有红点，文字频道只有人数（`roomItem.tsx:249-253`） | 加 unread badge，进频道清零 |
| **@提及高亮** | mentions 数据存了、发了（`MessageInput.tsx:71-75`），渲染时当普通文本（`MarkdownText.tsx:9-28`） | 渲染层加 mention 样式，是"能看见有人叫我"的关键 |
| **设备切换反馈** | 输入设备切换有 toast，输出 `setSinkId` 失败静默（`handler_audio/index.ts:67-85`） | 统一反馈 |
| **麦克风权限被拒的降级** | 拒绝权限 → `setMicrophoneEnabled(true)` 抛错 → **整次进房失败**（`packages/sfu-client/src/livekit-client.ts:156-164` + `useVoiceSession.ts:400-430`） | 允许"仅收听"进房，麦克风位置显示"点击授权" |
| **密码错后无法重新输入** | 语音房"重试"只是用同一个错误密码重发（`useVoiceSession.ts:512`） | 失败覆盖层提供"重新输入密码" |
| **乐观更新缺失** | 踢人/改角色要等 2 个 RTT（`manage/domains/$domainUUID/index.tsx:169-177`、`DomainMemberTable.tsx:96-104`） | 加 `onMutate` 乐观改 + 失败回滚 |
| **`socketStore.emitSpeaking` 无调用点** | 发言聚合链路只在非 livekit provider 下有意义；目前恒为 livekit，靠原生事件，能用但埋了坑 | 要么接线，要么删掉，别留着误导 |
| **私聊消息列表未虚拟化** | `chatWindow.tsx:72-88` 全量 map（文字房已虚拟化） | 复用 `MessageList` 的 virtualizer |
| **输入框 onBlur 未接线** | `Form` 的 `Field` 声明了 `onBlur` 但没绑到 input（`components/form/index.tsx:64,238`），只在提交后校验 | 补 onBlur，边填边纠错 |

---

## 4. 用户旅程摩擦表

### 4.1 访客：从收到邀请到能说话 = 6 步，断 2 处

| 步 | 用户动作 | 系统反应 | 摩擦 |
|---|---|---|---|
| 1 | 点邀请链接 `/invite/d/ABCD1234` | **404** | ❌ 断点 1（P0-1） |
| 2 | 手输 `/guest?code=...` | 访客页 | 无入口，只能手输 |
| 3 | 填昵称 → 进入 | 成功，但跳到 `/` → `/discover` | ❌ 断点 2：被送回发现页，不是目标域 |
| 4 | 在"我的域"里找到那个域 → 进入 | 域工作区 | 多绕一步 |
| 5 | 在频道列表点房间 | 桌面弹「双击进入」 | 手势不直观 |
| 6 | 双击 → 取消静音 | 说话 | 默认静音，无引导 |

### 4.2 首次登录的新用户

1. 登录成功 → 落地 `/` → 立刻重定向 `/discover`
2. 左栏是 `QuickActions`（快速创建房间/随机加入/前往域），但**三个都依赖 `currentDomainUUID`，而它不持久化**（`stores/domainStore.ts:42-53`），首次进来必然是 `null` → 三个按钮全弹「请先选择域」
3. 主区「我的域」是「暂无已加入的域」，公开域是「暂无公开域」
4. 用户唯一能做的事：点「新建域」—— 但"域"是什么、和 Discord 服务器什么关系，界面上没有任何解释

这是典型的"空产品首屏"：所有入口都在，但没有一条路径告诉他下一步做什么。

### 4.3 域内（老用户）

1. 侧栏点域图标 → `/domain/$uuid` → 左中栏换房间列表，主区显示「选择一个语音频道」+ 最多 6 个频道直达按钮
2. 双击频道 → 覆盖层「准备加入… 加载语音引擎… 连接媒体… 加入房间…」
3. 已连接：成员卡片网格，说话的人卡片 `ring-2 ring-success` 高亮
4. 想退出域 → **找不到入口**（P0-2）

---

## 5. 产品层面的三个判断

**① 「语音优先」的产品定位，落地成了「发现优先」。**
产品是游戏语音，核心动作是"进房说话"。但登录后的第一屏是"发现域"，进频道要双击，进房后还有一层连接覆盖层。而"发现"本身又是个半成品（只有搜索 + 分页，没有筛选/排序/推荐）。建议首屏按"有域 → 直接进上次的域；无域 → 引导创建/加入"分流，而不是所有人看同一页。

**② 「域」这个概念没有被解释过一次。**
自托管场景下用户可能第一次接触 guild 模型。界面上「域 / domain」到处出现（发现域、我的域、新建域、域管理），但没有一句说明"域 = 一个独立的语音服务器，有自己的成员、频道、权限"。新建域弹窗是目前唯一有机会解释的地方。

**③ 连接韧性没有被当成产品能力。**
`wsClient` 的重连退避写得很好（指数 + 抖动 + 无上限），说明有人认真考虑过断线。但断线的**用户侧叙事**是缺的：没有指示灯、没有 reconnecting 态、语音会被直接踢出、重连后不恢复现场。语音产品的口碑几乎就取决于"地铁里出来还能不能接着聊"。

---

## 6. 建议的修复顺序

| 顺序 | 事项 | 为什么先做 | 工作量 |
|---|---|---|---|
| 1 | P0-1 邀请路由 + 访客直达目标域 | 对外分享的链接全是 404，等于没有增长入口 | 小（1 个路由 + 1 个跳转） |
| 2 | P0-2 域 Header（离开/删除/邀请） | 已实现的功能挂不出去，用户退不出域 | 小（抽出组件 + 挂到布局） |
| 3 | P0-3 信令断线不销毁语音会话 | 网络抖一下就掉房，核心体验 | 中（拆 `resetRoomState` + 加 re-join） |
| 4 | P0-4 历史消息带密码 + `room:join` 走 ack | 私密文字房对非创建者不可用 | 小 |
| 5 | §2.4 失败反馈统一（静默 → 明确、英文 → 中文、双 toast → 单通道） | 影响面最广的一类体感问题 | 中（立规矩 + 逐处替换） |
| 6 | §2.5 消息发送 3 态 + 重试 | 用户以为发出去了，其实没有 | 小 |
| 7 | §2.2 危险操作确认统一 + `confirmPhrase` | 批量删访客、切 SFU 无确认，误操作代价高 | 中 |
| 8 | §2.6 `<AsyncPanel>` 三态组件 | 8 处"加载中显示空" | 中 |
| 9 | §2.3 连接指示灯 + §2.7 M 静音/Esc/密码可见性 | 语音产品的肌肉记忆 | 小~中 |

---

## 附：本次审查的取证边界

- P0 四条：逐个读了前后端两侧代码并确认（含路由树、布局渲染分支、后端 `requireRoomAccess`）
- 其余结论来自代码扫描，附带 `文件:行号`，未逐条运行验证
- 未覆盖：Go 侧并发/性能、LiveKit 部署参数、i18n、真正的多端一致性问题（需要真机）
- 已知与本次无关的既有问题：`app/web/src/components/profile/AvatarCropperModal.tsx` 曾是未跟踪文件（见 2026-11-08 记忆），本次 `tsc` 已通过
