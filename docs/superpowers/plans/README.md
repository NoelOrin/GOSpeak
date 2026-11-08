# Plans 索引 — 状态总览

> 更新于 2026-11-08。已失效、已被取代或已交付归档的计划已清理，仅保留仍在跟进中的计划。
> 状态标注见各计划文件头部 `> **Status**` 块。
> 图例: 🟠 进行中 · 🔴 未启动 · ⚠️ 部分完成

## 进行中 / 待跟进

### ⚠️ (4)

- [`2026-08-13-speaking-detection-event-driven.md`](./2026-08-13-speaking-detection-event-driven.md) — 发言检测事件驱动化 Implementation Plan — ⚠️ 部分完成（核心代码已落地，Task 1-6 待办）
- [`2026-08-25-guest-access-permissions.md`](./2026-08-25-guest-access-permissions.md) — 访客访问与权限 实现计划 — ⚠️ 部分完成
- [`2026-11-07-im-refactor-p1-message-foundation.md`](./2026-11-07-im-refactor-p1-message-foundation.md) — 文字 IM 重构 P1（消息地基）实现计划 — ⚠️ 部分完成
- [`2026-11-08-desktop-shell-identity-state.md`](./2026-11-08-desktop-shell-identity-state.md) — 桌面端外壳：图标、窗口记忆、托盘与登录态持久化 实现计划 — ⚠️ 部分完成

### ✅ 已交付（保留作参考）

- [`2026-08-28-ci-optimize.md`](./2026-08-28-ci-optimize.md) — CI 与提交链路优化 实现计划
- [`2026-08-29-session-proactive-refresh.md`](./2026-08-29-session-proactive-refresh.md) — 会话过期下发与主动续期 实现计划
- [`2026-09-01-ci-domain-version-permissions.md`](./2026-09-01-ci-domain-version-permissions.md) — CI 分域、版本同步与权限最小化 实现计划
- [`2026-09-04-ci-runtime-optimization.md`](./2026-09-04-ci-runtime-optimization.md) — CI 运行效率与发布构建复用 实现计划
- [`2026-11-07-react19-migration-foundation.md`](./2026-11-07-react19-migration-foundation.md) — React 19 迁移基座（阶段 0–4）实现计划
- [`2026-11-08-desktop-custom-titlebar.md`](./2026-11-08-desktop-custom-titlebar.md) — 桌面端自定义标题栏 实现计划

---
共 10 份计划（4 份待跟进 · 6 份已交付留档）。

## 已清理（2026-11-08）

以下计划的实现对象已从仓库移除，或已被后续计划取代，故连同对应规格一并删除（历史见 git log）：

| 计划 | 原因 |
|------|------|
| agora-sfu-e2e-integration | SFU 收敛为 LiveKit，仓库无 agora provider |
| srs-multistream-isolation / srs-selfhost-e2e / srs-room-aggregation | SRS 栈已从 deploy 移除 |
| mediasoup-completion | mediasoup 已删除（`packages/mediasoup-worker` 仅剩 node_modules） |
| tauri-mobile-hybrid-webrtc（plan + design） | 从未动工，仓库无 tauri 代码 |
| teamspeak-provider（plan + design） | 未动工，provider 目录仅剩 livekit |
| observability × 4 | 未动工：无 `internal/trace`、无 `/healthz`、无业务指标 counter |
| nats-multi-phase-optimization | 阶段一已落地，后续阶段被 LiveKit 收敛后的架构绕过 |
| bot-refactor-mute-manager-multiroom | 状态头已注明由后续 bot 计划承接，`packages/bot` 不存在 |
| project-restructure-decouple | 剩余项见 `docs/frontend-coupling-remaining.md`（已归档且 3 项全处理） |
| e2e-test-plan / full-phase-regression | 套件已部分落地，计划未系统执行且被 room-voice-e2e skill 取代 |
