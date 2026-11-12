import { create } from "zustand";
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
import {
	clearBackgroundFailure,
	reportBackgroundFailure,
} from "@/utils/backgroundFailure";

interface DomainState {
	myDomainUUIDs: string[];
	currentDomainUUID: string | null;
	domainCache: Record<string, Domain | undefined>;
	memberCache: Record<string, DomainMember[] | undefined>;
	myRolePermissions: Record<string, string[] | undefined>;
	domainLoading: Record<string, boolean | undefined>;
	memberLoading: Record<string, boolean | undefined>;
	domainErrors: Record<string, string | null | undefined>;
	memberErrors: Record<string, string | null | undefined>;
	/** 我的域内权限加载失败：按钮会静默失效，必须能看见原因。 */
	myPermissionsErrors: Record<string, string | null | undefined>;
	loading: boolean;
}

function errorMessage(error: unknown): string {
	if (error instanceof Error) return error.message;
	return String(error);
}

/**
 * 「上次所在域」只记 UUID（不含任何域数据），用于刷新/重开后首屏直达。
 * 有效性由路由层用 myDomainUUIDs 校验，退出域后会自动失效。
 */
const LAST_DOMAIN_KEY = "gospeak_last_domain";

function readLastDomain(): string | null {
	if (typeof window === "undefined") return null;
	try {
		return window.localStorage.getItem(LAST_DOMAIN_KEY) || null;
	} catch {
		return null;
	}
}

function persistLastDomain(uuid: string | null): void {
	if (typeof window === "undefined") return;
	try {
		if (uuid) window.localStorage.setItem(LAST_DOMAIN_KEY, uuid);
		else window.localStorage.removeItem(LAST_DOMAIN_KEY);
	} catch {
		// 隐私模式 / 存储配额不足时不影响内存状态
	}
}

const removedUUIDs = new Set<string>();
const loadGenerations = new Map<string, number>();
let myDomainsVersion = 0;
let myDomainsPending = 0;

const invalidateDomainLoads = (uuid: string) => {
	loadGenerations.set(uuid, (loadGenerations.get(uuid) ?? 0) + 1);
};

const currentGeneration = (uuid: string) => loadGenerations.get(uuid) ?? 0;

export const useDomainStore = create<DomainState>(() => ({
	myDomainUUIDs: [],
	currentDomainUUID: readLastDomain(),
	domainCache: {},
	memberCache: {},
	myRolePermissions: {},
	domainLoading: {},
	memberLoading: {},
	domainErrors: {},
	memberErrors: {},
	myPermissionsErrors: {},
	loading: false,
}));

const loadMyDomains = async () => {
	const version = ++myDomainsVersion;
	myDomainsPending += 1;
	useDomainStore.setState({ loading: true });
	try {
		const uuids = await myDomains();
		if (version === myDomainsVersion) {
			clearBackgroundFailure("my-domains");
			// 内容未变化时保留原数组引用：避免下游以数组为依赖的 effect 因引用变化重复触发
			useDomainStore.setState((prev) => ({
				myDomainUUIDs:
					prev.myDomainUUIDs.length === uuids.length &&
					prev.myDomainUUIDs.every((u, i) => u === uuids[i])
						? prev.myDomainUUIDs
						: uuids,
			}));
		}
	} catch (error) {
		console.error("loadMyDomains failed:", error);
		reportBackgroundFailure(
			"my-domains",
			error,
			"我的域列表加载失败，请检查网络后刷新",
		);
		throw error;
	} finally {
		myDomainsPending -= 1;
		if (myDomainsPending === 0) {
			useDomainStore.setState({ loading: false });
		}
	}
};

