# CI 运行效率与发布构建复用 — 设计规格

日期：2026-09-04
状态：已确认，进入实现

## 1. 背景与目标

当前 GitHub Actions 已完成 Node 24 兼容版本升级，但仍有三类可见开销和可靠性问题：

- 共享 pnpm 复合 Action 同时使用 `setup-node` 内置缓存和显式 `actions/cache`，同一 pnpm store 被重复声明；
- `release.yml` 的 10 个平台矩阵任务各自安装 pnpm 依赖并重新构建相同的 Web 前端；
- CI 的路径过滤和 Release 任务条件存在边界缺口：根目录包配置或集成测试改动可能未触发对应检查，普通 `release` 分支 push 可能让空 tag 继续流入发布任务。

目标：在不改变发布产物、质量门禁和现有 action 主版本的前提下，减少重复安装/构建，修复无效触发与空 tag 传播，并保留手动及 `workflow_call` 的可用性。

## 2. 非目标

- 不迁移 `release.yml` 到 `release-please`，不改变现有自动发版策略；
- 不引入 Turbo Remote Cache、第三方构建服务或新的 CI 平台；
- 不降低 Go race test、Biome、TypeScript、Go vet 或版本一致性检查；
- 不修改当前工作区已有的前端业务代码。

## 3. 方案

### 3.1 共享 pnpm 安装

`.github/actions/pnpm-install/action.yml` 保留 `pnpm/action-setup@v6` 和 `actions/setup-node@v6`，由 `setup-node` 统一负责 pnpm store 缓存，并显式指定 `pnpm-lock.yaml` 为缓存依赖文件。删除额外的 `actions/cache` 与 store path 输出，避免双重缓存恢复。

复合 Action 增加 `install` 布尔输入，默认安装依赖。Security 的 pnpm audit 只需要 pnpm、package manifest 和 lockfile，因此传入 `install: false`，跳过无意义的 workspace 安装。

### 3.2 CI 路径与任务条件

`ci.yml` 扩充 JS 变更范围，覆盖根 `package.json`、`test/**`、TypeScript 配置、pnpm 配置和构建脚本。手动触发与 `workflow_call` 视为强制运行，避免没有可靠 diff 基线时被 `paths-filter` 错误跳过。

`lint` 保持单个门禁，但按变更类型条件执行 Biome 与 Go vet：纯 Go 改动不安装 pnpm，纯 JS 改动不初始化 Go。Biome 使用 `pnpm exec` 调用本地依赖，避免 `npx` 的解析开销。

### 3.3 Release 前端产物复用

新增 `build-web` job，在 tag 解析和版本检查成功后构建一次 `@gospeak/web`，上传短期 `release-web-dist` artifact。10 个 Go 平台任务只下载该 artifact，删除各矩阵任务中的 pnpm 安装和前端构建。

二进制任务和校验和任务改为依赖结果明确成功的上游任务，不再用无条件 `always()` 放行失败或 skipped 的上游。所有发布构建 checkout `resolve-tag` 解析出的 tag，确保手动发布时构建内容与版本检查使用同一 ref。

### 3.4 Security 触发范围

Security 的分支 push 仅在依赖清单、锁文件、工作流或 Dependabot 配置变更时触发；定时扫描与手动触发保持不变。这样普通业务代码 push 不再重复执行依赖审计和 Go 漏洞扫描。

## 4. 数据流与失败处理

1. `ci.yml` 先运行 `changes`；普通 PR/push 按路径决定任务，手动/复用调用强制运行。
2. JS 变更触发 `build-js`，Go server 构建继续消费 `web-dist`；上游失败时下游不执行，但 `gate` 仍汇总并报告真实失败结果。
3. `release.yml` 普通 `release` 分支 push 若未生成 tag，只保留 `publish-release` 的 skipped 状态，`resolve-tag` 及其下游全部跳过；有效 tag 才能进入版本检查、前端构建和矩阵构建。
4. `dry_run` 与 `skip_build` 的既有语义保持不变，不生成发布构建产物。

## 5. 验收标准

- pnpm 复合 Action 只有一个缓存实现，并能通过 `install: false` 跳过安装；
- CI 过滤器覆盖根包配置、`test/**` 和 TypeScript 配置，手动/复用调用不会因路径过滤全部 skipped；
- `release.yml` 中前端只构建一次，矩阵任务均下载 `release-web-dist`，不再调用 pnpm 安装；
- 普通 release 分支 push 不会执行空 tag 的 `version-check`、`create-release` 或产物任务；
- YAML 可解析，`git diff --check` 通过，相关本地 TypeScript/Go 检查不受配置修改影响。

## 6. 回退

所有改动集中在 `.github` 工作流/复合 Action 和本设计文档。若远端 Action 行为与预期不符，可单独恢复 `pnpm-install` 的显式缓存或 `release.yml` 的矩阵构建，不涉及应用代码和数据库。
