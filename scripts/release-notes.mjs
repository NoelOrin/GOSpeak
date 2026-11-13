#!/usr/bin/env node
// GitHub Release 正文生成器 —— 两个组件只发一个 Release（挂在服务端 tag 上）。
//
// 正文结构：
//   1. 顶部「桌面端下载表格」：普通用户点一下就拿到 dmg / deb / exe，
//      链接指向本 Release 自己的 tag（桌面端安装包就在同一页 Assets 里）
//   2. 服务端部署指引（走镜像 + 文档，不给二进制直链）
//   3. 分隔线之后分两段：「服务端 release notes」与「桌面端 release notes」，
//      分别取自 .github/versions/{server,desktop}.yaml 的 notes 字段
//   4. 末尾可折叠的「提交记录」：贡献者概览 + 提交明细，默认收起
//
// 桌面端不再单独建 Release；`gospeak-desktop-v*` tag 仍会创建，仅用于
// changelog 基线（见 scripts/build-changelog.mjs 的 latestTag）。
//
// 服务端二进制不写进正文表格，而是作为 release 附件挂在 Assets 列表里；
// 桌面端安装包既挂在 Assets 里，也在正文表格中给出显式下载按钮
// （正文表格是给普通用户看的，Assets 列表是给运维/脚本取的）。
//
// 所有产物名由 scripts/release-naming.mjs 派生，本文件不手拼文件名。
//
// 用法：
//   node scripts/release-notes.mjs --version 1.0.0 --repo o/r \
//        --server-notes-file a.md --desktop-notes-file b.md --details-file c.md
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

/**
 * 桌面端下载表格（Markdown）。
 *
 * releaseTag 参数是下载链接所指向的 tag。合并发布后桌面端安装包挂在
 * 服务端 Release 上，所以调用方传入服务端 tag；不传时保持旧行为
 * （指向 gospeak-desktop-v* tag），供历史页面与测试使用。
 */
export function renderDesktopTable(version, repo, releaseTagArg) {
	const tag = releaseTagArg ?? releaseTag("desktop", version);
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
 * 统一的 Release 正文 —— 两个组件只发一个 Release，挂在服务端 tag 上。
 *
 * 结构（自上而下）：
 *   1. 桌面端下载表格（普通用户第一需求）；链接指向本 Release 自己的 tag，
 *      因为桌面端安装包这次就挂在同一个 Release 的 Assets 里
 *   2. 服务端部署指引
 *   3. 分隔线 + 「服务端 release notes」与「桌面端 release notes」两段，
 *      各自取自 .github/versions/<component>.yaml 的 notes 字段
 *   4. 可折叠的提交记录（贡献者概览 + 提交明细），默认收起
 *
 * 桌面端不再单独建 Release，因此这里没有「桌面端 release 是否已存在」的分支：
 * 下载链接始终可用。
 */
export function renderReleaseBody({
	version,
	desktopVersion,
	repo,
	serverNotes,
	desktopNotes,
	details,
}) {
	const tag = releaseTag("server", version);
	const parts = [];

	parts.push("## 下载 GOSpeak 桌面端");
	parts.push("");
	parts.push(
		"> [!TIP]",
		`> 服务端与桌面端同为 \`${version}\`，安装包与该版本服务端二进制都在本页 Assets 中。`,
	);
	parts.push("");
	parts.push(renderDesktopTable(desktopVersion, repo, tag));
	parts.push("");
	parts.push(renderInstallNotes());
	parts.push("");
	parts.push(renderDeploySection(repo, tag));
	parts.push("");
	parts.push("---");
	parts.push("");
	parts.push("## 服务端 release notes");
	parts.push("");
	parts.push((serverNotes ?? "").trim() || "* 本次发布无额外说明。");
	parts.push("");
	parts.push("## 桌面端 release notes");
	parts.push("");
	parts.push((desktopNotes ?? "").trim() || "* 本次发布无额外说明。");
	parts.push("");

	if ((details ?? "").trim()) {
		parts.push("---");
		parts.push("");
		parts.push(details.trim());
		parts.push("");
	}

	return parts.join("\n");
}

function parseArgs(argv) {
	const options = {};
	for (let i = 0; i < argv.length; i += 1) {
		const arg = argv[i];
		if (!arg.startsWith("--")) {
			throw new Error(
				`unexpected argument ${JSON.stringify(arg)}; options are --key value pairs`,
			);
		}
		const key = arg.slice(2);
		const value = argv[i + 1];
		if (value === undefined || value.startsWith("--")) {
			throw new Error(`option --${key} requires a value`);
		}
		options[key] = value;
		i += 1;
	}
	return options;
}

function readMaybe(path) {
	return path ? readFileSync(path, "utf8") : "";
}

function main() {
	const options = parseArgs(process.argv.slice(2));
	const { repo } = options;
	if (!repo) throw new Error("--repo is required");

	// 合并发布：客户端不再区分 component，--version 即服务端版本，
	// 桌面端版本缺省与之相同。
	const version = options.version;
	if (!version) throw new Error("--version is required");
	const desktopVersion = options["desktop-version"] || version;

	const body = renderReleaseBody({
		version,
		desktopVersion,
		repo,
		serverNotes: readMaybe(
			options["server-notes-file"] ?? options["notes-file"],
		),
		desktopNotes: readMaybe(options["desktop-notes-file"]),
		details: readMaybe(options["details-file"]),
	});

	process.stdout.write(body);
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
