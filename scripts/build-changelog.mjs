#!/usr/bin/env node
// 生成指定版本的 CHANGELOG 段落，并幂等地插入到 CHANGELOG.md 顶部（标题之下）。
// 用法：node scripts/build-changelog.mjs <version> [lastTag] [date] [--no-write] [--replace] [--exclude-ci]
//   version  必填，形如 X.Y.Z（可带 -pre/+build）
//   lastTag  可选，比较基线；缺省取 git describe --tags --abbrev=0 的最近 tag
//   date     可选，段落日期；缺省今天（YYYY-MM-DD）
// 默认写回 CHANGELOG.md 并把生成的段落打印到 stdout；--no-write 只打印段落。
// --exclude-ci 仅本次生成排除 CI 及发布自动化相关提交，默认不排除。

import fs from "node:fs";
import { execSync as ex } from "node:child_process";

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

const CC_PATTERN = /^(?<type>[a-zA-Z]+)(?:\((?<scope>[^)]*)\))?(?<break>!)?:[ \t]+(?<desc>.*)$/;
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
  if (type === "ci" || type === "release" || scope === "ci" || scope === "release") return true;
  if (CI_SUBJECT_PATTERNS.some((pattern) => pattern.test(commit.subject))) return true;

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

function latestTag() {
  const tag = sh("git describe --tags --abbrev=0 2>/dev/null");
  return tag || "";
}

function collectCommits(range) {
  // 每条提交：hash \x1f subject \x1f body \x1e
  const log = sh(`git log ${range} --pretty=format:'%H%x1f%s%x1f%b%x1e'`);
  if (!log) return [];
  const out = [];
  for (const entry of log.split("\x1e")) {
    if (!entry.trim()) continue;
    const [hash, subject, body = ""] = entry.split("\x1f");
    out.push({ hash: hash.trim().slice(0, 7), subject: subject.trim(), body: body.trim() });
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

function insertChangelog(newBlock, version, replaceExisting = false) {
  if (!fs.existsSync(CHANGELOG_PATH)) {
    fs.writeFileSync(CHANGELOG_PATH, `# Changelog / 更新日志\n\n${newBlock}`);
    return CHANGELOG_PATH;
  }
  let content = fs.readFileSync(CHANGELOG_PATH, "utf-8");
  const versionHeading = `## [${version}]`;
  if (replaceExisting) {
    const sections = content.split(/(?=^## )/m);
    const filtered = sections.filter((section) => !section.startsWith(versionHeading));
    const firstVersionIndex = filtered.findIndex((section) => section.startsWith("## "));
    filtered.splice(firstVersionIndex < 0 ? filtered.length : firstVersionIndex, 0, newBlock);
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
  const positional = args.filter(
    (arg) => arg !== "--no-write" && arg !== "--replace" && arg !== "--exclude-ci",
  );
  const version = positional[0];
  if (!version) {
    console.error(
      "usage: node scripts/build-changelog.mjs <version> [lastTag] [date] [--no-write] [--replace] [--exclude-ci]",
    );
    process.exit(1);
  }
  const lastTag = positional[1] || latestTag();
  const date = positional[2] || new Date().toISOString().slice(0, 10);
  const range = lastTag ? `${lastTag}..HEAD` : "HEAD";
  const commits = collectCommits(range);
  const block = buildSection(version, lastTag, date, commits, excludeCI);
  if (write) {
    insertChangelog(block, version, replaceExisting);
  }
  process.stdout.write(block);
  console.error(
    `\n[build-changelog] generated (write=${write}, version=${version}, base=${lastTag || "none"}, commits=${commits.length})`,
  );
}

main();
