import * as fs from "node:fs";
import * as path from "node:path";

export const PLUGIN_EXTS = new Set([".ts", ".js", ".mjs", ".cjs"]);

export function discoverPlugins(
	dir: string,
	resolveEntry: (p: string) => string | null,
): string[] {
	const results: string[] = [];
	if (!fs.existsSync(dir)) return results;
	const entries = fs.readdirSync(dir, { withFileTypes: true });
	for (const e of entries) {
		if (e.name.startsWith(".") || e.name === "node_modules") continue;
		const full = path.join(dir, e.name);
		if (e.isFile() && PLUGIN_EXTS.has(path.extname(e.name))) {
			results.push(path.resolve(full));
		} else if (e.isDirectory()) {
			const entry = resolveEntry(full);
			if (entry) results.push(entry);
		}
	}
	return results;
}

export function resolvePluginEntry(dirOrFile: string): string | null {
	const st = fs.statSync(dirOrFile);
	if (st.isFile()) {
		return PLUGIN_EXTS.has(path.extname(dirOrFile))
			? path.resolve(dirOrFile)
			: null;
	}
	// 目录：main.ts / main.js / <dirname>.ts / index.ts
	const name = path.basename(dirOrFile);
	const candidates = [
		"main.ts",
		"main.js",
		"main.mjs",
		`${name}.ts`,
		`${name}.js`,
		"index.ts",
		"index.js",
	];
	for (const c of candidates) {
		const p = path.join(dirOrFile, c);
		if (fs.existsSync(p)) return path.resolve(p);
	}
	return null;
}

export function pluginModulePath(absPath: string, pluginDir?: string): string {
	const dir = pluginDir ? path.resolve(pluginDir) : null;
	if (dir) {
		const rel = path.relative(dir, absPath);
		if (!rel.startsWith("..") && !path.isAbsolute(rel)) {
			const noExt = rel
				.split(path.sep)
				.join("/")
				.replace(/\.(ts|js|mjs|cjs)$/i, "");
			return `user_plugins/${noExt}`;
		}
	}
	const base = path.basename(absPath, path.extname(absPath));
	return `user_plugins/${base}`;
}

export function copyRecursive(src: string, dest: string): void {
	const st = fs.statSync(src);
	if (st.isDirectory()) {
		fs.mkdirSync(dest, { recursive: true });
		for (const name of fs.readdirSync(src)) {
			copyRecursive(path.join(src, name), path.join(dest, name));
		}
	} else {
		fs.mkdirSync(path.dirname(dest), { recursive: true });
		fs.copyFileSync(src, dest);
	}
}