const ensureDomainLoaded = async (uuid: string) => {
	const state = useDomainStore.getState();
	if (state.domainCache[uuid]) return state.domainCache[uuid];
	if (removedUUIDs.has(uuid)) return state.domainCache[uuid];

	const generation = currentGeneration(uuid);
	useDomainStore.setState((prev) => ({
		domainLoading: { ...prev.domainLoading, [uuid]: true },
		domainErrors: { ...prev.domainErrors, [uuid]: null },
	}));
	try {
		const domain = await getDomain(uuid);
		if (currentGeneration(uuid) === generation) {
			clearBackgroundFailure(`domain:${uuid}`);
			useDomainStore.setState((prev) => ({
				domainCache: { ...prev.domainCache, [uuid]: domain },
				domainErrors: { ...prev.domainErrors, [uuid]: null },
				domainLoading: { ...prev.domainLoading, [uuid]: false },
			}));
		}
		return domain;
	} catch (error) {
		if (currentGeneration(uuid) === generation) {
			useDomainStore.setState((prev) => ({
				domainErrors: { ...prev.domainErrors, [uuid]: errorMessage(error) },
				domainLoading: { ...prev.domainLoading, [uuid]: false },
			}));
			reportBackgroundFailure(
				`domain:${uuid}`,
				error,
				"域信息加载失败，请检查网络后重试",
			);
		}
		throw error;
	}
};

const setCurrentDomain = (uuid: string | null) => {
	persistLastDomain(uuid);
	useDomainStore.setState({ currentDomainUUID: uuid });
	if (uuid) {
		// 失败已由 ensureDomainLoaded 写入 domainErrors 并上报，这里只防未处理拒绝。
		void ensureDomainLoaded(uuid).catch(() => {});
	}
};

const loadMembers = async (domainUUID: string) => {
	if (removedUUIDs.has(domainUUID)) {
		return useDomainStore.getState().memberCache[domainUUID];
	}
	const generation = currentGeneration(domainUUID);
	useDomainStore.setState((prev) => ({
		memberLoading: { ...prev.memberLoading, [domainUUID]: true },
		memberErrors: { ...prev.memberErrors, [domainUUID]: null },
	}));
	try {
		const members = await domainMembers(domainUUID);
		if (currentGeneration(domainUUID) === generation) {
			clearBackgroundFailure(`members:${domainUUID}`);
			useDomainStore.setState((prev) => ({
				memberCache: { ...prev.memberCache, [domainUUID]: members },
				memberErrors: { ...prev.memberErrors, [domainUUID]: null },
				memberLoading: { ...prev.memberLoading, [domainUUID]: false },
			}));
		}
		return members;
	} catch (error) {
		if (currentGeneration(domainUUID) === generation) {
			useDomainStore.setState((prev) => ({
				memberErrors: {
					...prev.memberErrors,
					[domainUUID]: errorMessage(error),
				},
				memberLoading: { ...prev.memberLoading, [domainUUID]: false },
			}));
			reportBackgroundFailure(
				`members:${domainUUID}`,
				error,
				"成员列表加载失败，请检查网络后刷新",
			);
		}
		throw error;
	}
};

/**
 * 乐观更新专用：直接替换某个域的成员列表，不触碰 loading / error 标记。
 * 用于「移出成员 / 改成员角色」这类可本地预测的操作；失败时把原数组写回即可。
 */
const setMemberCache = (
	domainUUID: string,
	members: DomainMember[] | undefined,
) => {
	useDomainStore.setState((prev) => ({
		memberCache: { ...prev.memberCache, [domainUUID]: members },
	}));
};

const loadMyPermissions = async (domainUUID: string): Promise<string[]> => {
	try {
		const data = await myDomainPermissions(domainUUID);
		clearBackgroundFailure(`permissions:${domainUUID}`);
		useDomainStore.setState((prev) => ({
			myRolePermissions: {
				...prev.myRolePermissions,
				[domainUUID]: data.permissions,
			},
			myPermissionsErrors: {
				...prev.myPermissionsErrors,
				[domainUUID]: null,
			},
		}));
		return data.permissions;
	} catch (error) {
		const message = reportBackgroundFailure(
			`permissions:${domainUUID}`,
			error,
			"域内权限加载失败，管理操作可能暂不可用",
		);
		useDomainStore.setState((prev) => ({
			myPermissionsErrors: {
				...prev.myPermissionsErrors,
				[domainUUID]: message,
			},
		}));
		throw error;
	}
};

const addDomain = (domain: Domain) => {
	invalidateDomainLoads(domain.uuid);
	myDomainsVersion += 1;
	removedUUIDs.delete(domain.uuid);
	useDomainStore.setState((prev) => ({
		domainCache: { ...prev.domainCache, [domain.uuid]: domain },
		domainErrors: { ...prev.domainErrors, [domain.uuid]: null },
		domainLoading: { ...prev.domainLoading, [domain.uuid]: false },
		myDomainUUIDs: prev.myDomainUUIDs.includes(domain.uuid)
			? prev.myDomainUUIDs
			: [...prev.myDomainUUIDs, domain.uuid],
	}));
};

