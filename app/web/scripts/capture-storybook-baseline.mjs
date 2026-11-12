// Storybook 组件级基线截图。
// 前置：cd packages/components && pnpm storybook --ci（6006 端口）
// 用法：node scripts/capture-storybook-baseline.mjs [--actual]（在 app/web 下执行）
import { chromium } from "playwright";
import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = path.resolve(
	path.dirname(fileURLToPath(import.meta.url)),
	"../../..",
);
const outDir = path.resolve(
	repoRoot,
	process.argv.includes("--actual")
		? "packages/components/storybook-actual"
		: "packages/components/storybook-baseline",
);

const res = await fetch("http://localhost:6006/index.json");
const data = await res.json();
await mkdir(outDir, { recursive: true });

const browser = await chromium.launch();
const page = await browser.newPage({
	viewport: { width: 400, height: 240 },
	deviceScaleFactor: 2,
});
let n = 0;
for (const [id, entry] of Object.entries(data.entries)) {
	for (const theme of ["light", "dark"]) {
		await page.goto(
			`http://localhost:6006/iframe.html?id=${id}&globals=theme:${theme}`,
			{ waitUntil: "networkidle" },
		);
		await page.waitForTimeout(300);
		const name =
			id.replace(/--.*$/, "") +
			"-" +
			(entry.name ?? "story").toLowerCase().replace(/\s+/g, "-") +
			"-" +
			theme +
			".png";
		await page.screenshot({ path: path.join(outDir, name) });
		n++;
	}
}
await writeFile(path.join(outDir, ".last-run"), new Date().toISOString());
await browser.close();
console.log("captured", n, "storybook shots ->", outDir);
