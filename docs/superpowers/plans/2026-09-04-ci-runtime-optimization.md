# CI 运行效率与发布构建复用 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 减少 GitHub Actions 中 pnpm 与前端的重复工作，同时修复路径过滤和发布任务的无效传播。

**架构：** 共享 pnpm Action 使用 `setup-node` 的单一缓存，并支持审计任务跳过安装；PR/分支 CI 保持现有门禁拓扑，仅细化触发条件；Release 增加一次性 Web 构建 job，10 个 Go 平台从 artifact 复用前端资源，并通过显式上游结果阻止空 tag 或失败任务继续执行。

**技术栈：** GitHub Actions、pnpm 10.11、Node 22、Turbo、Go、现有 `upload-artifact@v7` / `download-artifact@v8`。

---

## 文件结构

| 文件 | 责任 |
|------|------|
| `.github/actions/pnpm-install/action.yml` | 统一 pnpm/Node 初始化、单一缓存和可选依赖安装 |
| `.github/workflows/ci.yml` | 路径过滤、质量门禁和 Web artifact 传递 |
| `.github/workflows/release.yml` | tag 校验、一次性 Web 构建、跨平台二进制矩阵 |
| `.github/workflows/security.yml` | 依赖安全扫描的触发范围和安装开销 |
| `docs/superpowers/specs/2026-09-04-ci-runtime-optimization-design.md` | 已确认的设计边界与验收标准 |

### 任务 1：精简共享 pnpm Action

**文件：**
- 修改：`.github/actions/pnpm-install/action.yml`
- 修改：`.github/workflows/security.yml`
- 验证：复合 Action 输入结构、无 node_modules 缓存、audit 命令无需安装

- [ ] **步骤 1：改为 setup-node 单一缓存**

保留 `pnpm/action-setup@v6` 与 `actions/setup-node@v6`，在 setup-node 中设置：

```yaml
with:
  node-version: 22
  cache: pnpm
  cache-dependency-path: pnpm-lock.yaml
```

删除 `pnpm store path` 输出和 `actions/cache` 步骤。

- [ ] **步骤 2：增加可选安装输入**

在 `inputs` 中增加默认值为 `true` 的 `install`，安装步骤使用：

```yaml
if: inputs.install == 'true'
run: pnpm install --frozen-lockfile --prefer-offline
```

- [ ] **步骤 3：让 Security audit 跳过 workspace 安装**

在 `.github/workflows/security.yml` 的 pnpm audit job 中传入：

```yaml
with:
  install: false
```

- [ ] **步骤 4：验证共享 Action**

运行：

```bash
rg -n "actions/cache|node_modules|cache-dependency-path|install:" .github/actions/pnpm-install/action.yml .github/workflows/security.yml
```

预期：只保留 setup-node 的 `cache: pnpm`，缓存路径不出现 `node_modules`，audit job 明确使用 `install: false`。

### 任务 2：修正 CI 路径和步骤级条件

**文件：**
- 修改：`.github/workflows/ci.yml`
- 验证：过滤器覆盖关键文件，手动/复用事件具有强制执行条件

- [ ] **步骤 1：补全 JS/CI 变更过滤器**

在 `js` 过滤器中加入根 `package.json`、`test/**`、`**/tsconfig*.json`、`.npmrc` 和相关构建脚本；在 `ci` 过滤器中加入 `scripts/**`，确保工作流/版本脚本改动触发门禁。

- [ ] **步骤 2：为手动和 workflow_call 增加强制运行条件**

为 `lint`、`typecheck`、`build-js`、`build-server` 和 `test-server` 的 job 条件加入：

```yaml
github.event_name == 'workflow_dispatch' || github.event_name == 'workflow_call'
```

同时保持现有路径条件和 `run_tests` 输入语义。

- [ ] **步骤 3：按变更类型跳过无关安装**

`lint` 保持一个 job，但给 pnpm Action、Biome、setup-go、Go vet 分别增加对应的 `if`；Biome 命令改为 `pnpm exec biome ci ...`。

- [ ] **步骤 4：验证 CI 条件**

运行：

```bash
rg -n "workflow_dispatch|workflow_call|tsconfig|test/|pnpm exec biome|pnpm-install|setup-go" .github/workflows/ci.yml
```

预期：手动/复用事件会绕过路径过滤，纯 Go/纯 JS lint 不再初始化无关工具链。

### 任务 3：Release 复用单次 Web 构建并修正依赖传播

**文件：**
- 修改：`.github/workflows/release.yml`
- 验证：job 图、artifact 名称、tag ref 和失败传播

- [ ] **步骤 1：收紧 tag 下游入口**

保留 `resolve-tag` 对 release/workflow_dispatch/成功自动发版 push 的入口条件；把 `version-check` 改为仅在 `resolve-tag` 成功且输出非空 tag 时运行。

- [ ] **步骤 2：新增一次性 `build-web` job**

让 job 依赖 `resolve-tag` 与 `version-check`，checkout 解析出的 tag，执行现有前端构建并上传：

```yaml
- uses: actions/upload-artifact@v7
  with:
    name: release-web-dist
    path: app/web/dist
    retention-days: 3
```

- [ ] **步骤 3：精简跨平台矩阵**

矩阵增加 `build-web` 依赖，删除 pnpm Action 和 Web 构建步骤，改为：

```yaml
- uses: actions/checkout@v7
  with:
    ref: ${{ needs.resolve-tag.outputs.tag }}
- uses: actions/download-artifact@v8
  with:
    name: release-web-dist
    path: app/web/dist
```

使用 checkout 后的 `git rev-parse HEAD` 作为嵌入的 release commit。

- [ ] **步骤 4：移除无条件 always 放行**

让 `create-release`、矩阵和 `checksums` 依赖的上游 job 均为 success 才继续；保留 `resolve-tag` 为必要的 `always()` 入口，因为 release 事件下 `publish-release` 是 skipped。

- [ ] **步骤 5：验证 Release 配置**

运行：

```bash
rg -n "build-web|release-web-dist|pnpm-install|download-artifact|resolve-tag|always\(\)|ref:" .github/workflows/release.yml
git diff --check
```

预期：只有 `build-web` 执行 pnpm 安装，矩阵只下载 Web artifact；普通分支 push 不会使用空 tag；diff 无空白错误。

### 任务 4：全局配置校验

**文件：**
- 检查：`.github/actions/pnpm-install/action.yml`
- 检查：`.github/workflows/ci.yml`
- 检查：`.github/workflows/release.yml`
- 检查：`.github/workflows/security.yml`

- [ ] **步骤 1：解析 YAML**

运行：

```bash
ruby -e 'require "yaml"; ARGV.each { |f| YAML.load_file(f); puts "ok #{f}" }' .github/actions/pnpm-install/action.yml .github/workflows/ci.yml .github/workflows/release.yml .github/workflows/security.yml
```

预期：四个文件均输出 `ok`。

- [ ] **步骤 2：运行受影响的本地检查**

运行：

```bash
pnpm exec biome ci --files-ignore-unknown=true ./app/web ./packages
(cd app/server && go vet ./...)
```

预期：Biome 与 Go vet 通过；若工作区原有改动导致失败，记录具体失败而不回退用户修改。

- [ ] **步骤 3：检查最终 diff 范围**

运行：

```bash
git status --short
git diff --stat -- .github docs/superpowers
```

预期：只出现本计划涉及的 CI/文档改动，以及任务开始前已存在的前端工作区改动。
