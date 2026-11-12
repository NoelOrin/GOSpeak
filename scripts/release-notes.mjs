#!/usr/bin/env node
// GitHub Release 正文生成器 —— 对齐 SyncTV 的发布页形态：
//
//   1. 正文顶部是「桌面端下载表格」：用户点一下就拿到 dmg / deb / exe
//   2. 中间是部署指引（服务端走镜像 + 文档，不给二进制直链）
//   3. 分隔线之后是「服务端 release notes」
//
// 服务端二进制不写进正文表格，而是作为 release 附件挂在 Assets 列表里；
// 桌面端安装包既挂在 Assets 里，也在正文表格中给出显式下载按钮
// （正文表格是给普通用户看的，Assets 列表是给运维/脚本取的）。
//
// 所有产物名由 scripts/release-naming.mjs 派生，本文件不手拼文件名。
//
// 用法：
//   node scripts/release-notes.mjs server  --version 1.0.0 --repo o/r --desktop-version 1.0.0 \
//        [--desktop-release-exists] [--notes-file /tmp/decl-notes.txt]
//   node scripts/release-notes.mjs desktop --version 1.0.0 --repo o/r [--notes-file ...]
//
// 输出写到 stdout。

import { readFileSync } from "node:fs";
import { pathToFileURL } from "node:url";

import {
	artifactForTarget,
	checksumName,
	releaseTag,
} from "./release-naming.mjs";

const BADGE = (label, color, logo) =>
	`![Download ${label}](https://img.shields.io/badge/Download-${label}-${color}?style=for-the-badge&logo=${logo}&logoColor=white)`;

const PLATFORM_BADGE = {
	dmg: BADGE("DMG", "000000", "apple"),
	deb: BADGE("DEB", "FCC624", "linux"),
	exe: BADGE("EXE", "0078D4", "windows11"),
};

function downloadUrl(repo, tag, asset) {
	return `https://github.com/${repo}/releases/download/${tag}/${asset}`;
}

/**
 * 桌面端可下载产物清单。os/arch/ext 与 release.yml 的构建矩阵一一对应，
 * 改矩阵时必须同步这里（单测会校验文件名仍符合命名规范）。
 */
export function desktopArtifacts(version) {
	return [
		{
			platform: "macOS",
			device: "Apple Silicon",
			bestFor: "M 系列 Mac",
			primary: { ext: "dmg", target: "darwin-arm64" },
			alternative: null,
		},
		{
			platform: "macOS",
			device: "Intel",
			bestFor: "Intel 芯片 Mac",
			primary: { ext: "dmg", target: "darwin-amd64" },
			alternative: null,
		},
		{
			platform: "Windows",
			device: "x64",
			bestFor: "64 位 Windows 10 / 11",
			primary: { ext: "exe", target: "windows-amd64" },
			alternative: null,
		},
		{
			platform: "Linux",
			device: "x64",
			bestFor: "Intel / AMD 的 Debian、Ubuntu",
			primary: { ext: "deb", target: "linux-amd64" },
			alternative: { ext: "tar.gz", target: "linux-amd64" },
		},
		{
			platform: "Linux",
			device: "arm64",
			bestFor: "ARM64 服务器与开发板",
			primary: { ext: "deb", target: "linux-arm64" },
			alternative: { ext: "tar.gz", target: "linux-arm64" },
		},
	].map((row) => ({
		...row,
		primaryAsset: artifactForTarget(
			"desktop",
			version,
			row.primary.target,
			row.primary.ext,
		),
		alternativeAsset: row.alternative
			? artifactForTarget(
					"desktop",
					version,
					row.alternative.target,
					row.alternative.ext,
				)
			: null,
	}));
}

function renderCell(ext, repo, tag, asset) {
	const badge =
		PLATFORM_BADGE[ext] ?? BADGE(ext.toUpperCase(), "24292F", "github");
	return `[${badge}](${downloadUrl(repo, tag, asset)})<br>\`${asset}\``;
}

/** 桌面端下载表格（Markdown）。 */
export function renderDesktopTable(version, repo) {
	const tag = releaseTag("desktop", version);
	const rows = desktopArtifacts(version).map((row) => {
		const recommended = renderCell(
			row.primary.ext,
			repo,
			tag,
			row.primaryAsset,
		);
		const alternative = row.alternativeAsset
			? `[Portable ${row.alternative.ext.toUpperCase()}](${downloadUrl(repo, tag, row.alternativeAsset)})`
			: "-";
		return `| **${row.platform}** | ${row.device} | ${recommended} | ${alternative} | ${row.bestFor} |`;
	});

	const sums = checksumName("desktop", version);
	rows.push(
		`| **校验和** | 全部文件 | [${BADGE("SHA-256", "24292F", "github")}](${downloadUrl(repo, tag, sums)})<br>\`${sums}\` | - | 完整性校验 |`,
	);

	return [
		"| 平台 | 架构 | 推荐下载 | 备选 | 适用 |",
		"|:---|:---|:---:|:---|:---|",
		...rows,
	].join("\n");
}

