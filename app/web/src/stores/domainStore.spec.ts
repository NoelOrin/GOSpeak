import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Domain, DomainMember } from "@/api/domain";
import { createDomainStore } from "./domainStore";

const {
	myDomainsMock,
	getDomainMock,
	domainMembersMock,
	leaveDomainMock,
	deleteDomainMock,
} = vi.hoisted(() => ({
	myDomainsMock: vi.fn(),
	getDomainMock: vi.fn(),
	domainMembersMock: vi.fn(),
	leaveDomainMock: vi.fn(),
	deleteDomainMock: vi.fn(),
}));

vi.mock("@/api/domain", () => ({
	myDomains: myDomainsMock,
	getDomain: getDomainMock,
	domainMembers: domainMembersMock,
	leaveDomain: leaveDomainMock,
	deleteDomain: deleteDomainMock,
}));

function makeDomain(overrides: Partial<Domain> = {}): Domain {
	return {
		id: 1,
		uuid: "g-1",
		name: "Test Domain",
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

function makeMembers(): DomainMember[] {
	return [
		{
			id: 1,
			domain_uuid: "g-1",
			user_uuid: "u-1",
			nickname: "Owner",
			role_name: "owner",
			joined_at: "2026-01-01T00:00:00Z",
		},
		{
			id: 2,
			domain_uuid: "g-1",
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

describe("domainStore", () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it("adds a domain so UUID, cache, and current domain are immediately readable", () => {
		const store = createDomainStore();
		const domain = makeDomain();

		store.addDomain(domain);
		store.setCurrentDomain(domain.uuid);

		expect(store.state.myDomainUUIDs).toContain(domain.uuid);
		expect(store.state.domainCache[domain.uuid]).toEqual(domain);
		expect(store.state.currentDomainUUID).toBe(domain.uuid);
	});

	it("removes a domain from ids and caches after leave or delete", async () => {
		const store = createDomainStore();
		const domain = makeDomain({ uuid: "g-1" });

		store.addDomain(domain);
		store.setCurrentDomain("g-1");
		store.removeDomain("g-1");

		expect(store.state.myDomainUUIDs).not.toContain("g-1");
		expect(store.state.domainCache["g-1"]).toBeUndefined();
		expect(store.state.memberCache["g-1"]).toBeUndefined();
		expect(store.state.currentDomainUUID).toBeNull();
	});

	it("clears domain caches after a successful leave API call", async () => {
		const store = createDomainStore();
		const domain = makeDomain();
		store.addDomain(domain);
		store.setState("memberCache", domain.uuid, makeMembers());
		store.setCurrentDomain(domain.uuid);
		leaveDomainMock.mockResolvedValue(undefined);

		await store.leaveAndClear(domain.uuid);

		expect(leaveDomainMock).toHaveBeenCalledWith(domain.uuid);
		expect(store.state.myDomainUUIDs).not.toContain(domain.uuid);
		expect(store.state.domainCache[domain.uuid]).toBeUndefined();
		expect(store.state.memberCache[domain.uuid]).toBeUndefined();
		expect(store.state.currentDomainUUID).toBeNull();
	});

	it("keeps caches and current domain when leave API fails", async () => {
		const store = createDomainStore();
		const domain = makeDomain();
		const members = makeMembers();
		store.addDomain(domain);
		store.setState("memberCache", domain.uuid, members);
		store.setCurrentDomain(domain.uuid);
		leaveDomainMock.mockRejectedValueOnce(new Error("network"));

		await expect(store.leaveAndClear(domain.uuid)).rejects.toThrow("network");

		expect(store.state.myDomainUUIDs).toContain(domain.uuid);
		expect(store.state.domainCache[domain.uuid]).toEqual(domain);
		expect(store.state.memberCache[domain.uuid]).toEqual(members);
		expect(store.state.currentDomainUUID).toBe(domain.uuid);
	});

	it("clears domain caches after a successful delete API call", async () => {
		const store = createDomainStore();
		const domain = makeDomain();
		store.addDomain(domain);
		store.setState("memberCache", domain.uuid, makeMembers());
		store.setCurrentDomain(domain.uuid);
		deleteDomainMock.mockResolvedValue(undefined);

		await store.deleteAndClear(domain.uuid);

		expect(deleteDomainMock).toHaveBeenCalledWith(domain.uuid);
		expect(store.state.myDomainUUIDs).not.toContain(domain.uuid);
		expect(store.state.domainCache[domain.uuid]).toBeUndefined();
		expect(store.state.memberCache[domain.uuid]).toBeUndefined();
		expect(store.state.currentDomainUUID).toBeNull();
	});

	it("keeps caches and current domain when delete API fails", async () => {
		const store = createDomainStore();
		const domain = makeDomain();
		const members = makeMembers();
		store.addDomain(domain);
		store.setState("memberCache", domain.uuid, members);
		store.setCurrentDomain(domain.uuid);
		deleteDomainMock.mockRejectedValueOnce(new Error("network"));

		await expect(store.deleteAndClear(domain.uuid)).rejects.toThrow("network");

		expect(store.state.myDomainUUIDs).toContain(domain.uuid);
		expect(store.state.domainCache[domain.uuid]).toEqual(domain);
		expect(store.state.memberCache[domain.uuid]).toEqual(members);
		expect(store.state.currentDomainUUID).toBe(domain.uuid);
	});

	it("keeps a failed member refresh retryable", async () => {
		domainMembersMock.mockRejectedValueOnce(new Error("network"));
		const store = createDomainStore();

		await expect(store.loadMembers("g-1")).rejects.toThrow("network");
		expect(store.state.memberLoading["g-1"]).toBe(false);
		expect(store.state.memberErrors["g-1"]).toBe("network");
	});

	it("caches members after a successful refresh", async () => {
		const members = makeMembers();
		domainMembersMock.mockResolvedValue(members);
		const store = createDomainStore();

		await store.loadMembers("g-1");

		expect(store.state.memberCache["g-1"]).toEqual(members);
		expect(store.state.memberLoading["g-1"]).toBe(false);
		expect(store.state.memberErrors["g-1"]).toBeNull();
	});

	it("keeps the previous member cache and allows a retry after failure", async () => {
		const members = makeMembers();
		domainMembersMock.mockResolvedValueOnce(members);
		const store = createDomainStore();

		await store.loadMembers("g-1");
		domainMembersMock.mockRejectedValueOnce(new Error("network"));
		await expect(store.loadMembers("g-1")).rejects.toThrow("network");

		expect(store.state.memberCache["g-1"]).toEqual(members);
		expect(store.state.memberErrors["g-1"]).toBe("network");

		const refreshed = [
			...members,
			{
				id: 3,
				domain_uuid: "g-1",
				user_uuid: "u-3",
				nickname: "Newcomer",
				role_name: "member",
				joined_at: "2026-01-03T00:00:00Z",
			},
		];
		domainMembersMock.mockResolvedValueOnce(refreshed);
		await store.loadMembers("g-1");

		expect(store.state.memberCache["g-1"]).toEqual(refreshed);
		expect(store.state.memberErrors["g-1"]).toBeNull();
	});

	it("loads a domain into the cache when missing", async () => {
		const domain = makeDomain();
		getDomainMock.mockResolvedValue(domain);
		const store = createDomainStore();

		await store.ensureDomainLoaded("g-1");

		expect(store.state.domainCache["g-1"]).toEqual(domain);
		expect(store.state.domainLoading["g-1"]).toBe(false);
		expect(store.state.domainErrors["g-1"]).toBeNull();
	});

	it("reports a domain load failure without blocking retry", async () => {
		getDomainMock.mockRejectedValueOnce(new Error("network"));
		const store = createDomainStore();

		await expect(store.ensureDomainLoaded("g-1")).rejects.toThrow("network");
		expect(store.state.domainLoading["g-1"]).toBe(false);
		expect(store.state.domainErrors["g-1"]).toBe("network");
	});

	it("loads my domain UUIDs and resets global loading", async () => {
		myDomainsMock.mockResolvedValue(["g-1", "g-2"]);
		const store = createDomainStore();

		const promise = store.loadMyDomains();
		expect(store.state.loading).toBe(true);
		await promise;

		expect(store.state.myDomainUUIDs).toEqual(["g-1", "g-2"]);
		expect(store.state.loading).toBe(false);
	});

	it("keeps loading true until the newest overlapping loadMyDomains resolves", async () => {
		const first = deferred<string[]>();
		const second = deferred<string[]>();
		myDomainsMock
			.mockReturnValueOnce(first.promise)
			.mockReturnValueOnce(second.promise);
		const store = createDomainStore();

		const firstPromise = store.loadMyDomains();
		const secondPromise = store.loadMyDomains();
		expect(store.state.loading).toBe(true);

		first.resolve(["g-1"]);
		await firstPromise;
		expect(store.state.loading).toBe(true);

		second.resolve(["g-1", "g-2"]);
		await secondPromise;
		expect(store.state.myDomainUUIDs).toEqual(["g-1", "g-2"]);
		expect(store.state.loading).toBe(false);
	});

	it("resets global loading when loadMyDomains fails", async () => {
		myDomainsMock.mockRejectedValue(new Error("network"));
		const store = createDomainStore();

		await expect(store.loadMyDomains()).rejects.toThrow("network");
		expect(store.state.loading).toBe(false);
	});

	it("updates an existing domain cache in place", () => {
		const store = createDomainStore();
		const domain = makeDomain();
		store.addDomain(domain);

		const updated = makeDomain({ name: "Updated Domain" });
		store.updateCachedDomain(updated);

		expect(store.state.domainCache["g-1"]).toEqual(updated);
	});

	it("does not repopulate domain cache when a load resolves after removal", async () => {
		const { promise, resolve } = deferred<Domain>();
		getDomainMock.mockReturnValueOnce(promise);
		const store = createDomainStore();

		const loading = store.ensureDomainLoaded("g-1");
		store.removeDomain("g-1");
		resolve(makeDomain({ name: "Stale Domain" }));
		await loading;

		expect(store.state.domainCache["g-1"]).toBeUndefined();
		expect(store.state.domainLoading["g-1"]).toBeUndefined();
		expect(store.state.domainErrors["g-1"]).toBeUndefined();
	});

	it("does not let a stale domain load overwrite a fresh re-add", async () => {
		const { promise, resolve } = deferred<Domain>();
		getDomainMock.mockReturnValueOnce(promise);
		const store = createDomainStore();

		const loading = store.ensureDomainLoaded("g-1");
		store.removeDomain("g-1");
		const fresh = makeDomain({ name: "Fresh Domain" });
		store.addDomain(fresh);
		resolve(makeDomain({ name: "Stale Domain" }));
		await loading;

		expect(store.state.domainCache["g-1"]).toEqual(fresh);
		expect(store.state.myDomainUUIDs).toEqual(["g-1"]);
	});

	it("does not repopulate member cache when a load resolves after removal", async () => {
		const { promise, resolve } = deferred<DomainMember[]>();
		domainMembersMock.mockReturnValueOnce(promise);
		const store = createDomainStore();

		const loading = store.loadMembers("g-1");
		store.removeDomain("g-1");
		resolve(makeMembers());
		await loading;

		expect(store.state.memberCache["g-1"]).toBeUndefined();
		expect(store.state.memberLoading["g-1"]).toBeUndefined();
		expect(store.state.memberErrors["g-1"]).toBeUndefined();
	});
	it("does not start a removed domain member load after removal", async () => {
		domainMembersMock.mockResolvedValue(makeMembers());
		const store = createDomainStore();

		store.removeDomain("g-1");
		await store.loadMembers("g-1");

		expect(domainMembersMock).not.toHaveBeenCalled();
		expect(store.state.memberCache["g-1"]).toBeUndefined();
		expect(store.state.memberLoading["g-1"]).toBeUndefined();
		expect(store.state.memberErrors["g-1"]).toBeUndefined();
	});

	it("does not let a stale my domains response re-add a removed domain", async () => {
		const { promise, resolve } = deferred<string[]>();
		myDomainsMock.mockReturnValueOnce(promise);
		const store = createDomainStore();

		const loading = store.loadMyDomains();
		store.removeDomain("g-1");
		resolve(["g-1"]);
		await loading;

		expect(store.state.myDomainUUIDs).not.toContain("g-1");
		expect(store.state.loading).toBe(false);
	});

	it("does not let a stale my domains response drop a freshly added domain", async () => {
		const { promise, resolve } = deferred<string[]>();
		myDomainsMock.mockReturnValueOnce(promise);
		const store = createDomainStore();

		const loading = store.loadMyDomains();
		store.addDomain(makeDomain({ uuid: "g-2" }));
		resolve([]);
		await loading;

		expect(store.state.myDomainUUIDs).toEqual(["g-2"]);
		expect(store.state.loading).toBe(false);
	});

	it("does not restore cache when updateCachedDomain runs after removal", () => {
		const store = createDomainStore();
		store.addDomain(makeDomain());
		store.removeDomain("g-1");

		store.updateCachedDomain(makeDomain({ name: "Updated Domain" }));

		expect(store.state.domainCache["g-1"]).toBeUndefined();
		expect(store.state.myDomainUUIDs).not.toContain("g-1");
	});
	it("updates a cached domain before my domains have loaded", () => {
		const store = createDomainStore();
		store.setState("domainCache", "g-1", makeDomain());

		const updated = makeDomain({ name: "Updated Domain" });
		store.updateCachedDomain(updated);

		expect(store.state.domainCache["g-1"]).toEqual(updated);
	});

	it("loads the selected domain when it is not already cached", async () => {
		const domain = makeDomain();
		getDomainMock.mockResolvedValue(domain);
		const store = createDomainStore();

		store.setCurrentDomain("g-1");

		expect(store.state.currentDomainUUID).toBe("g-1");
		await vi.waitFor(() => {
			expect(store.state.domainCache["g-1"]).toEqual(domain);
		});
	});
});
