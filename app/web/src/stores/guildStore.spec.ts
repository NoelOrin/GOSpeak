import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Guild, GuildMember } from "@/api/guild";
import { createGuildStore } from "./guildStore";

const {
	myGuildsMock,
	getGuildMock,
	guildMembersMock,
	leaveGuildMock,
	deleteGuildMock,
} = vi.hoisted(() => ({
	myGuildsMock: vi.fn(),
	getGuildMock: vi.fn(),
	guildMembersMock: vi.fn(),
	leaveGuildMock: vi.fn(),
	deleteGuildMock: vi.fn(),
}));

vi.mock("@/api/guild", () => ({
	myGuilds: myGuildsMock,
	getGuild: getGuildMock,
	guildMembers: guildMembersMock,
	leaveGuild: leaveGuildMock,
	deleteGuild: deleteGuildMock,
}));

function makeGuild(overrides: Partial<Guild> = {}): Guild {
	return {
		id: 1,
		uuid: "g-1",
		name: "Test Guild",
		icon_url: "",
		description: "",
		owner_uuid: "u-1",
		invite_code: "",
		max_rooms: 20,
		is_public: false,
		created_at: "2026-01-01T00:00:00Z",
		...overrides,
	};
}

function makeMembers(): GuildMember[] {
	return [
		{
			id: 1,
			guild_uuid: "g-1",
			user_uuid: "u-1",
			nickname: "Owner",
			role_name: "owner",
			joined_at: "2026-01-01T00:00:00Z",
		},
		{
			id: 2,
			guild_uuid: "g-1",
			user_uuid: "u-2",
			nickname: "Member",
			role_name: "member",
			joined_at: "2026-01-02T00:00:00Z",
		},
	];
}

function deferred<T>() {
	let resolve!: (value: T | PromiseLike<T>) => void;
	let reject!: (reason?: unknown) => void;
	const promise = new Promise<T>((res, rej) => {
		resolve = res;
		reject = rej;
	});
	return { promise, resolve, reject };
}

