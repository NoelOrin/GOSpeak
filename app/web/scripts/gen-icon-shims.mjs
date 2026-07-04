// 自动生成 cui-solid-icons 子集 shim 文件。
// 原因：cui-solid-icons 各 icon-set 是单文件 minified ESM,千余图标共享 helper 且未标
// /* @__PURE__ */,rolldown tree-shake 失败整包打入 bundle(f7 全量 2.1M,实测全进)。
// shim 仅 re-export 实际被引用的图标,命名导出可 tree-shake → 只用到的图标进 bundle。
//
// 扫描范围:
//   1. node_modules/cui-solid/dist/cui.min.esm.js 的 `import { ... } from "cui-solid-icons/<set>"`
//   2. 业务源码 src/**/*.{ts,tsx} 直接 import `cui-solid-icons/<set>`
// 收集所有 set→图标名,生成 vendor/cui-solid-icons-<set>.ts。
// 只为有引用的 set 生成文件;无引用的 set 不生成(vite alias 正则匹配到无文件会报错,
// 故 vite.config.ts 的 alias 只列已知的 f7/feather;新增 set 需在 vite.config.ts 加一行)
//
// 用法: node scripts/gen-icon-shims.mjs
// 推荐接入 package.json prebuild: "prebuild": "node scripts/gen-icon-shims.mjs"
//
// cui-solid 升级或业务新增图标后重跑即可,无需手改 shim。

import { readFileSync, writeFileSync, existsSync, readdirSync, statSync } from "node:fs";
import path from "node:path";

const ROOT = path.resolve(import.meta.dirname, "..");
const VENDOR_DIR = path.join(ROOT, "vendor");
const CUI_SOLID = path.join(ROOT, "node_modules/cui-solid/dist/cui.min.esm.js");

// 匹配 `import { A, B as C, D } from "cui-solid-icons/<set>"`
const IMPORT_RE = /import\s*\{([^}]+)\}\s*from\s*["']cui-solid-icons\/([a-z0-9]+)["']/g;

function collectFromFile(filePath, into) {
	if (!existsSync(filePath)) return;
	const text = readFileSync(filePath, "utf8");
	for (const m of text.matchAll(IMPORT_RE)) {
		const set = m[2];
		const names = m[1]
			.split(",")
			.map((s) => s.trim().split(/\s+as\s+/)[0].trim())
			.filter(Boolean);
		(into[set] ??= new Set()).add(...names);
	}
}

function walkDir(dir, exts, into) {
	for (const entry of readdirSync(dir)) {
		const full = path.join(dir, entry);
		const st = statSync(full);
		if (st.isDirectory()) {
			if (entry === "node_modules") continue;
			walkDir(full, exts, into);
		} else if (exts.some((e) => full.endsWith(e))) {
			collectFromFile(full, into);
		}
	}
}

function renderShim(set, icons) {
	const sorted = [...icons].sort();
	const body = sorted.map((n) => `\t${n},`).join("\n");
	return `// 自动生成,勿手改。由 scripts/gen-icon-shims.mjs 产出。
// cui-solid-icons/${set} 子集 shim —— 原包未标 /* @__PURE__ */,tree-shake 失败整包打入 bundle。
// 仅 re-export 实际被引用的图标。追加新 import 后重跑脚本: node scripts/gen-icon-shims.mjs
export {
${body}
} from "cui-solid-icons/${set}";
`;
}

const collected = {};
collectFromFile(CUI_SOLID, collected);
walkDir(path.join(ROOT, "src"), [".ts", ".tsx"], collected);

const sets = Object.keys(collected).sort();
let changed = 0;
for (const set of sets) {
	const target = path.join(VENDOR_DIR, `cui-solid-icons-${set}.ts`);
	const content = renderShim(set, collected[set]);
	if (!existsSync(target) || readFileSync(target, "utf8") !== content) {
		writeFileSync(target, content);
		changed++;
	}
	console.log(`[gen-icon-shims] ${set}: ${collected[set].size} icons → ${path.relative(ROOT, target)}`);
}

// 删除已无引用的旧 shim(stale set)
if (existsSync(VENDOR_DIR)) {
	for (const f of readdirSync(VENDOR_DIR)) {
		const m = f.match(/^cui-solid-icons-([a-z0-9]+)\.ts$/);
		if (m && !collected[m[1]]) {
			writeFileSync(path.join(VENDOR_DIR, f), `// 自动生成,无引用,占位保留以匹配 vite alias。\nexport {} from "cui-solid-icons/${m[1]}";\n`);
			console.log(`[gen-icon-shims] ${m[1]}: 无引用,保留占位 shim (vite alias 仍指向此文件)`);
		}
	}
}

console.log(`[gen-icon-shims] done. ${changed} file(s) updated. sets: ${sets.join(", ") || "(none)"}`);
