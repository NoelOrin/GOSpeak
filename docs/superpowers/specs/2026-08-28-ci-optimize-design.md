# CI 与提交链路优化 — 设计规格

日期：2026-08-28
分支：feat/ci-optimize（基线 release@47a6592，2026-08-28）
状态：待用户审查
作者：Codex + NoelOrin

## 1. 背景与目标

GOSpeak 当前提交链路与 CI 存在四类痛点：
- `build.yml` 7 个 job 各自 `pnpm install`，无 `paths` 过滤，`docs/**` 改动也触发 Go 构建；
- `release.yml` 以 `needs: build` 串行 + `cancel-in-progress: true`，`release` 并发易被误取消，Docker/二进制需等全量构建才起；
- `actions/checkout` 混用 v7/v4，`pnpm-install` 同时缓存 `pnpm store + node_modules` 导致膨胀与不可复现；
- `lefthook.yml` 的 `pre-push` 串行跑 `biome ci / go vet / typecheck / go test`，本地 push 常被卡 2-3 分钟，`CHANGELOG` 与 Release notes 需手工整理 28 条提交。

目标：收敛为"本地快反馈 -> PR 门禁 -> 攒批发版 -> 产物发布"单一可信链路，`CHANGELOG` 全自动，CI 按变更路径精准触发。

已确认决策：
- 发版策略：B 方案 `release-please`（攒常驻 Release PR，合并才真正发版，`CHANGELOG` 全自动，支持 `Release-as` 强制指定）。
- 本地校验：`pre-push` 保留轻量 `go vet` 与 `typecheck`，`go test -race` 完全下沉 CI。

## 2. 非目标

- 不引入 `changesets / turbo remote cache / preview env` 等平台化能力（YAGNI）。
- 不改变 `release` 为默认分支、`squash merge`、受 `~DEFAULT_BRANCH` Ruleset（`non_fast_forward`）保护的分支策略。
- 不改变 `Node 22 / pnpm 10.11 / Go 1.26 / Turbo` 基础栈。

## 3. 架构总览

```
本地 (lefthook)                 PR (ci.yml + pr-title.yml)               release 分支
──────────────                ──────────────────────────                  ───────────
git commit                     feat/* --PR(squash)--> release            release-please.yml (bot)
 ├─ pre-commit: biome --write   ├─ ci.yml: lint/typecheck/build/test     常驻 PR "chore: release 0.3.3"
 ├─ commit-msg: commitlint(去wip)├─ pr-title.yml: 标题必须 conventional   自动更新 CHANGELOG + 版本
 └─ git push                    └─ mergeState: CLEAN                      合并 Release PR
    └─ pre-push: biome ci + go vet + typecheck ──────────────────────► release.yml
                                                                           on: release.published
                                                                           ├─ docker (ghcr.io, amd64/arm64)
                                                                           └─ binaries (10 平台) + SHA256
```

职责：
- `ci.yml`（由 `build.yml` 重做）：PR/push 的质量门禁，暴露 `workflow_call` 供 `release.yml` 复用。`concurrency.cancel` 仅对非 `release` 生效。
- `release-please.yml`：只负责算版本、写 `CHANGELOG.md`、维护常驻 Release PR、打 tag、建 GitHub Release。
- `release.yml`：只负责 tag 发布后构建产物，触发改为 `release: published`，与 `ci.yml` 并行，不再 `needs: build` 串行。
- `docs.yml`：保持不变，仅收紧 `paths` 触发。
- `pr-title.yml`：新增，保障 squash 后 `release` 历史干净。

## 4. 组件设计

### 4.1 本地提交链路

- `pre-commit`：`biome check --write`（仅 `staged_files`，`stage_fixed: true`）+ `go vet ./...`（`app/server/**/*.go` 变更才触发）。
- `commit-msg`：`commitlint` 保留，`type-enum` 从现有 `feat|fix|docs|style|refactor|perf|test|chore|revert|build|ci|types|wip` 中移除 `wip`，`wip` 禁止合入 `release`。
- `pre-push`：瘦身为 `biome ci` + `go vet` + `typecheck`（并行，目标 <30s，`typecheck` 保留系本次修正）。删除 `go test -count=1`，下沉 CI。需全量时手动 `pnpm typecheck && pnpm test:server:go -- -race`。
- `package.json` 可选新增 `pnpm commit`（`cz-git`）引导规范输入。
- 新增 `.github/workflows/pr-title.yml`：`amannn/action-semantic-pull-request@v5`，标题必须 `feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert|types(!)?: subject`，失败则 `BLOCKED`。

### 4.2 共享安装

- ` .github/actions/pnpm-install/action.yml`：统一 `pnpm/action-setup@v6` + `actions/setup-node@v6`（node 22，`cache: pnpm`）。
- 仅缓存 `pnpm store`（`pnpm store path`），删除 `node_modules` 缓存，`key: pnpm-store-${hashFiles('pnpm-lock.yaml')}`。
- 全仓 `actions/checkout` 统一至 `v4` 并 pin SHA，`permissions: contents: read` 最小化。

### 4.3 CI 门禁（ci.yml）

