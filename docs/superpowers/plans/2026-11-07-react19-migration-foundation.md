# React 19 迁移 — 基座计划（阶段 0–4）实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

> **历史档案（2026-11-07 迁移基座已落地）**：本文是迁移实施计划，文中的 SolidJS 现状、`_solid-legacy/` 暂存步骤与待办勾选均属当时语境，不代表当前状态。当前实现以仓库根 `AGENTS.md`、`app/web/docs/AGENTS.md` 与 `app/web/README.md` 为准。

**目标：** 为 React 19 全量迁移打好基座：视觉基线入库、`react-2.0` 分支脚手架（React 19 + TanStack React 全家桶 + zustand + 主题 token）、核心层复用、7 个 store 的 zustand 重写、`packages/components` + Storybook。

**架构：** 规格见 `docs/superpowers/specs/2026-11-07-react19-migration-design.md`。长分支 `react-2.0` 在 `app/web` 原地换栈；框架无关核心层（`api/`、`socket/`）原样复用；store 拆「纯 TS 服务 + zustand 状态壳」；通用组件独立 `packages/components`（源码直消费，同 `@gospeak/sfu-client` 模式）+ Storybook 组件级视觉验收。

**技术栈：** React 19、@vitejs/plugin-react、@tanstack/react-router + react-query + react-virtual、react-hook-form + zod、zustand、sonner、lucide-react、shadcn/ui（Radix）、Tailwind 4（去 daisyUI）、Vitest + @testing-library/react、Storybook 8（react-vite）。

**本计划不含：** 阶段 5 页面模块重写、阶段 6 e2e 回归、阶段 7 cutover——基座定型后另立计划。

**执行前置：**
- IM P1 落地后才执行本计划（用户已拍板顺序）；若提前执行，任务 1 的 chat 相关基线路由要在 P1 落地后重录（manifest 中已标注）
- 起服务前先探测复用已运行实例（前端 3000/3002、后端 8998），不要新开实例
- 仓库可能有并发会话，动 git 前先看 `git status`

---

## 文件结构总览

```
GOSpeak/
├── packages/components/                  # 新建：通用 UI 组件包
│   ├── package.json                      # @gospeak/components，源码直消费（同 sfu-client）
│   ├── tsconfig.json
│   ├── .storybook/                       # main.ts / preview.tsx（主题装饰器，亮暗可切）
│   └── src/
│       ├── theme.css                     # daisyUI token → shadcn CSS vars 映射（亮/暗）
│       ├── index.ts                      # 导出桶
│       └── components/
│           ├── button.tsx                # shadcn 基件
│           ├── avatar.tsx
│           ├── dialog.tsx
│           ├── input.tsx
│           ├── toast.tsx                 # sonner 封装
│           └── *.stories.tsx             # 每组件一个 story（含交互态）
├── app/web/
│   ├── visual-baseline/                  # 新建：页面级视觉基线
│   │   ├── capture.mjs                   # Playwright 截图脚本
│   │   ├── compare.mjs                   # 像素 diff 脚本
│   │   └── routes.manifest.json          # 路由 × 状态 × 账号 × P1 后重录标注
│   ├── package.json                      # 依赖换栈
│   ├── vite.config.ts                    # 去 solid，换 react
│   ├── tsconfig.json                     # jsx: react-jsx
│   └── src/
│       ├── main.tsx                      # React 入口（重写）
│       ├── styles.css                    # 去 daisyUI plugin，@import components/theme.css
│       ├── stores/
│       │   ├── userStore.ts              # zustand 重写（逻辑 1:1 移植）
│       │   ├── socketStore.ts            # zustand 重写（最重）
│       │   ├── chatStore.ts / domainStore.ts / audioDeviceStore.ts
│       │   ├── voiceChatStore.ts / themeStore.ts / guestStore.ts
│       │   └── speakingStore.ts          # handler_audio/speakingStore.ts 迁入，zustand 化
│       ├── socket/
│       │   ├── wsClient.ts               # 原样保留（框架无关）
│       │   ├── roomState.ts / tabLock.ts / providerReload.ts   # 原样保留
│       │   └── socketEvents.ts           # 去 solid 化（改 zustand 或纯回调）
│       └── api/                          # 原样保留（框架无关）
```

---

### 任务 1：视觉基线录制（在 dev-2.0 上执行，先于分支创建）

**文件：**
- 创建：`app/web/visual-baseline/capture.mjs`
- 创建：`app/web/visual-baseline/compare.mjs`
- 创建：`app/web/visual-baseline/routes.manifest.json`
- 创建：`app/web/visual-baseline/baseline/**/*.png`（脚本产物，全部入库）

- [ ] **步骤 1：确认服务可复用或启动**

```bash
curl -sf http://localhost:8998/ping || (cd app/server && pnpm dev) &
curl -sf http://localhost:3000 -o /dev/null || (cd app/web && pnpm dev) &
# 等待就绪后继续；基线必须录自当前 Solid 真实渲染结果
```

- [ ] **步骤 2：写 `routes.manifest.json`**

```json
{
  "baseUrl": "http://localhost:3000",
  "accounts": { "admin": { "name": "admin", "password": "ASK_USER_FOR_PASSWORD" } },
  "note": "chat/discover/domain 等受 IM P1 影响的路由标注 rescheduleAfterP1，P1 落地后重录",
  "routes": [
    { "id": "login",    "path": "/login",    "auth": false, "themes": ["light", "dark"] },
    { "id": "register", "path": "/register", "auth": false, "themes": ["light", "dark"] },
    { "id": "guest",    "path": "/guest",    "auth": false, "themes": ["light", "dark"] },
    { "id": "home",        "path": "/",      "auth": true,  "themes": ["light", "dark"] },
    { "id": "room-detail", "path": "/",      "auth": true,  "themes": ["light", "dark"],
      "actions": ["openFirstRoom"] },
    { "id": "chat",        "path": "/chat",  "auth": true,  "themes": ["light", "dark"], "rescheduleAfterP1": true },
    { "id": "discover",    "path": "/discover", "auth": true, "themes": ["light", "dark"] },
    { "id": "domain",      "path": "/domain",   "auth": true, "themes": ["light", "dark"] },
    { "id": "manage-users",   "path": "/manage", "auth": true, "themes": ["light", "dark"] },
    { "id": "manage-roles",   "path": "/manage", "auth": true, "themes": ["light", "dark"],
      "actions": ["clickTab:roles"] },
    { "id": "profile",     "path": "/profile",  "auth": true, "themes": ["light", "dark"] }
  ]
}
```

