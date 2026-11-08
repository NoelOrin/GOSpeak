import assert from "node:assert/strict";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";
import test from "node:test";

const SCRIPT = fileURLToPath(new URL("./build-changelog.mjs", import.meta.url));
const ROOT = fileURLToPath(new URL("../", import.meta.url));

function generate(...args) {
	return spawnSync(process.execPath, [SCRIPT, ...args], {
		cwd: ROOT,
		encoding: "utf8",
	});
}

test("includes CI changes by default", () => {
	const result = generate("0.4.0", "v0.3.1", "2026-09-05", "--no-write");

	assert.equal(result.status, 0, result.stderr);
	assert.match(result.stdout, /^### CI\/CD$/m);
});

test("excludes CI changes only with the explicit flag", () => {
	const result = generate(
		"0.4.0",
		"v0.3.1",
		"2026-09-05",
		"--no-write",
		"--exclude-ci",
	);

	assert.equal(result.status, 0, result.stderr);
	assert.doesNotMatch(result.stdout, /^### CI\/CD$/m);
});
