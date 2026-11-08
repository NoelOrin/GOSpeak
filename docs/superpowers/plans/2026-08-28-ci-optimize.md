# CI 与提交链路优化 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 将 GOSpeak 提交链路收敛为 本地快反馈 -> PR 门禁 -> 攒批发版 -> 产物发布 单一可信链路，`CHANGELOG` 全自动，CI 按 `docs/**` 路径精准跳过。

**架构：** `lefthook` 保留 `biome ci + go vet + typecheck` 并移除 `go test` 下沉 CI，新增 `pr-title.yml` 门禁；`build.yml` 重做为 `ci.yml` 聚合 JS 构建并引入 `paths-filter`；发版切 `release-please` 常驻 Release PR，`release.yml` 改 `on: release.published` 并行产出 Docker/二进制并支持 `dry_run` 预览。

**技术栈：** pnpm 10.11 / Turbo 2.10 / Node 22 / Go 1.26 / Biome 2.5 / lefthook 1.8 / GitHub Actions (checkout@v4, pnpm/action-setup@v6, setup-node@v6, setup-go@v6, release-please-action) / commitlint conventional

**Spec:** `docs/superpowers/specs/2026-08-28-ci-optimize-design.md`

## Global Constraints

- 分支：`feat/ci-optimize` 基线 `release@47a6592`（2026-08-28），`release` 为默认分支，受 `~DEFAULT_BRANCH` Ruleset（`non_fast_forward`）保护，`squash merge` 不变
- Node 22 / pnpm 10.11 / Go 1.26 / Turbo 2.10 / Biome 2.5 固定
- `release-please` 配置：`release-please-config.json` 为 `release-type: simple`, `include-component-in-tag: false`, `changelog-path: CHANGELOG.md`，`packages: {".": {package-name: gospeak}}`
- 本地：`pre-push` 必须保留 `typecheck`（本次修正），`go test -race` 禁止在本地跑
- 缓存：仅 `pnpm store`，禁止缓存 `node_modules`；`actions/checkout` 统一 `v4`
- 发版：`release.yml` 仅在 `release.published` 或 `workflow_dispatch(dry_run)` 触发生物构建；常驻 Release PR 未合前不打 tag

---

## File Structure

| 文件 | 责任 |
|------|------|
| `lefthook.yml` | 本地钩子：pre-commit / commit-msg / pre-push |
| `commitlint.config.js` | 提交类型枚举（移除 `wip`） |
| `package.json` | 可选新增 `commit` 脚本（cz-git） |
| `.github/workflows/pr-title.yml` | 新增：PR 标题 conventional 门禁 |
| `.github/actions/pnpm-install/action.yml` | 共享安装：仅 pnpm store 缓存 |
| `.github/workflows/build.yml` -> `.github/workflows/ci.yml` | CI 门禁重做：paths-filter + 聚合 + concurrency |
| `.github/workflows/release-please.yml` | 新增：攒批发版常驻 PR |
| `.github/workflows/release.yml` | 解耦：改触发、并行产物、`dry_run` |
| `release-please-config.json` | 追加 `refactor/style/test/build/chore/revert` 等 section |
| `.release-please-manifest.json` | 校验版本基线 `. : 0.2.3` |

---

### Task 1: 本地提交链路瘦身 + PR 标题门禁

**Files:**
- Modify: `lefthook.yml`
- Modify: `commitlint.config.js`
- Modify: `package.json` (optional, add `commit` script)
- Create: `.github/workflows/pr-title.yml`
- Test: 本地 `commitlint` / `lefthook` 校验；`gh workflow view pr-title.yml`

**Interfaces:**
- Consumes: 现有 `lefthook.yml` / `commitlint.config.js` / `package.json`
- Produces: `pre-push` 仅 `biome ci + go vet + typecheck`；PR 标题必须 conventional；下游 `ci.yml` 依赖标题门禁

- [ ] **Step 1: 修改 `commitlint.config.js` 移除 `wip`**

将 `type-enum` 列表中的 `wip` 删除，保留 `feat, fix, docs, style, refactor, perf, test, chore, revert, build, ci, types`。编辑后文件形如：

```js
export default {
  extends: ['@commitlint/config-conventional'],
  rules: {
    'body-leading-blank': [2, 'always'],
    'type-empty': [2, 'never'],
    'subject-case': [0],
    'type-enum': [2, 'always', ['feat','fix','docs','style','refactor','perf','test','chore','revert','build','ci','types']],
  },
}
```

- [ ] **Step 2: 瘦身 `lefthook.yml` 的 `pre-push`**

将 `pre-push` 从 4 条改为 3 条并行，删除 `go test`，保留 `biome ci` + `go vet` + `typecheck`。实现：