（执行时按 `app/web/src/pages` 实际路由核对路径与 actions；admin 密码非默认，向用户索取或从本地 dev 环境读取。）

- [ ] **步骤 3：写 `capture.mjs`**

```js
// app/web/visual-baseline/capture.mjs
import { chromium } from "playwright";
import { mkdir, readFile, writeFile } from "node:fs/promises";

const manifest = JSON.parse(await readFile(new URL("./routes.manifest.json", import.meta.url)));
const OUT = new URL("./baseline/", import.meta.url);
await mkdir(OUT, { recursive: true });

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 }, deviceScaleFactor: 2 });
const page = await ctx.newPage();

const THEMES = { light: "[data-theme=light]", dark: "[data-theme=dark]" };

async function setTheme(theme) {
  await page.evaluate((sel) => {
    document.documentElement.setAttribute("data-theme", sel === "[data-theme=dark]" ? "dark" : "light");
    localStorage.setItem("theme", sel === "[data-theme=dark]" ? "dark" : "light");
  }, THEMES[theme]);
}

async function login(name, password) {
  await page.goto(`${manifest.baseUrl}/login`);
  await page.getByPlaceholder(/用户名|username/i).fill(name);
  await page.locator("input[type=password]").fill(password);
  await page.locator("button[type=submit]").click();
  await page.waitForURL((u) => !u.pathname.startsWith("/login"), { timeout: 10_000 });
}

const actions = {
  openFirstRoom: async () => {
    await page.locator("[data-room-item], .room-item").first().click();
    await page.waitForTimeout(1500);
  },
  "clickTab:roles": async () => {
    await page.getByRole("tab", { name: /role|角色/i }).click();
    await page.waitForTimeout(500);
  },
};

async function run() {
  const authed = manifest.routes.some((r) => r.auth);
  if (authed) await login(manifest.accounts.admin.name, manifest.accounts.admin.password);
  for (const route of manifest.routes) {
    if (route.rescheduleAfterP1) continue; // P1 落地后重录
    for (const theme of route.themes) {
      await page.goto(`${manifest.baseUrl}${route.path}`, { waitUntil: "networkidle" });
      await setTheme(theme);
      await page.waitForTimeout(400);
      for (const action of route.actions ?? []) await actions[action]();
      await page.screenshot({ path: new URL(`${route.id}-${theme}.png`, OUT).pathname, fullPage: true });
      console.log(`captured ${route.id}-${theme}.png`);
    }
  }
  await writeFile(new URL(".last-run", OUT), new Date().toISOString());
  await browser.close();
}
run();
```

（选择器执行时以真实 DOM 为准修正；无 data-room-item 时改用文本选择器。）

- [ ] **步骤 4：跑脚本产出基线**

```bash
cd app/web && node visual-baseline/capture.mjs
ls visual-baseline/baseline/*.png | wc -l   # ≥ 2×未标 P1 的路由数
```

预期：每路由亮/暗两张，人工抽查确认无登录失效页/空白页混入。

- [ ] **步骤 5：写 `compare.mjs`（验收门工具，后续每模块用）**

```js
// app/web/visual-baseline/compare.mjs — 用法：node compare.mjs <routeId> <actual.png>
import { PNG } from "pngjs";
import { readFile } from "node:fs/promises";
import pixelmatch from "pixelmatch";

const [routeId, actualPath] = process.argv.slice(2);
const a = PNG.sync.read(await readFile(new URL(`./baseline/${routeId}-light.png`, import.meta.url)));
const b = PNG.sync.read(await readFile(actualPath));
const { width, height } = { width: Math.min(a.width, b.width), height: Math.min(a.height, b.height) };
const diff = new PNG({ width, height });
const mismatched = pixelmatch(
  stripAlpha(a), stripAlpha(b), diff.data, width, height, { threshold: 0.1 },
);
console.log(JSON.stringify({ routeId, mismatched, total: width * height, ratio: +(mismatched / (width * height)).toFixed(4) }));
function stripAlpha(png) {
  const out = Buffer.alloc(png.width * png.height * 4);
  for (let y = 0; y < height; y++) out.set(png.data.subarray(y * png.width * 4, y * png.width * 4 + width * 4), y * width * 4);
  return out;
}
```

（`pnpm add -D pngjs pixelmatch` 到 app/web devDependencies。尺寸不一致时裁到交集并输出 ratio，判定阈值：ratio < 0.02 通过，0.02–0.05 交视觉审查代理判定，> 0.05 不通过。）

- [ ] **步骤 6：Commit**

```bash
git add app/web/visual-baseline
git commit -m "chore: capture visual baseline for React migration"
```

---

### 任务 2：`react-2.0` 分支 + `@gospeak/components` 包骨架

**文件：**
- 创建：`packages/components/package.json`
- 创建：`packages/components/tsconfig.json`
- 创建：`packages/components/src/index.ts`
- 创建：`packages/components/src/components/.gitkeep`

- [ ] **步骤 1：建分支**

```bash
cd /Users/noelorin/GOSpeak && git status --short   # 必须干净
git checkout -b react-2.0
```

- [ ] **步骤 2：创建包（镜像 sfu-client 的源码直消费模式）**

`packages/components/package.json`：

