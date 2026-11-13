import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import test from "node:test";

import { buildDetailsBlock, collectContributors } from "./build-changelog.mjs";

const SCRIPT = fileURLToPath(new URL("./build-changelog.mjs", import.meta.url));
const ROOT = fileURLToPath(new URL("../", import.meta.url));

function generate(...args) {
	return spawnSync(process.execPath, [SCRIPT, ...args], {
		cwd: ROOT,
		encoding: "utf8",
	});
}

// 用 HEAD 作为基线，区间内没有任何提交，输出只包含标题与「无分类变更」，
// 不依赖仓库当前恰好存在哪些 commit —— 这正是旧测试脆弱的原因。
test("emits a section header even when the range is empty", () => {
	const result = generate("9.9.9", "HEAD", "2026-01-01", "--no-write");

	assert.equal(result.status, 0, result.stderr);
	assert.match(result.stdout, /^## \[9\.9\.9\]/m);
	assert.match(result.stdout, /无分类变更/);
});

test("excludes CI commits only with the explicit flag", () => {
	const withCI = generate("9.9.9", "HEAD", "2026-01-01", "--no-write");
	assert.doesNotMatch(withCI.stdout, /^### CI\/CD$/m);

	const excluded = generate(
		"9.9.9",
		"HEAD",
		"2026-01-01",
		"--no-write",
		"--exclude-ci",
	);
	assert.equal(excluded.status, 0, excluded.stderr);
	assert.doesNotMatch(excluded.stdout, /^### CI\/CD$/m);
});

test("accepts component flags without treating them as positional args", () => {
	const result = generate("9.9.9", "--no-write", "--server");
	assert.equal(result.status, 0, result.stderr);
	assert.match(result.stdout, /^## \[9\.9\.9\]/m);

	const desktop = generate("9.9.9", "--no-write", "--desktop");
	assert.equal(desktop.status, 0, desktop.stderr);
	assert.match(desktop.stdout, /^## \[9\.9\.9\]/m);
});

test("fails without a version argument", () => {
	const result = generate("--no-write");
	assert.equal(result.status, 1);
	assert.match(result.stderr, /usage:/);
});

function commit(subject, name, email, hash = "abc1234") {
	return { hash, subject, body: "", authorName: name, authorEmail: email };
}

test("merges one person's multiple git identities into a single contributor", () => {
	// 本地提交（自定义邮箱）与 GitHub 合并提交（noreply）是同一人。
	const contributors = collectContributors([
		commit("feat(web): a", "noelorin", "noelorin@outlook.com", "1111111"),
		commit("fix(web): b", "noelorin", "noelorin@outlook.com", "2222222"),
		commit(
			"chore: merge",
			"NoelOrin",
			"112020558+NoelOrin@users.noreply.github.com",
			"3333333",
		),
	]);

	assert.equal(contributors.length, 1, "identities must collapse to one row");
	assert.equal(contributors[0].name, "NoelOrin");
	assert.equal(contributors[0].commits, 3);
});

test("keeps genuinely different people apart", () => {
	const contributors = collectContributors([
		commit("feat: a", "noelorin", "noelorin@outlook.com", "1111111"),
		commit("fix: b", "alice", "123+alice@users.noreply.github.com", "2222222"),
	]);

	assert.equal(contributors.length, 2);
	// alice 有 noreply → 规范化成 Alice；noelorin 只有自定义邮箱 → 保留原名。
	assert.deepEqual(
		contributors.map((c) => c.name).sort(),
		["Alice", "noelorin"].sort(),
	);
});

test("details block collapses commit list and skips merge commits", () => {
	const details = buildDetailsBlock([
		commit(
			"feat(web): 新增骨架",
			"noelorin",
			"noelorin@outlook.com",
			"1111111",
		),
		commit(
			"Merge pull request #41 from NoelOrin/x",
			"NoelOrin",
			"112020558+NoelOrin@users.noreply.github.com",
			"2222222",
		),
	]);

	assert.match(details, /^<details>$/m);
	assert.match(
		details,
		/<summary>提交记录（1 commits · 1 位贡献者）<\/summary>/,
	);
	// 只有自定义邮箱时无法推断 GitHub login，展示名取 authorName。
	assert.match(details, /\*\*noelorin\*\*/);
	assert.doesNotMatch(details, /Merge pull request/);
	assert.match(details, /\| `1111111` \| feat \| 新增骨架 \| noelorin \|/);
});

test("details block is empty when there is nothing worth listing", () => {
	const details = buildDetailsBlock([
		commit(
			"Merge pull request #1 from a/b",
			"NoelOrin",
			"112020558+NoelOrin@users.noreply.github.com",
		),
	]);
	assert.equal(details, "");
});

test("--details-only emits just the collapsed block", () => {
	const result = generate(
		"9.9.9",
		"HEAD",
		"2026-01-01",
		"--no-write",
		"--details-only",
	);
	assert.equal(result.status, 0, result.stderr);
	// 空区间没有可列出的提交，因此不产生 details 块，也不会输出分类清单标题。
	assert.doesNotMatch(result.stdout, /^## \[/m);
});
