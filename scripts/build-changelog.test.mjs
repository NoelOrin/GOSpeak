import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import test from "node:test";

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
