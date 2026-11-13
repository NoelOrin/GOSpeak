#!/usr/bin/env node
// 生成指定版本的 CHANGELOG 段落，并幂等地插入到 CHANGELOG.md 顶部（标题之下）。
// 用法：node scripts/build-changelog.mjs <version> [lastTag] [date] [--no-write] [--replace] [--exclude-ci] [--server|--desktop]
//   version  必填，形如 X.Y.Z（可带 -pre/+build）
//   lastTag  可选，比较基线；缺省按组件取最近 tag（--server/--desktop 决定），
//            未指定组件时取仓库最近 tag
//   date     可选，段落日期；缺省今天（YYYY-MM-DD）
// 默认写回 CHANGELOG.md 并把生成的段落打印到 stdout；--no-write 只打印段落。
// --exclude-ci 仅本次生成排除 CI 及发布自动化相关提交，默认不排除。

import fs from "node:fs";
import { execSync as ex } from "node:child_process";
import { pathToFileURL } from "node:url";

import { tagPrefix } from "./release-naming.mjs";

const REPO = "https://github.com/NoelOrin/GOSpeak";
const CHANGELOG_PATH = "CHANGELOG.md";

// type -> 展示 section 标题，与历史 release-please 的 changelog-sections 对齐
const SECTION_OF = {
	feat: "Features",
	fix: "Bug Fixes",
	perf: "Performance",
	docs: "Documentation",
	ci: "CI/CD",
	refactor: "Refactoring",
	style: "Styles",
	test: "Tests",
	build: "Build",
	chore: "Chores",
	revert: "Reverts",
};
const SECTION_ORDER = [
	"Features",
	"Bug Fixes",
	"Performance",
	"Documentation",
	"CI/CD",
	"Refactoring",
	"Styles",
	"Tests",
	"Build",
	"Chores",
	"Reverts",
];

const CC_PATTERN =
	/^(?<type>[a-zA-Z]+)(?:\((?<scope>[^)]*)\))?(?<break>!)?:[ \t]+(?<desc>.*)$/;
const CI_SUBJECT_PATTERNS = [
	/\bci(?:\/cd)?\b/i,
	/\brelease-please\b/i,
	/\bgithub actions?\b/i,
	/\bworkflows?\b/i,
	/\bcross[- ]compile(?:d)?\b/i,
	/\bversion consistency\b/i,
	/\brelease workflow\b/i,
	/\bcreate-release\b/i,
	/\bpublish release\b/i,
	/\bdependabot\b/i,
	/发版流程|发布流程|发版|发布时|版本一致性/,
];

function isCIRelated(commit, match) {
	const type = match?.groups?.type?.toLowerCase();
	const scope = match?.groups?.scope?.toLowerCase();
	if (
		type === "ci" ||
		type === "release" ||
		scope === "ci" ||
		scope === "release"
	)
		return true;
	if (CI_SUBJECT_PATTERNS.some((pattern) => pattern.test(commit.subject)))
		return true;

	const changedFiles = sh(
		`git diff-tree --no-commit-id --name-only -r --root ${commit.hash}`,
	)
		.split("\n")
		.filter(Boolean);
	return changedFiles.some(
		(file) =>
			file.startsWith(".github/workflows/") ||
			file.startsWith(".github/actions/") ||
			file === "lefthook.yml" ||
			file === "commitlint.config.js",
	);
}

function sh(cmd) {
	try {
		return ex(cmd, { encoding: "utf-8" }).trim();
	} catch {
		return "";
	}
}

function latestTag(component) {
	// 按组件取最近 tag：server 只认 gospeak-server-v*，desktop 只认
	// gospeak-desktop-v*。不区分组件会让桌面端拿主体 tag 当基线，
	// 把主体提交历史错算成桌面端变更。
	const tags = sh("git tag --list --sort=-creatordate")
		.split("\n")
		.map((t) => t.trim())
		.filter(Boolean);
	if (!component) {
		return tags[0] || "";
	}
	const prefix = tagPrefix(component);
	return tags.find((t) => t.startsWith(prefix)) || "";
}

function collectCommits(range) {
	// 每条提交：hash \x1f subject \x1f body \x1f authorName \x1f authorEmail
	// authorEmail 用于把同一人的多个 Git 身份归并成一行贡献者。
	const log = sh(
		`git log ${range} --pretty=format:'%H%x1f%s%x1f%b%x1f%an%x1f%ae%x1e'`,
	);
	if (!log) return [];
	const out = [];
	for (const entry of log.split("\x1e")) {
		if (!entry.trim()) continue;
		const [hash, subject, body = "", authorName = "", authorEmail = ""] =
			entry.split("\x1f");
		out.push({
			hash: hash.trim().slice(0, 7),
			subject: subject.trim(),
			body: body.trim(),
			authorName: authorName.trim(),
			authorEmail: authorEmail.trim(),
		});
	}
	return out;
}