```json
{
  "name": "@gospeak/components",
  "version": "1.0.0",
  "private": true,
  "type": "module",
  "main": "./src/index.ts",
  "module": "./src/index.ts",
  "types": "./src/index.ts",
  "exports": {
    ".": { "types": "./src/index.ts", "import": "./src/index.ts" },
    "./theme.css": "./src/theme.css"
  },
  "scripts": {
    "build": "tsc",
    "lint": "biome check",
    "dev": "tsc --watch",
    "test": "vitest run",
    "storybook": "storybook dev -p 6006",
    "build-storybook": "storybook build"
  },
  "dependencies": {
    "@radix-ui/react-avatar": "^1.1.10",
    "@radix-ui/react-dialog": "^1.1.15",
    "@radix-ui/react-slot": "^1.2.3",
    "class-variance-authority": "^0.7.1",
    "clsx": "^2.1.1",
    "lucide-react": "^0.561.0",
    "sonner": "^2.0.7",
    "tailwind-merge": "^3.3.1"
  },
  "peerDependencies": {
    "react": "^19.0.0",
    "react-dom": "^19.0.0"
  },
  "devDependencies": {
    "@storybook/react-vite": "^8.6.14",
    "@types/react": "^19.2.0",
    "@types/react-dom": "^19.2.0",
    "storybook": "^8.6.14",
    "tailwindcss": "^4.0.6",
    "typescript": "^5.9.3",
    "vitest": "^4.1.9"
  }
}
```

`packages/components/tsconfig.json`：

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ESNext",
    "moduleResolution": "bundler",
    "jsx": "react-jsx",
    "strict": true,
    "skipLibCheck": true,
    "noEmit": true,
    "baseUrl": ".",
    "paths": { "@/*": ["./src/*"] }
  },
  "include": ["src", ".storybook"]
}
```

`packages/components/src/index.ts`：

```ts
export {};
```

- [ ] **步骤 3：安装并验证构建**

```bash
pnpm install
cd packages/components && pnpm build && pnpm lint
```

预期：tsc 无错误（空导出）、biome 通过。

- [ ] **步骤 4：Commit**

```bash
git add packages/components pnpm-lock.yaml
git commit -m "feat: add @gospeak/components workspace package skeleton"
```

---

### 任务 3：`app/web` 换栈脚手架

**文件：**
- 修改：`app/web/package.json`（依赖整体替换）
- 修改：`app/web/vite.config.ts`（重写）
- 修改：`app/web/tsconfig.json`（jsx 改 react-jsx）
- 修改：`app/web/src/styles.css`（去 daisyUI/solid-notifications，引 components/theme.css）
- 创建：`app/web/src/theme.ts`（亮暗切换写 `<html data-theme>`，供 React 入口用）
- 修改：`app/web/src/main.tsx`（React 入口 + 空壳路由树）
- 删除：`app/web/src/routeTree.gen.ts`（router 插件会重新生成 React 版）、`app/web/scripts/cui-icons-plugin.mjs`、`app/web/scripts/gen-icon-shims.mjs`、`app/web/vendor/`（cui 残留）
- 暂存移除：`app/web/src/pages/**`、`components/**`、`layouts/**`（`git mv` 到 `_solid-legacy/` 暂存目录，页面计划逐模块回迁后最终删除；保证分支上任意 commit 可运行）

**注意：** 换栈后 `app/web` 即不可运行的中间态会破坏「分支任意 commit 可运行」——因此本任务把旧前端整体挪进 `_solid-legacy/` 且暂不接路由，新前端空壳独立可跑；旧代码仅作移植参考，阶段 5 完成后随 cutover 删除。

- [ ] **步骤 1：改 `app/web/package.json` 依赖**

dependencies 移除：`solid-js`、`solid-notifications`、`lucide-solid`、`cui-solid`、`cui-solid-icons`、`@tanstack/solid-form`、`@tanstack/solid-query`、`@tanstack/solid-query-devtools`、`@tanstack/solid-router`、`@tanstack/solid-router-devtools`、`@tanstack/solid-virtual`、`@gospeak/sfu-client` 保留。
dependencies 新增：

```json
{
  "@gospeak/components": "workspace:*",
  "@tanstack/react-query": "^5.90.15",
  "@tanstack/react-query-devtools": "^5.91.1",
  "@tanstack/react-router": "^1.133.20",
  "@tanstack/react-router-devtools": "^1.133.20",
  "@tanstack/react-virtual": "^3.13.35",
  "react": "^19.2.0",
  "react-dom": "^19.2.0",
  "react-hook-form": "^7.65.0",
  "@hookform/resolvers": "^5.2.2",
  "zustand": "^5.0.8"
}
```

devDependencies 移除：`vite-plugin-solid`、`@tanstack/router-cli`（`tsr watch` 不再需要）、`solid` 相关；新增：

```json
{
  "@vitejs/plugin-react": "^5.1.1",
  "@testing-library/react": "^16.3.0",
  "@testing-library/user-event": "^14.6.1",
  "pngjs": "^7.0.0",
  "pixelmatch": "^7.0.0"
}
```

保留：`daisyui` 先不删（基线页面对照期间 legacy 目录还要用）——注意：`_solid-legacy/` 不参与构建，`daisyui` 从 `styles.css` 解除引用后即可移除；若 legacy 对照需要，临时保留至 cutover。

- [ ] **步骤 2：重写 `vite.config.ts`（保留端口/代理/分包骨架，换插件）**

```ts
import { defineConfig } from "vite";
import { devtools } from "@tanstack/devtools-vite";
import { tanstackRouter } from "@tanstack/router-plugin/vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import path from "node:path";
import net from "node:net";

function findAvailablePort(startPort: number): Promise<number> {
  if (startPort >= 65536) return Promise.resolve(42069);
  return new Promise((resolve) => {
    const server = net.createServer();
    server.listen(startPort, () => {
      server.close(() => resolve(startPort));
    });
    server.on("error", () => resolve(findAvailablePort(startPort + 1)));
  });
}

export default defineConfig(async ({ mode }) => {
  const isProduction = mode === "production";
  const devtoolsPort = isProduction ? 42069 : await findAvailablePort(42069);
  return {
    base: "/",
    plugins: [
      devtools({ eventBusConfig: { port: devtoolsPort } }),
      tanstackRouter({
        target: "react",
        autoCodeSplitting: true,
        routesDirectory: "./src/pages",
        generatedRouteTree: "./src/routeTree.gen.ts",
        quoteStyle: "single",
        semicolons: false,
        routeFileIgnorePrefix: "-",
        routeFileIgnorePattern: "components",
      }),
      react(),
      tailwindcss(),
    ],
    server: {
      port: 3000,
      strictPort: false,
      open: true,
      host: "0.0.0.0",
      proxy: {
        "/api/v1": { target: "http://localhost:8998", changeOrigin: true, ws: true },
        "/ws": { target: "http://localhost:8998", changeOrigin: true, ws: true },
        "/rtc/v1": {
          target: "http://localhost:1985",
          changeOrigin: true,
          configure: (proxy) => {
            proxy.on("proxyReq", (proxyReq) => proxyReq.setHeader("Connection", "close"));
          },
        },
      },
      watch: {
        ignored: ["!**/packages/sfu-client/src/**", "!**/packages/components/src/**"],
      },
    },
    resolve: {
      alias: {
        "@": path.resolve(__dirname, "./src"),
        "#": path.resolve(__dirname, "./types"),
      },
    },
    optimizeDeps: {
      exclude: ["fsevents", "@gospeak/sfu-client", "@gospeak/components"],
      rolldownOptions: { define: { global: "globalThis" }, treeshake: true },
    },
    esbuild: {
      drop: isProduction ? ["console", "debugger"] : [],
      legalComments: "inline",
    },
    build: {
      target: "es2022",
      cssMinify: isProduction ? "lightningcss" : false,
      minify: isProduction ? "esbuild" : false,
      sourcemap: isProduction ? false : "inline",
      cssCodeSplit: true,
      reportCompressedSize: false,
      chunkSizeWarningLimit: 500,
      rollupOptions: {
        output: {
          manualChunks: (id: string) => {
            const chunkGroups = {
              vendor: ["/react/", "/react-dom/"],
              router: ["@tanstack/react-router"],
              ui: ["@radix-ui/", "clsx", "sonner", "lucide-react"],
            };
            if (id.includes("node_modules")) {
              for (const [chunkName, deps] of Object.entries(chunkGroups)) {
                if (deps.some((dep) => id.includes(dep))) return chunkName;
              }
            }
            return undefined;
          },
          assetFileNames: (a: { names?: string[] }) =>
            (a.names?.[0] ?? "").endsWith(".css") ? "assets/[name].[hash].css" : "assets/[name].[hash].[ext]",
          chunkFileNames: "chunks/[name].[hash].js",
          entryFileNames: "entries/[name].[hash].js",
        },
        input: { index: "index.html" },
      },
    },
  };
});
```

- [ ] **步骤 3：`tsconfig.json` 换 JSX**

`compilerOptions` 中 `"jsx": "preserve"` → `"jsx": "react-jsx"`；`"types"` 若含 solid 相关则移除；新增 `"lib": ["ES2022", "DOM", "DOM.Iterable"]` 保持不变即可。

- [ ] **步骤 4：主题 token 落地（daisyUI → shadcn vars）**

先在浏览器 devtools（或临时脚本 `document.createElement` 探测）从当前 daisyUI acid 主题读取实际值：`--color-base-100/200/300`、`--color-base-content`、`--color-primary` 及 `--color-primary-content`、secondary/accent/neutral/error/success/warning/info 同理、`--radius-selector/field/box`。写入 `packages/components/src/theme.css`（abys̶s 暗色同理，从 abyss 主题读值）：

```css
/* packages/components/src/theme.css — 值来自 daisyUI acid / abyss 实测，勿凭记忆填色 */
:root {
  --background: <base-100 值>;
  --foreground: <base-content 值>;
  --card: <base-100 值>;
  --card-foreground: <base-content 值>;
  --primary: <primary 值>;
  --primary-foreground: <primary-content 值>;
  --secondary: <secondary 值>;
  --secondary-foreground: <secondary-content 值>;
  --muted: <base-200 值>;
  --muted-foreground: <base-content 值 60% 不透明度>;
  --accent: <accent 值>;
  --accent-foreground: <accent-content 值>;
  --destructive: <error 值>;
  --destructive-foreground: <error-content 值>;
  --border: <base-300 值>;
  --input: <base-300 值>;
  --ring: <primary 值>;
  --radius: <radius-box 值>;
}
.dark {
  /* 同结构，值取自 abyss 主题实测 */
}
@theme inline {
  --color-background: var(--background);
  --color-foreground: var(--foreground);
  --color-primary: var(--primary);
  --color-primary-foreground: var(--primary-foreground);
  --color-secondary: var(--secondary);
  --color-secondary-foreground: var(--secondary-foreground);
  --color-muted: var(--muted);
  --color-muted-foreground: var(--muted-foreground);
  --color-accent: var(--accent);
  --color-accent-foreground: var(--accent-foreground);
  --color-destructive: var(--destructive);
  --color-border: var(--border);
  --color-input: var(--input);
  --color-ring: var(--ring);
  --radius-box: var(--radius);
  --radius-field: <radius-field 值>;
}
```

（`<...>` 占位在执行时用实测值替换后此文件不得再含占位符；对照物是基线截图，不是 daisyUI 文档。）

- [ ] **步骤 5：`styles.css` 与 `main.tsx` 空壳**

`src/styles.css`：

```css
@import "tailwindcss";
@import "@gospeak/components/theme.css";
@import "./assets/styles/global.css";
```

`src/main.tsx`：

```tsx
import { createRoot } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider, createRouter } from "@tanstack/react-router";
import { routeTree } from "./routeTree.gen";
import "./styles.css";

