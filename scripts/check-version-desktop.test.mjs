import assert from "node:assert/strict";
import { mkdirSync, mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";

import { checkVersionState } from "./check-version.mjs";

/**
 * The desktop rule reads three sources: .github/versions/desktop.yaml,
 * app/desktop/package.json and app/desktop/mygo.config.ts. The last one is what
 * `mygo build` injects as packageVersion, which mygo.Updater.Check compares
 * update manifests against — a drift there makes a release offer itself as an
 * update forever.
 */
function createDesktopRoot({ declared = "1.0.0", pkg = declared, config = declared } = {}) {
	const root = mkdtempSync(join(tmpdir(), "gospeak-desktop-version-"));
	mkdirSync(join(root, ".github/versions"), { recursive: true });
	mkdirSync(join(root, "app/desktop"), { recursive: true });
	writeFileSync(
		join(root, ".github/versions/desktop.yaml"),
		`version: "${declared}"\nnotes: |\n  release\n`,
	);
	writeFileSync(
		join(root, "app/desktop/package.json"),
		JSON.stringify({ name: "@gospeak/desktop-app", version: pkg }),
	);
	writeFileSync(
		join(root, "app/desktop/mygo.config.ts"),
		`import { defineConfig } from "mygo-cli";\n\nexport default defineConfig({\n\tname: "GOSpeak",\n\tversion: "${config}",\n\tidentifier: "dev.gospeak.desktop",\n});\n`,
	);
	return root;
}

test("accepts a desktop release whose three version sources agree", () => {
	const root = createDesktopRoot();

	assert.deepEqual(checkVersionState(root, "desktop-v1.0.0", "desktop"), {
		component: "desktop",
		declared: "1.0.0",
		issues: [],
	});
});

test("reports a mygo.config.ts version that drifted from desktop.yaml", () => {
	const root = createDesktopRoot({ config: "1.0.0", declared: "1.0.1" });

	const result = checkVersionState(root, "", "desktop");

	assert.deepEqual(result.issues, [
		"app/desktop/mygo.config.ts version 1.0.0 does not match .github/versions/desktop.yaml 1.0.1",
	]);
});

test("reports a package.json version that drifted while mygo.config.ts agrees", () => {
	const root = createDesktopRoot({ declared: "1.0.0", pkg: "0.9.0" });

	assert.deepEqual(checkVersionState(root, "", "desktop").issues, [
		"app/desktop/package.json version 0.9.0 does not match .github/versions/desktop.yaml 1.0.0",
	]);
});

test("fails when mygo.config.ts has no version field", () => {
	const root = createDesktopRoot();
	writeFileSync(
		join(root, "app/desktop/mygo.config.ts"),
		`export default defineConfig({ name: "GOSpeak" });\n`,
	);

	assert.throws(
		() => checkVersionState(root, "", "desktop"),
		/must declare a "version:" string field/,
	);
});
