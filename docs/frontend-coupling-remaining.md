# 前端耦合优化 — 未完成项计划（归档）

> **状态**: 本文件所列 3 项已全部处理完毕。
> - #1 ✅ apiClient token callback — 已通过 `apiClientAuth.ts` 注入模式解决
> - #2 ✅ socketStore 音频解耦 — 通过 `onActiveSpeakers` 回调桥接解耦 (2026-07-25)
> - #3 ⚠️ roomDetail 拆 hooks — 组件 94 行已拆 3 个 hooks，不再臃肿

**后端 SFU 解耦**: 当前 4 个启用 provider（livekit/srs/agora/cloudflare），其余实现保留但未注册。

---
