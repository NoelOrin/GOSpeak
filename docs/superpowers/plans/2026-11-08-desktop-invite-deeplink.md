# 桌面端邀请链接直达应用 实施计划

**目标：** 邀请链接在桌面端可分享（指向真实服务器），点 `gospeak://invite/d/<CODE>` 能唤起应用直达邀请页，粘贴/剪贴板里的邀请链接也能直接进。

**规格：** `docs/superpowers/specs/2026-11-08-desktop-invite-deeplink-design.md`

**架构：** 宿主（Go）负责协议注册、URL 归一化、窗口聚焦与冷启动暂存；TS SDK 只暴露已有的 `onDeepLink` / `getInitialDeepLink`；Web 端负责解析链接、跳转、切服务器与剪贴板识别。

## 任务 1：宿主深链

- [x] `mygo.config.ts` 增加 `urlSchemes: ["gospeak"]`
- [x] `deeplink.go`：`invitePathPattern`、`invitePathFromLink`、`pending/ready` 状态、`deliverDeepLink`、`registerDeepLink`
- [x] `main.go` 在 `WhenReady` 之前调用 `registerDeepLink()`
- [x] `deeplink_test.go` 覆盖各形态

## 任务 2：Web 链接解析与分享基址

- [x] `parseInviteLink` / `domainDeepLinkUrl` / `domainInviteUrl(code, base)`
- [x] `utils/publicBaseUrl.ts`：缓存 + `usePublicBaseUrl`
- [x] `main.tsx` 预热缓存；`DomainMenu`、`(app)/link` 传入基址
- [x] `InviteShareModal` 桌面端给深链复制入口

## 任务 3：深链跳转与待处理邀请码

- [x] `hooks/useDeepLinkNavigation.ts`（先订阅后取 Initial）
- [x] `utils/pendingInvite.ts` + `hooks/usePendingInvite.ts`
- [x] `components/common/desktopInviteBridge.tsx` 挂到 `__root.tsx`

## 任务 4：服务器地址框

- [x] 挂载读一次剪贴板，识别邀请后提示
- [x] 回车/「连接并加入」按链接处理（带 server 切服务器 + pending）
- [x] 失焦不抢跑；手动编辑清除识别结果

## 验证

```bash
cd app/desktop && go test ./... && go vet ./...
pnpm --filter @gospeak/web exec tsc --noEmit
pnpm --filter @gospeak/web exec vitest run
pnpm exec biome check app/web/src
node scripts/check-desktop-commands.mjs
```

## 手工验证

1. 桌面端配置服务器后复制邀请链接：应是后端地址，不再是 `127.0.0.1:47000`。
2. 系统里打开 `gospeak://invite/d/<CODE>`：应用被唤起/聚焦并落在邀请页。
3. 应用已在前台时再次打开深链：页面跳转（不重复跳同一页）。
4. 把 `https://host/invite/d/<CODE>` 放进剪贴板后打开设置：提示出现；点「连接并加入」切服务器并进邀请页。
5. 冷启动带深链（先退出应用，再用深链启动）：仍能进邀请页。
