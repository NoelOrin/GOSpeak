# 发言检测（Active Speaker）事件驱动化 Implementation Plan

> **Status (2026-08-13):** ⚠️ 部分完成 — 核心代码已落地未提交（服务端去重 / join 回放 / 前端清理 / 滞回 / AudioWorklet 事件驱动采样 / 文档矩阵）；待办项见 Task 1-6。

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 SRS/Cloudflare 的发言检测完全事件驱动（AudioWorkletProcessor，无 JS 定时器轮询），并把发言检测收敛到抽象层（`packages/sfu-client` 的 `LocalSpeakingMeter`，SRS/Cloudflare 共享），保持判定语义不变；随后补齐跨实例回放、参数可配置、mediasoup 事件驱动化与 e2e 自动断言。

**Architecture:**

```
音频硬件 → AudioWorkletProcessor(音频线程, 按块 postMessage 采样事件, 无 JS 定时器)
        → 主线程回调: AnalyserNode 频域均值(阈值语义不变) → SpeakingDetector 滞回
        → 状态翻转才触发 onLocalSpeakingChange → member:speaking → 服务端聚合 → room:active-speakers
```

**Tech Stack:** Web Audio API（AudioWorkletNode + AudioWorkletProcessor + AnalyserNode）· Go signal Hub · WebSocket 信令 · Vitest / Go test

## 背景与现状

| Provider | 检测机制 | 链路 | 状态 |
|----------|----------|------|------|
| LiveKit | SFU 原生 `ActiveSpeakersChanged` | 客户端 `onActiveSpeakers` → 前端高亮 | ✅ 未改动 |
| Agora | SFU 原生 `volume-indicator`（level>5） | 同上 | ✅ 未改动 |
| SRS | 本地 WebAudio 分析（原 RAF 轮询 → 已改 AudioWorklet 事件驱动） | `onLocalSpeakingChange` → `member:speaking` → 服务端聚合 → `room:active-speakers` | ✅ 已落地 |
| Cloudflare | 本地 WebAudio 分析（原 setInterval 轮询 → 已改 AudioWorklet 事件驱动） | 同上 | ✅ 已落地 |
| mediasoup | 本地分析远端轨道电平（`setInterval` 500ms 轮询，无滞回） | 客户端 `onActiveSpeakers` | ⛔ 已停用（未注册） |

## 已完成（2026-08-13）

- [x] 服务端 `OnMemberSpeaking` 同值去重：状态未变化不广播（`hub_room_events.go`）
- [x] `room:join:sfu` 成功后回放当前 active speakers（`hub_room_join.go` `OnRoomJoinSFU`）
- [x] 前端离开/断连/切房清空 `speakingIdentities`（`socketStore.ts` 三处）
- [x] `bindSignalActiveSpeakers` 同值节流兜底（`providers.ts`）
- [x] `SpeakingDetector` 滞回状态机 + 单测（`packages/sfu-client/src/speaking-detector.ts`）
- [x] `LocalSpeakingMeter` 抽象层 AudioWorklet 事件驱动采样（`speaking-meter.ts` + 内联 worklet `speaking-meter-worklet.ts`，Blob URL 加载，与打包器无关）
- [x] SRS/Cloudflare 移除 RAF / setInterval 轮询并接入 meter（`srs-client.ts` / `cloudflare-client.ts`）
- [x] `docs/sfu-provider-maturity.md` §6 发言检测能力矩阵
- [x] 单测：server 去重+回放 / detector / meter（8 用例）/ socketStore 清理；vitest 35（sfu-client）+ 232（web）通过；`vite build` 产物确认 worklet 内联

## 待办任务

### Task 1: 跨实例 join 回放合并 KV Speaking

**Files:** Modify `app/server/internal/signal/state_sync_room.go`（`mergeMemberSnapshot`）、`app/server/internal/signal/hub_room_events.go`（`computeActiveSpeakersLocked` / `broadcastActiveSpeakers`）、`app/server/internal/signal/hub_room_join.go`（回放调用）；Test `hub_stability_test.go`

背景：KV `MemberRecord.Speaking` 存在（`bus/kv_store.go`）但仅 join 时写入（`hub_room_join.go` 快照构造）；`mergeMemberSnapshot` 丢弃 Speaking；`OnMemberSpeaking` 不刷新 KV。当前 join 回放仅含本实例成员发言态（`computeActiveSpeakersLocked` 只读本地 `h.rooms`）。

- [ ] **Step 1: 写失败测试** — `TestOnRoomJoinSFU_ReplayMergesRemoteSpeaking`：构造含远端成员（KV 快照 `Speaking=true`）的 membershipStore，本地无发言者；新成员 join 后回放广播 identities 应含远端成员。
  Run: `go test ./internal/signal/ -run TestOnRoomJoinSFU_ReplayMergesRemoteSpeaking`
  Expected: 失败（当前仅本地聚合）
- [ ] **Step 2: 最小实现** — 新增 `computeActiveSpeakersMerged`：本地 `Speaking` ∪ KV 快照 `Speaking=true` 的未过期远端成员；`broadcastActiveSpeakers` 与 join 回放改用合并版。注意陈旧标记：发言变化不写 KV，合并结果可能滞后，需在注释/文档标注。
- [ ] **Step 3: 回归** — Run: `go test ./internal/signal/ -count=1`；Expected: 全部通过
- [ ] **Step 4: Commit** — `feat(signal): merge remote speaking into join replay`

