import { beforeEach, describe, expect, it, vi } from "vitest";

const myDomainsMock = vi.fn();
vi.mock("@/api/domain", () => ({
	myDomains: (...args: unknown[]) => myDomainsMock(...args),
	myDomainPermissions: vi.fn(),
	getDomain: vi.fn(),
	domainMembers: vi.fn(),
	deleteDomain: vi.fn(),
	leaveDomain: vi.fn(),
}));

import domainStore from "./domainStore";

describe("domainStore.loadMyDomains", () => {
	beforeEach(() => {
		myDomainsMock.mockReset();
	});

	it("数据未变化时保留同一数组引用（下游 resource 不重新挂起）", async () => {
		myDomainsMock.mockResolvedValue(["a", "b"]);
		await domainStore.loadMyDomains();
		const first = domainStore.state.myDomainUUIDs;
		await domainStore.loadMyDomains();
		expect(domainStore.state.myDomainUUIDs).toBe(first);
	});

	it("数据变化时更新为新数组", async () => {
		myDomainsMock.mockResolvedValueOnce(["a"]);
		myDomainsMock.mockResolvedValueOnce(["a", "b"]);
		await domainStore.loadMyDomains();
		await domainStore.loadMyDomains();
		expect(domainStore.state.myDomainUUIDs).toEqual(["a", "b"]);
	});

	it("并发去重：旧版本响应不覆盖新版本", async () => {
		let resolveSlow!: (v: string[]) => void;
		myDomainsMock.mockImplementationOnce(
			() => new Promise<string[]>((r) => (resolveSlow = r)),
		);
		const first = domainStore.loadMyDomains();
		myDomainsMock.mockResolvedValueOnce(["new"]);
		const second = domainStore.loadMyDomains();
		await second;
		resolveSlow(["stale"]);
		await first;
		expect(domainStore.state.myDomainUUIDs).toEqual(["new"]);
	});
});