- `on.push/pull_request` 增加 `paths-ignore: ['docs/**','**.md']` 语义，配合 `dorny/paths-filter` 的 `changes` 探针，使 `docs/**` 纯文档变更跳过 Go/JS 任务。
- 聚合：`lint`（`biome ci + pnpm check + go vet`）单 job；新增 `typecheck`（`pnpm typecheck`）必过门禁。
- 合并 JS 构建：`build-web / build-sfu-client / build-mediasoup-worker` 合为 `build-js`，一次 `pnpm install` + `turbo run build --filter=@gospeak/web --filter=@gospeak/sfu-client ...`。
- `build-server` 保留 `needs: build-js` 传递 `web-dist` artifact（`actions/download-artifact@v4`），`test-server` 并行独立（`go test ./... -race -count=1`，`run_tests` 透传）。
- `concurrency: group: ci-${{github.ref}}, cancel-in-progress: ${{github.ref != 'refs/heads/release'}}`。

### 4.4 发版链路

- `release-please.yml`：`on.push.branches: [release]`，配置 `release-type: simple`（或 `go` 按需），`include-v-in-tag: true`，自动维护 `CHANGELOG.md`，`release-as` 覆盖，产出常驻 `chore(release): 0.3.x` PR。
- `release.yml`：`on.release.types: [published]` + `workflow_dispatch`（新增 `dry_run: boolean`）。`docker` 与 `build-release-binaries` 改为 `needs: [auto-version]` 语义的并行（实际依赖 `release-please` 已打 tag），产物上传改 `softprops/action-gh-release` append，避免并发写冲突。`concurrency.group: release` 且 `cancel-in-progress` 仅对 `push` 生效。

## 5. 数据流

1. 本地提交：`git add` -> `pre-commit`（staged 定向）-> `commit-msg`（`type-enum` 去 `wip`）-> `git push` -> `pre-push`（`biome ci + go vet + typecheck`），任一失败中断推送。
2. PR 门禁：`feat/*` -> PR 到 `release` -> 并行 `pr-title.yml` + `ci.yml`。`ci.yml` 先 `changes` 探针，再按需 `lint / typecheck / build-js / build-server / test-server`，`web-dist` 仅 `build-js` 成功才上传。
3. 攒批发版：`squash merge` 到 `release` -> `release-please` 扫描自上次 tag 的提交，按 `feat→minor / fix→patch / feat!或BREAKING CHANGE→major` 推断，更新 `CHANGELOG.md` 并维护同一 Release PR。
4. 产物发布：合并 Release PR -> 自动打 `v0.3.x` tag + `GitHub Release(published)` -> `release.yml` 并行 `docker` 与 `10 平台矩阵`，`checksums` 聚合 `SHA256SUMS` 追加上传。`dry_run: true` 仅预览 `tag + changelog diff`。

## 6. 错误处理与边界

- 本地：`pre-push` 任一失败阻断推送；`commitlint` 失败阻断提交；`stage_fixed` 保证 `biome --write` 自动重入暂存。
- PR：`pr-title` 失败 `BLOCKED`；`ci` 任一 required check 失败 `BLOCKED`；`docs` 探针误判则全量跑，不卡死；`Release-as: 1.0.0` 可覆盖推断。
- 发版：`CHANGELOG` 冲突由 bot 自动 rebase，失败评论提示；`BREAKING CHANGE` 仅识别 `!` 与 body 关键字；常驻 PR 未合时多次 push 仅更新同一 PR；`release.yml` `fail-fast: false`，`SHA256SUMS` 仅聚合成功产物；`dry_run` 不打 tag 不推镜像；已发 tag 可 `gh release delete --cleanup-tag` 回退。

## 7. 测试与验收

- 本地：`wip: foo` 被 `commitlint` 拒；`git push` <30s（`biome ci+go vet+typecheck`）；`go test` 不再阻塞推送。
- PR：`wip:` / `update docs` 标题被 `pr-title` 标 `BLOCKED`；`docs/**` 纯文档 PR <60s 且 Go/JS 跳过；`app/server` 改动仍触发 `go vet+build-server+test-server` 且 `typecheck` 必过。
- 发版：`feat:` 合到 `release` 更新 Release PR 为 `chore(release): 0.3.3` 并追加 `CHANGELOG`；`Release-as: 1.0.0` 生效；合并 Release PR 后自动打 tag 建 Release，`release.yml` 并行产出镜像与二进制 + `SHA256SUMS`，`dry_run` 可预览。
- 量化：`git push` <30s、纯 `docs` PR <60s、`pnpm store` 命中 >90%、`CHANGELOG` 与 `v0.3.0` 手写版格式一致。

## 8. 风险与回退

- `release-please` 首个 Release PR 需验证 `CHANGELOG.md` 中文标题与现有 `## [0.3.0]` 格式兼容，隔离在 `feat/ci-optimize` 验证后再合 `release`。
- `release` 为默认分支且受 Ruleset 保护，`squash` 策略不变，回退仅需关闭 Release PR 或 `gh release delete`，无分支副作用。
- 本地 `typecheck` 保留为轻量校验，若仍偏慢可改为 `pnpm typecheck --filter` 定向或 CI 兜底。