function classify(commits, excludeCI = false) {
	// section -> [{ scope, desc }]
	const map = {};
	for (const sec of SECTION_ORDER) map[sec] = [];

	for (const c of commits) {
		const m = c.subject.match(CC_PATTERN);
		if (excludeCI && isCIRelated(c, m)) continue;
		let section = "Chores";
		let scope = "";
		let desc = c.subject;
		if (m && m.groups) {
			const type = m.groups.type.toLowerCase();
			scope = m.groups.scope || "";
			desc = m.groups.desc || c.subject;
			section = SECTION_OF[type] || "Chores";
		}
		map[section].push({ scope, desc });
	}
	return map;
}

function buildSection(version, lastTag, date, commits, excludeCI = false) {
	const compare = lastTag ? `${lastTag}...${version}` : `HEAD...${version}`;
	const lines = [];
	lines.push(`## [${version}](${REPO}/compare/${compare}) (${date})`);
	lines.push("");
	const map = classify(commits, excludeCI);
	let any = false;
	for (const sec of SECTION_ORDER) {
		const items = map[sec];
		if (!items.length) continue;
		any = true;
		lines.push(`### ${sec}`);
		lines.push("");
		for (const it of items) {
			const scoped = it.scope ? `${it.scope}: ` : "";
			lines.push(`* ${scoped}${it.desc}`);
		}
		lines.push("");
	}
	if (!any) {
		lines.push("* 无分类变更");
		lines.push("");
	}
	return lines.join("\n").replace(/\n+$/, "\n");
}

/**
 * 把同一人的多个 Git 身份归并成一个贡献者。
 *
 * 同一个人常见两种身份：本地提交（自定义邮箱）与 GitHub 网页/合并提交
 * （`<id>+<login>@users.noreply.github.com`）。这里按「noreply 里的 login，
 * 否则邮箱本地部分，否则作者名」取归并键，大小写不敏感。
 */
function contributorKey(commit) {
	const email = (commit.authorEmail || "").toLowerCase();
	const noreply = email.match(
		/^(?:\d+\+)?([^@]+)@users\.noreply\.github\.com$/,
	);
	if (noreply) return noreply[1];
	const local = email.split("@")[0];
	if (local) return local;
	return (commit.authorName || "").toLowerCase();
}

/** noreply 邮箱里带出的 GitHub login，用作规范展示名。 */
function githubLogin(commit) {
	return (
		(commit.authorEmail || "").match(
			/^(?:\d+\+)?([^@]+)@users\.noreply\.github\.com$/i,
		)?.[1] ?? ""
	);
}

function titleCase(value) {
	return value ? value.charAt(0).toUpperCase() + value.slice(1) : value;
}

/**
 * 贡献者汇总：按归并键聚合提交数，按提交数降序。
 *
 * 同一人的展示名统一取 GitHub login（首字母大写）；找不到 login 时退回
 * 该作者最早一次提交的 authorName，避免同一人在不同提交里名字大小写不一
 * 导致展示跳动。
 */
export function collectContributors(commits) {
	const byKey = new Map();
	for (const c of commits) {
		const key = contributorKey(c);
		const login = githubLogin(c);
		const prev = byKey.get(key);
		if (prev) {
			prev.commits += 1;
			if (!prev.login && login) prev.login = login;
			continue;
		}
		byKey.set(key, {
			key,
			login,
			authorName: c.authorName || "",
			commits: 1,
		});
	}
	return [...byKey.values()]
		.map((c) => ({
			key: c.key,
			name: titleCase(c.login) || c.authorName || c.key,
			commits: c.commits,
		}))
		.sort((a, b) => b.commits - a.commits || a.name.localeCompare(b.name));
}

/** 提交明细里展示的类型标签；未带 Conventional Commits 前缀时留空。 */
function commitType(subject) {
	const m = subject.match(CC_PATTERN);
	return m?.groups?.type ? m.groups.type.toLowerCase() : "";
}

/**
 * 可折叠的「提交记录」块。
 *
 * GitHub Release 正文里 `<details>` 默认折叠，把 30 行提交明细收起来，
 * 正文只留贡献者概览，避免把发布说明挤成流水账。
 * merge commit 与发版自动化提交不列入明细。
 */
