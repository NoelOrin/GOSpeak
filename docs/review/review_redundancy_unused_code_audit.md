# GOSpeak 冗余与未使用代码审计

- 审计日期：2026-08-10
- 范围：GOSpeak monorepo（Go 后端、SolidJS 前端、packages、deploy）
- 方式：4 路并行扫描（Go 死代码、前端 TS 死代码、孤立模块/包、依赖与仓库冗余）
- 结论：本次仅产出清单，未修改任何代码

## 扫描方法

| 路数 | 工具/方法 | 说明 |
|------|-----------|------|
| 1. Go 后端 | `GOTOOLCHAIN=go1.26.0 go run golang.org/x/tools/cmd/deadcode@v0.45.0 ./...` | 基于 main 入口的编译期可达性分析 |
| 2. 前端 TS | `npx ts-prune --project app/web/tsconfig.json` | 编译器级未使用导出检测，过滤 `node_modules` 与 `(used in module)` |
| 3. 孤立模块/包 | 静态 import 图脚本 + `go list` / 引用检索 | 统计未引用文件、未注册 provider、未部署服务 |
| 4. 依赖与仓库 | import 文本扫描 + `go mod tidy -diff` + `git ls-files` | 未使用依赖、重复 lockfile、被跟踪的构建缓存/产物 |

可信度说明：

- Go `deadcode` 结果为编译期可达性结论，可靠；标记为“仅测试引用”的函数只在 `*_test.go` 中出现。
- ts-prune 为编译器级结论，可靠；少量符号在字符串/注释中存在同名文本，删除前建议二次复核。
- 孤立文件为静态 import 图结论，未覆盖运行时文件系统扫描、包入口和部署脚本，已逐项人工排除。

## R 级：整条“已禁用未删除”链路

### MediaSoup

MediaSoup 前后端三端实现全部标注“已禁用保留”，但代码、镜像配置和依赖仍在仓库中：

- `app/server/internal/sfu/providers/mediasoup/`：`bridge.go`、`provider.go`、`signal.go` 及测试，整个 provider 包不可达；
- `packages/mediasoup-worker/`：独立 worker 进程实现；
- `packages/sfu-client/src/mediasoup-client.ts`：前端 client，未在 factory 注册；
- `deploy/docker-compose.yml`：`mediasoup-worker` 服务整段注释禁用；
- `deploy/mediasoup-worker/Dockerfile`、`deploy/env` 相关配置。

### Daily

- `app/server/internal/sfu/providers/daily/`：整个 provider 包不可达；
- `packages/sfu-client/src/daily-client.ts`：前端 client，未在 factory 注册；
- `@daily-co/daily-js` 依赖仅被该死文件引用。

### Bot 运行时

- `packages/bot/`：83 个源文件的完整 Bot 运行时（core/runtime/media/speech/tts/plugins），但没有任何 compose 服务或部署入口，仅被根 `package.json` typecheck 和 docs 引用；
- 服务端另有 Go 实现 `app/server/internal/plugin/builtin/botbase/`，存在两套 Bot 实现，Node 侧未接线。

处理建议：从“删除”或“激活”二选一。若短期不启用，优先删除 Daily 链路；MediaSoup 和 Bot 若确定保留路线，应补上 compose 服务或明确移出主仓库。

## Y 级：运行时死代码

### Go 后端（deadcode 确认不可达）

以下函数在主程序链路中不可达，多数只被测试引用：

| 位置 | 符号 |
|------|------|
| `internal/config/config.go` | `Current`、`SetCurrent` |
| `internal/logger/logger.go`、`std.go` | `InitFromEnv`、`Entry`、`WithField`、`WithError`、`Writer`、`WriterLevel`、`Close`、`Trace`、`Tracef`、`Debug`、`Debugf`、`Info`、`Infof`、`Error`、`Errorf`、`Fatalf`、`Panic`、`Panicf`、`RedirectStdLogLevel` |
| `internal/middleware/auth.go`、`domain.go` | `SetPermissionChecker`、`SetTokenVersionChecker`、`SetBotTokenChecker`、`RequireOwnerOrPermission`、`SetDomainChecker` |
| `internal/authstate/blacklist.go` | `IsBlacklisted`（运行时使用 `IsBlacklistedErr`） |
| `internal/bus/embedded.go` | `StartEmbeddedServer` |
| `internal/cluster/scheduler.go`、`types.go` | `ChooseNodes`、`EncodeLabels` |
| `internal/webui/embed.go` | `FileSystem`（`FS` 被使用） |
| `internal/router/routes/system/routes.go` | `Register`（实际使用 `RegisterProtected`） |
| `internal/handler/srs_callback_handler.go` | `NewSRSCallbackHandler`（实际使用 `NewSRSCallbackHandlerWithResolver`） |
| `internal/repository/oauth_account_repo.go` | `GetByUserID`、`Create` |
| `internal/repository/role_repo.go` | `GetByName` |
| `internal/service/conversation_service.go` | `ConvTo` |
| `internal/service/leader_fence.go` | `Active` |
| `internal/service/sfu_service.go` | `Capabilities` |
| `internal/handler/room_handler.go` | `notifyRoomList` |
| `internal/plugin/builtin/botbase/embed.go` | `EmbeddedFS` |
| `internal/signal/hub_room.go`、`recover.go` | `roomInitByIdentity`、`logPanic`、`safeHandler`、`safeHandlerAck`、`safeHandlerNoData` |