const queryClient = new QueryClient();
const router = createRouter({ routeTree, context: { queryClient } });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}

createRoot(document.getElementById("root")!).render(
  <QueryClientProvider client={queryClient}>
    <RouterProvider router={router} />
  </QueryClientProvider>,
);
```

`src/pages/index.tsx`（空壳首页，验证路由与主题）：

```tsx
import { createFileRoute } from "@tanstack/react-router";

export const Route = createFileRoute("/")({ component: Home });

function Home() {
  return <div className="bg-background text-foreground min-h-dvh">GOSpeak React</div>;
}
```

同时清空 `index.html` 中 solid 入口脚本引用（保留 `<div id="root">`）。

- [ ] **步骤 6：迁移文件到 `_solid-legacy/`**

```bash
cd app/web/src
mkdir -p _solid-legacy
git mv pages components layouts handler_audio _solid-legacy/   # 暂存，供移植对照
git mv routeTree.gen.ts _solid-legacy/
```

（`stores/`、`socket/`、`api/` 留在原位，后续任务原地重写/复用。`_solid-legacy/` 加入 biome ignore 与 tsconfig exclude。）

- [ ] **步骤 7：验证**

```bash
cd app/web && pnpm dev    # 空壳首页可打开，暗色切换正常
pnpm build && pnpm typecheck
```

预期：dev 起来、build 无 solid 引用报错。失败优先查漏删的 solid import。

- [ ] **步骤 8：Commit**

```bash
git add -A app/web packages/components
git commit -m "feat!: scaffold React 19 app shell with theme tokens"
```

---

### 任务 4：核心层复用 + 去 solid 化两个耦合点

**文件：**
- 保留不动：`app/web/src/api/**`、`app/web/src/socket/wsClient.ts`、`roomState.ts`、`tabLock.ts`、`providerReload.ts`、`types.ts`、`events.ts`、`protocol/`、`utils/`
- 修改：`app/web/src/socket/socketEvents.ts`（去 solid-js 依赖）
- 创建：`app/web/src/stores/speakingStore.ts`（由 `handler_audio/speakingStore.ts` 移植，zustand 化；`_solid-legacy/handler_audio/` 删除）
- 测试：`app/web/src/socket/socketEvents.test.ts`（已有，随改随修）、`app/web/src/stores/speakingStore.test.ts`

- [ ] **步骤 1：检查 `socketEvents.ts` 的 solid 用点**

读 `_solid-legacy/../socket/socketEvents.ts`（273 行）中所有 `from "solid-js"` import（预计是 `createSignal` 做 speaking/在线状态缓存）。若仅为内部信号：改为 zustand：

```ts
// 模式示例（以实际代码为准逐个替换 createSignal）
import { create } from "zustand";

interface SocketEventsState {
  speaking: Record<string, boolean>;
  setSpeaking: (identity: string, on: boolean) => void;
}

export const useSocketEventsState = create<SocketEventsState>((set) => ({
  speaking: {},
  setSpeaking: (identity, on) =>
    set((s) => ({ speaking: { ...s.speaking, [identity]: on } })),
}));
```

- [ ] **步骤 2：写 `speakingStore.test.ts`（先失败）**

```ts
import { describe, expect, it } from "vitest";
import { speakingStore } from "./speakingStore";

describe("speakingStore", () => {
  it("setSpeaking 切换单个身份", () => {
    speakingStore.getState().setSpeaking("u1", true);
    expect(speakingStore.getState().speaking["u1"]).toBe(true);
    speakingStore.getState().setSpeaking("u1", false);
    expect(speakingStore.getState().speaking["u1"]).toBe(false);
  });
  it("clear 移除离开成员的发言态", () => {
    speakingStore.getState().setSpeaking("u2", true);
    speakingStore.getState().clear(["u2"]);
    expect(speakingStore.getState().speaking["u2"]).toBeUndefined();
  });
});
```

（以 `_solid-legacy/handler_audio/speakingStore.ts` 的实际 API 为准对齐用例名；上面未定义的 `clear` 若源里叫别的名字，跟随源命名。）

- [ ] **步骤 3：跑测试确认失败 → 实现 → 通过**

```bash
cd app/web && pnpm vitest run src/stores/speakingStore.test.ts
```

预期：先 FAIL（模块不存在），实现后 PASS。

- [ ] **步骤 4：全量单测回归**

```bash
pnpm vitest run
```

预期：`socket/`、`api/` 相关既有测试全绿（这些文件未动，若红说明误改）。

- [ ] **步骤 5：Commit**

```bash
git add -A app/web/src
git commit -m "refactor: decouple socketEvents from solid, port speakingStore to zustand"
```

---

### 任务 5：zustand `userStore`（1:1 逻辑移植）

**文件：**
- 修改：`app/web/src/stores/userStore.ts`（重写，逻辑照搬现有 170 行）
- 测试：`app/web/src/stores/userStore.test.ts`（已有 211 行，断言不变，改 zustand 消费方式）

- [ ] **步骤 1：重写 `userStore.ts`**

```ts
import { create } from "zustand";
import { getProfile, logout as logoutApi } from "@/api/auth";
import {
  readSessionExpiry,
  recordSessionExpiry as persistSessionExpiry,
  refreshSession,
} from "@/api/authTransport";

export interface UserInfo {
  id: number;
  uuid: string;
  name: string;
  display_name: string;
  avatar: string;
  role: string;
  is_guest?: boolean;
  permissions?: string[];
}

const STORAGE_USER = "user";
const EXPIRE_MARGIN_MS = 60_000;
const UNVERIFIED_GRACE_MS = 10 * 60_000;

interface UserState {
  user: UserInfo | null;
  userStale: boolean;
  login: (u: UserInfo, expiresIn?: number) => void;
  fetchProfile: () => Promise<boolean>;
  clearAuth: () => void;
  logout: () => Promise<void>;
  ensureSession: () => Promise<boolean>;
  recordSessionExpiry: (expiresIn: number | null | undefined) => void;
}

let ensureSessionPromise: Promise<boolean> | null = null;
let sessionExpiresAt: number | null = readSessionExpiry();
let lastVerifiedAt = 0;

function isRateLimitedError(e: unknown): boolean {
  const code = (e as { response?: { data?: { code?: number } } })?.response?.data?.code;
  return code === 1017;
}

export const useUserStore = create<UserState>((set, get) => ({
  user: (() => {
    try {
      const cached = localStorage.getItem(STORAGE_USER);
      return cached ? (JSON.parse(cached) as UserInfo) : null;
    } catch {
      localStorage.removeItem(STORAGE_USER);
      return null;
    }
  })(),
  userStale: false,

  login: (u, expiresIn) => {
    localStorage.setItem(STORAGE_USER, JSON.stringify(u));
    persistSessionExpiry(expiresIn);
    sessionExpiresAt = readSessionExpiry();
    set({ user: u, userStale: false });
  },

  fetchProfile: async () => {
    try {
      const profile = await getProfile();
      const updated = { ...get().user, ...profile } as UserInfo;
      localStorage.setItem(STORAGE_USER, JSON.stringify(updated));
      set({ user: updated, userStale: false });
      return true;
    } catch {
      set({ userStale: true });
      return false;
    }
  },

  clearAuth: () => {
    localStorage.removeItem(STORAGE_USER);
    persistSessionExpiry(null);
    sessionExpiresAt = null;
    lastVerifiedAt = 0;
    set({ user: null, userStale: false });
  },

  logout: async () => {
    try {
      await logoutApi();
    } catch {
      // 即使服务端登出失败，也继续清除本地状态
    }
    get().clearAuth();
  },

  recordSessionExpiry: (expiresIn) => {
    persistSessionExpiry(expiresIn);
    sessionExpiresAt = readSessionExpiry();
  },

  /**
   * 确保会话可用（过期时间驱动的懒续期），逻辑与 Solid 版一致：
   * 快速放行 / 临近过期 refresh / 宽限窗口 / 冷启动探测 / 1017 不清缓存。
   * 注意：快速路径同步完成时 in-flight promise 清理必须挂外层 .finally。
   */
  ensureSession: () => {
    if (ensureSessionPromise) return ensureSessionPromise;
    const attempt = (async () => {
      try {
        sessionExpiresAt = readSessionExpiry();
        const { user } = get();
        if (user) {
          if (sessionExpiresAt) {
            if (Date.now() < sessionExpiresAt - EXPIRE_MARGIN_MS) return true;
            get().recordSessionExpiry(await refreshSession());
            const ok = await get().fetchProfile();
            if (ok) lastVerifiedAt = Date.now();
            return ok;
          }
          if (lastVerifiedAt && Date.now() - lastVerifiedAt < UNVERIFIED_GRACE_MS) return true;
        }
        if (await get().fetchProfile()) {
          lastVerifiedAt = Date.now();
          if (!sessionExpiresAt) {
            void refreshSession().then(() => { sessionExpiresAt = readSessionExpiry(); }).catch(() => {});
          }
          return true;
        }
        get().recordSessionExpiry(await refreshSession());
        const ok = await get().fetchProfile();
        if (ok) lastVerifiedAt = Date.now();
        return ok;
      } catch (e) {
        if (isRateLimitedError(e) && get().user) return true;
        get().clearAuth();
        return false;
      }
    })();
    const settled = attempt.finally(() => {
      if (ensureSessionPromise === settled) ensureSessionPromise = null;
    });
    ensureSessionPromise = settled;
    return settled;
  },
}));