export function buildDetailsBlock(commits, { excludeCI = false } = {}) {
	const meaningful = commits.filter((c) => {
		if (/^Merge (pull request|branch)\b/.test(c.subject)) return false;
		const m = c.subject.match(CC_PATTERN);
		if (excludeCI && isCIRelated(c, m)) return false;
		// 版本号提升提交本身不属于功能变更。
		if (
			m?.groups?.type?.toLowerCase() === "chore" &&
			/release/i.test(m.groups.scope ?? "")
		) {
			return false;
		}
		return true;
	});

	const contributors = collectContributors(meaningful);
	const total = meaningful.length;
	if (total === 0) return "";

	const lines = [];
	lines.push("<details>");
	lines.push(
		`<summary>提交记录（${total} commits · ${contributors.length} 位贡献者）</summary>`,
	);
	lines.push("");
	lines.push("### 贡献者");
	lines.push("");
	for (const c of contributors) {
		lines.push(`- **${c.name}** — ${c.commits} commits`);
	}
	lines.push("");
	lines.push("### 提交明细");
	lines.push("");
	lines.push("| Commit | 类型 | 说明 | 作者 |");
	lines.push("|:---|:---|:---|:---|");
	for (const c of meaningful) {
		const type = commitType(c.subject);
		const desc = (
			c.subject.match(CC_PATTERN)?.groups?.desc ?? c.subject
		).replace(/\|/g, "\\|");
		const author = (c.authorName || "-").replace(/\|/g, "\\|");
		lines.push(`| \`${c.hash}\` | ${type || "-"} | ${desc} | ${author} |`);
	}
	lines.push("");
	lines.push("</details>");

	return lines.join("\n");
}

function insertChangelog(newBlock, version, replaceExisting = false) {
	if (!fs.existsSync(CHANGELOG_PATH)) {
		fs.writeFileSync(CHANGELOG_PATH, `# Changelog / 更新日志\n\n${newBlock}`);
		return CHANGELOG_PATH;
	}
	let content = fs.readFileSync(CHANGELOG_PATH, "utf-8");
	const versionHeading = `## [${version}]`;
	if (replaceExisting) {
		const sections = content.split(/(?=^## )/m);
		const filtered = sections.filter(
			(section) => !section.startsWith(versionHeading),
		);
		const firstVersionIndex = filtered.findIndex((section) =>
			section.startsWith("## "),
		);
		filtered.splice(
			firstVersionIndex < 0 ? filtered.length : firstVersionIndex,
			0,
			newBlock,
		);
		const replaced = filtered.join("");
		if (replaced !== content) {
			fs.writeFileSync(CHANGELOG_PATH, replaced);
			return CHANGELOG_PATH;
		}
	}
	// 幂等：若已含该版本标题则跳过插入
	if (content.includes(versionHeading)) {
		return CHANGELOG_PATH;
	}
	const headerMatch = content.match(/^#\s+Changelog[^\n]*\n/);
	if (headerMatch) {
		const idx = headerMatch[0].length;
		content = content.slice(0, idx) + "\n" + newBlock + content.slice(idx);
	} else {
		content = `# Changelog / 更新日志\n\n${newBlock}${content}`;
	}
	fs.writeFileSync(CHANGELOG_PATH, content);
	return CHANGELOG_PATH;
}

function main() {
	const args = process.argv.slice(2);
	const write = !args.includes("--no-write");
	const replaceExisting = args.includes("--replace");
	const excludeCI = args.includes("--exclude-ci");
	const emitDetails = args.includes("--details");
	// --details-only 只输出可折叠的提交记录块（Release 正文用），
	// 不输出 buildSection 的分类清单、也不写回 CHANGELOG.md。
	const detailsOnly = args.includes("--details-only");
	const positional = args.filter(
		(arg) =>
			arg !== "--no-write" &&
			arg !== "--replace" &&
			arg !== "--exclude-ci" &&
			arg !== "--details" &&
			arg !== "--details-only" &&
			arg !== "--server" &&
			arg !== "--desktop",
	);
	const component = args.includes("--desktop")
		? "desktop"
		: args.includes("--server")
			? "server"
			: undefined;
	const version = positional[0];
	if (!version) {
		console.error(
			"usage: node scripts/build-changelog.mjs <version> [lastTag] [date] [--no-write] [--replace] [--exclude-ci]",
		);
		process.exit(1);
	}
	const lastTag = positional[1] || latestTag(component);
	const date = positional[2] || new Date().toISOString().slice(0, 10);
	const range = lastTag ? `${lastTag}..HEAD` : "HEAD";
	const commits = collectCommits(range);
	if (detailsOnly) {
		const only = buildDetailsBlock(commits, { excludeCI });
		if (only) process.stdout.write(`${only}\n`);
		console.error(
			`\n[build-changelog] details-only (version=${version}, base=${lastTag || "none"}, commits=${commits.length})`,
		);
		return;
	}

	const block = buildSection(version, lastTag, date, commits, excludeCI);
	if (write) {
		insertChangelog(block, version, replaceExisting);
	}
	process.stdout.write(block);
	if (emitDetails) {
		const details = buildDetailsBlock(commits, { excludeCI });
		if (details) {
			process.stdout.write(`\n\n${details}\n`);
		}
	}
	console.error(
		`\n[build-changelog] generated (write=${write}, version=${version}, base=${lastTag || "none"}, commits=${commits.length})`,
	);
}

if (
	process.argv[1] &&
	import.meta.url === pathToFileURL(process.argv[1]).href
) {
	main();
}