function renderInstallNotes() {
	return [
		"### 安装说明",
		"",
		"- macOS 分 Apple Silicon 与 Intel 两个版本，下错架构打不开。",
		"- macOS 安装包未签名，首次打开如被拦截，去「系统设置 → 隐私与安全性」点「仍要打开」。",
		"- Windows 提供 EXE 安装包，双击安装。",
		"- Linux 优先用 `.deb`（双击即装），`.tar.gz` 为免安装便携包。",
		"- `SHA256SUMS` 文件可用于校验下载完整性。",
	].join("\n");
}

function renderDeploySection(repo, tag) {
	return [
		"## 部署 GOSpeak 服务端",
		"",
		"| 方式 | 文档 |",
		"|:---|:---|",
		`| **Docker Compose** | [在服务器 / NAS / 虚拟机上部署](https://github.com/${repo}/blob/${tag}/deploy/DEPLOY.md) |`,
		`| **容器镜像** | \`ghcr.io/${repo.toLowerCase()}:${tag.replace(/^gospeak-server-v/, "")}\` |`,
		`| **二进制** | 见本页底部 Assets 列表（按 \`gospeak-server-\` 前缀） |`,
		"",
		`[浏览 ${tag} 的部署文档](https://github.com/${repo}/tree/${tag}/deploy)`,
	].join("\n");
}

/**
 * 服务端 release 正文：桌面端下载表格 → 部署指引 → 分隔线 → 服务端 release notes。
 *
 * desktopReleaseExists=false 时不渲染下载表格，只给 Releases 页链接，
 * 避免指向一个还不存在的 release 造成 404。
 */
export function renderServerBody({
	version,
	repo,
	desktopVersion,
	desktopReleaseExists,
	notes,
}) {
	const parts = [];

	parts.push("## 下载 GOSpeak 桌面端");
	parts.push("");
	if (desktopReleaseExists && desktopVersion) {
		const desktopTag = releaseTag("desktop", desktopVersion);
		parts.push(
			`> [!TIP]`,
			`> 桌面端 \`${desktopTag}\` 与服务端 \`${releaseTag("server", version)}\` 配套发布。`,
			`> [查看完整桌面端发布](https://github.com/${repo}/releases/tag/${desktopTag})`,
		);
		parts.push("");
		parts.push(renderDesktopTable(desktopVersion, repo));
	} else {
		parts.push(
			`桌面端安装包见 [Releases](https://github.com/${repo}/releases) 页面中 \`gospeak-desktop-v\` 开头的版本。`,
		);
	}
	parts.push("");
	parts.push(renderInstallNotes());
	parts.push("");
	parts.push(renderDeploySection(repo, releaseTag("server", version)));
	parts.push("");
	parts.push("---");
	parts.push("");
	parts.push("## 服务端 release notes");
	parts.push("");
	parts.push((notes ?? "").trim() || "* 本次发布无额外说明。");
	parts.push("");

	return parts.join("\n");
}

/** 桌面端 release 正文：下载表格 → 安装说明 → 桌面端 release notes。 */
export function renderDesktopBody({ version, repo, notes }) {
	return [
		"## 下载 GOSpeak 桌面端",
		"",
		renderDesktopTable(version, repo),
		"",
		renderInstallNotes(),
		"",
		"---",
		"",
		"## 桌面端 release notes",
		"",
		(notes ?? "").trim() || "* 本次发布无额外说明。",
		"",
	].join("\n");
}

function parseArgs(argv) {
	const [component, ...rest] = argv;
	const options = { component };
	for (let i = 0; i < rest.length; i += 1) {
		const arg = rest[i];
		if (!arg.startsWith("--")) continue;
		const key = arg.slice(2);
		if (key === "desktop-release-exists") {
			options.desktopReleaseExists = true;
			continue;
		}
		const value = rest[i + 1];
		if (value === undefined || value.startsWith("--")) {
			throw new Error(`option --${key} requires a value`);
		}
		options[key] = value;
		i += 1;
	}
	return options;
}

function main() {
	const options = parseArgs(process.argv.slice(2));
	const { component, version, repo } = options;
	if (component !== "server" && component !== "desktop") {
		throw new Error(
			`component must be "server" or "desktop", got ${JSON.stringify(component)}`,
		);
	}
	if (!version) throw new Error("--version is required");
	if (!repo) throw new Error("--repo is required");

	const notes = options["notes-file"]
		? readFileSync(options["notes-file"], "utf8")
		: "";

	const body =
		component === "server"
			? renderServerBody({
					version,
					repo,
					desktopVersion: options["desktop-version"],
					desktopReleaseExists: options.desktopReleaseExists === true,
					notes,
				})
			: renderDesktopBody({ version, repo, notes });

	process.stdout.write(body);
}

if (import.meta.url === pathToFileURL(process.argv[1]).href) {
	try {
		main();
	} catch (err) {
		process.stderr.write(
			`::error::${err instanceof Error ? err.message : err}\n`,
		);
		process.exitCode = 1;
	}
}
