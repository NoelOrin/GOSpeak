import { readdirSync, readFileSync } from "node:fs";
import { join, relative } from "node:path";
import { pathToFileURL } from "node:url";

// 圆角必须走 packages/components/src/theme.css 的 token 档位，
// 这里拦住任意值写法（rounded-[4px]）与 CSS 里的字面量（border-radius: 4px）。
const SCAN_DIRS = ["app/web/src", "packages/components/src"];
const SKIP_DIRS = new Set(["node_modules", "dist", "_solid-legacy"]);
const SCRIPT_EXTS = /\.(tsx?|jsx?)$/;
const STYLE_EXTS = /\.css$/;
const ARBITRARY_RADIUS = /\brounded-[trblsexy]*-\[[^\]]+\]/;
const LITERAL_RADIUS = /border-radius:\s*-?(?:\d|\.\d)(?:px|rem|em)/;

function walk(dir, files = []) {
	for (const entry of readdirSync(dir, { withFileTypes: true })) {
		if (entry.isDirectory()) {
			if (!SKIP_DIRS.has(entry.name)) {
				walk(join(dir, entry.name), files);
			}
			continue;
		}
		if (SCRIPT_EXTS.test(entry.name) || STYLE_EXTS.test(entry.name)) {
			files.push(join(dir, entry.name));
		}
	}
	return files;
}

export function checkRadiusTokens(rootDir) {
	const violations = [];

	for (const dir of SCAN_DIRS) {
		let files;
		try {
			files = walk(join(rootDir, dir));
		} catch {
			continue;
		}

		for (const file of files) {
			const isStyle = STYLE_EXTS.test(file);
			const pattern = isStyle ? LITERAL_RADIUS : ARBITRARY_RADIUS;
			const lines = readFileSync(file, "utf8").split("\n");

			lines.forEach((line, index) => {
				const match = pattern.exec(line);
				if (match) {
					violations.push({
						file: relative(rootDir, file),
						line: index + 1,
						found: match[0],
					});
				}
			});
		}
	}

	return violations;
}

function main() {
	const rootDir = process.argv[2] ?? process.cwd();
	const violations = checkRadiusTokens(rootDir);

	if (violations.length > 0) {
		for (const violation of violations) {
			console.error(
				`::error file=${violation.file},line=${violation.line}::${violation.found} 必须改用圆角 token（见 packages/components/src/theme.css）`,
			);
		}
		console.error(`radius token check failed: ${violations.length} violation(s)`);
		process.exitCode = 1;
		return;
	}

	console.log("radius token check ok");
}

if (import.meta.url === pathToFileURL(process.argv[1]).href) {
	main();
}
