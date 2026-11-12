import type { QueryClient, QueryKey } from "@tanstack/react-query";

/**
 * 通用乐观更新执行器。
 *
 * 管理页原本的写操作都是「await 请求 → 成功后才刷新列表」，网络往返期间
 * 界面毫无反应，看起来像卡住；失败时也没有可回滚的中间态。这里把流程统一成：
 *
 *   snapshot → apply（立刻改本地状态）→ request → 失败 restore → settle 对账
 *
 * 数据源可以是 React Query 缓存、zustand store，或组件 useState —— 调用方
 * 只需提供快照 / 应用 / 恢复三个闭包，不绑定某一种状态容器。
 *
 * @returns 请求是否成功（失败时已经回滚并触发过 onError / settle）
 */
export interface OptimisticTask<T> {
	/** 改动前取快照；可异步（React Query 需要 await cancelQueries）。 */
	snapshot: () => T | Promise<T>;
	/** 把预测结果立刻写进本地状态。 */
	apply: () => void;
	/** 用快照恢复本地状态。 */
	restore: (snapshot: T) => void;
	/** 真正的网络请求。 */
	request: () => Promise<unknown>;
	/**
	 * 成功或失败后的对账，例如 invalidateQueries / 重新拉列表。
	 * 内部会吞掉对账自身的异常：主操作已成功时不该因为刷新失败被误报为失败。
	 */
	settle?: () => unknown;
	/** 请求失败时回调（此时已回滚）。用于 toast / 记录错误文案。 */
	onError?: (error: unknown) => void;
	/** 请求成功时回调（在 settle 对账之前执行）。 */
	onSuccess?: () => void;
}

function isPromiseLike<T>(value: T | Promise<T>): value is Promise<T> {
	return (
		typeof value === "object" &&
		value !== null &&
		"then" in value &&
		typeof (value as { then: unknown }).then === "function"
	);
}

export async function runOptimistic<T>(
	task: OptimisticTask<T>,
): Promise<boolean> {
	// 同步快照（zustand / useState）保持同步：乐观值在点击的同一 tick 就落到界面，
	// 不必多余地等一个微任务；React Query 快照需要 await cancelQueries，走异步分支。
	const raw = task.snapshot();
	const snapshot: T = isPromiseLike(raw) ? await raw : raw;
	task.apply();
	let succeeded = true;
	try {
		await task.request();
	} catch (error) {
		succeeded = false;
		task.restore(snapshot);
		try {
			task.onError?.(error);
		} catch {
			// 错误提示自身异常不该盖掉已经完成的回滚
		}
	}
	if (succeeded) {
		try {
			task.onSuccess?.();
		} catch {
			// 成功提示自身异常不该把成功的操作变成未处理的 Promise 拒绝
		}
	}
	if (task.settle) {
		try {
			await task.settle();
		} catch {
			// 对账失败不改变主操作结果；调用方如需提示请在 settle 内自行处理。
		}
	}
	return succeeded;
}

export interface QueryCacheSnapshot {
	/** 每个匹配到的 query key 及其原始值，用于整体回滚。 */
	entries: [QueryKey, unknown][];
}

/**
 * 取消在途查询并快照所有匹配缓存。
 *
 * cancelQueries 很关键：分页/搜索切换时可能有旧请求在飞，若直接
 * setQueriesData，旧响应回来会把乐观值覆盖掉，造成「UI 闪回」。
 */
export async function snapshotQueries(
	queryClient: QueryClient,
	filters: { queryKey: QueryKey }[],
): Promise<QueryCacheSnapshot> {
	await Promise.all(filters.map((filter) => queryClient.cancelQueries(filter)));
	return {
		entries: filters.flatMap((filter) =>
			queryClient
				.getQueriesData(filter)
				.map(([key, value]) => [key, value] as [QueryKey, unknown]),
		),
	};
}

/** 把 snapshotQueries 得到的缓存原样写回。 */
export function restoreQueries(
	queryClient: QueryClient,
	snapshot: QueryCacheSnapshot,
): void {
	for (const [key, value] of snapshot.entries) {
		queryClient.setQueryData(key, value);
	}
}

/** 对一组 query（含分页/多关键词缓存）统一应用变换。 */
export function updateQueryData<T>(
	queryClient: QueryClient,
	filter: { queryKey: QueryKey },
	updater: (prev: T) => T,
): void {
	queryClient.setQueriesData<T>(filter, (prev) =>
		prev === undefined ? prev : updater(prev),
	);
}

/** 让 invalidate 变成可 await 的对账动作，供 settle 使用。 */
export async function invalidateQueries(
	queryClient: QueryClient,
	filters: { queryKey: QueryKey }[],
): Promise<void> {
	await Promise.all(
		filters.map((filter) => queryClient.invalidateQueries(filter)),
	);
}
