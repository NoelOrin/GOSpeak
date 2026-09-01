import assert from "node:assert/strict";
import { mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";

import { checkVersionState } from "./check-version.mjs";

function createVersionRoot({ version = "0.2.3", packageVersion = version, manifestVersion = version } = {}) {
	const root = mkdtempSync(join(tmpdir(), "gospeak-version-"));
	writeFileSync(join(root, "VERSION"), `${version}\n`);
	writeFileSync(join(root, "package.json"), JSON.stringify({ version: packageVersion }));
	writeFileSync(join(root, ".release-please-manifest.json"), JSON.stringify({ ".": manifestVersion }));
	return root;
}

test("accepts synchronized product version and release tag", () => {
	const root = createVersionRoot();

	assert.deepEqual(checkVersionState(root, "v0.2.3"), {
		version: "0.2.3",
		issues: [],
	});
});

test("reports mismatched release metadata", () => {
	const root = createVersionRoot({ packageVersion: "1.0.0", manifestVersion: "0.2.2" });

	const result = checkVersionState(root, "v0.2.4");

	assert.equal(result.version, "0.2.3");
	assert.deepEqual(result.issues, [
		"package.json version 1.0.0 does not match VERSION 0.2.3",
		".release-please-manifest.json version 0.2.2 does not match VERSION 0.2.3",
		"release tag v0.2.4 does not match VERSION 0.2.3",
	]);
});

test("rejects an invalid VERSION value", () => {
	const root = createVersionRoot({ version: "dev" });

	assert.throws(() => checkVersionState(root), /VERSION must contain a valid SemVer/);
});
