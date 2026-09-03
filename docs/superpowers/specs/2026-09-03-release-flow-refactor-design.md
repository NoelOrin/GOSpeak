# 发版流程重构设计（push 即发版，零 PR）

- 日期：2026-09-03
- 状态：设计已定稿，待用户审查后转实现计划
- 范围：仅 GitHub Actions 发版流程 + 一个 Node 脚本，不涉及任何业务代码

## 背景与问题

现状发版是「半手工闭环」：

- `release-please.yml` 监听 `release` 分支 push，自动开 Release PR（基于 conventional commits 算版本、生成 CHANGELOG/VERSION 段落）。
- 合并 Release PR 后，release-please 打 tag，`release.yml` 在 `release: published` 时交叉编译 10 平台二进制 + 生成 SHA256SUMS。
- 由于仓库分支保护要求走 PR、且 Actions 此前无 PR 创建权限，手动发版时只能手工合并发布 commit、打 tag、`gh release create`，再触发 `release.yml`。

用户诉求：取消 PR 发版路径，改为「推送形如 `release: publish release X.Y.Z` 的提交到 `release` 分支即全自动发版」，并保证 GitHub Release notes 完整（按 conventional commits 分类，风格与原 release-please 一致）。仓库内 `CHANGELOG.md` 不由发版 workflow 自动回写。

## 目标

1. 单路径、零 PR：发版唯一触发为 push 到 `release` 且 HEAD commit message 匹配 `^release: publish release (\d+\.\d+\.\d+([-/].*)?)$`。
2. 推送即全自动：解析版本 → 生成 Release notes（不修改仓库文件）→ 打 annotated tag `X.Y.Z` → 创建 GitHub Release → 在同一次 workflow 中继续出产物。
3. 彻底脱离 release-please：删除 `release-please.yml`、`.release-please-manifest.json`、`release-please-config.json`。
4. Release notes 自算分类：新增 `scripts/build-changelog.mjs`，按 conventional commits 从上一 tag 到 HEAD 归类 feat/fix/refactor/perf/docs/ci/style/test/build/chore/revert，生成与原 release-please 风格一致的 `## [X.Y.Z]` 段落；发版 CI 只输出 notes，不修改仓库内 `CHANGELOG.md`。

## 非目标（YAGNI）

- 不引入 changelog 生成依赖（standard-version / changesets / conventional-changelog-cli）。
- 不保留 release-please 兼容开关、不写回写 manifest。
- 不自动维护仓库内 `CHANGELOG.md`；该文件通过独立变更维护。
- 不做 dry-run 预览（沿用 `release.yml` 现有 `workflow_dispatch` 的 `dry_run` 即可）。

## 架构与组件

### 1. 触发识别（放在 `release.yml`）

`release.yml` 的 `on` 由单纯的 `release: published` 扩展为：

```yaml
on:
  release:
    types: [published]
  push:
    branches: [release]
```

新增 job `publish-release`（仅 `push` 路径、且 HEAD 匹配标识时运行），与现有 `release` 产物 job 解耦。现有 `release: published` 触发的产物 job 完全不变。

`publish-release` job 第一步解析 HEAD commit：

- 正则：`^release: publish release (\d+\.\d+\.\d+([-/].*)?)$`
- 命中 → 设 `VERSION`、`TRIGGER=true`；未命中 → `TRIGGER=false` 并跳过后续步骤（普通 push 不受影响）。
- 若 tag `X.Y.Z` 已存在或 GitHub Release 已存在，跳过对应动作避免重复。

### 2. CHANGELOG 生成脚本（`scripts/build-changelog.mjs`，新增）

职责：给定版本 `X.Y.Z` 与上一 tag（默认 `git describe --tags --abbrev=0` 取最近 tag，无则全量），读 `git log <lastTag>..HEAD`（或全量），逐条解析 conventional commit `type(scope): subject`，按 type 归类，输出一段标准 Markdown：

```markdown
## [X.Y.Z](https://github.com/NoelOrin/GOSpeak/compare/<lastTag>...X.Y.Z) (YYYY-MM-DD)

### Features
* <subject> ([hash](url/commit/hash))

### Bug Fixes
* ...

### Refactoring / Performance / Documentation / CI/CD / Styles / Tests / Build / Chores / Reverts
* ...
```

分类到 section 的映射（与原 `release-please-config.json` 的 `changelog-sections` 对齐）：

