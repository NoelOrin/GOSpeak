import { describe, expect, it } from "vitest";
import {
	createPluginPrivateKV,
	createSharedKV,
	MemoryKVStore,
} from "./kvStore";

describe("shared + private KV", () => {
	it("isolates private keys per plugin while sharing sharedKv", async () => {
		const root = new MemoryKVStore();
		const aPrivate = createPluginPrivateKV(root, "welcome");
		const bPrivate = createPluginPrivateKV(root, "moderation");
		const aShared = createSharedKV(root);
		const bShared = createSharedKV(root);

		await aPrivate.set("enabled", true);
		await bPrivate.set("enabled", false);
		await aShared.set("welcome:disabled-by-mute", true);

		expect(await aPrivate.get("enabled")).toBe(true);
		expect(await bPrivate.get("enabled")).toBe(false);
		// private stores do not leak across plugins
		expect(await bPrivate.get("enabled")).not.toBe(
			await aPrivate.get("enabled"),
		);

		// same shared key visible to both
		expect(await aShared.get("welcome:disabled-by-mute")).toBe(true);
		expect(await bShared.get("welcome:disabled-by-mute")).toBe(true);
	});

	it("private keys() / clear() only touch own namespace", async () => {
		const root = new MemoryKVStore();
		const a = createPluginPrivateKV(root, "a");
		const b = createPluginPrivateKV(root, "b");
		const shared = createSharedKV(root);

		await a.set("x", 1);
		await b.set("y", 2);
		await shared.set("z", 3);

		expect(await a.keys!()).toEqual(["x"]);
		await a.clear!();
		expect(await a.get("x")).toBeUndefined();
		expect(await b.get("y")).toBe(2);
		expect(await shared.get("z")).toBe(3);
	});

	it("does not collide private key with shared key of same name", async () => {
		const root = new MemoryKVStore();
		const priv = createPluginPrivateKV(root, "p");
		const shared = createSharedKV(root);
		await priv.set("flag", "private");
		await shared.set("flag", "shared");
		expect(await priv.get("flag")).toBe("private");
		expect(await shared.get("flag")).toBe("shared");
	});
});
