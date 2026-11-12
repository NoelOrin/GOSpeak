# React 19 前端全量迁移设计

- 日期：2026-11-07
- 状态：已批准（头脑风暴定稿）；**2026-11-07 已落地完成**
- 前置依赖：IM 重构 P1（`2026-11-07-im-refactor-p1-message-foundation-design.md`）先在 Solid 上完成落地，本迁移以「P1 落地后的 dev-2.0」为起点

> **历史档案**：本文以迁移前的 SolidJS 现状为起点，文中「现在 / 现状」均指当时状态。当前技术栈、目录与命令以仓库根 `AGENTS.md`、`app/web/docs/AGENTS.md` 与 `app/web/README.md` 为准。

## 1. 目标与非目标

### 目标

- 前端从 SolidJS 1.9 全量迁移到 React 19，一次性 cutover，不留双栈
- 吃到主流组件库生态：shadcn/ui（Radix 无头组件）+ 自有主题 token，**保留当前视觉风格不换皮**
- 通用 UI 组件独立为 workspace 包 `packages/components`，配 Storybook 统一管理
- 全程不允许 UI 一致性破坏；动手前录制 UI 视觉基线，迁移以基线比对为验收门
- TanStack Router / Query 保留（换 React 适配层），团队已有心智延续

### 非目标（YAGNI）

- 不上 Next.js / SSR / RSC，保留 Vite SPA 形态
- 不做渐进双栈兼容层（strangler）
- 不改 monorepo 结构，不动后端与 `packages/sfu-client`
- 不在迁移过程中夹带 IM P1 之外的任何功能性改动

## 2. 技术栈映射

| 现在（Solid） | 迁移后（React 19） | 说明 |
|---|---|---|
| solid-js | react 19 | 函数组件 + hooks |
| vite-plugin-solid | @vitejs/plugin-react | rolldown-vite、Tailwind 4 不动 |
| @tanstack/solid-router | @tanstack/react-router | 文件路由 / routeTree 机制一致 |
| @tanstack/solid-query | @tanstack/react-query | API 近似同构 |
| @tanstack/solid-form | react-hook-form + zod | 表单场景简单，不引入 @tanstack/react-form |
| @tanstack/solid-virtual | @tanstack/react-virtual | 消息列表虚拟滚动 |
| lucide-solid | lucide-react | 全量机械换 import |
| cui-solid（残留 2 处） | shadcn/ui 对应组件 | 顺手清掉 |
| solid-notifications | sonner | toast |
| daisyUI | 移除，shadcn/ui + 自有主题 token | 见 §3 |
| 自定义 Solid store ×7 | zustand | 见 §4 |
| jsdom + vitest | vitest + @testing-library/react | 现有 `*.test.ts` 随模块重写 |

## 3. 主题保真策略与视觉基线

### 主题层

- shadcn/ui 的 CSS variables 主题机制承载现有风格：**从当前 daisyUI 主题提取全部 token 值**（亮/暗两套：`--background` / `--primary` / `--radius` 等），写入 `packages/components` 的 `theme.css`，Radix 组件壳按现有样式微调（圆角、边框、hover 态对齐现状）
- daisyUI 语义类（`btn` / `modal` / `dropdown` …）不逐一翻译；重写各页面时用 shadcn 对应组件替换，视觉以基线截图为准绳，不凭记忆
- Tailwind 4 `@theme` 配置保留，工具类体系不变

### 视觉基线（阶段 0，第一优先级）

- 在 Solid 现状上用 Playwright（已有 devDependency）逐路由录制截图基线：
  - 路由覆盖：login / register / guest、home 房间列表、房间详情（含成员侧栏）、聊天页（会话列表 + 窗口 + 私聊）、discover、domain 管理、manage 各管理页、profile、settings 各 tab
  - 亮/暗两套；关键交互态（dropdown 展开、modal 打开、speaking 高亮）单独捕获
- 基线截图 commit 进仓库 `app/web/visual-baseline/`，附 `routes.manifest.json`（路由 × 状态 × 测试账号 × 操作脚本）
- **验收门**：每个完成的页面/组件跑同脚本出图，与基线并排 diff（像素 diff + 视觉审查代理把关），不达标不算完成；允许的偏差只能是明确记录过的有意变更

## 4. 状态层设计

现状：7 个 `createRoot` store，socketStore 既是状态又是 WS 事件中枢，Solid 响应式与业务逻辑纠缠。

