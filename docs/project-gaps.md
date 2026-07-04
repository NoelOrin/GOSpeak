# 项目缺口全景

**更新**: 2026-07-03

## 成熟度评估

| 层 | 完成度 | 说明 |
|----|--------|------|
| 后端核心 | ~85% | 认证/用户/房间/信令/SFU 抽象层完备 |
| 后端测试 | ~20% | 仅 auth/oauth/signal/user 有集成测试 |
| 前端核心 | ~60% | 房间加入/离开/音频控制可用，大功能缺失 |
| 前端测试 | 0% | Vitest 零文件，无 Playwright E2E |
| 文档 | ~70% | 架构/部署/API 参考齐，测试/贡献指南缺 |

---

## 功能缺失

### 文本聊天 — 🔴 最大缺口

后端: 无 chat model/repo/service/handler/route，信令层无聊天事件。
前端: `chat/input.tsx` `chat/output.tsx` 实为音频控件 (mic/speaker 音量)，非文本聊天。
全链路空白。

### bot 模块 — 🟡 未启动

`packages/bot/` 仅含 `package.json` (依赖 hono + await-to-js)，零源码。

### mediasoup-worker — 🟡 初步可用

`packages/mediasoup-worker/src/` 共 3 文件 ~295 行。有 Express API + worker 逻辑，离生产距离大。

### 前端占位/简陋文件

| 文件 | 状态 |
|------|------|
| `hooks/useTitle.ts` | 空文件 (0 字节)，浏览器标签标题未实现 |
| `src/utils/` | 空目录，工具函数未迁移 |
| `layouts/ErrorComponent.tsx` | 7 行简陋错误组件 |
| `home/homePage.tsx` | 仅 `<QuickActions compact />` 占位 |
| `(app)/index/index.tsx` | 纯占位避免路由空转 |

---

## 测试覆盖缺口

### 前端测试

- `app/web/test/` 不存在
- Vitest 0 测试文件
- Playwright E2E 框架未搭

### 后端集成测试

已有: auth / oauth / signal / user

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
| 测试指南 | ❌ 缺失 |
| 贡献者指南 | ❌ 缺失 |
| 架构决策记录 (ADR) | ❌ 缺失 |
| 用户使用指南 | 📅 延后 (等 UI 稳定) |

### AGENTS.md 需同步

1. 路由表缺 storage / mute / permission 三个路由组
2. Provider 成熟度表缺 Daily、SRS
3. `hooks/livekit/` 目录已删除，引用需替换为 `packages/sfu-client`
4. `useSubcribeTrack.ts` / `useTitle.ts` 从 hooks 列表移除或标记待实现