const updateCachedDomain = (domain: Domain) => {
	if (
		removedUUIDs.has(domain.uuid) ||
		!useDomainStore.getState().domainCache[domain.uuid]
	) {
		return;
	}
	invalidateDomainLoads(domain.uuid);
	useDomainStore.setState((prev) => ({
		domainCache: { ...prev.domainCache, [domain.uuid]: domain },
		domainErrors: { ...prev.domainErrors, [domain.uuid]: null },
		domainLoading: { ...prev.domainLoading, [domain.uuid]: false },
	}));
};

const removeDomain = (uuid: string) => {
	invalidateDomainLoads(uuid);
	myDomainsVersion += 1;
	removedUUIDs.add(uuid);
	useDomainStore.setState((prev) => ({
		myDomainUUIDs: prev.myDomainUUIDs.filter((u) => u !== uuid),
		domainCache: { ...prev.domainCache, [uuid]: undefined },
		memberCache: { ...prev.memberCache, [uuid]: undefined },
		myRolePermissions: { ...prev.myRolePermissions, [uuid]: undefined },
		domainLoading: { ...prev.domainLoading, [uuid]: undefined },
		memberLoading: { ...prev.memberLoading, [uuid]: undefined },
		domainErrors: { ...prev.domainErrors, [uuid]: undefined },
		memberErrors: { ...prev.memberErrors, [uuid]: undefined },
		myPermissionsErrors: { ...prev.myPermissionsErrors, [uuid]: undefined },
		currentDomainUUID:
			prev.currentDomainUUID === uuid ? null : prev.currentDomainUUID,
	}));
	persistLastDomain(useDomainStore.getState().currentDomainUUID);
};

const leaveAndClear = async (uuid: string) => {
	await leaveDomain(uuid);
	removeDomain(uuid);
};

const deleteAndClear = async (uuid: string) => {
	await deleteDomain(uuid);
	removeDomain(uuid);
};

/** 兼容旧细粒度写法的测试辅助：setState("memberCache", uuid, members) */
function setStateInStore(key: string, a?: unknown, b?: unknown): void {
	if (b === undefined && a !== undefined) {
		useDomainStore.setState({ [key]: a } as Partial<DomainState>);
		return;
	}
	useDomainStore.setState(
		(prev) =>
			({
				[key]: {
					...((prev as unknown as Record<string, unknown>)[key] as object),
					[a as string]: b,
				},
			}) as Partial<DomainState>,
	);
}

/**
 * 测试隔离用：重置单例到初始状态并返回门面。
 * 生产代码直接使用 default 导出的 domainStore 单例。
 */
export function createDomainStore() {
	persistLastDomain(null);
	useDomainStore.setState({
		myDomainUUIDs: [],
		currentDomainUUID: null,
		domainCache: {},
		memberCache: {},
		myRolePermissions: {},
		domainLoading: {},
		memberLoading: {},
		domainErrors: {},
		memberErrors: {},
		myPermissionsErrors: {},
		loading: false,
	});
	removedUUIDs.clear();
	loadGenerations.clear();
	myDomainsVersion = 0;
	myDomainsPending = 0;
	return domainStore;
}

export const domainStore = {
	/** 快照读取（每次访问返回当前 state）；组件内请用 useDomainStore 选择器 */
	get state() {
		return useDomainStore.getState();
	},
	setState: setStateInStore,
	loadMyDomains,
	ensureDomainLoaded,
	setCurrentDomain,
	activateDomain: (uuid: string) => {
		setCurrentDomain(uuid);
		// 两个 loader 内部已写 error state 并上报，catch 只防未处理拒绝。
		void loadMembers(uuid).catch(() => {});
		void loadMyPermissions(uuid).catch(() => {});
		// 访客能力必须在"进入域"这一层拉取：原先放在 domain 页组件里，
		// 而移动端布局不渲染该页 children，导致移动端访客永远拿不到禁说/禁聊配置。
	},
	loadMembers,
	setMemberCache,
	loadMyPermissions,
	addDomain,
	updateCachedDomain,
	removeDomain,
	leaveAndClear,
	deleteAndClear,
};

export default domainStore;
