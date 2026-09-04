# 项目结构整理与去耦合 实施计划

> **Status (2026-08-13):** ⚠️ 部分完成 — apiClient/roomDetail/socketStore 拆分已落地; 剩余项见 docs/frontend-coupling-remaining.md (已归档)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement task-by-task.

**Goal:** 清理已识别的命名不一致 + 残余前端耦合 + Go 后端 internal 扁平结构优化

**Architecture:** 4 个独立 Task，互不阻塞，可独立 PR

**Tech Stack:** Go 1.22+, SolidJS + TypeScript, pnpm workspace

## Global Constraints

- Go `internal/` 包路径变更用 `git mv` 保持历史
- 前端组件外部接口（exports / props）不变
- Go 编译在每个 Task 后通过: `cd app/server && go build ./...`
- 前端 lint 通过: `pnpm --filter @go-rtc/web lint`

**前提说明:** `docs/frontend-coupling-remaining.md` 列出的 3 项中，#1 (apiClient token callback) 已通过 `apiClientAuth.ts` 注入模式解决，#3 (roomDetail 拆 hooks) 已部分执行 (94 行，使用 3 个 hooks)。本计划仅覆盖仍存在的实干项。

---

### Task 1: 修复目录命名 typo — `settting/` → `setting/`

**Files:**
- Rename: `app/web/src/components/modal/settting/` → `app/web/src/components/modal/setting/`
- Modify: `app/web/src/layouts/layout.tsx`

**Interfaces:**
- Consumes: none
- Produces: correct directory path for SettingModal

- [ ] **Step 1: git mv 修正目录名**

```bash
cd /Users/noelorin/GOSpeak
git mv app/web/src/components/modal/settting app/web/src/components/modal/setting
```

Expected: directory renamed, git detects as rename

- [ ] **Step 2: 更新 import 路径**

Edit `app/web/src/layouts/layout.tsx:7`:

```diff
- import SettingModal from "@/components/modal/settting/settingModal";
+ import SettingModal from "@/components/modal/setting/settingModal";
```

- [ ] **Step 3: 验证编译通过**

```bash
cd app/web && pnpm check 2>&1 | head -20
```

Expected: no module resolution errors

- [ ] **Step 4: Commit**

```bash
git add app/web/src/components/modal/setting app/web/src/layouts/layout.tsx
git commit -m "fix: correct typo in modal directory name (settting → setting)"
```

---

### Task 2: socketStore → handler_audio 解耦

**Files:**
- Modify: `app/web/src/stores/socketStore.ts`
- Modify: `app/web/src/components/room/hooks/useRoomAudioBridge.ts`
- No change: `handler_audio/speakingStore.ts` (exports unchanged)

**Interfaces:**
- Consumes: socketStore has `ROOM_ACTIVE_SPEAKERS` socket event handler
- Produces: socketStore exposes `onActiveSpeakers(cb)` registration; consumer (useRoomAudioBridge) subscribes and writes to speakingStore

- [ ] **Step 1: socketStore 添加 onActiveSpeakers 回调注册**

Edit `app/web/src/stores/socketStore.ts`:

在 stores 内部添加回调列表变量和注册函数（solid-js 或 plain callback 皆可）：

```typescript
// 在文件顶部 module helpers 区域添加
type SpeakerCallback = (identities: string[]) => void;
let activeSpeakerCallbacks: SpeakerCallback[] = [];

export function onActiveSpeakers(cb: SpeakerCallback) {
  activeSpeakerCallbacks.push(cb);
  return () => {
    activeSpeakerCallbacks = activeSpeakerCallbacks.filter(c => c !== cb);
  };
}
```

- [ ] **Step 2: 替换直接调用**

Edit `app/web/src/stores/socketStore.ts` line ~304:

```diff
- import { setSpeakingIdentities } from "@/handler_audio/speakingStore";
// ... (keep the comment noting the decoupling direction, update it)

// 在 ROOM_ACTIVE_SPEAKERS handler 内:
- setSpeakingIdentities(event?.identities ?? []);
+ activeSpeakerCallbacks.forEach(cb => cb(event?.identities ?? []));
```

同时更新顶部注释：

```diff
- // NOTE: store -> audio 写入是已知耦合；房间 UI 直接读 speakingStore。
- // 若后续要彻底解耦，改为 onActiveSpeakers 订阅，由 useRoomAudioBridge/voiceChat 写入。
+ // NOTE: socketStore 通过 onActiveSpeakers 广播发言者列表；
+ // useRoomAudioBridge 等消费者订阅后写入 speakingStore。
```

- [ ] **Step 3: useRoomAudioBridge 订阅 onActiveSpeakers**

Edit `app/web/src/components/room/hooks/useRoomAudioBridge.ts`:

```diff
+ import { onActiveSpeakers } from "@/stores/socketStore";
+ import { setSpeakingIdentities } from "@/handler_audio/speakingStore";
// 在适当生命周期（如 onMount）添加：
+ onCleanup(onActiveSpeakers((identities) => {
+   setSpeakingIdentities(identities);
+ }));
```

- [ ] **Step 4: 验证编译 + 逻辑**

```bash
cd app/web && pnpm check 2>&1 | head -30
```

Expected: compilation passes, no type errors

- [ ] **Step 5: Commit**

```bash
git add app/web/src/stores/socketStore.ts app/web/src/components/room/hooks/useRoomAudioBridge.ts
git commit -m "refactor: decouple socketStore from handler_audio via onActiveSpeakers callback"
```

