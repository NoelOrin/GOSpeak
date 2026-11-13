#!/usr/bin/env node
// 发布命名规范的单一事实来源。
//
// 规范：gospeak-<component>-v<semver>[-<os>-<arch>][.ext]
//
//   组件    tag                      二进制                              校验和
//   server  gospeak-server-v1.0.0     gospeak-server-v1.0.0-linux-amd64    SHA256SUMS-gospeak-server-v1.0.0.txt
//   desktop gospeak-desktop-v1.0.0    gospeak-desktop-v1.0.0-darwin-arm64.dmg
//                                                                        SHA256SUMS-gospeak-desktop-v1.0.0.txt
//
// CI（version-bump.yml / release.yml / docker.yml）、校验脚本（check-version.mjs）
// 与文档必须全部经由本模块派生名字，禁止在 workflow 里手拼字符串。
//
// 用法（CLI，供 workflow 取值）：
//   node scripts/release-naming.mjs tag server 1.0.0
//   node scripts/release-naming.mjs artifact desktop 1.0.0 darwin arm64 dmg
//   node scripts/release-naming.mjs checksum server 1.0.0
//   node scripts/release-naming.mjs parse gospeak-desktop-v1.0.0

import { pathToFileURL } from "node:url";

const COMPONENTS = ["server", "desktop"];

const SEMVER_PATTERN =
	/^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$/;

function assertComponent(component) {
	if (!COMPONENTS.includes(component)) {
		throw new Error(
			`unknown component ${JSON.stringify(component)}; expected one of ${COMPONENTS.join(", ")}`,
		);
	}
	return component;
}

function assertVersion(version) {
	const value = String(version ?? "").trim();
	if (!SEMVER_PATTERN.test(value)) {
		throw new Error(
			`invalid SemVer ${JSON.stringify(value)}; expected e.g. 1.0.0`,
		);
	}
	return value;
}

/** 发布 tag：gospeak-server-v1.0.0 / gospeak-desktop-v1.0.0 */
export function releaseTag(component, version) {
	return `gospeak-${assertComponent(component)}-v${assertVersion(version)}`;
}

/** tag 前缀（含尾随 v），用于 startsWith 判断：gospeak-server-v */
export function tagPrefix(component) {
	return `gospeak-${assertComponent(component)}-v`;
}

/**
 * 从 tag 反解 { component, version }。不是本规范的 tag 返回 null。
 * 历史裸版本号 tag（0.2.1 / v0.3.1）不属于本规范，一律返回 null。
 */
export function parseReleaseTag(tag) {
	const value = String(tag ?? "").trim();
	for (const component of COMPONENTS) {
		const prefix = tagPrefix(component);
		if (!value.startsWith(prefix)) continue;
		const version = value.slice(prefix.length);
		if (!SEMVER_PATTERN.test(version)) continue;
		return { component, version };
	}
	return null;
}

/**
 * 发布产物名：gospeak-<component>-v<version>-<os>-<arch>[.ext]
 * os/arch 缺省时只返回前缀部分（用于目录/通配）。
 */
export function artifactName(component, version, os, arch, ext) {
	const base = `${releaseTag(component, version)}${os ? `-${os}` : ""}${arch ? `-${arch}` : ""}`;
	const suffix = String(ext ?? "").trim();
	if (suffix === "") return base;
	return `${base}.${suffix.replace(/^\./, "")}`;
}

/**
 * 按构建矩阵的 target 命名产物：gospeak-<component>-v<version>-<target>[.ext]。
 *
 * 矩阵 target 已经是 `<os>-<arch>` 形式（linux-amd64 / linux-armv7 /
 * windows-arm64 ...）。必须用它而不是 goos+goarch：linux-armv7 的
 * goarch 只是 "arm"，拼出来会丢掉 v7 版本位，与 arm64 产物难以区分。
 */
export function artifactForTarget(component, version, target, ext) {
	const value = String(target ?? "").trim();
	if (value === "") {
		throw new Error("artifactForTarget requires a non-empty target");
	}
	return artifactName(component, version, value, "", ext);
}

/** 校验和文件名：SHA256SUMS-gospeak-server-v1.0.0.txt */
export function checksumName(component, version) {
	return `SHA256SUMS-${releaseTag(component, version)}.txt`;
}

/** 发布产物的 artifact 上传名（GitHub Actions artifact，不含版本以免跨版本冲突） */
export function artifactUploadName(component, target) {
	return `gospeak-${assertComponent(component)}-${target}`;
}

export const RELEASE_COMPONENTS = COMPONENTS;

function main() {
	const [command, ...rest] = process.argv.slice(2);
	const emit = (value) => {
		if (typeof value !== "string") {
			throw new Error(`no value produced for command ${command}`);
		}
		process.stdout.write(`${value}\n`);
	};

	switch (command) {
		case "tag":
			emit(releaseTag(rest[0], rest[1]));
			break;
		case "prefix":
			emit(tagPrefix(rest[0]));
			break;
		case "artifact":
			emit(artifactName(rest[0], rest[1], rest[2], rest[3], rest[4]));
			break;
		case "artifact-target":
			emit(artifactForTarget(rest[0], rest[1], rest[2], rest[3]));
			break;
		case "checksum":
			emit(checksumName(rest[0], rest[1]));
			break;
		case "upload-name":
			emit(artifactUploadName(rest[0], rest[1]));
			break;
		case "parse": {
			const parsed = parseReleaseTag(rest[0]);
			if (!parsed) {
				process.stderr.write(
					`not a release tag under the naming convention: ${JSON.stringify(rest[0])}\n`,
				);
				process.exitCode = 1;
				return;
			}
			emit(`${parsed.component} ${parsed.version}`);
			break;
		}
		default:
			process.stderr.write(
				`usage: release-naming.mjs <tag|prefix|artifact|artifact-target|checksum|upload-name|parse> ...\n`,
			);
			process.exitCode = 2;
	}
}

if (
	process.argv[1] &&
	import.meta.url === pathToFileURL(process.argv[1]).href
) {
	try {
		main();
	} catch (err) {
		process.stderr.write(
			`::error::${err instanceof Error ? err.message : err}\n`,
		);
		process.exitCode = 1;
	}
}
