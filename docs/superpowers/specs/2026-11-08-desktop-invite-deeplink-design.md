# 桌面端邀请链接直达应用（深链 + 剪贴板识别）

**状态：** 已批准（2026-11-08）

## 背景

分享出去的邀请链接是 `domainInviteUrl()` 用 `window.location.origin` 拼出来的。桌面端的页面 origin 是宿主自己的回环代理（`http://127.0.0.1:<port>`），于是桌面用户复制出来的链接别人根本打不开；同时应用内没有任何入口消费外部的邀请链接 —— `gospeak://` 协议既没在打包配置里注册，`DeepLink.Open` 也没有 Go 侧调用者，OS 事件根本到不了页面。

目标：

1. 邀请链接在桌面端也能分享（指向真实后端服务器）。
2. 点一条链接能唤起/聚焦桌面端并直接进邀请页。
3. 应用已打开时，把邀请链接粘进「服务器地址」框（或剪贴板里已有）也能直接进。

## 链接形态

| 形态 | 用途 | 处理 |
| --- | --- | --- |
| `gospeak://invite/d/<CODE>` | 唤起桌面端 | 宿主归一化为 `/invite/d/<CODE>` 交给页面 |
| `https://<server>/invite/d/<CODE>` | 通用分享链接 | 页面解析出 code + server |
| `http://127.0.0.1:<port>/invite/d/<CODE>` | 桌面端旧链接/本地 | 只取 code，不把回环地址当服务器 |
| `<CODE>` / `/invite/d/<CODE>` | 手输 | 只取 code |

解析统一走 `parseInviteLink()`（`app/web/src/utils/domainInvite.ts`），回环 host（`127.0.0.1`、`localhost`、`::1`）一律不产出 server。

## 宿主（app/desktop）

- `mygo.config.ts` 声明 `urlSchemes: ["gospeak"]`：打包时写进 `CFBundleURLTypes` / Windows 安装器注册表 / `.desktop` MimeType。
- `registerDeepLink()` 在 `Run` 之前执行：`RegisterURLScheme("gospeak")` 兜底未打包运行，并注册 `App.OnOpenURL`。
- `deliverDeepLink()`：正则从 URL 里取 `/invite/d/<CODE>`（`gospeak://invite/d/CODE` 的 `invite` 是 host，所以匹配路径段即可），先 `showMainWindow()`，再决定投递方式：
  - 页面还没调过 `Initial` → 存进 `pending`（冷启动，页面脚本尚未订阅事件）；
  - 已就绪 → 广播 `deeplink.open`。
- `DeepLink.Initial()` 取走 `pending` 并标记就绪；`DeepLink.Open()` 供页面显式转发。
- Windows / Linux 上深链是「新进程带 URL 启动」，`RequestSingleInstanceLock` 会把参数转给已运行的实例，MyGo 的 `deliverArgs` 再喂给 `OnOpenURL`，路径一致。

## Web 端

- `useDeepLinkNavigation()`：**先** `onDeepLink` 订阅、**再** `getInitialDeepLink()`，顺序反了会丢冷启动链接。只接受 `/` 开头的应用内路径。
- `usePendingInvite()`：切服务器导致页面重载后，把重载前存下的邀请码接上（`utils/pendingInvite.ts`，10 分钟 TTL，一次性取走）。
- 两者由 `components/common/desktopInviteBridge.tsx` 挂到根路由；Web 构建里 hook 内的 `__GOSPEAK_DESKTOP__` 分支直接返回。
- `utils/publicBaseUrl.ts`：桌面端分享基址改用宿主持久化的后端地址，未配置时退回 origin；`usePublicBaseUrl()` 让宿主地址异步就绪后重算链接与二维码。
- `InviteShareModal` 在桌面端额外给一条 `gospeak://` 深链和复制按钮。

## 服务器地址框

- 挂载时读一次剪贴板（与「发现域」页同款，失败静默）：识别到邀请就提示「检测到邀请链接 CODE（服务器 …）」，带服务器的链接会把地址填进输入框。
- **不自动保存**：只有回车或点「连接并加入」才生效，避免误切服务器（切服务器会清会话并重载）。
- 输入框内容本身就是邀请链接时，回车/失焦按链接处理：带 server 就切过去（`setPendingInvite` → `setBaseUrl` → 重载 → `usePendingInvite` 接手），不带 server 就直接跳邀请页。
- 焦点离开输入框（例如去点按钮）不会触发保存，避免抢在用户确认前切服务器。

## 错误处理

- 非法/非邀请深链：`invitePathFromLink` 返回空，宿主静默忽略。
- 剪贴板未授权：静默跳过，保留手动输入。
- 邀请码过期：沿用 `/invite/d/$code` 页现有的「邀请链接已失效」提示。
- `setBaseUrl` 失败：清掉 pending，输入框回到可编辑态并提示重试。

## 测试

- Go `deeplink_test.go`：深链 host 形式、`gospeak:///` 路径形式、大小写、https/http 链接、查询串、裸码、错误字符集、非邀请路径；`pending` 一次性取走。
- Web `domainInvite.test.ts`：`parseInviteLink` 保留 server / 回环不算 server / 深链无 server / 裸码与相对路径。
- Web `pendingInvite.test.ts`：一次性取走、TTL 过期、损坏载荷、主动清除。

## 不在本次范围

- 浏览器里打开 http 邀请链接自动拉起桌面端（需要服务端中转页）。
- 房间/频道类深链。
- 剪贴板持续监听。