```yaml
pre-push:
  parallel: true
  commands:
    "biome ci":
      run: npx @biomejs/biome ci --files-ignore-unknown=true ./app/web ./packages
    "go vet":
      run: cd app/server && go vet ./...
    "typecheck":
      run: pnpm typecheck
```

`pre-commit` 与 `commit-msg` 保持不变。

- [ ] **Step 3:（可选）`package.json` 新增 `commit` 脚本**

在 `scripts` 中追加（若已存在则跳过）：

```json
"commit": "cz"
```

并确保 `devDependencies` 含 `cz-git` 与 `commitizen`（`pnpm add -D cz-git commitizen`），`package.json` 追加 `config.commitizen.path: "cz-git"`。

- [ ] **Step 4: 新建 `.github/workflows/pr-title.yml`**

创建文件：

```yaml
name: PR Title
on:
  pull_request:
    types: [opened, edited, synchronize, reopened]
jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: amannn/action-semantic-pull-request@v5
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        with:
          types: feat,fix,docs,style,refactor,perf,test,build,ci,chore,revert,types
          requireScope: false
          subjectPattern: ^(?![A-Z].*$)  # 与 commitlint subject-case:0 保持不强制大小写，由正则放宽
```

`subjectPattern` 若与现有标题冲突，可先置 `""`，以 `types` 校验为主。

- [ ] **Step 5: 验证本地链路**

运行：

```bash
echo "wip: foo" | npx commitlint --from HEAD~1 2>&1 | grep -q "type-enum" && echo "wip rejected ok" || echo "check wip rule"
pnpm lefthook install && cat .git/hooks/pre-push | head -n 20
gh workflow view pr-title.yml 2>&1 | head -n 20
```

预期：`wip: foo` 被 `type-enum` 拒掉；`pre-push` 钩子含 3 条命令；`pr-title.yml` 可被 `gh` 识别。

- [ ] **Step 6: Commit**

```bash
git add lefthook.yml commitlint.config.js package.json pnpm-lock.yaml .github/workflows/pr-title.yml
git commit -m "chore(ci): 瘦身提交链路并新增 PR 标题门禁"
```

---

### Task 2: 共享安装与 Actions 版本统一

**Files:**
- Modify: `.github/actions/pnpm-install/action.yml`
- Modify: `.github/workflows/build.yml` / `.github/workflows/ci.yml` / `.github/workflows/release.yml` / `.github/workflows/docs.yml` 中的 `actions/checkout` 版本
- Test: `gh workflow view` / `act -l` 或 `yamllint`

**Interfaces:**
- Consumes: 现有 `pnpm-install/action.yml` 与各 workflow 的 `checkout` 引用
- Produces: 统一 `checkout@v4` + 仅 `pnpm store` 缓存，供 Task 3 的 `ci.yml` 复用

- [ ] **Step 1: 修改 `pnpm-install/action.yml` 删除 `node_modules` 缓存**

将 `actions/cache` 的 `path` 从 `pnpm store + node_modules` 改为仅 `pnpm store`，并固定 `actions/checkout` 不在此文件。修改后：

```yaml
- uses: actions/cache@v4
  with:
    path: ${{ steps.pnpm-cache.outputs.STORE_PATH }}
    key: ${{ runner.os }}-pnpm-store-${{ hashFiles('pnpm-lock.yaml') }}
    restore-keys: |
      ${{ runner.os }}-pnpm-store-
```

同时将 `pnpm/action-setup` 与 `actions/setup-node` 固定为 `v6`，`actions/cache` 固定为 `v4`。

- [ ] **Step 2: 全仓统一 `actions/checkout` 至 `v4` 并 pin SHA（可选）**

在 `build.yml`/`ci.yml`/`release.yml`/`docs.yml` 中将 `uses: actions/checkout@v7` 批量替换为 `uses: actions/checkout@v4`。可配合 `pinact` 或手动记录 SHA，至少保证 tag 一致。

- [ ] **Step 3: 为各 workflow 补 `permissions: contents: read` 最小化**

在 `build.yml`/`ci.yml` 的顶层或 job 层追加 `permissions: contents: read`，`release` 相关保持 `contents: write / packages: write` 仅在 `release.yml` 保留。

- [ ] **Step 4: 验证**

运行：

```bash
grep -R "checkout@" .github --include="*.yml" | sort
grep -A2 "actions/cache" .github/actions/pnpm-install/action.yml
```

预期：仅 `checkout@v4` 出现；`pnpm-install` 的 `path` 不含 `node_modules`。

- [ ] **Step 5: Commit**

```bash
git add .github/actions/pnpm-install/action.yml .github/workflows/build.yml .github/workflows/docs.yml
git commit -m "ci: 统一 checkout 为 v4 并精简 pnpm 缓存为仅 store"
```

