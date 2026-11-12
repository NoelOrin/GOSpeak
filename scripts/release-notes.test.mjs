import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import test from "node:test";

import {
	desktopArtifacts,
	renderDesktopBody,
	renderDesktopTable,
	renderServerBody,
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

test("server body puts desktop downloads above the release notes", () => {
	const body = renderServerBody({
		version: "1.0.0",
		repo: REPO,
		desktopVersion: "1.0.0",
		desktopReleaseExists: true,
		notes: "服务端变更内容",
	});

	const downloads = body.indexOf("## 下载 GOSpeak 桌面端");
	const notes = body.indexOf("## 服务端 release notes");
	assert.ok(downloads >= 0, "downloads section missing");
	assert.ok(notes >= 0, "server notes section missing");
	assert.ok(downloads < notes, "downloads must come before release notes");

	// 服务端 release notes 必须是最后一段（分隔线之后）。
	assert.ok(body.indexOf("---") < notes);
	assert.ok(body.trimEnd().endsWith("服务端变更内容"));
});

test("server body links every desktop artifact by absolute release URL", () => {
	const body = renderServerBody({
		version: "1.0.0",
		repo: REPO,
		desktopVersion: "1.0.0",
		desktopReleaseExists: true,
		notes: "",
	});

	const desktopTag = releaseTag("desktop", "1.0.0");
	for (const row of desktopArtifacts("1.0.0")) {
		const url = `https://github.com/${REPO}/releases/download/${desktopTag}/${row.primaryAsset}`;
		assert.ok(
			body.includes(url),
			`missing download url for ${row.primaryAsset}`,
		);
	}

	const sums = checksumName("desktop", "1.0.0");
	assert.ok(body.includes(`/releases/download/${desktopTag}/${sums}`));
});

test("server body never advertises server binaries in the download table", () => {
	const body = renderServerBody({
		version: "1.0.0",
		repo: REPO,
		desktopVersion: "1.0.0",
		desktopReleaseExists: true,
		notes: "",
	});

	const table = renderDesktopTable("1.0.0", REPO);
	assert.doesNotMatch(table, /gospeak-server-/);
	// 服务端二进制只以 Assets 说明的形式出现，不给直链。
	assert.doesNotMatch(body, /releases\/download\/[^)\s]*gospeak-server-/);
	assert.match(body, /Assets 列表/);
});

test("server body falls back to the releases page when desktop is absent", () => {
	const body = renderServerBody({
		version: "1.0.0",
		repo: REPO,
		desktopVersion: undefined,
		desktopReleaseExists: false,
		notes: "",
	});

	assert.match(body, /https:\/\/github\.com\/NoelOrin\/GOSpeak\/releases\)/);
	assert.doesNotMatch(body, /releases\/download/);
});

test("desktop body carries the same download table", () => {
	const body = renderDesktopBody({
		version: "2.0.0",
		repo: REPO,
		notes: "桌面端变更",
	});

	assert.match(body, /gospeak-desktop-v2\.0\.0-darwin-arm64\.dmg/);
	assert.match(body, /## 桌面端 release notes/);
	assert.ok(body.trimEnd().endsWith("桌面端变更"));
});

test("empty notes degrade to a readable placeholder", () => {
	const body = renderDesktopBody({
		version: "1.0.0",
		repo: REPO,
		notes: "   ",
	});
	assert.match(body, /本次发布无额外说明/);
});

test("CLI renders a server body and requires --repo", () => {
	const ok = spawnSync(
		process.execPath,
		[SCRIPT, "server", "--version", "1.0.0", "--repo", REPO],
		{ encoding: "utf8" },
	);
	assert.equal(ok.status, 0, ok.stderr);
	assert.match(ok.stdout, /## 下载 GOSpeak 桌面端/);
	assert.match(ok.stdout, /## 服务端 release notes/);

	const missing = spawnSync(
		process.execPath,
		[SCRIPT, "server", "--version", "1.0.0"],
		{
			encoding: "utf8",
		},
	);
	assert.equal(missing.status, 1);
	assert.match(missing.stderr, /--repo is required/);
});