### 前端孤儿文件（静态 import 图确认无引用）

- `app/web/src/components/common/commonModal.tsx`
- `app/web/src/components/common/visible.tsx`
- `app/web/src/components/dashboard/index.tsx`
- `app/web/src/components/profile/avatarUpload.tsx`
- `app/web/src/components/storage/fileUpload.tsx`
- `app/web/src/layouts/ErrorComponent.tsx`（入口实际使用 TanStack Router 自带组件）
- `app/web/src/layouts/common/footer.tsx`
- `app/web/src/types/room.ts`（类型已由 `socket/types.ts` 取代）
- `app/web/src/types/userInfo.ts`（同上）
- `app/web/src/utils/pip-keepalive.ts`

### 前端未使用导出（ts-prune 标记）

- `app/web/src/api/auth.ts`：`changePassword`
- `app/web/src/api/conversation.ts`：`sendDirectMessage`
- `app/web/src/api/message.ts`：`sendMessage`、`editMessage`、`deleteMessage`、`unreactMessage`
- `app/web/src/api/plugin.ts`：`getPlugin`
- `app/web/src/api/sfu.ts`：`getSFUConfig`
- `app/web/src/api/email.ts`：`VerifyEmailCodeInput`
- `app/web/src/utils/permissions.ts`：`requirePermission`
- `app/web/src/components/room/session/voiceSessionTypes.ts`：`VoiceSessionView`
- `app/web/src/components/room/hooks/useRoomAudioBridge.ts`：`teardownRoomAudioBridge`
- `app/web/src/handler_audio/notificationSounds.ts`：`isNotificationSoundEnabled`
- `app/web/src/handler_audio/index.ts`：`setAudioOutputDevice`、`setVolumeByIdentity`、`setMutedByIdentity`、`setMasterVolume`、`setMasterMuted`
- `app/web/src/stores/socketStore.ts`：`MuteEvent`、`UnmuteEvent`

## G 级：依赖与仓库冗余

### 前端未使用依赖（源码零 import）

`@tanstack/solid-start`、`zod`、`async-validator`、`class-variance-authority`、`solid-split-pane`、`tailwind-merge`、`tailwindcss-animate`、`@tanstack/router-cli`、`@types/color`。

注意：`daisyui` 经 CSS `@plugin "daisyui"` 使用，不属于未使用依赖；`@types/node`、`typescript`、`esbuild`、`jsdom`、`playwright` 为工具链依赖，不算冗余。

### 重复依赖

- `@tanstack/router-plugin` 同时存在于 `app/web` 的 `dependencies`（1.133.21）和 `devDependencies`（1.141.1）；
- `@commitlint/cli` 在根 `package.json` 与 `app/web/package.json` 重复。

### 被 git 跟踪的冗余产物

- `app/web/pnpm-lock.yaml`：与根 pnpm workspace 重复的独立 lockfile；
- `app/docs/.vitepress/cache/`：约 2.8MB VitePress 构建缓存；
- `findings.json`、`test-results/.last-run.json`：review/测试产物。

### 文档过时

- AGENTS.md 及若干架构文档仍写 `app/sfu-client` 路径，实际只有 `packages/sfu-client`。

## 已排除项

- `internal/redis`：本地空目录，未跟踪，不属于代码冗余。
- `packages/bot/src/plugins/builtin/index.ts`、`plugins/example/echoPlugin.ts`：运行时通过插件目录扫描动态加载，不是死代码。
- `packages/sfu-client/src/index.ts`、`packages/mediasoup-worker/src/index.ts`：包入口/进程入口，不是死代码。
- `go.mod`：`go mod tidy -diff` 无输出，无冗余 Go 依赖。

## 建议下一步

1. 与产品确认 Daily/MediaSoup 是否保留，决定删除或补齐激活链路；
2. 删除 R/Y 级清单中的死代码，并同步清理相关依赖与 docker-compose 注释；
3. 清理未使用依赖、重复 lockfile、被跟踪的构建缓存；
4. 更新 AGENTS.md 与架构文档中的过时路径；
5. 清理完成后跑 `go test ./...`、`pnpm typecheck`、前端 vitest 与构建做回归验证。
