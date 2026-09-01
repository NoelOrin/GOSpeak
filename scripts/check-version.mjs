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
	const manifest = readJson(`${rootDir}/.release-please-manifest.json`);
	const packageVersion = parseVersion(packageJson.version, "package.json version");
	const manifestVersion = parseVersion(manifest["."], ".release-please-manifest.json version");
	const issues = [];

	if (packageVersion !== version) {
		issues.push(`package.json version ${packageVersion} does not match VERSION ${version}`);
	}
	if (manifestVersion !== version) {
		issues.push(`.release-please-manifest.json version ${manifestVersion} does not match VERSION ${version}`);
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
	const releaseTag = process.argv[3] ?? process.env.RELEASE_TAG ?? process.env.GITHUB_REF_NAME ?? "";
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