- feat → Features
- fix → Bug Fixes
- perf → Performance
- docs → Documentation
- ci → CI/CD
- refactor → Refactoring
- style → Styles
- test → Tests
- build → Build
- chore → Chores
- revert → Reverts

非 conventional 提交（如 `Merge branch ...`、中文提交）归入 `Chores` 或忽略（与原 release-please 行为一致：跳过无法解析的提交，不报错）。scope 保留在标题中。

`release.yml` 调用方式：`node scripts/build-changelog.mjs <VERSION> --no-write > /tmp/release-notes.md`，只生成 notes，不修改 `CHANGELOG.md`。脚本默认写回模式仅供本地手工维护 CHANGELOG 使用。

脚本同时支持本地直接运行（`node scripts/build-changelog.mjs 0.6.0`）便于验证，不依赖 GitHub 环境。

### 3. 打 tag + 建 Release（`publish-release` job 续）

顺序（均在 `TRIGGER=true` 后）：

1. 调用 `node scripts/build-changelog.mjs $VERSION --no-write > /tmp/release-notes.md` 生成 Release notes，不修改仓库文件。
2. `git tag -a $VERSION -m "Release $VERSION" && git push origin $VERSION`（已存在则跳过）。
3. `gh release create $VERSION --title "$VERSION" --notes-file /tmp/release-notes.md`（已存在则跳过）；预发布版本（`VERSION` 含 `-`/`+`）加 `--prerelease --latest=false`。

注意：发布提交路径由同一次 `release.yml` workflow 继续执行产物 job；外部创建的 GitHub Release 仍可通过 `release: published` 进入产物链，不依赖 GITHUB_TOKEN 触发新的 workflow。

### 4. 清理 release-please

删除以下文件（发版完全脱离 release-please）：

- `.github/workflows/release-please.yml`
- `.release-please-manifest.json`
- `release-please-config.json`

## 数据流

```
push release (HEAD = "release: publish release X.Y.Z")
  └─ release.yml / publish-release job
       ├─ 解析 VERSION（正则）
       ├─ scripts/build-changelog.mjs VERSION --no-write
       │    └─ git log <lastTag>..HEAD → 分类 → 输出 Release notes
       ├─ git tag -a X.Y.Z → push tag
       └─ gh release create X.Y.Z (notes = 同段落)
            └─ 同一次 release.yml workflow 的产物 job
```

## 错误处理

- 非发布提交：正则不匹配 → `TRIGGER=false` → 全部步骤 `if` 跳过，普通 push 零影响。
- 版本已发过：tag 已存在跳过打 tag；Release 已存在跳过建 Release；notes 仍按当前提交生成，不修改 CHANGELOG。
- 脚本失败：`build-changelog.mjs` 任何异常 `exit 1`，job 标红、不打 tag、不建 Release，不触发半成品发版。
- 网络/权限：打 tag 与建 Release 需要 `contents: write`（已在 job `permissions` 声明）；`gh` 用 `GITHUB_TOKEN`。
- 并发 push：workflow 级 concurrency 仍按 `github.ref` 串行；tag/Release 创建均做存在性检查，避免重复。

## 测试与验证

1. 本地：`node scripts/build-changelog.mjs 0.6.0 --no-write` 检查输出段落分类正确且不修改 `CHANGELOG.md`；另验证默认模式仍可供手工维护。
2. 单测（轻量）：`scripts/build-changelog.mjs` 的提交解析/分类函数可用 Node 内置 `node --test` 覆盖（可选，YAGNI 下限：至少本地跑通一次）。
3. CI 验证：在一个临时分支模拟 push `release: publish release 0.0.0-test` 观察 `publish-release` job 是否生成 notes、打 tag、建 Release 并在同一次 workflow 继续产物；验证后删除该测试 tag/Release（或仅 `dry_run` 路径）。
4. 回归：确认删除 release-please 后，普通 push 到 `release` 不再有 release-please run；现有 `release.yml` 产物 job 在 `release: published` 时仍正常。

## 实现步骤（概览，转 writing-plans 细化）

1. 新增 `scripts/build-changelog.mjs`（解析 + 分类 + notes 输出，保留本地写回模式）。
2. 改造 `release.yml`：扩展 `on`（加 `push: branches: [release]`）、新增 `publish-release` job（解析/生成/打 tag/建 Release），并让同一次 workflow 接续产物 job。
3. 删除 release-please 三件套（`release-please.yml` / `.release-please-manifest.json` / `release-please-config.json`）。
4. 本地验证脚本；CI 用测试版本验证整链；清理测试产物。
