import { createRoot } from "solid-js";
import { createStore } from "solid-js/store";
import {
	type Domain,
	type DomainMember,
	getDomain,
	domainMembers,
	deleteDomain,
	leaveDomain,
	myDomainPermissions,
	myDomains,
} from "@/api/domain";

interface DomainState {
	myDomainUUIDs: string[];
	currentDomainUUID: string | null;
	domainCache: Record<string, Domain>;
	memberCache: Record<string, DomainMember[]>;
	myRolePermissions: Record<string, string[]>;
	domainLoading: Record<string, boolean>;
	memberLoading: Record<string, boolean>;
	domainErrors: Record<string, string | null>;
	memberErrors: Record<string, string | null>;
	loading: boolean;
}

function errorMessage(error: unknown): string {
	if (error instanceof Error) return error.message;
	return String(error);
}

export function createDomainStore() {
	const [state, setState] = createStore<DomainState>({
		myDomainUUIDs: [],
		currentDomainUUID: null,
		domainCache: {},
		memberCache: {},
		myRolePermissions: {},
		domainLoading: {},
		memberLoading: {},
		domainErrors: {},
		memberErrors: {},
		loading: false,
	});

	const removedUUIDs = new Set<string>();
	const loadGenerations = new Map<string, number>();
	let myDomainsVersion = 0;
	let myDomainsPending = 0;

	const invalidateDomainLoads = (uuid: string) => {
		loadGenerations.set(uuid, (loadGenerations.get(uuid) ?? 0) + 1);
	};

	const currentGeneration = (uuid: string) => loadGenerations.get(uuid) ?? 0;

	const loadMyDomains = async () => {
		const version = ++myDomainsVersion;
		myDomainsPending += 1;
		setState("loading", true);
		try {
			const uuids = await myDomains();
			if (version === myDomainsVersion) {
				// 内容未变化时保留原数组引用：下游 createResource 以数组为 source，
				// 引用变化会触发重新请求并挂起（全仓无 Suspense 边界时整树空白闪烁）
				setState("myDomainUUIDs", (prev) =>
					prev.length === uuids.length && prev.every((u, i) => u === uuids[i])
						? prev
						: uuids,
				);
			}
		} catch (error) {
			console.error("loadMyDomains failed:", error);
			throw error;
		} finally {
			myDomainsPending -= 1;
			if (myDomainsPending === 0) {
				setState("loading", false);
			}
		}
	};

	const ensureDomainLoaded = async (uuid: string) => {
		if (state.domainCache[uuid]) return state.domainCache[uuid];
		if (removedUUIDs.has(uuid)) return state.domainCache[uuid];

		const generation = currentGeneration(uuid);
		setState("domainLoading", uuid, true);
		setState("domainErrors", uuid, null);
		try {
			const domain = await getDomain(uuid);
			if (currentGeneration(uuid) === generation) {
				setState("domainCache", uuid, domain);
				setState("domainErrors", uuid, null);
				setState("domainLoading", uuid, false);
			}
			return domain;
		} catch (error) {
			if (currentGeneration(uuid) === generation) {
				setState("domainErrors", uuid, errorMessage(error));
				setState("domainLoading", uuid, false);
			}
			throw error;
		}
	};

	const setCurrentDomain = (uuid: string | null) => {
		setState("currentDomainUUID", uuid);
		if (uuid) {
			void ensureDomainLoaded(uuid).catch(() => {});
		}
	};

	const activateDomain = (uuid: string) => {
		setCurrentDomain(uuid);
		void loadMembers(uuid).catch(() => {});
		void loadMyPermissions(uuid).catch(() => {});
	};

	const loadMembers = async (domainUUID: string) => {
		if (removedUUIDs.has(domainUUID)) return state.memberCache[domainUUID];
		const generation = currentGeneration(domainUUID);
		setState("memberLoading", domainUUID, true);
		setState("memberErrors", domainUUID, null);
		try {
			const members = await domainMembers(domainUUID);
			if (currentGeneration(domainUUID) === generation) {
				setState("memberCache", domainUUID, members);
				setState("memberErrors", domainUUID, null);
				setState("memberLoading", domainUUID, false);
			}
			return members;
		} catch (error) {
			if (currentGeneration(domainUUID) === generation) {
				setState("memberErrors", domainUUID, errorMessage(error));
				setState("memberLoading", domainUUID, false);
			}
			throw error;
		}
	};

	const loadMyPermissions = async (domainUUID: string): Promise<string[]> => {
		const data = await myDomainPermissions(domainUUID);
		setState("myRolePermissions", domainUUID, data.permissions);
		return data.permissions;
	};

	const addDomain = (domain: Domain) => {
		invalidateDomainLoads(domain.uuid);
		myDomainsVersion += 1;
		removedUUIDs.delete(domain.uuid);
		setState("domainCache", domain.uuid, domain);
		setState("domainErrors", domain.uuid, null);
		setState("domainLoading", domain.uuid, false);
		setState("myDomainUUIDs", (prev) =>
			prev.includes(domain.uuid) ? prev : [...prev, domain.uuid],
		);
	};

	const updateCachedDomain = (domain: Domain) => {
		if (removedUUIDs.has(domain.uuid) || !state.domainCache[domain.uuid]) {
			return;
		}
		invalidateDomainLoads(domain.uuid);
		setState("domainCache", domain.uuid, domain);
		setState("domainErrors", domain.uuid, null);
		setState("domainLoading", domain.uuid, false);
	};

	const removeDomain = (uuid: string) => {
		invalidateDomainLoads(uuid);
		myDomainsVersion += 1;
		removedUUIDs.add(uuid);
		setState("myDomainUUIDs", (prev) => prev.filter((u) => u !== uuid));
		setState("domainCache", (prev) => ({ ...prev, [uuid]: undefined }));
		setState("memberCache", (prev) => ({ ...prev, [uuid]: undefined }));
		setState("myRolePermissions", (prev) => ({ ...prev, [uuid]: undefined }));
		setState("domainLoading", (prev) => ({ ...prev, [uuid]: undefined }));
		setState("memberLoading", (prev) => ({ ...prev, [uuid]: undefined }));
		setState("domainErrors", (prev) => ({ ...prev, [uuid]: undefined }));
		setState("memberErrors", (prev) => ({ ...prev, [uuid]: undefined }));
		if (state.currentDomainUUID === uuid) {
			setState("currentDomainUUID", null);
		}
	};

	const leaveAndClear = async (uuid: string) => {
		await leaveDomain(uuid);
		removeDomain(uuid);
	};

	const deleteAndClear = async (uuid: string) => {
		await deleteDomain(uuid);
		removeDomain(uuid);
	};

	return {
		state,
		setState,
		loadMyDomains,
		ensureDomainLoaded,
		setCurrentDomain,
		activateDomain,
		loadMembers,
		loadMyPermissions,
		updateCachedDomain,
		addDomain,
		removeDomain,
		leaveAndClear,
		deleteAndClear,
	};
}

export default createRoot(createDomainStore);
