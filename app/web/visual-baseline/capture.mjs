// 视觉基线截图脚本。用法：node capture.mjs [--only=routeId1,routeId2] [--base-url=URL]
// 基线录自 Solid 现状（dev-2.0，worktree :3001），迁移期间以同脚本对新前端出图后用 compare.mjs 比对。
// 规则（v3，两边对称）：
// - auth:false 路由在未登录 context 先截（beforeLoad 会把已登录会话重定向走）
// - 无 Header 的页面（login/register/guest）用 evaluate 设 data-theme + .dark
// - 有 Header 的页面（authed 路由）必须点击 header 主题切换按钮走真实 toggle 流程——
//   Solid 版 Header 的 initTheme 会在挂载时覆盖 DOM 属性，evaluate 会被打回（基线 v2 的 bug）
import { chromium } from "playwright";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const manifest = JSON.parse(await readFile(path.join(here, "routes.manifest.json"), "utf8"));
const OUT = path.join(here, process.argv.includes("--actual") ? "actual" : "baseline");
const only = process.argv.find((a) => a.startsWith("--only="))?.split("=")[1]?.split(",") ?? null;
const baseUrlArg = process.argv.find((a) => a.startsWith("--base-url="))?.split("=")[1];
if (baseUrlArg) manifest.baseUrl = baseUrlArg;
await mkdir(OUT, { recursive: true });

const browser = await chromium.launch();
const VIEWPORT = { width: 1440, height: 900 };
const selected = manifest.routes.filter(
  (r) => !r.rescheduleAfterP1 && (!only || only.includes(r.id)),
);

async function newPage() {
  const ctx = await browser.newContext({
    viewport: VIEWPORT,
    deviceScaleFactor: 2,
  });
  return ctx.newPage();
}

async function setThemeViaEvaluate(page, theme) {
  await page.evaluate((t) => {
    // light = 各端默认主题（Solid acid / React :root=acid）
    if (t === "dark") {
      document.documentElement.setAttribute("data-theme", "dark");
    } else {
      document.documentElement.removeAttribute("data-theme");
    }
    document.documentElement.classList.toggle("dark", t === "dark");
    try {
      localStorage.setItem("theme-is-dark", String(t === "dark"));
    } catch {}
  }, theme);
  await page.waitForTimeout(400);
}

/**
 * 登录后的主题切换：React 版的主题源在 zustand store，模块加载时读 localStorage
 * 的 theme-is-dark 再 applyTheme()，所以不能只改 DOM（会被 initTheme 覆盖），
 * 也不能点 header 按钮（React header 已无文本型主题按钮）。
 * 做法：写 localStorage → reload，让 store 初始化时就读到目标主题。
 */
async function setThemeViaStorage(page, theme) {
  await page.evaluate((t) => {
    try {
      localStorage.setItem("theme-is-dark", String(t === "dark"));
    } catch {}
  }, theme);
  await page.reload({ waitUntil: "load" });
  await page.waitForTimeout(800);
}

async function shoot(page, routeId, theme) {
  await page.screenshot({ path: path.join(OUT, `${routeId}-${theme}.png`), fullPage: true });
  console.log(`captured ${routeId}-${theme}.png`);
}

async function run() {
  // 1) 未登录路由：独立 context，先截（这些页面没有 Header，evaluate 安全）
  const anon = selected.filter((r) => !r.auth);
  if (anon.length) {
    const page = await newPage();
    for (const route of anon) {
      for (const theme of route.themes) {
        await page.goto(`${manifest.baseUrl}${route.path}`, { waitUntil: "load", timeout: 30_000 });
        await setThemeViaEvaluate(page, theme);
        await page.waitForTimeout(1200);
        await shoot(page, route.id, theme);
      }
    }
    await page.context().close();
  }

  // 2) 登录路由：登录后逐路由截，主题走 header 按钮真实切换
  const authed = selected.filter((r) => r.auth);
  if (authed.length) {
    const page = await newPage();
    await page.goto(`${manifest.baseUrl}/login`, { waitUntil: "load" });
    await page.evaluate(() => {
      try {
        localStorage.setItem("theme-is-dark", "false");
      } catch {}
    });
    await page.getByPlaceholder("请输入用户名").fill(manifest.accounts.admin.name);
    await page.locator("input[type=password]").fill(manifest.accounts.admin.password);
    await page.locator("button[type=submit], button:has-text('登录')").first().click();
    await page.waitForURL((u) => !u.pathname.startsWith("/login"), { timeout: 15_000 });
    for (const route of authed) {
      // 先按 light 落到该路由，再按需 reload 切到 dark
      await page.evaluate(() => {
        try {
          localStorage.setItem("theme-is-dark", "false");
        } catch {}
      });
      for (const theme of route.themes) {
        // manage/monitor 挂 SSE 长连接，networkidle 永不触发，统一用 load + settle
        await page.goto(`${manifest.baseUrl}${route.path}`, { waitUntil: "load", timeout: 30_000 });
        await page.waitForTimeout(600);
        await setThemeViaStorage(page, theme);
        await page.waitForTimeout(1200);
        await shoot(page, route.id, theme);
      }
      // 回首页避免上个路由的局部状态影响下一个
      await page.goto(`${manifest.baseUrl}/`, { waitUntil: "load" });
    }
    await page.context().close();
  }

  await writeFile(path.join(OUT, ".last-run"), new Date().toISOString());
  await browser.close();
}
run();