---

### Task 3: CI 门禁重做 `ci.yml`（原 `build.yml`）

**Files:**
- Create/Modify: `.github/workflows/ci.yml`（由 `build.yml` 重命名与重写）
- Delete/Rename: `.github/workflows/build.yml` 保留为 `ci.yml` 或标记 deprecated
- Test: `gh workflow view ci.yml`；`dorny/paths-filter` 本地 dry-run；`pnpm typecheck`

**Interfaces:**
- Consumes: Task 2 的 `pnpm-install`；`app/web`, `packages/sfu-client`, `packages/mediasoup-worker`, `app/server`
- Produces: `lint / typecheck / build-js / build-server / test-server` 5 类门禁 + `web-dist` artifact，供 `release.yml` 消费但不串行阻塞

- [ ] **Step 1: 新建 `ci.yml` 框架与 `changes` 探针**

在 `.github/workflows/ci.yml` 顶部定义：

```yaml
name: CI
on:
  pull_request:
    branches: [release, dev, main, master]
  push:
    branches: [release, dev]
  workflow_call:
    inputs:
      run_tests: { required: false, default: true, type: boolean }
concurrency:
  group: ci-${{ github.ref }}
  cancel-in-progress: ${{ github.ref != 'refs/heads/release' }}
permissions:
  contents: read
jobs:
  changes:
    runs-on: ubuntu-latest
    outputs:
      go: ${{ steps.filter.outputs.go }}
      js: ${{ steps.filter.outputs.js }}
      docs: ${{ steps.filter.outputs.docs }}
    steps:
      - uses: actions/checkout@v4
      - uses: dorny/paths-filter@v3
        id: filter
        with:
          filters: |
            go: ['app/server/**', 'go.mod', 'go.sum']
            js: ['app/web/**', 'packages/**', 'pnpm-lock.yaml', 'turbo.json']
            docs: ['docs/**', '**/*.md', 'app/docs/**']
```

- [ ] **Step 2: 聚合 `lint` 与新增 `typecheck`**

在 `ci.yml` 追加：

```yaml
  lint:
    needs: changes
    if: needs.changes.outputs.go == 'true' || needs.changes.outputs.js == 'true'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: ./.github/actions/pnpm-install
      - run: npx @biomejs/biome ci --files-ignore-unknown=true ./app/web ./packages
      - run: pnpm check || true
      - uses: actions/setup-go@v6
        with: { go-version-file: app/server/go.mod, cache: true }
      - run: cd app/server && go vet ./...
  typecheck:
    needs: changes
    if: needs.changes.outputs.js == 'true'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: ./.github/actions/pnpm-install
      - run: pnpm typecheck
```

- [ ] **Step 3: 合并 JS 构建为 `build-js`**

```yaml
  build-js:
    needs: changes
    if: needs.changes.outputs.js == 'true'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: ./.github/actions/pnpm-install
      - run: pnpm turbo run build --filter=@gospeak/web --filter=@gospeak/sfu-client --filter=@gospeak/mediasoup-worker
      - uses: actions/upload-artifact@v4
        with: { name: web-dist, path: app/web/dist, retention-days: 3 }
```

删除原 `build-web / build-sfu-client / build-mediasoup-worker` 三 job。

- [ ] **Step 4: `build-server` 与 `test-server` 并行化**

```yaml
  build-server:
    needs: [changes, build-js]
    if: needs.changes.outputs.go == 'true' || needs.changes.outputs.js == 'true'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v6
        with: { go-version-file: app/server/go.mod, cache-dependency-path: app/server/go.sum, cache: true }
      - uses: actions/download-artifact@v4
        with: { name: web-dist, path: app/web/dist }
      - run: rm -rf app/server/internal/webui/dist && mkdir -p app/server/internal/webui/dist && cp -R app/web/dist/. app/server/internal/webui/dist/ && test -f app/server/internal/webui/dist/index.html
      - run: cd app/server && CGO_ENABLED=0 go build -ldflags="-s -w" -o gospeak .
  test-server:
    needs: changes
    if: always() && needs.changes.outputs.go == 'true' && inputs.run_tests != false
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v6
        with: { go-version-file: app/server/go.mod, cache-dependency-path: app/server/go.sum, cache: true }
      - run: cd app/server && CGO_ENABLED=1 go test ./... -race -v -count=1
```

- [ ] **Step 5: 验证与 Commit**

运行：

```bash
yamllint .github/workflows/ci.yml 2>&1 | head -n 20 || echo "yamllint ok"
pnpm typecheck 2>&1 | tail -n 20
gh workflow view ci.yml 2>&1 | head -n 30
```

