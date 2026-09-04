# 发版流程重构设计（Release Flow Refactor）

- 日期：2026-09-03
- 范围：`.github/workflows/release.yml`、`scripts/build-changelog.mjs`、`scripts/check-version.mjs`、`commitlint.config.js`、`package.json`
- 目标：去掉 release-please 三件套，改为"向 `release` 分支推 `release: publish release X.Y.Z` 即自动发版"的零 PR 流程。

## 背景与决策

旧流程用 release-please（`release-please.yml` / `.release-please-manifest.json` / `release-please-config.json`）自动开 PR 并发版。重构后改为：

1. 删除 release-please 三件套，删除写死的 `scripts/generate-changelog.mjs`。
2. 新增 `scripts/build-changelog.mjs`：按 conventional commits 分类生成 Release notes，支持 `--no-write`（CI 用）与默认写回（本地维护 CHANGELOG.md 用）。
3. 自动发版触发标识：`release: publish release X.Y.Z`（push 到 `release` 分支，且 HEAD commit message 以该前缀开头）。
4. `commitlint.config.js` 增加 `release` type，允许该触发提交通过校验。
5. **CHANGELOG.md 不回写**：`release` 分支受保护，CI 的 `GITHUB_TOKEN` 无法直推；且用户明确选择不回写。Release notes 仅在 Release body 展示，CHANGELOG.md 由本地 `build-changelog.mjs`（默认模式）或手工维护。
6. 不给 `github-actions` 配置 `release` 分支保护 bypass。

## Workflow 结构

`.github/workflows/release.yml` 三个触发入口：

- 路径 A（自动发版 / 零 PR）：`push: branches: [release]` 且 HEAD commit 匹配 `release: publish release X.Y.Z` → `publish-release` 自动打 tag + 建 GitHub Release，再在同一次 workflow 内续跑产物链。
- 路径 B（外部 Release）：`release: [published]` → 交叉编译 10 平台二进制 + SHA256SUMS。
- 路径 C（手动预览）：`workflow_dispatch`（`tag` / `skip_build` / `dry_run`）。

job 依赖链：

```
publish-release (push only, 打 tag + Release)
      │
   resolve-tag ──► version-check ──► create-release ──► build-release-binaries (×10) ──► checksums
```

`resolve-tag` 是产物链入口，从 `event.release.tag_name` / `inputs.tag` / `publish-release.outputs.tag` 解析版本，三者依次兜底。

## 关键修复（本轮验证发现）

### 1. skipped 状态沿 needs 链向下游传播

现象：`workflow_dispatch` 触发时 `publish-release` 因 `if` 不满足被 skip，导致 `version-check` / `create-release` / build / checksums 全部 `skipped`（runner=None，未真正调度）。

根因：GitHub Actions 中，当上游 job 被 `if` 跳过（skipped），下游 job 若未显式 `if: always()`（或别的条件），会继承 skipped 状态，即使下游只 `needs` 一个"已成功"的中间 job。

修复：给下游所有 job 加 `if: always()` 阻断传播（`version-check` 直接 `if: always()`；`create-release` / `build-release-binaries` / `checksums` 包成 `always() && (原有条件)`）。验证 run `33740048786` 全绿。

### 2. build 步骤 `matrix.ext` 坏替换

现象：所有 build job 报 `bad substitution: gospeak-...${matrix.ext}`。

根因：build 的 `run:` 里写成了 `gospeak-${{ matrix.target }}${matrix.ext}`，`${{ matrix.target }}` 是 GitHub 表达式会被展开，但 `${matrix.ext}` 漏写 `${{ }}`，被直接传给 bash，bash 不认带点的变量名 `matrix.ext` 而报错。

修复：三处 `${matrix.ext}` → `${{ matrix.ext }}`（build 输出、upload-artifact 路径、release upload 路径）。

### 3. tag 创建缺 git identity

现象：早期失败 run `33735520044` 报 `Committer identity unknown`（`git tag -a` 需要提交者身份）。

根因：取消 CHANGELOG 回写时把 `Configure git` 步骤一并删了。

修复：在 `Generate Release notes` 与 `Create tag` 之间加 `Configure git identity` 步骤（`github-actions[bot]` 身份）。

## CHANGELOG 完整性说明

- `CHANGELOG.md` 的 `0.4.0` 段落为手工维护，已存在且完整（基线 `v0.3.2`，全分类）。
- CI 仅用 `--no-write` 生成 Release body，不回写 `CHANGELOG.md`。
- PR 合并到 `release` 分支不会自动更新 `CHANGELOG.md`；发版前应在本地跑 `node scripts/build-changelog.mjs <version> <lastTag>`（默认写回模式）生成段落，或手工补充。
- 本地验证：`node scripts/build-changelog.mjs 0.4.0 v0.3.2 --no-write` 正常输出分类段落。

## 验证结果

- 自动发版路径 run `33740048786`：conclusion=success。
  - `Resolve Tag` / `Check Release Version` / `Create GitHub Release` / 10× `Build` / `Generate & Upload SHA256SUMS` 全绿。
  - Release `0.4.0` 含 11 个 asset：10 平台二进制（linux/darwin/freebsd/openbsd/windows × amd64/arm64，linux 另含 armv7）+ `SHA256SUMS.txt`。
- 远端 tag `0.4.0` 指向 `release` 分支 HEAD `6093eca`（由 `publish-release` 创建，后因修复流程未重打，沿用已存在 tag）。