export const userStore = {
  getUser: () => useUserStore.getState().user,
  isLoggedIn: () => !!useUserStore.getState().user,
  ensureSession: () => useUserStore.getState().ensureSession(),
};
```

- [ ] **步骤 2：移植 `userStore.test.ts`**

打开现有 `src/stores/userStore.test.ts`（211 行），把 signal 断言（`user()`）替换为 `useUserStore.getState().user` / `setState`；用例集合（懒续期四分支、1017 不清缓存、快速路径 promise 清理）一条不删。mock `@/api/auth`、`@/api/authTransport` 的方式不变。

- [ ] **步骤 3：跑测试**

```bash
cd app/web && pnpm vitest run src/stores/userStore.test.ts
```

预期：PASS，且用例数不少于移植前（`grep -c "it(" `）。

- [ ] **步骤 4：Commit**

```bash
git add app/web/src/stores/userStore.ts app/web/src/stores/userStore.test.ts
git commit -m "refactor: port userStore to zustand"
```

---

### 任务 6：zustand `socketStore`（最重）

**文件：**
- 修改：`app/web/src/stores/socketStore.ts`（重写；源文件 545 行 + `createRoot` 壳）
- 测试：`app/web/src/stores/socketStore.test.ts`（已有 171 行，用例集合不变）

**porting 规则（适用整个文件）：**

| Solid 形态 | zustand 形态 |
|---|---|
| `createSignal<T>(init)` | store 字段 `T` + setter action |
| `createEffect(() => …)` 订阅 WS 事件 | 模块级 `bindSocketEvents()` 在 `connect()` 时执行一次，事件回调直接 `set()` |
| `createRoot(() => { … })` 包裹 | 删除壳，模块作用域直接 `create()` |
| 组件侧 `socketStore.rooms()` | 组件用 `useSocketStore((s) => s.rooms)`；非组件侧用 `useSocketStore.getState()` |
| `return { rooms, members, … }` 信号桶 | `useSocketStore` + 导出 `socketStore` 命令式门面（`connect/disconnect/joinRoom/…` 方法签名不变，降低调用侧移植成本） |

- [ ] **步骤 1：读全源文件与测试，列状态清单**

读 `_solid-legacy/../stores/socketStore.ts` 全文与 `socketStore.test.ts`，在 PR 描述或 commit message 里列出全部 signal → 字段映射（`connected/connecting/rooms/currentRoom/selectedRoomInfo/activeSFUProvider/speechRestricted/speechRestrictionInfo/…`）。

- [ ] **步骤 2：写 zustand 骨架（状态 + 命令式门面）**

```ts
import { create } from "zustand";
import { createWSClient, type SignalSocket } from "@/socket/wsClient";
import { EVENTS } from "@/socket/events";
import type { RoomInfo, MemberInfo, MuteEvent } from "@/socket/types";

