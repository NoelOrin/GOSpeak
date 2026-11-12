#!/usr/bin/env node
// 版本一致性校验 —— 以 .github/versions/*.yaml 的显式声明为单一事实来源。
//
// 用法：node scripts/check-version.mjs [rootDir] [releaseTag] [component]
//   rootDir     可选，仓库根目录（默认 process.cwd()）
//   releaseTag  可选，发布 tag；tag 事件时缺省取 GITHUB_REF_NAME
//   component   可选，"server" | "desktop"；缺省时按 tag 前缀推断
//               （gospeak-desktop-v* → desktop，gospeak-server-v* → server），
//               非 tag 事件为 server
//
// 组件与校验规则：
//   server  源：.github/versions/server.yaml
//           校验：根 VERSION、根 package.json 的 version 必须与 yaml 一致；
//                 发布 tag（gospeak-server-v<version>）必须与 yaml 一致。
//   desktop 源：.github/versions/desktop.yaml
//           校验：app/desktop/package.json 与 app/desktop/mygo.config.ts 的
//                 version 必须与 yaml 一致；发布 tag
//                 （gospeak-desktop-v<version>）必须与 yaml 一致。
//
// mygo.config.ts 是版本号的第三份副本（它决定 .app / dmg / 安装包的文件名），
// 此前无人校验、可静默漂移，故纳入本脚本。
//
// tag 命名规范由 scripts/release-naming.mjs 统一定义，本脚本不重复拼字符串。

import { readFileSync } from "node:fs";
import { pathToFileURL } from "node:url";

import { parseReleaseTag, tagPrefix } from "./release-naming.mjs";

const SEMVER_PATTERN =
	/^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$/;

const COMPONENT_ALIASES = { main: "server" };

function readJson(path) {
	return JSON.parse(readFileSync(path, "utf8"));
}

function parseVersion(value, source) {
	const version = String(value ?? "").trim();
	if (!SEMVER_PATTERN.test(version)) {
		throw new Error(
			`${source} must contain a valid SemVer, got ${JSON.stringify(version)}`,
		);
	}
	return version;
}

function normalizeComponent(component) {
	const value = String(component ?? "").trim();
	if (value === "") return undefined;
	const resolved = COMPONENT_ALIASES[value] ?? value;
	if (resolved !== "server" && resolved !== "desktop") {
		throw new Error(
			`unknown component ${JSON.stringify(component)}; expected "server" or "desktop"`,
		);
	}
	return resolved;
}

/** 解析版本声明 yaml 的顶层 version 字段（容忍引号 / 行尾注释；零依赖）。 */
function readDeclaredVersion(path) {
	let raw;
	try {
		raw = readFileSync(path, "utf8");
	} catch {
		throw new Error(
			`version declaration file not found: ${path} (create it with a "version:" field)`,
		);
	}
	const match = raw.match(/^version:\s*["']?([^\s#"']+)["']?\s*(?:#.*)?$/m);
	if (!match) {
		throw new Error(`${path} must declare a top-level "version:" field`);
	}
	return parseVersion(match[1], path);
}

/** 从 mygo.config.ts 读取 version 字段（TS 配置，正则取值，零依赖）。 */
function readMygoConfigVersion(path) {
	let raw;
	try {
		raw = readFileSync(path, "utf8");
	} catch {
		throw new Error(`mygo config not found: ${path}`);
	}
	const match = raw.match(/^\s*version:\s*["']([^"']+)["']/m);
	if (!match) {
		throw new Error(`${path} must declare a "version" field`);
	}
	return parseVersion(match[1], `${path} version`);
}

function declaredPathFor(rootDir, component) {
	return `${rootDir}/.github/versions/${component}.yaml`;
}

function checkServer(rootDir, declared, issues) {
	const version = parseVersion(
		readFileSync(`${rootDir}/VERSION`, "utf8"),
		"VERSION",
	);
	const packageVersion = parseVersion(
		readJson(`${rootDir}/package.json`).version,
		"package.json version",
	);
	if (packageVersion !== version) {
		issues.push(
			`package.json version ${packageVersion} does not match VERSION ${version}`,
		);
	}
	if (version !== declared) {
		issues.push(
			`VERSION ${version} does not match .github/versions/server.yaml ${declared}`,
		);
	}
}

function checkDesktop(rootDir, declared, issues) {
	const packageVersion = parseVersion(
		readJson(`${rootDir}/app/desktop/package.json`).version,
		"app/desktop/package.json version",
	);
	if (packageVersion !== declared) {
		issues.push(
			`app/desktop/package.json version ${packageVersion} does not match .github/versions/desktop.yaml ${declared}`,
		);
	}

	const mygoVersion = readMygoConfigVersion(
		`${rootDir}/app/desktop/mygo.config.ts`,
	);
	if (mygoVersion !== declared) {
		issues.push(
			`app/desktop/mygo.config.ts version ${mygoVersion} does not match .github/versions/desktop.yaml ${declared}`,
		);
	}
}

export function checkVersionState(rootDir, releaseTag_, component) {
	// 显式参数 > tag 前缀推断 > server 兜底
	const tag = String(releaseTag_ ?? "").trim();
	const parsed = tag === "" ? null : parseReleaseTag(tag);
	const comp = normalizeComponent(component) ?? parsed?.component ?? "server";

	const declared = readDeclaredVersion(declaredPathFor(rootDir, comp));
	const issues = [];

	if (comp === "desktop") {
		checkDesktop(rootDir, declared, issues);
	} else {
		checkServer(rootDir, declared, issues);
	}

	if (tag !== "") {
		if (!parsed) {
			issues.push(
				`release tag ${tag} does not follow the naming convention ${tagPrefix(comp)}<version>`,
			);
		} else if (parsed.version !== declared) {
			issues.push(
				`release tag ${tag} does not match ${comp} declared version ${declared}`,
			);
		}
	}

	return { component: comp, declared, issues };
}

function main() {
	const rootDir = process.argv[2] ?? process.cwd();
	const isTagEvent = process.env.GITHUB_REF_TYPE === "tag";
	// 仅在显式传入 RELEASE_TAG 或当前为 tag 事件时，才把 ref 当作发布标签校验。
	// push 到分支时 GITHUB_REF_NAME 是分支名（如 "release"），并非合法 tag，
	// 此时不应把它当作发布标签去校验，否则会出现假阳性失败。
	const releaseTagArg = isTagEvent
		? (process.argv[3] ??
			process.env.RELEASE_TAG ??
			process.env.GITHUB_REF_NAME ??
			"")
		: (process.argv[3] ?? process.env.RELEASE_TAG ?? "");
	const component =
		process.argv[4] ?? process.env.GOSPEAK_COMPONENT ?? undefined;

	let result;
	try {
		result = checkVersionState(rootDir, releaseTagArg, component);
	} catch (err) {
		console.error(`::error::${err instanceof Error ? err.message : err}`);
		process.exitCode = 1;
		return;
	}

	if (result.issues.length > 0) {
		for (const issue of result.issues) {
			console.error(`::error::${issue}`);
		}
		process.exitCode = 1;
		return;
	}

	console.log(
		`version consistency ok: component=${result.component} declared=${result.declared}`,
	);
}

if (import.meta.url === pathToFileURL(process.argv[1]).href) {
	main();
}