### Task 2: 发言阈值/滞回参数可配置化

**Files:** Modify `packages/sfu-client/src/types.ts`（`SFUClientOptions`）、`srs-client.ts`（构造器硬编码 `threshold 10 / holdOn 120 / holdOff 300`）、`cloudflare-client.ts`（`threshold 10 / holdOn 150 / holdOff 500`）、`LocalSpeakingMeter` 选项贯通；可选 Modify 服务端 `JoinTokenResponse` 下发

- [ ] **Step 1: 写失败测试** — meter 级：`new LocalSpeakingMeter({ threshold: 30 })` 在中等音量下不触发；当前 meter 已支持选项但两个客户端把参数写死，先加客户端透传测试。
- [ ] **Step 2: 最小实现** — `SFUClientOptions.speaking` 配置块（threshold/holdOnMs/holdOffMs），两个客户端构造 meter 时读取；缺省保持现值。
- [ ] **Step 3: 回归** — Run: `pnpm --filter @gospeak/sfu-client test && build`；Expected: 通过
- [ ] **Step 4: Commit** — `feat(sfu-client): make speaking detection params configurable`

### Task 3: mediasoup（停用 provider）事件驱动化

**Files:** Modify `packages/sfu-client/src/mediasoup-client.ts`（`activeSpeakerTimer` setInterval 500ms 轮询远端轨道 `getLevel()`，硬编码 0.01、无滞回）；不改 `factory.go`（维持停用）

- [ ] **Step 1: 写失败测试** — 断言客户端无 `setInterval` 轮询残留（或 meter 化后状态翻转语义）。
- [ ] **Step 2: 最小实现** — 复用 `LocalSpeakingMeter` 思路分析远端轨道（每远端轨道一个 meter 实例 + 共享 `SpeakingDetector`），移除定时器；或抽取通用「轨道电平事件驱动采样」到 meter 层供本地/远端共用。
- [ ] **Step 3: 回归** — Run: `pnpm --filter @gospeak/sfu-client test`；Expected: 通过
- [ ] **Step 4: Commit** — `refactor(sfu-client): mediasoup speaking via event-driven meter`

### Task 4: 可选 RMS 指标

**Files:** Modify `packages/sfu-client/src/speaking-meter.ts` / `speaking-meter-worklet.ts`（工作台已可算 RMS，增加 `useRms` 选项 + `rmsThreshold`）；Docs `docs/sfu-provider-maturity.md` §6

- [ ] **Step 1: 实现** — worklet 在采样事件里附带 RMS；`useRms=true` 时主线程用 RMS 阈值替代频域均值。
- [ ] **Step 2: 校准** — 真实麦克风采集记录频域均值阈值 10 对应的 RMS 经验区间，写进文档；默认仍保持频域均值（避免灵敏度回归）。
- [ ] **Step 3: 测试+Commit** — meter 用例覆盖 `useRms` 分支；`feat(sfu-client): optional RMS speaking metric`

### Task 5: e2e 自动断言发言链路

**Files:** Modify `.agents/skills/room-voice-e2e/scripts/`（runner 增加 speaking 断言：一方出声 → 对方 `speaking` 高亮/事件出现）；Test `test/room/room.test.ts`（API 层可加 `member:speaking` → `room:active-speakers` 信令断言）

- [ ] **Step 1: API 层** — `room.test.ts` 增加：A 上报 `member:speaking=true` → B 收到 `room:active-speakers` 含 A（join 回放 + 去重两条用例）。
- [ ] **Step 2: 浏览器层** — room-voice-e2e 套件加「多用户发言高亮」场景（当前 speaking 仅为条件手动检查）。
- [ ] **Step 3: 归档** — 结果 Markdown 存 `agent_test_logs/`（命名 `{内容}-{时间}.md`）。

### Task 6: 文档：调参说明 + 浏览器兼容矩阵

**Files:** Modify `docs/sfu-provider-maturity.md` §6、`AGENTS.md` 发言检测小节

- [ ] 阈值（频域均值 > 10）与滞回参数含义、调参入口（Task 2 落地后）。
- [ ] AudioWorklet 兼容矩阵：Chrome 66+ / Safari 14.1+ / Firefox 76+；不支持时降级禁用 + warn。

## 验收清单

- [ ] `go build ./... && go test ./internal/signal/ -count=1`（含 `-race`）
- [ ] `gofmt -l app/server/internal/signal/` 为空
- [ ] `pnpm --filter @gospeak/sfu-client test && build`
- [ ] `pnpm --filter @gospeak/web test && build`（确认 worklet 内联进产物：`grep -rl registerProcessor dist/`）
- [ ] room-voice-e2e 手动验证 SRS / Cloudflare 发言高亮链路

## 残余风险

- AudioWorklet 需现代浏览器；不支持时发言检测降级禁用（console.warn），不崩溃
- 工作台 Blob URL 内联加载与打包器无关；若未来回归 `new URL(..., import.meta.url)` 方案需复验 Vite 生产构建（workspace 链接包不重写该调用）
- 判定阈值语义（频域均值 > 10）保持不变；RMS 选项（Task 4）须先校准再考虑默认
- Task 1 合并 KV Speaking 可能带回陈旧标记（发言变化不写 KV），以实时事件广播为准，文档标注

---

参考：`docs/sfu-provider-maturity.md` §6 · `docs/superpowers/plans/README.md`（索引维护）· `.agents/skills/room-voice-e2e`