interface SocketState {
  connected: boolean;
  connecting: boolean;
  rooms: RoomInfo[];
  currentRoom: string | null;
  selectedRoomInfo: RoomInfo | null;
  activeSFUProvider?: string;
  speechRestricted: boolean;
  speechRestrictionInfo: MuteEvent | null;
  privateNewHandler?: (msg: unknown) => void;
  setConnected: (v: boolean) => void;
  setConnecting: (v: boolean) => void;
  setRooms: (rooms: RoomInfo[]) => void;
  setCurrentRoom: (room: string | null) => void;
  setSelectedRoomInfo: (info: RoomInfo | null) => void;
  setActiveSFUProvider: (p?: string) => void;
  setSpeechRestricted: (v: boolean, info?: MuteEvent | null) => void;
  setPrivateNewHandler: (cb?: (msg: unknown) => void) => void;
}

export const useSocketStore = create<SocketState>((set) => ({
  connected: false,
  connecting: false,
  rooms: [],
  currentRoom: null,
  selectedRoomInfo: null,
  activeSFUProvider: undefined,
  speechRestricted: false,
  speechRestrictionInfo: null,
  setConnected: (v) => set({ connected: v }),
  setConnecting: (v) => set({ connecting: v }),
  setRooms: (rooms) => set({ rooms }),
  setCurrentRoom: (room) => set({ currentRoom: room }),
  setSelectedRoomInfo: (info) => set({ selectedRoomInfo: info }),
  setActiveSFUProvider: (p) => set({ activeSFUProvider: p }),
  setSpeechRestricted: (v, info = null) => set({ speechRestricted: v, speechRestrictionInfo: info }),
  setPrivateNewHandler: (cb) => set({ privateNewHandler: cb }),
}));

