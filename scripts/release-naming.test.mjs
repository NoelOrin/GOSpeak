import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import test from "node:test";

import {
	artifactForTarget,
	artifactName,
	artifactUploadName,
	checksumName,
	parseReleaseTag,
	releaseTag,
	tagPrefix,
} from "./release-naming.mjs";

const SCRIPT = fileURLToPath(new URL("./release-naming.mjs", import.meta.url));

test("server tag carries the component and a v prefix", () => {
	assert.equal(releaseTag("server", "1.0.0"), "gospeak-server-v1.0.0");
	assert.equal(releaseTag("desktop", "1.0.0"), "gospeak-desktop-v1.0.0");
	assert.equal(tagPrefix("server"), "gospeak-server-v");
});

test("artifact names encode component, version, os and arch", () => {
	assert.equal(
		artifactName("server", "1.0.0", "linux", "amd64"),
		"gospeak-server-v1.0.0-linux-amd64",
	);
	assert.equal(
		artifactName("server", "1.0.0", "windows", "amd64", "exe"),
		"gospeak-server-v1.0.0-windows-amd64.exe",
	);
	assert.equal(
		artifactName("desktop", "1.0.0", "darwin", "arm64", "dmg"),
		"gospeak-desktop-v1.0.0-darwin-arm64.dmg",
	);
});

test("checksum file names never collide between components", () => {
	assert.equal(
		checksumName("server", "1.0.0"),
		"SHA256SUMS-gospeak-server-v1.0.0.txt",
	);
	assert.equal(
		checksumName("desktop", "1.0.0"),
		"SHA256SUMS-gospeak-desktop-v1.0.0.txt",
	);
	assert.notEqual(
		checksumName("server", "1.0.0"),
		checksumName("desktop", "1.0.0"),
	);
});

test("parseReleaseTag round-trips every component", () => {
	for (const component of ["server", "desktop"]) {
		assert.deepEqual(parseReleaseTag(releaseTag(component, "2.3.4-rc.1")), {
			component,
			version: "2.3.4-rc.1",
		});
	}
});

test("parseReleaseTag rejects legacy and malformed tags", () => {
	for (const tag of [
		"0.4.0",
		"v0.3.1",
		"desktop-v1.0.0",
		"gospeak-server-1.0.0",
		"gospeak-v1.0.0",
		"",
	]) {
		assert.equal(
			parseReleaseTag(tag),
			null,
			`expected null for ${JSON.stringify(tag)}`,
		);
	}
});

test("desktop does not shadow server when parsing prefixes", () => {
	assert.deepEqual(parseReleaseTag("gospeak-desktop-v1.0.0"), {
		component: "desktop",
		version: "1.0.0",
	});
	assert.deepEqual(parseReleaseTag("gospeak-server-v1.0.0"), {
		component: "server",
		version: "1.0.0",
	});
});

test("upload artifact names stay stable across versions", () => {
	assert.equal(
		artifactUploadName("server", "linux-amd64"),
		"gospeak-server-linux-amd64",
	);
	assert.equal(
		artifactUploadName("desktop", "darwin-arm64"),
		"gospeak-desktop-darwin-arm64",
	);
});

test("artifactForTarget preserves the ARMv7 version bit that goarch loses", () => {
	// goarch 对 ARMv7 只是 "arm"；用 goos+goarch 拼会得到 linux-arm，
	// 与 linux-arm64 只差一个后缀，极易混淆。target 保留 v7。
	assert.equal(
		artifactForTarget("server", "1.0.0", "linux-armv7"),
		"gospeak-server-v1.0.0-linux-armv7",
	);
	assert.notEqual(
		artifactForTarget("server", "1.0.0", "linux-armv7"),
		artifactForTarget("server", "1.0.0", "linux-arm64"),
	);
	assert.equal(
		artifactForTarget("server", "1.0.0", "windows-amd64", ".exe"),
		"gospeak-server-v1.0.0-windows-amd64.exe",
	);
});

test("artifactForTarget rejects an empty target", () => {
	assert.throws(
		() => artifactForTarget("server", "1.0.0", ""),
		/non-empty target/,
	);
});

test("rejects unknown components and invalid versions", () => {
	assert.throws(() => releaseTag("web", "1.0.0"), /unknown component/);
	assert.throws(() => releaseTag("server", "dev"), /invalid SemVer/);
	assert.throws(
		() => artifactName("server", "1.0", "linux", "amd64"),
		/invalid SemVer/,
	);
});

test("CLI emits the same values the module computes", () => {
	const run = (...args) =>
		spawnSync(process.execPath, [SCRIPT, ...args], { encoding: "utf8" });

	const tag = run("tag", "server", "1.0.0");
	assert.equal(tag.status, 0, tag.stderr);
	assert.equal(tag.stdout.trim(), "gospeak-server-v1.0.0");

	const parsed = run("parse", "gospeak-desktop-v1.0.0");
	assert.equal(parsed.status, 0, parsed.stderr);
	assert.equal(parsed.stdout.trim(), "desktop 1.0.0");

	const legacy = run("parse", "0.4.0");
	assert.equal(legacy.status, 1);
});