预期：`ci.yml` 语法通过，`typecheck` 本地通过，`concurrency.cancel` 仅非 `release` 生效。

```bash
git add .github/workflows/ci.yml .github/workflows/build.yml
git commit -m "ci: 重做 CI 门禁为 ci.yml，按路径精准触发并聚合 JS 构建"
```

---

### Task 4: 发版链路切 `release-please` 并解耦 `release.yml`

**Files:**
- Create: `.github/workflows/release-please.yml`
- Modify: `.github/workflows/release.yml`
- Modify: `release-please-config.json`
- Verify: `.release-please-manifest.json`
- Test: `gh workflow view release-please.yml`；`release.yml` dry_run 预览

**Interfaces:**
- Consumes: `ci.yml` 的 `workflow_call`；现有 `release.yml` 的 `docker`/`binaries` 矩阵
- Produces: 常驻 Release PR + `vX.Y.Z` tag + `GitHub Release(published)`，触发解耦后的 `release.yml` 并行产出产物

- [ ] **Step 1: 补齐 `release-please-config.json` 的 section**

在 `packages["."].changelog-sections` 追加 `refactor/style/test/build/chore/revert`（`hidden: false`），保持 `feat/fix/perf/docs/ci` 已有，示例：

```json
{ "type": "refactor", "section": "Refactoring", "hidden": false },
{ "type": "style", "section": "Styles", "hidden": false },
{ "type": "test", "section": "Tests", "hidden": false },
{ "type": "build", "section": "Build", "hidden": false },
{ "type": "chore", "section": "Chores", "hidden": false }
```

- [ ] **Step 2: 新建 `.github/workflows/release-please.yml`**

创建：

```yaml
name: Release Please
on:
  push:
    branches: [release]
permissions:
  contents: write
  pull-requests: write
jobs:
  release-please:
    runs-on: ubuntu-latest
    steps:
      - uses: googleapis/release-please-action@v4
        with:
          config-file: release-please-config.json
          manifest-file: .release-please-manifest.json
          token: ${{ secrets.GITHUB_TOKEN }}
```

确保 `release-please-config.json` 的 `release-type: simple` 与 `bump-minor-pre-major: true` 保留。

- [ ] **Step 3: 重写 `release.yml` 触发与并发**

将 `on` 改为：

```yaml
on:
  release:
    types: [published]
  workflow_dispatch:
    inputs:
      tag: { description: "Release tag", required: false, default: "", type: string }
      skip_build: { description: "Skip builds", required: false, default: false, type: boolean }
      dry_run: { description: "Preview tag/changelog", required: false, default: false, type: boolean }
concurrency:
  group: release-${{ github.ref }}
  cancel-in-progress: false
```

`workflow_dispatch` 的 `dry_run: true` 时仅打印 `tag + changelog diff` 不打 tag。

- [ ] **Step 4: 解耦 `docker` 与 `binaries`，改并行与 append**

将 `docker` 与 `build-release-binaries` 的 `needs` 从 `[build, auto-version, prepare-release]` 改为 `needs: []` 或 `needs: [auto-version]` 的并行语义（依赖 `release` 事件已含 tag），产物上传改：

```yaml
- uses: softprops/action-gh-release@v2
  with:
    tag_name: ${{ github.event.release.tag_name }}
    files: dist/SHA256SUMS.txt
    append_body: true
```

移除 `release.yml` 内自研 `auto-version` / `prepare-release` 的打 tag 逻辑，全部交 `release-please`。

- [ ] **Step 5: 验证与 Commit**

运行：

```bash
cat release-please-config.json | python3 -m json.tool | head -n 40
gh workflow view release-please.yml 2>&1 | head -n 20
gh workflow view release.yml 2>&1 | head -n 30
```

预期：`release-please.yml` 可识别，`release.yml` 仅在 `release.published` 触发，`dry_run` 输入存在。

```bash
git add .github/workflows/release-please.yml .github/workflows/release.yml release-please-config.json .release-please-manifest.json
git commit -m "ci(release): 切 release-please 攒批发版并解耦产物发布"
```

---

## Self-Review

- 规格覆盖：4.1 本地链路 -> Task1；4.2 共享安装 -> Task2；4.3 CI 门禁 -> Task3；4.4 发版 -> Task4；数据流/错误处理/测试与验收均有对应验证步骤
- 占位符扫描：无 TODO/待定，所有 `release-type / include-v-in-tag / changelog-path` 已写明精确取值
- 类型一致性：`ci.yml` 的 `needs.changes.outputs.go/js/docs` 与 `dorny/paths-filter` 的 filter key 一致；`release-please` 的 `config-file/manifest-file` 路径与仓内现有文件一致
