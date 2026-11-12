import assert from "node:assert/strict";
import { mkdirSync, mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";

import { checkVersionState } from "./check-version.mjs";

function writeDeclaration(root, component, version) {
	const dir = join(root, ".github", "versions");
	mkdirSync(dir, { recursive: true });
	writeFileSync(join(dir, `${component}.yaml`), `version: "${version}"\n`);
}

function createServerRoot({
	version = "0.2.3",
	packageVersion = version,
} = {}) {
	const root = mkdtempSync(join(tmpdir(), "gospeak-version-server-"));
	writeFileSync(join(root, "VERSION"), `${version}\n`);
	writeFileSync(
		join(root, "package.json"),
		JSON.stringify({ version: packageVersion }),
	);
	writeDeclaration(root, "server", version);
	return root;
}

function createDesktopRoot({
	version = "1.0.0",
	packageVersion = version,
	mygoVersion = version,
} = {}) {
	const root = mkdtempSync(join(tmpdir(), "gospeak-version-desktop-"));
	const dir = join(root, "app", "desktop");
	mkdirSync(dir, { recursive: true });
	writeFileSync(
		join(dir, "package.json"),
		JSON.stringify({ version: packageVersion }),
	);
	writeFileSync(
		join(dir, "mygo.config.ts"),
		`export default {\n\tversion: "${mygoVersion}",\n};\n`,
	);
	writeDeclaration(root, "desktop", version);
	return root;
}

test("accepts a synchronized server version and release tag", () => {
	const root = createServerRoot();

	assert.deepEqual(checkVersionState(root, "gospeak-server-v0.2.3"), {
		component: "server",
		declared: "0.2.3",
		issues: [],
	});
});

test("reports mismatched server release metadata", () => {
	const root = createServerRoot({ packageVersion: "1.0.0" });

	const result = checkVersionState(root, "gospeak-server-v0.2.4");

	assert.equal(result.component, "server");
	assert.equal(result.declared, "0.2.3");
	assert.deepEqual(result.issues, [
		"package.json version 1.0.0 does not match VERSION 0.2.3",
		"release tag gospeak-server-v0.2.4 does not match server declared version 0.2.3",
	]);
});

test("rejects an invalid VERSION value", () => {
	const root = createServerRoot({ version: "dev" });
	writeFileSync(
		join(root, ".github", "versions", "server.yaml"),
		'version: "dev"\n',
	);

	assert.throws(() => checkVersionState(root), /must contain a valid SemVer/);
});

test("accepts a synchronized desktop version including mygo config", () => {
	const root = createDesktopRoot();

	assert.deepEqual(checkVersionState(root, "gospeak-desktop-v1.0.0"), {
		component: "desktop",
		declared: "1.0.0",
		issues: [],
	});
});

test("detects desktop package.json drift", () => {
	const root = createDesktopRoot({ packageVersion: "0.9.0" });

	const result = checkVersionState(root, "", "desktop");

	assert.deepEqual(result.issues, [
		"app/desktop/package.json version 0.9.0 does not match .github/versions/desktop.yaml 1.0.0",
	]);
});

test("detects mygo.config.ts drift that package.json cannot catch", () => {
	const root = createDesktopRoot({ mygoVersion: "0.9.0" });

	const result = checkVersionState(root, "", "desktop");

	assert.deepEqual(result.issues, [
		"app/desktop/mygo.config.ts version 0.9.0 does not match .github/versions/desktop.yaml 1.0.0",
	]);
});

test("infers the component from the tag prefix", () => {
	const desktopRoot = createDesktopRoot();
	assert.equal(
		checkVersionState(desktopRoot, "gospeak-desktop-v1.0.0").component,
		"desktop",
	);

	const serverRoot = createServerRoot();
	assert.equal(
		checkVersionState(serverRoot, "gospeak-server-v0.2.3").component,
		"server",
	);
});

test("rejects tags that predate the naming convention", () => {
	for (const tag of ["0.4.0", "v0.3.1", "desktop-v1.0.0"]) {
		const root = createServerRoot();
		const result = checkVersionState(root, tag);
		assert.equal(result.issues.length, 1, `expected one issue for ${tag}`);
		assert.match(result.issues[0], /does not follow the naming convention/);
	}
});

test("accepts the legacy 'main' alias for the server component", () => {
	const root = createServerRoot();
	assert.equal(checkVersionState(root, "", "main").component, "server");
});

test("rejects an unknown component", () => {
	const root = createServerRoot();
	assert.throws(() => checkVersionState(root, "", "web"), /unknown component/);
});
