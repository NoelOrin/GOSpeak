import { readdirSync, readFileSync } from "node:fs";
import { join, relative } from "node:path";
import { pathToFileURL } from "node:url";
import { checkRadiusTokens } from "./check-radius-tokens.mjs";

// UI token 护栏。规范写在 packages/components/src/theme.css 与
// docs/review/ui-review-2026-11-08.md，这里防止它被新代码绕开。
//
// 拦住 5 类回退：
//   1. 任意圆角        rounded-[6px]                 → 用 rounded-sm/md/lg/xl/2xl
//   2. ds-* 语义双轨   ds-text / ds-panel / ds-border → 用 text-foreground / bg-card / border-border
//   3. 任意字号        text-[13px]                    → 用 text-2xs/xs/sm/base/lg/xl
//   4. 文字透明度档位  text-foreground/45             → 用 text-foreground / -muted-foreground / -faint-foreground
//   5. 裸圆角与越界图标 rounded / size={15}           → 用 rounded-sm / size={12|14|16|20|24|32}
const SCAN_DIRS = ["app/web/src", "packages/components/src"];
const SKIP_DIRS = new Set(["node_modules", "dist", "_solid-legacy"]);
const SCRIPT_EXTS = /\.(tsx?|jsx?)$/;

const DS_DUPLICATE =
	/\bds-(?:text|body|muted|faint|border|bg|panel|userbar|subtle|hover|active|deep|blurple|divider|input-bg|msg-row|danger)\b/;
const ARBITRARY_FONT_SIZE = /\btext-\[\d+(?:\.\d+)?(?:px|rem)\]/;
const FOREGROUND_OPACITY = /\btext-foreground\/\d+/;
const BARE_ROUNDED = /(?<![\w-])rounded(?![\w-])/;
const ICON_SIZE = /<[A-Z][A-Za-z0-9]*\s+size=\{(\d+)\}/g;
const ICON_SIZES = new Set([12, 14, 16, 20, 24, 32]);

const RULES = [
	{ pattern: DS_DUPLICATE, hint: "ds-* 语义层已并入 theme.css token，直接用 Tailwind token" },
	{
		pattern: ARBITRARY_FONT_SIZE,
		hint: "字号只允许 text-2xs/xs/sm/base/lg/xl",
	},
	{
		pattern: FOREGROUND_OPACITY,
		hint: "文字层级只有 text-foreground / -muted-foreground / -faint-foreground",
	},
	{ pattern: BARE_ROUNDED, hint: "圆角必须显式写档位（rounded-sm/lg/xl/2xl/full）" },
];

function walk(dir, files = []) {
	for (const entry of readdirSync(dir, { withFileTypes: true })) {
		if (entry.isDirectory()) {
			if (!SKIP_DIRS.has(entry.name)) {
				walk(join(dir, entry.name), files);
			}
			continue;
		}
		if (SCRIPT_EXTS.test(entry.name)) {
			files.push(join(dir, entry.name));
		}
	}
	return files;
}

export function checkUiTokens(rootDir) {
	const violations = [];

	for (const dir of SCAN_DIRS) {
		let files;
		try {
			files = walk(join(rootDir, dir));
		} catch {
			continue;
		}

		for (const file of files) {
			const lines = readFileSync(file, "utf8").split("\n");

			lines.forEach((line, index) => {
				for (const rule of RULES) {
					const match = rule.pattern.exec(line);
					if (match) {
						violations.push({
							file: relative(rootDir, file),
							line: index + 1,
							found: match[0],
							hint: rule.hint,
						});
					}
				}

				ICON_SIZE.lastIndex = 0;
				for (const match of line.matchAll(ICON_SIZE)) {
					const size = Number(match[1]);
					if (ICON_SIZES.has(size)) continue;
					violations.push({
						file: relative(rootDir, file),
						line: index + 1,
						found: `size={${size}}`,
						hint: `图标尺寸只有 ${[...ICON_SIZES].join(" / ")} 六档`,
					});
				}
			});
		}
	}

	return violations;
}

function main() {
	const rootDir = process.argv[2] ?? process.cwd();
	const violations = [
		...checkRadiusTokens(rootDir).map((v) => ({
			...v,
			hint: "圆角必须走 theme.css 的 token 档位",
		})),
		...checkUiTokens(rootDir),
	];

	if (violations.length > 0) {
		for (const violation of violations) {
			console.error(
				`::error file=${violation.file},line=${violation.line}::${violation.found} — ${violation.hint}`,
			);
		}
		console.error(`ui token check failed: ${violations.length} violation(s)`);
		process.exitCode = 1;
		return;
	}

	console.log("ui token check ok");
}

if (import.meta.url === pathToFileURL(process.argv[1]).href) {
	main();
}
