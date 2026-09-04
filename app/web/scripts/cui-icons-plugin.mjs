// vite 插件: cui-solid-icons icon-set tree-shake。
// 原因: cui-solid-icons 各 set (f7/feather 等) 是单文件 minified ESM,千余图标组件
// 以 `function XX(i){...}` 声明 + 末尾 `export { XX as F7Yyy, ... }` 全量导出。
// rolldown 对这种模式保守,整包打入 bundle (f7 实测 2.0 MB rendered)。
//
// 插件 transform 原文件,把 export 块重写为只含"实际被引用"的图标别名。
// rolldown 据此删未引用的 function 声明 (无副作用),实现 tree-shake。
// 不改 function 体,无 shim 中转,无循环引用,runtime 正常。
//
// 引用集来源: 扫描 cui-solid/dist/cui.min.esm.js + 业务 src/**/*.{ts,tsx} 的
// `import { ... } from "cui-solid-icons/<set>"`。
//
// 触发: buildStart 扫描一次,transform 时用。dev 模式启动时也扫一次。

import { readFileSync, existsSync, readdirSync, statSync } from "node:fs";
import path from "node:path";

const IMPORT_RE = /import\s*\{([^}]+)\}\s*from\s*["']cui-solid-icons\/([a-z0-9]+)["']/g;

function collectFromFile(file, into) {
	if (!existsSync(file)) return;
	const text = readFileSync(file, "utf8");
	for (const m of text.matchAll(IMPORT_RE)) {
		const set = m[2];
		const names = m[1]
			.split(",")
			.map((s) => s.trim().split(/\s+as\s+/)[0].trim())
			.filter(Boolean);
		if (!into[set]) into[set] = new Set();
		names.forEach((n) => into[set].add(n));
	}
}

function walkDir(dir, into) {
	for (const entry of readdirSync(dir)) {
		const full = path.join(dir, entry);
		const st = statSync(full);
		if (st.isDirectory()) {
			if (entry === "node_modules") continue;
			walkDir(full, into);
		} else if (full.endsWith(".ts") || full.endsWith(".tsx")) {
			collectFromFile(full, into);
		}
	}
}

// 平衡括号找 export 块闭合 }
function findExportBlock(src) {
	const expIdx = src.search(/export\s*\{/);
	if (expIdx < 0) return null;
	let i = expIdx;
	while (i < src.length && src[i] !== "{") i++;
	const open = i;
	let depth = 1;
	i++;
	while (i < src.length && depth > 0) {
		const ch = src[i];
		if (ch === "{") depth++;
		else if (ch === "}") depth--;
		i++;
	}
	return { start: expIdx, open, close: i - 1 };
}

function rewriteExports(src, keepNames) {
	const blk = findExportBlock(src);
	if (!blk) return null;
	const inner = src.slice(blk.open + 1, blk.close);
	// 别名可含 $ 和 _ (minified 名如 x$),\w 不匹配 $,故用 [\w$]
	const pairs = [...inner.matchAll(/([\w$]+)\s+as\s+([\w$]+)/g)];
	const lines = [];
	for (const p of pairs) {
		if (keepNames.has(p[2])) lines.push(`  ${p[1]} as ${p[2]},`);
	}
	if (lines.length === 0) return null;
	const newBlock = `export {\n${lines.join("\n")}\n}`;
	return src.slice(0, blk.start) + newBlock + src.slice(blk.close + 1);
}

export function cuiIconsTreeShakePlugin() {
	let root = process.cwd();
	const referenced = {};

	function scan() {
		for (const k of Object.keys(referenced)) delete referenced[k];
		const cuiSolid = path.join(root, "node_modules/cui-solid/dist/cui.min.esm.js");
		collectFromFile(cuiSolid, referenced);
		walkDir(path.join(root, "src"), referenced);
	}

	return {
		name: "cui-icons-tree-shake",
		configResolved(cfg) {
			root = cfg.root;
			scan();
		},
		configureServer() {
			scan();
		},
		transform(src, id) {
			const m = id.match(/cui-solid-icons\/dist\/([a-z0-9]+)\/\1\.min\.esm\.js$/);
			if (!m) return null;
			const set = m[1];
			const names = referenced[set];
			if (!names || names.size === 0) return null;
			const out = rewriteExports(src, names);
			if (!out) return null;
			return { code: out, map: null };
		},
	};
}