---

### Task 3: Go SFU providers 归入 `internal/sfu/providers/`

**Files:**
- Move (git mv): 6 directories into `internal/sfu/providers/`
  - `internal/livekit/` → `internal/sfu/providers/livekit/`
  - `internal/srs/` → `internal/sfu/providers/srs/`
  - `internal/agora/` → `internal/sfu/providers/agora/`
  - `internal/daily/` → `internal/sfu/providers/daily/`
  - `internal/mediasoup/` → `internal/sfu/providers/mediasoup/`
  - `internal/cloudflare/` → `internal/sfu/providers/cloudflare/`
- Modify: files importing old paths (8 files total)

**Interfaces:**
- Consumes: old `GOSpeak/internal/{provider}` import paths
- Produces: new `GOSpeak/internal/sfu/providers/{provider}` import paths

- [ ] **Step 1: 创建 providers 目录 + git mv 所有 SFU 实现**

```bash
cd /Users/noelorin/GOSpeak/app/server
mkdir -p internal/sfu/providers
git mv internal/livekit internal/sfu/providers/livekit
git mv internal/srs internal/sfu/providers/srs
git mv internal/agora internal/sfu/providers/agora
git mv internal/daily internal/sfu/providers/daily
git mv internal/mediasoup internal/sfu/providers/mediasoup
git mv internal/cloudflare internal/sfu/providers/cloudflare
```

Expected: git detects all as renames

- [ ] **Step 2: 更新 factory 导入路径**

Edit `app/server/internal/sfu/factory/factory.go`:

更新 import 块中的 6 条路径:
```diff
- "GOSpeak/internal/agora"
+ "GOSpeak/internal/sfu/providers/agora"
// (同样更新 cloudflare, daily, livekit, mediasoup, srs)
```

- [ ] **Step 3: 更新 handler/service 外部导入路径 (3 files)**

Edit `app/server/internal/handler/srs_callback_handler.go`:
```diff
- "GOSpeak/internal/srs"
+ "GOSpeak/internal/sfu/providers/srs"
```

Edit `app/server/internal/handler/cloudflare_handler.go`:
```diff
- "GOSpeak/internal/cloudflare"
+ "GOSpeak/internal/sfu/providers/cloudflare"
```

Edit `app/server/internal/service/cloudflare_media_service.go`:
```diff
- "GOSpeak/internal/cloudflare"
+ "GOSpeak/internal/sfu/providers/cloudflare"
```

- [ ] **Step 4: 更新 gin.go 导入路径**

Edit `app/server/internal/server/gin.go` (if imports mediasoup):
```diff
- "GOSpeak/internal/mediasoup"
+ "GOSpeak/internal/sfu/providers/mediasoup"
```

- [ ] **Step 5: 验证 Go 编译**

```bash
cd /Users/noelorin/GOSpeak/app/server && go build ./...
```

Expected: compilation succeeds with no errors

- [ ] **Step 6: Commit**

```bash
git add app/server/internal/sfu/providers app/server/internal/sfu/factory/factory.go app/server/internal/handler/srs_callback_handler.go app/server/internal/handler/cloudflare_handler.go app/server/internal/service/cloudflare_media_service.go
git add app/server/internal/server/gin.go
git commit -m "refactor: consolidate SFU providers under internal/sfu/providers/"
```

---

### Task 4: webui 构建产物标记/分离

**Files:**
- Modify: `app/server/internal/router/router.go`
- Add: `app/server/internal/webui/embed.go` (if not exist)
- Maybe: add `.gitignore` entry or generate script

**Interfaces:**
- Consumes: `internal/webui/` embedded FS (build artifact from frontend)
- Produces: clear separation of generated vs source code

- [ ] **Step 1: 检查 webui 内容并确认嵌入方式**

```bash
ls -la app/server/internal/webui/
cat app/server/internal/router/router.go | grep -A5 -B5 webui
```

了解当前 webui 是通过 `//go:embed` 还是直接引用。

- [ ] **Step 2: 按检查结果做对应处理**

**情况 A** — 如果使用 `//go:embed`:
- 在 `internal/webui/` 内添加 `embed.go`，将 `embed.FS` 包装为导出变量
- 路由层通过引用该变量加载静态资源

**情况 B** — 如果仅路由注册引用:
- 在 `internal/webui/` 内加 `README.md` 注明 "此目录由前端构建生成，勿手动编辑"
- 在根 `.gitignore` 或 `app/server/.gitignore` 确认 `webui/dist/` 等构建产物的策略

- [ ] **Step 3: 验证 Go 编译**

```bash
cd /Users/noelorin/GOSpeak/app/server && go build ./...
```

Expected: compilation succeeds

- [ ] **Step 4: Commit**

```bash
git add app/server/internal/webui/ app/server/internal/router/router.go
git commit -m "chore: mark webui as build artifact, add embed documentation"
```

---

### Task 5 (可选): 更新 project-gaps.md 和 frontend-coupling-remaining.md

- [ ] **Step 1: 更新 frontend-coupling-remaining.md**

标记 #1 (apiClient) 和 #3 (roomDetail) 为已完成 / 部分完成。保留 #2 (socketStore) 引用本计划 Task 2。

- [ ] **Step 2: Commit**

```bash
git add docs/
git commit -m "docs: update coupling status after restructuring tasks"
```
