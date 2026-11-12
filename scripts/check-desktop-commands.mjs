#!/usr/bin/env node
// 校验桌面桥接契约：MyGo 以 "<Struct>.<Method>" 注册绑定方法（见
// `mygo generate` 的产物 app/desktop/mygo.ts），TS SDK 必须逐字使用同一串。
// 命令名写错时 MyGo 返回 "method X is not bound" 的 rejected Promise，而
// 大部分能力方法会静默吞掉错误，因此在 CI 阶段拦截。
import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";
import { pathToFileURL } from "node:url";

const DESKTOP_DIR = "app/desktop";
const TS_DIR = "packages/desktop/src/capabilities";

/** 从 Go 源码里取出 `mygo.Bind(X{})` 绑定的类型名。 */
export function extractBoundTypes(source) {
	const types = new Set();
	for (const m of source.matchAll(
		/mygo\.Bind\(\s*&?([A-Z]\w*)\s*\{\s*\}\s*\)/g,
	)) {
		types.add(m[1]);
	}
	return types;
}

/** 从一组 Go 源文件里取出 `<Type>.<Method>` 形式的绑定方法全集。 */
export function extractGoBindings(files) {
	const bound = new Set();
	for (const file of files) {
		for (const t of extractBoundTypes(file.source)) bound.add(t);
	}
	const methods = new Set();
	for (const file of files) {
		for (const m of file.source.matchAll(
			/^func\s+\(([A-Z]\w*)\)\s+([A-Z]\w*)\s*\(/gm,
		)) {
			const [, recv, method] = m;
			if (!bound.has(recv)) continue;
			methods.add(`${recv}.${method}`);
		}
	}
	return methods;
}

/** 从 TS SDK 里取出 `command("X", "Y")` 的命令名全集。 */
export function extractTsCommands(files) {
	const commands = new Set();
	for (const file of files) {
		for (const m of file.source.matchAll(
			/command\(\s*"([A-Z]\w*)"\s*,\s*"([A-Z]\w*)"\s*\)/g,
		)) {
			commands.add(`${m[1]}.${m[2]}`);
		}
	}
	return commands;
}

/** 双向比对，返回两侧的差集。只有 missing 会让校验失败。 */
export function checkCommands({ bindings, commands }) {
	const missing = [...commands].filter((c) => !bindings.has(c)).sort();
	const unused = [...bindings].filter((b) => !commands.has(b)).sort();
	return { missing, unused, ok: missing.length === 0 };
}

function goFiles(dir) {
	return readdirSync(dir)
		.filter((f) => f.endsWith(".go") && !f.endsWith("_test.go"))
		.map((f) => ({
			path: join(dir, f),
			source: readFileSync(join(dir, f), "utf8"),
		}));
}

function tsFiles(dir) {
	return readdirSync(dir)
		.filter((f) => f.endsWith(".ts"))
		.map((f) => ({
			path: join(dir, f),
			source: readFileSync(join(dir, f), "utf8"),
		}));
}

export function checkRepo(root = ".") {
	const bindings = extractGoBindings(goFiles(join(root, DESKTOP_DIR)));
	const commands = extractTsCommands(tsFiles(join(root, TS_DIR)));
	return { bindings, commands, ...checkCommands({ bindings, commands }) };
}

if (import.meta.url === pathToFileURL(process.argv[1]).href) {
	const { missing, unused } = checkRepo(".");
	for (const b of unused) {
		// 只是提示：有些绑定方法由宿主自身调用（托盘、生命周期），SDK 不经过。
		console.warn(`::warning::宿主绑定了但 SDK 未调用: ${b}`);
	}
	for (const c of missing) {
		console.error(`::error::SDK 调用了宿主未绑定的方法: ${c}`);
	}
	if (missing.length > 0) process.exit(1);
	console.log("desktop bridge commands are in sync");
}
