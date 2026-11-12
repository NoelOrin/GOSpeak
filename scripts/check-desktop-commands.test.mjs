import assert from "node:assert/strict";
import test from "node:test";

import {
	checkCommands,
	extractGoBindings,
	extractTsCommands,
} from "./check-desktop-commands.mjs";

const goSource = `
package main

func main() {
	mygo.Bind(Window{})
	mygo.Bind(ServerConfig{})
}
`;

const windowGo = `
package main

func (Window) Minimize(ctx context.Context) {}
func (Window) IsMaximized(ctx context.Context) bool { return false }
`;

const serverGo = `
package main

func (ServerConfig) GetBaseURL() string { return "" }
func (ServerConfig) SetBaseURL(newURL string) {}
`;

const tsSource = `
const native = {
  minimize: () => bridge.invoke(command("Window", "Minimize")),
  getBaseUrl: () => bridge.invoke(command("ServerConfig", "GetBaseURL")),
};
`;

test("extracts bound Go services and their exported methods", () => {
	const bindings = extractGoBindings([
		{ path: "main.go", source: goSource },
		{ path: "window.go", source: windowGo },
		{ path: "server_config.go", source: serverGo },
	]);

	assert.deepEqual([...bindings].sort(), [
		"ServerConfig.GetBaseURL",
		"ServerConfig.SetBaseURL",
		"Window.IsMaximized",
		"Window.Minimize",
	]);
});

test("extracts command names from the TypeScript SDK", () => {
	const commands = extractTsCommands([
		{ path: "capabilities/window.ts", source: tsSource },
	]);

	assert.deepEqual([...commands].sort(), [
		"ServerConfig.GetBaseURL",
		"Window.Minimize",
	]);
});

test("reports commands the host does not bind", () => {
	const result = checkCommands({
		bindings: new Set(["Window.Minimize"]),
		commands: new Set(["Window.Minimize", "ServerConfig.GetBaseURL"]),
	});

	assert.deepEqual(result.missing, ["ServerConfig.GetBaseURL"]);
	assert.deepEqual(result.unused, []);
	assert.equal(result.ok, false);
});

test("reports bound methods the SDK never calls, as a warning", () => {
	const result = checkCommands({
		bindings: new Set(["Window.Minimize", "DeepLink.Open"]),
		commands: new Set(["Window.Minimize"]),
	});

	assert.deepEqual(result.missing, []);
	assert.deepEqual(result.unused, ["DeepLink.Open"]);
	assert.equal(result.ok, true);
});

test("a missing command fails the check even when nothing else is wrong", () => {
	const result = checkCommands({
		bindings: new Set(["Window.Minimize"]),
		commands: new Set(["window.minimize"]),
	});

	assert.deepEqual(result.missing, ["window.minimize"]);
	assert.equal(result.ok, false);
});

test("ignores Go methods that are not exported or not on bound types", () => {
	const bindings = extractGoBindings([
		{ path: "main.go", source: goSource },
		{
			path: "window.go",
			source: `
package main

func (Window) Minimize(ctx context.Context) {}
func (window) hidden(ctx context.Context) {}
func (proxyServer) serveDist() {}
`,
		},
	]);

	assert.deepEqual([...bindings], ["Window.Minimize"]);
});