let client: ReturnType<typeof createWSClient> | null = null;

export const socketStore = {
  async connect() {
    if (useSocketStore.getState().connected || useSocketStore.getState().connecting) return;
    useSocketStore.getState().setConnecting(true);
    client = createWSClient();
    bindSocketEvents(client);
    await client.connect();
    useSocketStore.getState().setConnecting(false);
    useSocketStore.getState().setConnected(true);
  },
  // disconnect / createRoom / joinRoom / joinRoomSFU / leaveRoom / listRooms /
  // kickMember / onActivity / onPresence / onRoomKicked —— 逐个从源文件移植，
  // 方法签名保持不变（调用侧零改动），内部 signal 读写换 useSocketStore.getState()/set。
};
```

（`bindSocketEvents` 中每个 `onServerEvent` 分支照搬源文件的 roomState 合并逻辑；`roomState.ts` 的纯函数原样复用。以上骨架中省略的方法在实现时必须逐个移植，不得留空。）

- [ ] **步骤 3：移植 `socketStore.test.ts`（171 行，用例一条不删）**

mock `createWSClient`（vi.mock 返回 fake client + 事件触发器），断言改 `useSocketStore.getState()`。重点用例：join ack 后 `currentRoom` 更新、member_joined/left 后 `rooms[].members` 合并、mute 事件置 `speechRestricted`、断线重连后状态恢复。

- [ ] **步骤 4：跑测试**

```bash
cd app/web && pnpm vitest run src/stores/socketStore.test.ts
```

预期：PASS，用例数 ≥ 移植前。

- [ ] **步骤 5：Commit**

```bash
git add app/web/src/stores/socketStore.ts app/web/src/stores/socketStore.test.ts
git commit -m "refactor: port socketStore to zustand"
```

---

### 任务 7：其余五个 store + chatStore 移植

**文件：**
- 修改：`stores/domainStore.ts`（214 行）、`stores/audioDeviceStore.ts`（248 行）、`stores/voiceChatStore.ts`（171 行）、`stores/themeStore.ts`（134 行）、`stores/chatStore.ts`（731 行）
- 测试：各自已有 `*.test.ts` / `*.spec.ts` 用例集合不变

- [ ] **步骤 1：逐 store 应用任务 6 的 porting 规则表**

顺序按依赖与体量：`themeStore` → `guestStore` → `audioDeviceStore` → `voiceChatStore` → `domainStore` → `chatStore`（最大，最后做）。每个 store 一个独立 commit。共同的转换规则：

1. `createSignal` → store 字段；`createStore/produce`（若细粒度对象）→ zustand `set` 展开更新
2. localStorage/IndexedDB（`idb-keyval`）持久化逻辑保持不变——它与框架无关，只在 store 初始化时执行
3. `themeStore` 对 `<html data-theme>` 的 DOM 写入保持不变（React 外直接操作，符合现有实现）
4. 导出形态统一：`useXxxStore`（hook）+ `xxxStore` 命令式门面（方法签名不变）

- [ ] **步骤 2：chatStore 特别说明**

731 行是最大单体，含 IndexedDB 持久化 + 会话/消息状态。移植时若超过 400 行按职责拆文件：`chatStore.ts`（状态壳）+ `chatService.ts`（纯 TS：消息去重、会话合并、未读计数），service 层先移植并直接复用现有 `chatStore.test.ts` / `chatStore.domain.test.ts` 的断言改指向 service，store 只测壳。

- [ ] **步骤 3：每 store 的验证与提交（重复六次，每次一个 commit）**

```bash
pnpm vitest run src/stores/<name>.test.ts src/stores/<name>.spec.ts 2>/dev/null
# 全绿后：
git add app/web/src/stores/<name>*.ts
git commit -m "refactor: port <name>Store to zustand"
```

- [ ] **步骤 4：store 层全量回归**

```bash
pnpm vitest run
pnpm typecheck
```

预期：全部通过；`grep -r "solid-js" src/stores src/socket src/api` 零命中。

---

### 任务 8：`packages/components` 基件 + Storybook

**文件：**
- 创建：`packages/components/.storybook/main.ts`、`preview.tsx`
- 创建：`packages/components/src/components/button.tsx`、`avatar.tsx`、`dialog.tsx`、`input.tsx`、`toast.tsx`、`card.tsx`
- 创建：对应 `*.stories.tsx` ×6
- 修改：`packages/components/src/index.ts`（导出桶）
- 创建：`packages/components/src/lib/utils.ts`（`cn()`）

- [ ] **步骤 1：Storybook 配置**

`.storybook/main.ts`：

```ts
import type { StorybookConfig } from "@storybook/react-vite";

const config: StorybookConfig = {
  stories: ["../src/**/*.stories.@(ts|tsx)"],
  addons: [],
  framework: { name: "@storybook/react-vite", options: {} },
};
export default config;
```

`.storybook/preview.tsx`：

```tsx
import type { Preview } from "@storybook/react-vite";
import "../src/theme.css";

const preview: Preview = {
  parameters: { layout: "centered" },
  globalTypes: {
    theme: { defaultValue: "light", toolbar: { items: ["light", "dark"] } },
  },
  decorators: [
    (Story, context) => (
      <div data-theme={context.globals.theme} className="bg-background text-foreground p-4 rounded">
        <Story />
      </div>
    ),
  ],
};
export default preview;
```

（preview.tsx 需在 `main.ts` 同级建 `tsconfig` 引用或让 vitest 忽略；Storybook 8 + React 19 已兼容。）

- [ ] **步骤 2：`cn()` 与基件（shadcn 标准实现，风格对齐基线）**

`src/lib/utils.ts`：

```ts
import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
```

`src/components/button.tsx`：

```tsx
import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";
import { forwardRef } from "react";
import { cn } from "../lib/utils";

