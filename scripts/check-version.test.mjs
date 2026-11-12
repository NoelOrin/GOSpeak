import assert from "node:assert/strict";
import { mkdirSync, mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";

import { checkVersionState } from "./check-version.mjs";

function createVersionRoot({
	version = "0.2.3",
	packageVersion = version,
	declared = "0.2.3",
} = {}) {
	const root = mkdtempSync(join(tmpdir(), "gospeak-version-"));
	mkdirSync(join(root, ".github/versions"), { recursive: true });
	writeFileSync(join(root, "VERSION"), `${version}\n`);
	writeFileSync(join(root, "package.json"), JSON.stringify({ version: packageVersion }));
	writeFileSync(
		join(root, ".github/versions/main.yaml"),
		`version: "${declared}"\n`,
	);
	return root;
}

test("accepts synchronized product version and release tag", () => {
	const root = createVersionRoot();

	assert.deepEqual(checkVersionState(root, "v0.2.3"), {
		component: "main",
		declared: "0.2.3",
		issues: [],
	});
});

test("reports mismatched release metadata", () => {
	const root = createVersionRoot({ packageVersion: "1.0.0" });

	const result = checkVersionState(root, "v0.2.4");

	assert.equal(result.declared, "0.2.3");
	assert.deepEqual(result.issues, [
		"package.json version 1.0.0 does not match VERSION 0.2.3",
		"release tag v0.2.4 does not match main declared version 0.2.3",
	]);
});

test("rejects an invalid VERSION value", () => {
	const root = createVersionRoot({ version: "dev" });

	assert.throws(() => checkVersionState(root), /must contain a valid SemVer/);
});

test("infers the desktop component from a desktop-v tag", () => {
	const root = mkdtempSync(join(tmpdir(), "gospeak-version-"));
	mkdirSync(join(root, ".github/versions"), { recursive: true });
	mkdirSync(join(root, "app/desktop"), { recursive: true });
	writeFileSync(join(root, ".github/versions/desktop.yaml"), `version: "1.0.0"\n`);
	writeFileSync(
		join(root, "app/desktop/package.json"),
		JSON.stringify({ version: "1.0.0" }),
	);
	writeFileSync(
		join(root, "app/desktop/mygo.config.ts"),
		`export default defineConfig({\n\tversion: "1.0.0",\n});\n`,
	);

	assert.deepEqual(checkVersionState(root, "desktop-v1.0.0"), {
		component: "desktop",
		declared: "1.0.0",
		issues: [],
	});
});

test("rejects a tag whose prefix does not match the component", () => {
	const root = createVersionRoot();

	assert.throws(
		() => checkVersionState(root, "desktop-v0.2.3", "main"),
		/release tag "desktop-v0.2.3" must start with "v" for the main component/,
	);
});
