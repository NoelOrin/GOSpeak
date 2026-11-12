# 项目缺口全景

**更新**: 2026-11-08

## 成熟度评估

| 层 | 完成度 | 说明 |
|----|--------|------|
| 后端核心 | ~90% | 认证/用户/房间/信令/SFU（LiveKit）+ 文本聊天完备 |
| 后端测试 | ~60% | `test/` 15 个集成套件（auth/oauth/signal/user/role/room/mute/domain/message/storage/sfu/email/system/conversation）+ Go 单测 |
| 前端核心 | ~85% | React 19 全量迁移完成，房间/语音/文字/管理页可用 |
| 前端测试 | ~50% | Vitest 46 文件 / 255 用例；Playwright 用于视觉基线与房间语音 E2E |
| 桌面端 | ~80% | 原生宿主（托盘/自启/覆盖层/自定义标题栏/回环代理）可用，自动更新待接 |
| 文档 | ~85% | 架构/部署/API/SFU/桌面端齐备（`app/docs` 文档站 + 根文档） |

---

## 功能缺失

### 文本聊天 — 🟢 MVP 已交付

后端: Message model/repo/service/handler/route 完整实现，信令层 message:* 事件齐全，
broadcast-first + JetStream async persist 架构，双槽 Hub (text/voice)。
前端: chatStore + MessageList (virtual scroll) + MessageInput + TextRoomPanel 完整可用，
垂直分栏 layout 支持文字/语音房间同时在线。
剩余: 附件上传、消息搜索、线程回复、Markdown 渲染。

### bot 模块 — ⚪ 已移除

`packages/bot/` 已从仓库移除（未实现），相关计划一并清理。


### 前端占位/简陋文件

React 19 迁移时已全部重写，下列旧占位文件均已不存在：

| 旧文件 | 现状 |
|------|------|
| `hooks/useTitle.ts` | 已删除；标题由 `components/titleBar/routeTitle.ts` 统一推导 |
| `layouts/ErrorComponent.tsx` | 已由 `components/common/errorBoundary` 三级边界取代 |
| `home/homePage.tsx`、`(app)/index/index.tsx` | 已由 `pages/(app)/index.tsx` 等文件路由取代 |

---

## 测试覆盖缺口

### 前端测试

- Vitest：46 个测试文件 / 255 个用例，覆盖 stores、socket、api、组件与工具
- Playwright：视觉基线（`app/web/visual-baseline`）与房间语音 E2E（`room-voice-e2e` skill）

### 后端集成测试

已有（15 个套件）：auth / oauth / signal / user / role / room / mute / domain / message / storage / sfu / email / system / conversation / smoke

**缺少测试的模块**:

| 模块 | 端点 | 优先级 |
|------|------|--------|
| Mute 禁言 | `POST /mute/create`、`DELETE /mute/:userId`、`GET /mute/list`、`GET /mute/:userId` | 中 |
| Storage 存储 | `GET /storage/config`、`PUT /storage/config` | 低 |
| SFU Config | `GET /sfu/config`、`PUT /sfu/config` | 低 |
| OAuth Admin | CRUD `/oauth/admin/providers` | 低 |
| Permission RBAC | 角色赋权、权限校验 | 低 |

---

## 文档缺口

| 文档 | 状态 |
|------|------|
| 文档站（`app/docs`，VitePress） | ✅ guide / sfu / deployment / architecture 四组齐备，含桌面客户端页 |
| 桌面端说明（`app/desktop/README.md`） | ✅ 架构、开发、打包、新增能力流程 |
| 测试指南 | ⚠️ 仅有 `agent_test_logs/AGENTS.md` 的测试日志规范 |
| 贡献者指南 | ❌ 缺失 |
| 架构决策记录 (ADR) | ❌ 缺失 |
| 用户使用指南 | ✅ 文档站 `guide/usage` + `guide/desktop` |

### AGENTS.md 已同步

1. ✅ 路由表已含 storage / mute / permission 路由组
2. ✅ `hooks/livekit/` 引用已替换为 `packages/sfu-client`
3. ✅ `useSubcribeTrack.ts` / `useTitle.ts` 已随 React 迁移移除，标题逻辑迁至 `components/titleBar/`
4. ✅ 桌面端能力边界与自定义标题栏规范已补入「Web / 桌面能力边界」小节
