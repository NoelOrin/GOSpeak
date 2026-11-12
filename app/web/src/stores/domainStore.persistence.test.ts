import { beforeEach, describe, expect, it, vi } from "vitest";

// domainStore 现在会在加载失败时上报 toast；core store 不该在测试里
// 拖进整个组件库（首次 transform 会超过 10s 超时）。
vi.mock("@gospeak/components", () => ({
	toast: { error: vi.fn(), warning: vi.fn(), success: vi.fn() },
}));
vi.mock("@/api/domain", () => ({
	myDomains: vi.fn(async () => []),
	getDomain: vi.fn(async () => undefined),
	domainMembers: vi.fn(async () => []),
	myDomainPermissions: vi.fn(async () => ({ permissions: [] })),
	leaveDomain: vi.fn(async () => {}),
	deleteDomain: vi.fn(async () => {}),
}));

const LAST_DOMAIN_KEY = "gospeak_last_domain";

describe("last domain persistence", () => {
	beforeEach(() => {
		localStorage.removeItem(LAST_DOMAIN_KEY);
	});

	it("persists the current domain and clears it when that domain is removed", async () => {
		const { createDomainStore } = await import("./domainStore");
		const store = createDomainStore();

		store.setCurrentDomain("g-1");
		expect(localStorage.getItem(LAST_DOMAIN_KEY)).toBe("g-1");
		expect(store.state.currentDomainUUID).toBe("g-1");

		store.removeDomain("g-1");
		expect(store.state.currentDomainUUID).toBeNull();
		expect(localStorage.getItem(LAST_DOMAIN_KEY)).toBeNull();
	});

	it("keeps the remembered domain when a different domain is removed", async () => {
		const { createDomainStore } = await import("./domainStore");
		const store = createDomainStore();

		store.setCurrentDomain("g-1");
		store.removeDomain("g-2");

		expect(store.state.currentDomainUUID).toBe("g-1");
		expect(localStorage.getItem(LAST_DOMAIN_KEY)).toBe("g-1");
	});

	it("clears persistence when the store is reset (logout / test isolation)", async () => {
		const { createDomainStore } = await import("./domainStore");
		createDomainStore().setCurrentDomain("g-1");
		expect(localStorage.getItem(LAST_DOMAIN_KEY)).toBe("g-1");

		const fresh = createDomainStore();
		expect(fresh.state.currentDomainUUID).toBeNull();
		expect(localStorage.getItem(LAST_DOMAIN_KEY)).toBeNull();
	});

	it("hydrates the last domain on a fresh module load (page reload)", async () => {
		localStorage.setItem(LAST_DOMAIN_KEY, "g-remembered");
		vi.resetModules();
		const { useDomainStore } = await import("./domainStore");

		expect(useDomainStore.getState().currentDomainUUID).toBe("g-remembered");
	});
});