const buttonVariants = cva(
  "inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-[--radius-field] text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50",
  {
    variants: {
      variant: {
        default: "bg-primary text-primary-foreground hover:opacity-90",
        outline: "border border-border bg-transparent hover:bg-muted",
        ghost: "hover:bg-muted",
        destructive: "bg-destructive text-destructive-foreground hover:opacity-90",
      },
      size: {
        default: "h-9 px-4 py-2",
        sm: "h-8 px-3 text-xs",
        icon: "h-9 w-9",
      },
    },
    defaultVariants: { variant: "default", size: "default" },
  },
);

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {
  asChild?: boolean;
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, asChild = false, ...props }, ref) => {
    const Comp = asChild ? Slot : "button";
    return <Comp ref={ref} className={cn(buttonVariants({ variant, size }), className)} {...props} />;
  },
);
Button.displayName = "Button";
```

`avatar.tsx` / `dialog.tsx` / `input.tsx` / `card.tsx` 用 shadcn 官方对应版本（Radix 封装 + cn），`toast.tsx` 薄封装 sonner：

```tsx
// src/components/toast.tsx
import { Toaster as Sonner, toast } from "sonner";

export const Toaster = Sonner;
export const toast = Object.assign(toast, {
  error: (msg: string) => toast.error(msg),
});
```

- [ ] **步骤 3：每个组件一个 story（含交互态）**

`src/components/button.stories.tsx`：

```tsx
import type { Meta, StoryObj } from "@storybook/react";
import { Button } from "./button";

const meta: Meta<typeof Button> = {
  title: "UI/Button",
  component: Button,
};
export default meta;

export const Default: StoryObj = { args: { children: "确认" } };
export const Outline: StoryObj = { args: { children: "取消", variant: "outline" } };
export const Destructive: StoryObj = { args: { children: "踢出", variant: "destructive" } };
export const Disabled: StoryObj = { args: { children: "禁用", disabled: true } };
```

（avatar/dialog/input/card/toast 同模式各写 default/主要变体 story；dialog story 里加一个受控打开的交互态。）

- [ ] **步骤 4：验证**

```bash
cd packages/components && pnpm build && pnpm storybook --ci
```

预期：Storybook 6006 端口起来，逐 story 目检亮/暗两套与基线风格一致；组件截图（Storybook `--stories-json` 或截图插件产出的图）存 `packages/components/storybook-baseline/` 作为组件级基线。

- [ ] **步骤 5：turbo 接入 + Commit**

根 `turbo.json` 的 `build`/`lint`/`test` 任务对 workspace 自动生效（无需改 turbo 配置，确认 `pnpm build` 包含新包即可）。

```bash
git add packages/components
git commit -m "feat: add shadcn base components with storybook"
```

---

### 任务 9：布局壳 + 通用组件迁入 + 基线比对

**文件：**
- 创建：`app/web/src/layouts/layout.tsx`（React 版根布局，参考 `_solid-legacy/layouts/layout.tsx`）
- 创建：`app/web/src/components/common/`（从 `_solid-legacy/components/common/` 移植：Avatar、Modal→Dialog 封装、Toast→sonner 封装、Divider、ProviderIcon（lucide-react 重写品牌图标分支）等，逐文件移植）
- 修改：`app/web/src/pages/index.tsx`（空壳首页换真实布局壳）
- 修改：`packages/components/src/index.ts`（home 壳需要的组件若属通用则下沉进包）

- [ ] **步骤 1：逐文件移植 `components/common/`**

每个文件：读 legacy 源码 → React 函数组件重写（signal → useState/useMemo，`<Show>` → `&&`/三元，`<For>` → `.map`，`Portal` → Radix 自带或 `createPortal`）→ icon 换 `lucide-react` 同名组件 → 放入 `app/web/src/components/common/`（跨页面复用的下沉 `packages/components`）。daisyUI 类名**不保留**，替换为 shadcn 组件 + Tailwind 工具类，视觉对照该组件的基线截图。

- [ ] **步骤 2：布局壳接入路由**

`layouts/layout.tsx`（根布局：侧栏 + header + content 出口，对照基线 `home-light.png`）+ `pages/__root.tsx`：

```tsx
import { createRootRouteWithContext, Outlet } from "@tanstack/react-router";
import type { QueryClient } from "@tanstack/react-query";

export const Route = createRootRouteWithContext<{ queryClient: QueryClient }>()({
  component: () => <Outlet />,
});
```

（侧栏/header 结构照 legacy `layouts/common/` 移植；登录态守卫 `beforeLoad` 里调 `userStore.ensureSession()`，失败 redirect `/login`。）

- [ ] **步骤 3：视觉比对（第一次过验收门）**

```bash
cd app/web && node visual-baseline/capture.mjs --only=home   # 对新前端出图（登录后手动 cookie 注入或脚本直登）
node visual-baseline/compare.mjs home visual-baseline/actual/home-light.png
```

预期：ratio < 0.02；否则修样式直到过门。基线比对的 credential/登录细节在执行时按 dev 环境实情调整。

- [ ] **步骤 4：web 单测补齐 + 全量回归**

```bash
pnpm vitest run && pnpm typecheck && pnpm build
```

- [ ] **步骤 5：Commit**

```bash
git add -A app/web packages/components
git commit -m "feat: port layout shell and common components to React"
```

---

## 自检记录

- **规格覆盖度**：规格 §3 主题基线 → 任务 1/3；§4 状态层 → 任务 4–7；§5 组件包+Storybook → 任务 2/8；§6 阶段 0–4 → 任务 1–9；阶段 5–7 明确不在本计划。无遗漏。
- **占位符**：theme.css 色值与 manifest 密码是「执行时实测替换」类标注（非设计占位，值只能来自运行环境实测），已用显式说明标注；其余步骤均含实际代码/命令。
- **类型一致性**：`useXxxStore` + `xxxStore` 门面命名规则统一（任务 5/6/7）；`ensureSession` 签名一致；`cn()` 定义于任务 8、任务 9 消费一致。
