#!/usr/bin/env node
// 版本一致性校验 —— 以 .github/versions/*.yaml 的显式声明为单一事实来源。
//
// 用法：node scripts/check-version.mjs [rootDir] [releaseTag] [component]
//   rootDir     可选，仓库根目录（默认 process.cwd()）
//   releaseTag  可选，发布 tag；push tag 事件时缺省取 GITHUB_REF_NAME
//   component   可选，"main" | "desktop"；缺省时按 tag 前缀推断
//               （desktop-v* → desktop，v* → main），非 tag 事件为 main
//
// 组件与校验规则：
//   main    源：.github/versions/main.yaml
//           校验：根 VERSION、根 package.json 的 version 必须与 yaml 一致；
//                 发布 tag（形如 v<version>）必须与 yaml 一致。
//   desktop 源：.github/versions/desktop.yaml
//           校验：app/desktop/package.json 的 version 必须与 yaml 一致；
//                 发布 tag（形如 desktop-v<version>）必须与 yaml 一致。
//
// 保持向后兼容：`node scripts/check-version.mjs` / `. <tag>`（无组件参数）仍按
// main 组件校验，ci.yml / docker.yml 的既有调用无需修改。

import { readFileSync } from "node:fs";
import { pathToFileURL } from "node:url";

const SEMVER_PATTERN =
	/^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$/;

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

/** 按组件剥离 tag 前缀：desktop-v1.2.3 → 1.2.3；v1.2.3 → 1.2.3。 */
function normalizeTag(tag, component) {
	const value = String(tag ?? "").trim();
	const prefix = component === "desktop" ? "desktop-v" : "v";
	if (!value.startsWith(prefix)) {
		throw new Error(
			`release tag ${JSON.stringify(value)} must start with ${JSON.stringify(prefix)} for the ${component} component`,
		);
	}
	return parseVersion(value.slice(prefix.length), "release tag");
}

function inferComponent(tag) {
	const value = String(tag ?? "").trim();
	if (value.startsWith("desktop-v")) return "desktop";
	return "main";
}

export function checkVersionState(rootDir, releaseTag = "", component) {
	// 显式参数 > tag 前缀推断 > main 兜底
	const comp =
		component ?? (releaseTag ? inferComponent(releaseTag) : "main");

	const declaredPath =
		comp === "desktop"
			? `${rootDir}/.github/versions/desktop.yaml`
			: `${rootDir}/.github/versions/main.yaml`;
	const declared = readDeclaredVersion(declaredPath);

	const issues = [];

	if (comp === "desktop") {
		const pkg = readJson(`${rootDir}/app/desktop/package.json`);
		const pkgVersion = parseVersion(
			pkg.version,
			"app/desktop/package.json version",
		);
		if (pkgVersion !== declared) {
			issues.push(
				`app/desktop/package.json version ${pkgVersion} does not match .github/versions/desktop.yaml ${declared}`,
			);
		}
	} else {
		const version = parseVersion(
			readFileSync(`${rootDir}/VERSION`, "utf8"),
			"VERSION",
		);
		const packageJson = readJson(`${rootDir}/package.json`);
		const packageVersion = parseVersion(
			packageJson.version,
			"package.json version",
		);
		if (packageVersion !== version) {
			issues.push(
				`package.json version ${packageVersion} does not match VERSION ${version}`,
			);
		}
		if (version !== declared) {
			issues.push(
				`VERSION ${version} does not match .github/versions/main.yaml ${declared}`,
			);
		}
	}

	const tag = String(releaseTag ?? "").trim();
	if (tag !== "") {
		const tagVersion = normalizeTag(tag, comp);
		if (tagVersion !== declared) {
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
	const releaseTag = isTagEvent
		? (process.argv[3] ?? process.env.RELEASE_TAG ?? process.env.GITHUB_REF_NAME ?? "")
		: (process.argv[3] ?? process.env.RELEASE_TAG ?? "");
	const component =
		process.argv[4] ?? process.env.GOSPEAK_COMPONENT ?? undefined;

	let result;
	try {
		result = checkVersionState(rootDir, releaseTag, component);
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
