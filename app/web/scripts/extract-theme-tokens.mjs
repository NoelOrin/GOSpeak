// 从运行中的 daisyUI 应用提取主题 token 实测值，输出 theme.css 片段。
// 用法：node scripts/extract-theme-tokens.mjs [baseUrl]（需老前端仍在运行）
import { chromium } from "playwright";

const base = process.argv[2] ?? "http://localhost:3000";
const VARS = [
	"--color-base-100",
	"--color-base-200",
	"--color-base-300",
	"--color-base-content",
	"--color-primary",
	"--color-primary-content",
	"--color-secondary",
	"--color-secondary-content",
	"--color-accent",
	"--color-accent-content",
	"--color-neutral",
	"--color-neutral-content",
	"--color-error",
	"--color-error-content",
	"--color-success",
	"--color-success-content",
	"--color-warning",
	"--color-warning-content",
	"--color-info",
	"--color-info-content",
	"--radius-selector",
	"--radius-field",
	"--radius-box",
	"--border",
];

const browser = await chromium.launch();
const page = await browser.newPage();
await page.goto(`${base}/login`, { waitUntil: "load" });

for (const theme of ["acid", "dark"]) {
	const values = await page.evaluate((vars) => {
		document.documentElement.setAttribute("data-theme", vars.theme);
		document.documentElement.classList.toggle("dark", vars.theme === "dark");
		const cs = getComputedStyle(document.documentElement);
		return Object.fromEntries(vars.names.map((n) => [n, cs.getPropertyValue(n).trim()]));
	}, { theme, names: VARS });
	console.log(`/* ${theme} */`);
	for (const [k, v] of Object.entries(values)) console.log(`${k}: ${v}`);
}
await browser.close();