- **框架无关层复用**：`socket/` 下的 wsClient / roomState / providerReload / tabLock 本来就基本框架无关，保持纯 TS 原样或微调复用；voice session 的 adapter 模式（`providers.ts`）同理
- **store 拆两层**：业务逻辑（纯 TS 服务）+ zustand store（状态壳），React 组件只订阅 zustand
- 选 zustand 的理由：store API 与现有消费形态最接近，迁移心智负担最小，且 store 逻辑可在无 React 环境下单测
- 声音链路（`handler_audio/`、`useVoiceSession`）hook 壳重写为 React hooks，`packages/sfu-client` 不动

## 5. 通用组件包 `packages/components`

- 新建 pnpm workspace 包（与 `packages/sfu-client` 同模式），`app/web` 以 `workspace:*` 消费
- 内容：shadcn/ui 基件 + 业务通用组件（Avatar / Modal / Toast / ProviderIcon / audioControl 通用壳 / ManageDataTable 等）
- 主题 token 随包走；Storybook `preview` 挂主题装饰器，亮/暗可切
- **Storybook 承接组件级视觉验收**：每个组件一个 story（含交互态），Storybook 截图作为组件基线；页面级基线由 Playwright 在 `app/web` 承担，两层验收互补
- turbo 编排新增该包的 build / lint / storybook 任务；biome 覆盖该包

## 6. 分支策略、重写顺序与 cutover

**分支**：从 dev-2.0（P1 落地后）拉长分支 `react-2.0`，在 `app/web` 原地换栈；主干 P1 之后不再动前端（后端可继续开发），定期 rebase 主干进长分支（API 层与框架无关，无 UI 冲突面）。

| 阶段 | 内容 | 验收 |
|---|---|---|
| 0 | 视觉基线录制（§3） | 基线入库 + manifest |
| 1 | 脚手架：React 19 + plugin-react + TanStack React 全家桶 + zustand + shadcn init + `packages/components` 包 + 主题 token 落地 | dev server 起得来，空壳路由树，Storybook 起得来 |
| 2 | 框架无关核心层复用：`api/`、`socket/`、voice session services | 逻辑层单测过 |
| 3 | zustand store ×7 重写（socketStore 最重：WS 事件绑定 + 房间状态机） | socket 事件流单测 |
| 4 | 通用组件：shadcn 基件 + `common/` 映射迁入 `packages/components`，逐组件 story | Storybook 截图与基线比对 |
| 5 | 页面模块按依赖序重写，每模块过视觉门：① login/register/guest → ② home 房间列表 → ③ 房间详情 + 语音会话（最重，room-voice-e2e 验媒体链路）→ ④ 聊天/会话页 → ⑤ discover/domain → ⑥ manage 九表（ManageDataTable 用 @tanstack/react-table 重写）→ ⑦ profile/settings | 逐模块截图 diff |
| 6 | 端到端回归：room-voice-e2e 全场景、test/ API 集成（API 层不受影响）、web 单测重写补齐 | 全绿 |
| 7 | cutover：一次性切换入口，删除 solid 全部依赖与残留代码，更新 AGENTS.md / turbo / CI / biome | 构建产物干净，无 solid 残留 |

**回滚保险**：cutover 前旧前端完整存在于主干；切换后出问题 revert 单个 commit 即可回到 Solid 版本。

## 7. 测试与验收总览

| 层 | 手段 |
|---|---|
| 组件视觉 | Storybook 截图 vs 组件基线 |
| 页面视觉 | Playwright 截图 vs 页面基线（亮/暗） |
| 逻辑层 | vitest：store 事件流、wsClient、纯 TS 服务 |
| 组件交互 | @testing-library/react |
| 媒体链路 | room-voice-e2e（join / 切房 / 推拉流 / 多人音频） |
| API 契约 | test/ 集成测试（不受迁移影响，迁移前后必须同等全绿） |
| 静态检查 | biome + tsc --noEmit + go vet 不回归 |

## 8. 风险与对策

| 风险 | 对策 |
|---|---|
| 全量重写周期长、中途偏离 | 阶段化里程碑 + 每阶段基线验收门；偏差早暴露 |
| socketStore 状态机重写出错 | 逻辑层先于 UI 重写并有独立单测；voice e2e 兜底 |
| 视觉漂移 | 基线先行 + 双层视觉验收 + 视觉审查代理把关 |
| 长分支与主干漂移 | 后端先行期定期 rebase；前端冻结进长分支 |
| cutover 后发现严重问题 | revert 单 commit 回滚到 Solid 版本 |
