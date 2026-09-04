import { readFileSync } from "node:fs";
import { pathToFileURL } from "node:url";

const SEMVER_PATTERN = /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$/;

function readJson(path) {
	return JSON.parse(readFileSync(path, "utf8"));
}

function parseVersion(value, source) {
	const version = String(value ?? "").trim();
	if (!SEMVER_PATTERN.test(version)) {
		throw new Error(`${source} must contain a valid SemVer, got ${JSON.stringify(version)}`);
	}
	return version;
}

function normalizeTag(tag) {
	const value = String(tag ?? "").trim();
	return value.startsWith("v") ? value.slice(1) : value;
}

export function checkVersionState(rootDir, releaseTag = "") {
	const version = parseVersion(readFileSync(`${rootDir}/VERSION`, "utf8"), "VERSION");
	const packageJson = readJson(`${rootDir}/package.json`);
	const packageVersion = parseVersion(packageJson.version, "package.json version");
	const issues = [];

	if (packageVersion !== version) {
		issues.push(`package.json version ${packageVersion} does not match VERSION ${version}`);
	}

	const tag = String(releaseTag ?? "").trim();
	if (tag !== "") {
		const tagVersion = parseVersion(normalizeTag(tag), "release tag");
		if (tagVersion !== version) {
			issues.push(`release tag ${tag} does not match VERSION ${version}`);
		}
	}

	return { version, issues };
}

function main() {
	const rootDir = process.argv[2] ?? process.cwd();
	const isTagEvent = process.env.GITHUB_REF_TYPE === "tag";
	// 仅在显式传入 RELEASE_TAG 或当前为 tag 事件时，才把 ref 当作发布标签校验。
	// push 到分支时 GITHUB_REF_NAME 是分支名（如 "release"），并非合法 SemVer，
	// 此时不应把它当作发布标签去校验，否则会出现假阳性失败。
	const releaseTag = isTagEvent
		? (process.argv[3] ?? process.env.RELEASE_TAG ?? process.env.GITHUB_REF_NAME ?? "")
		: (process.argv[3] ?? process.env.RELEASE_TAG ?? "");
	const result = checkVersionState(rootDir, releaseTag);

	if (result.issues.length > 0) {
		for (const issue of result.issues) {
			console.error(`::error::${issue}`);
		}
		process.exitCode = 1;
		return;
	}

	console.log(`version consistency ok: ${result.version}`);
}

if (import.meta.url === pathToFileURL(process.argv[1]).href) {
	main();
}
