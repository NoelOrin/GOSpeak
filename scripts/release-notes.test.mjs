import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import test from "node:test";

import {
	desktopArtifacts,
	renderDesktopTable,
	renderReleaseBody,
} from "./release-notes.mjs";
import { checksumName, releaseTag } from "./release-naming.mjs";

const SCRIPT = fileURLToPath(new URL("./release-notes.mjs", import.meta.url));
const REPO = "NoelOrin/GOSpeak";

test("desktop artifacts follow the naming convention for every row", () => {
	for (const row of desktopArtifacts("1.0.0")) {
		assert.match(
			row.primaryAsset,
			/^gospeak-desktop-v1\.0\.0-(darwin|windows|linux)-(amd64|arm64)\.(dmg|deb|exe|tar\.gz)$/,
		);
		if (row.alternativeAsset) {
			assert.match(row.alternativeAsset, /^gospeak-desktop-v1\.0\.0-/);
		}
	}
});

test("single body carries both release notes sections", () => {
	const body = renderReleaseBody({
		version: "1.0.0",
		desktopVersion: "1.0.0",
		repo: REPO,
		serverNotes: "服务端变更",
		desktopNotes: "桌面端变更",
		details: "",
	});

	const serverIdx = body.indexOf("## 服务端 release notes");
	const desktopIdx = body.indexOf("## 桌面端 release notes");
	assert.ok(serverIdx >= 0, "server notes section missing");
	assert.ok(desktopIdx >= 0, "desktop notes section missing");
	assert.ok(serverIdx < desktopIdx, "server notes must precede desktop notes");
	assert.ok(body.includes("服务端变更"));
	assert.ok(body.includes("桌面端变更"));
});

test("download table comes before deployment and notes", () => {
	const body = renderReleaseBody({
		version: "1.0.0",
		desktopVersion: "1.0.0",
		repo: REPO,
		serverNotes: "",
		desktopNotes: "",
		details: "",
	});

	const downloads = body.indexOf("## 下载 GOSpeak 桌面端");
	const deploy = body.indexOf("## 部署 GOSpeak 服务端");
	const notes = body.indexOf("## 服务端 release notes");
	assert.ok(downloads >= 0 && deploy >= 0 && notes >= 0);
	assert.ok(downloads < deploy && deploy < notes);
});

test("desktop downloads point at the server tag in the merged release", () => {
	const body = renderReleaseBody({
		version: "1.0.1",
		desktopVersion: "1.0.1",
		repo: REPO,
		serverNotes: "",
		desktopNotes: "",
		details: "",
	});

	const serverTag = releaseTag("server", "1.0.1");
	const desktopTag = releaseTag("desktop", "1.0.1");
	for (const row of desktopArtifacts("1.0.1")) {
		assert.ok(
			body.includes(`/releases/download/${serverTag}/${row.primaryAsset}`),
			`missing merged-release url for ${row.primaryAsset}`,
		);
		assert.ok(
			!body.includes(`/releases/download/${desktopTag}/`),
			"merged body must not link to a separate desktop release",
		);
	}

	const sums = checksumName("desktop", "1.0.1");
	assert.ok(body.includes(`/releases/download/${serverTag}/${sums}`));
});

test("server binaries are never advertised as direct download links", () => {
	const body = renderReleaseBody({
		version: "1.0.0",
		desktopVersion: "1.0.0",
		repo: REPO,
		serverNotes: "",
		desktopNotes: "",
		details: "",
	});

	const table = renderDesktopTable("1.0.0", REPO);
	assert.doesNotMatch(table, /gospeak-server-/);
	// 注意不能只匹配 tag 名：合并发布后下载链接的 tag 就是 gospeak-server-v*，
	// 真正要拦的是「服务端二进制产物」被当成直链。
	assert.doesNotMatch(
		body,
		/releases\/download\/[^)\s]*\/gospeak-server-v[0-9.]+-(linux|darwin|freebsd|openbsd|windows)-/,
	);
	assert.match(body, /Assets 列表/);
});

test("commit details render as a collapsed details block", () => {
	const details = [
		"<details>",
		"<summary>提交记录（2 commits · 1 位贡献者）</summary>",
		"",
		"### 贡献者",
		"",
		"- **NoelOrin** — 2 commits",
		"",
		"</details>",
	].join("\n");

	const body = renderReleaseBody({
		version: "1.0.0",
		desktopVersion: "1.0.0",
		repo: REPO,
		serverNotes: "服务端",
		desktopNotes: "桌面端",
		details,
	});

	assert.ok(body.includes("<details>"));
	assert.ok(body.includes("</details>"));
	// 折叠块必须排在两段 release notes 之后，不能挤占发布说明。
	const desktopIdx = body.indexOf("## 桌面端 release notes");
	const detailsIdx = body.indexOf("<details>");
	assert.ok(desktopIdx < detailsIdx);
});

test("omitting details leaves no empty details block", () => {
	const body = renderReleaseBody({
		version: "1.0.0",
		desktopVersion: "1.0.0",
		repo: REPO,
		serverNotes: "",
		desktopNotes: "",
		details: "   ",
	});
	assert.doesNotMatch(body, /<details>/);
});

test("empty notes degrade to a readable placeholder", () => {
	const body = renderReleaseBody({
		version: "1.0.0",
		desktopVersion: "1.0.0",
		repo: REPO,
		serverNotes: "  ",
		desktopNotes: "",
		details: "",
	});
	const placeholders = body.match(/本次发布无额外说明/g) ?? [];
	assert.equal(placeholders.length, 2, "both notes sections need a fallback");
});

test("CLI renders the unified body and requires --repo", () => {
	const ok = spawnSync(
		process.execPath,
		[SCRIPT, "--version", "1.0.0", "--repo", REPO],
		{ encoding: "utf8" },
	);
	assert.equal(ok.status, 0, ok.stderr);
	assert.match(ok.stdout, /## 下载 GOSpeak 桌面端/);
	assert.match(ok.stdout, /## 服务端 release notes/);
	assert.match(ok.stdout, /## 桌面端 release notes/);

	const missing = spawnSync(process.execPath, [SCRIPT, "--version", "1.0.0"], {
		encoding: "utf8",
	});
	assert.equal(missing.status, 1);
	assert.match(missing.stderr, /--repo is required/);
});