describe("guildStore", () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it("adds a guild so UUID, cache, and current guild are immediately readable", () => {
		const store = createGuildStore();
		const guild = makeGuild();

		store.addGuild(guild);
		store.setCurrentGuild(guild.uuid);

		expect(store.state.myGuildUUIDs).toContain(guild.uuid);
		expect(store.state.guildCache[guild.uuid]).toEqual(guild);
		expect(store.state.currentGuildUUID).toBe(guild.uuid);
	});

	it("removes a guild from ids and caches after leave or delete", async () => {
		const store = createGuildStore();
		const guild = makeGuild({ uuid: "g-1" });

		store.addGuild(guild);
		store.setCurrentGuild("g-1");
		store.removeGuild("g-1");

		expect(store.state.myGuildUUIDs).not.toContain("g-1");
		expect(store.state.guildCache["g-1"]).toBeUndefined();
		expect(store.state.memberCache["g-1"]).toBeUndefined();
		expect(store.state.currentGuildUUID).toBeNull();
	});

	it("clears guild caches after a successful leave API call", async () => {
		const store = createGuildStore();
		const guild = makeGuild();
		store.addGuild(guild);
		store.setState("memberCache", guild.uuid, makeMembers());
		store.setCurrentGuild(guild.uuid);
		leaveGuildMock.mockResolvedValue(undefined);

		await store.leaveAndClear(guild.uuid);

		expect(leaveGuildMock).toHaveBeenCalledWith(guild.uuid);
		expect(store.state.myGuildUUIDs).not.toContain(guild.uuid);
		expect(store.state.guildCache[guild.uuid]).toBeUndefined();
		expect(store.state.memberCache[guild.uuid]).toBeUndefined();
		expect(store.state.currentGuildUUID).toBeNull();
	});

	it("keeps caches and current guild when leave API fails", async () => {
		const store = createGuildStore();
		const guild = makeGuild();
		const members = makeMembers();
		store.addGuild(guild);
		store.setState("memberCache", guild.uuid, members);
		store.setCurrentGuild(guild.uuid);
		leaveGuildMock.mockRejectedValueOnce(new Error("network"));

		await expect(store.leaveAndClear(guild.uuid)).rejects.toThrow("network");

		expect(store.state.myGuildUUIDs).toContain(guild.uuid);
		expect(store.state.guildCache[guild.uuid]).toEqual(guild);
		expect(store.state.memberCache[guild.uuid]).toEqual(members);
		expect(store.state.currentGuildUUID).toBe(guild.uuid);
	});

	it("clears guild caches after a successful delete API call", async () => {
		const store = createGuildStore();
		const guild = makeGuild();
		store.addGuild(guild);
		store.setState("memberCache", guild.uuid, makeMembers());
		store.setCurrentGuild(guild.uuid);
		deleteGuildMock.mockResolvedValue(undefined);

		await store.deleteAndClear(guild.uuid);

		expect(deleteGuildMock).toHaveBeenCalledWith(guild.uuid);
		expect(store.state.myGuildUUIDs).not.toContain(guild.uuid);
		expect(store.state.guildCache[guild.uuid]).toBeUndefined();
		expect(store.state.memberCache[guild.uuid]).toBeUndefined();
		expect(store.state.currentGuildUUID).toBeNull();
	});

	it("keeps caches and current guild when delete API fails", async () => {
		const store = createGuildStore();
		const guild = makeGuild();
		const members = makeMembers();
		store.addGuild(guild);
		store.setState("memberCache", guild.uuid, members);
		store.setCurrentGuild(guild.uuid);
		deleteGuildMock.mockRejectedValueOnce(new Error("network"));

		await expect(store.deleteAndClear(guild.uuid)).rejects.toThrow("network");

		expect(store.state.myGuildUUIDs).toContain(guild.uuid);
		expect(store.state.guildCache[guild.uuid]).toEqual(guild);
		expect(store.state.memberCache[guild.uuid]).toEqual(members);
		expect(store.state.currentGuildUUID).toBe(guild.uuid);
	});

	it("keeps a failed member refresh retryable", async () => {
		guildMembersMock.mockRejectedValueOnce(new Error("network"));
		const store = createGuildStore();

		await expect(store.loadMembers("g-1")).rejects.toThrow("network");
		expect(store.state.memberLoading["g-1"]).toBe(false);
		expect(store.state.memberErrors["g-1"]).toBe("network");
	});

	it("caches members after a successful refresh", async () => {
		const members = makeMembers();
		guildMembersMock.mockResolvedValue(members);
		const store = createGuildStore();

		await store.loadMembers("g-1");

		expect(store.state.memberCache["g-1"]).toEqual(members);
		expect(store.state.memberLoading["g-1"]).toBe(false);
		expect(store.state.memberErrors["g-1"]).toBeNull();
	});

	it("keeps the previous member cache and allows a retry after failure", async () => {
		const members = makeMembers();
		guildMembersMock.mockResolvedValueOnce(members);
		const store = createGuildStore();

		await store.loadMembers("g-1");
		guildMembersMock.mockRejectedValueOnce(new Error("network"));
		await expect(store.loadMembers("g-1")).rejects.toThrow("network");

		expect(store.state.memberCache["g-1"]).toEqual(members);
		expect(store.state.memberErrors["g-1"]).toBe("network");

		const refreshed = [
			...members,
			{
				id: 3,
				guild_uuid: "g-1",
				user_uuid: "u-3",
				nickname: "Newcomer",
				role_name: "member",
				joined_at: "2026-01-03T00:00:00Z",
			},
		];
		guildMembersMock.mockResolvedValueOnce(refreshed);
		await store.loadMembers("g-1");

		expect(store.state.memberCache["g-1"]).toEqual(refreshed);
		expect(store.state.memberErrors["g-1"]).toBeNull();
	});

	it("loads a guild into the cache when missing", async () => {
		const guild = makeGuild();
		getGuildMock.mockResolvedValue(guild);
		const store = createGuildStore();

		await store.ensureGuildLoaded("g-1");

		expect(store.state.guildCache["g-1"]).toEqual(guild);
		expect(store.state.guildLoading["g-1"]).toBe(false);
		expect(store.state.guildErrors["g-1"]).toBeNull();
	});

	it("reports a guild load failure without blocking retry", async () => {
		getGuildMock.mockRejectedValueOnce(new Error("network"));
		const store = createGuildStore();

		await expect(store.ensureGuildLoaded("g-1")).rejects.toThrow("network");
		expect(store.state.guildLoading["g-1"]).toBe(false);
		expect(store.state.guildErrors["g-1"]).toBe("network");
	});

	it("loads my guild UUIDs and resets global loading", async () => {
		myGuildsMock.mockResolvedValue(["g-1", "g-2"]);
		const store = createGuildStore();

		const promise = store.loadMyGuilds();
		expect(store.state.loading).toBe(true);
		await promise;

		expect(store.state.myGuildUUIDs).toEqual(["g-1", "g-2"]);
		expect(store.state.loading).toBe(false);
	});

	it("keeps loading true until the newest overlapping loadMyGuilds resolves", async () => {
		const first = deferred<string[]>();
		const second = deferred<string[]>();
		myGuildsMock
			.mockReturnValueOnce(first.promise)
			.mockReturnValueOnce(second.promise);
		const store = createGuildStore();

		const firstPromise = store.loadMyGuilds();
		const secondPromise = store.loadMyGuilds();
		expect(store.state.loading).toBe(true);

		first.resolve(["g-1"]);
		await firstPromise;
		expect(store.state.loading).toBe(true);

		second.resolve(["g-1", "g-2"]);
		await secondPromise;
		expect(store.state.myGuildUUIDs).toEqual(["g-1", "g-2"]);
		expect(store.state.loading).toBe(false);
	});

	it("resets global loading when loadMyGuilds fails", async () => {
		myGuildsMock.mockRejectedValue(new Error("network"));
		const store = createGuildStore();

		await expect(store.loadMyGuilds()).rejects.toThrow("network");
		expect(store.state.loading).toBe(false);
	});

	it("updates an existing guild cache in place", () => {
		const store = createGuildStore();
		const guild = makeGuild();
		store.addGuild(guild);

		const updated = makeGuild({ name: "Updated Guild" });
		store.updateCachedGuild(updated);

		expect(store.state.guildCache["g-1"]).toEqual(updated);
	});

	it("does not repopulate guild cache when a load resolves after removal", async () => {
		const { promise, resolve } = deferred<Guild>();
		getGuildMock.mockReturnValueOnce(promise);
		const store = createGuildStore();

		const loading = store.ensureGuildLoaded("g-1");
		store.removeGuild("g-1");
		resolve(makeGuild({ name: "Stale Guild" }));
		await loading;

		expect(store.state.guildCache["g-1"]).toBeUndefined();
		expect(store.state.guildLoading["g-1"]).toBeUndefined();
		expect(store.state.guildErrors["g-1"]).toBeUndefined();
	});

	it("does not let a stale guild load overwrite a fresh re-add", async () => {
		const { promise, resolve } = deferred<Guild>();
		getGuildMock.mockReturnValueOnce(promise);
		const store = createGuildStore();

		const loading = store.ensureGuildLoaded("g-1");
		store.removeGuild("g-1");
		const fresh = makeGuild({ name: "Fresh Guild" });
		store.addGuild(fresh);
		resolve(makeGuild({ name: "Stale Guild" }));
		await loading;

		expect(store.state.guildCache["g-1"]).toEqual(fresh);
		expect(store.state.myGuildUUIDs).toEqual(["g-1"]);
	});

	it("does not repopulate member cache when a load resolves after removal", async () => {
		const { promise, resolve } = deferred<GuildMember[]>();
		guildMembersMock.mockReturnValueOnce(promise);
		const store = createGuildStore();

		const loading = store.loadMembers("g-1");
		store.removeGuild("g-1");
		resolve(makeMembers());
		await loading;

		expect(store.state.memberCache["g-1"]).toBeUndefined();
		expect(store.state.memberLoading["g-1"]).toBeUndefined();
		expect(store.state.memberErrors["g-1"]).toBeUndefined();
	});
	it("does not start a removed guild member load after removal", async () => {
		guildMembersMock.mockResolvedValue(makeMembers());
		const store = createGuildStore();

		store.removeGuild("g-1");
		await store.loadMembers("g-1");

		expect(guildMembersMock).not.toHaveBeenCalled();
		expect(store.state.memberCache["g-1"]).toBeUndefined();
		expect(store.state.memberLoading["g-1"]).toBeUndefined();
		expect(store.state.memberErrors["g-1"]).toBeUndefined();
	});

	it("does not let a stale my guilds response re-add a removed guild", async () => {
		const { promise, resolve } = deferred<string[]>();
		myGuildsMock.mockReturnValueOnce(promise);
		const store = createGuildStore();

		const loading = store.loadMyGuilds();
		store.removeGuild("g-1");
		resolve(["g-1"]);
		await loading;

		expect(store.state.myGuildUUIDs).not.toContain("g-1");
		expect(store.state.loading).toBe(false);
	});

	it("does not let a stale my guilds response drop a freshly added guild", async () => {
		const { promise, resolve } = deferred<string[]>();
		myGuildsMock.mockReturnValueOnce(promise);
		const store = createGuildStore();

		const loading = store.loadMyGuilds();
		store.addGuild(makeGuild({ uuid: "g-2" }));
		resolve([]);
		await loading;

		expect(store.state.myGuildUUIDs).toEqual(["g-2"]);
		expect(store.state.loading).toBe(false);
	});

	it("does not restore cache when updateCachedGuild runs after removal", () => {
		const store = createGuildStore();
		store.addGuild(makeGuild());
		store.removeGuild("g-1");

		store.updateCachedGuild(makeGuild({ name: "Updated Guild" }));

		expect(store.state.guildCache["g-1"]).toBeUndefined();
		expect(store.state.myGuildUUIDs).not.toContain("g-1");
	});
	it("updates a cached guild before my guilds have loaded", () => {
		const store = createGuildStore();
		store.setState("guildCache", "g-1", makeGuild());

		const updated = makeGuild({ name: "Updated Guild" });
		store.updateCachedGuild(updated);

		expect(store.state.guildCache["g-1"]).toEqual(updated);
	});

	it("loads the selected guild when it is not already cached", async () => {
		const guild = makeGuild();
		getGuildMock.mockResolvedValue(guild);
		const store = createGuildStore();

		store.setCurrentGuild("g-1");

		expect(store.state.currentGuildUUID).toBe("g-1");
		await vi.waitFor(() => {
			expect(store.state.guildCache["g-1"]).toEqual(guild);
		});
	});
});
