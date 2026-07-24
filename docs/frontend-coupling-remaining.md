# 前端耦合优化 — 未完成项计划（归档）

> **状态**: 本文件所列 3 项已全部处理完毕。
> - #1 ✅ apiClient token callback — 已通过 `apiClientAuth.ts` 注入模式解决
> - #2 ✅ socketStore 音频解耦 — 通过 `onActiveSpeakers` 回调桥接解耦 (2026-07-25)
> - #3 ⚠️ roomDetail 拆 hooks — 组件 94 行已拆 3 个 hooks，不再臃肿

**后端 SFU 解耦**: 6 个 SFU provider (`livekit/srs/agora/daily/mediasoup/cloudflare`) 已于 2026-07-25 从 `internal/` 顶层归入 `internal/sfu/providers/`。

---
