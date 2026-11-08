import { QueryClient } from "@tanstack/react-query";
import { describe, expect, it, vi } from "vitest";
import {
	invalidateQueries,
	restoreQueries,
	runOptimistic,
	snapshotQueries,
	updateQueryData,
} from "./optimistic";

function deferred() {
	let resolve!: () => void;
	let reject!: (error: Error) => void;
	const promise = new Promise<void>((res, rej) => {
		resolve = res;
		reject = rej;
	});
	return { promise, resolve, reject };
}

describe("runOptimistic", () => {
	it("applies before the request resolves, then reconciles on success", async () => {
		const steps: string[] = [];
		const gate = deferred();
		const state = { value: "before" };

		const task = runOptimistic({
			snapshot: () => {
				steps.push("snapshot");
				return state.value;
			},
			apply: () => {
				steps.push("apply");
				state.value = "optimistic";
			},
			restore: (value) => {
				state.value = value;
			},
			request: () => {
				steps.push("request");
				return gate.promise;
			},
			settle: () => {
				steps.push("settle");
			},
			onSuccess: () => steps.push("success"),
		});

		// 请求还没回来时，界面已经显示预测结果
		expect(state.value).toBe("optimistic");
		expect(steps).toEqual(["snapshot", "apply", "request"]);

		gate.resolve();
		expect(await task).toBe(true);
		expect(steps).toEqual([
			"snapshot",
			"apply",
			"request",
			"success",
			"settle",
		]);
		expect(state.value).toBe("optimistic");
	});

	it("rolls back to the snapshot when the request fails", async () => {
		const state = { value: "before" };
		const onError = vi.fn();
		const onSuccess = vi.fn();
		const settle = vi.fn();

		const ok = await runOptimistic({
			snapshot: () => state.value,
			apply: () => {
				state.value = "optimistic";
			},
			restore: (value) => {
				state.value = value;
			},
			request: () => Promise.reject(new Error("boom")),
			settle,
			onError,
			onSuccess,
		});

		expect(ok).toBe(false);
		expect(state.value).toBe("before");
		expect(onError).toHaveBeenCalledOnce();
		expect(onSuccess).not.toHaveBeenCalled();
		expect(settle).toHaveBeenCalledOnce();
	});

	it("swallows settle failures so a successful action is not reported as failed", async () => {
		const ok = await runOptimistic({
			snapshot: () => 1,
			apply: () => {},
			restore: () => {},
			request: () => Promise.resolve(),
			settle: () => Promise.reject(new Error("refetch failed")),
		});
		expect(ok).toBe(true);
	});

	it("does not let an onSuccess throw turn success into a failure", async () => {
		const onError = vi.fn();
		const ok = await runOptimistic({
			snapshot: () => 1,
			apply: () => {},
			restore: () => {},
			request: () => Promise.resolve(),
			onSuccess: () => {
				throw new Error("toast exploded");
			},
			onError,
		});
		expect(ok).toBe(true);
		expect(onError).not.toHaveBeenCalled();
	});

	it("waits for async snapshots before applying", async () => {
		const steps: string[] = [];
		await runOptimistic({
			snapshot: async () => {
				await Promise.resolve();
				steps.push("snapshot");
				return 1;
			},
			apply: () => steps.push("apply"),
			restore: () => {},
			request: () => {
				steps.push("request");
				return Promise.resolve();
			},
		});
		expect(steps).toEqual(["snapshot", "apply", "request"]);
	});
});

describe("query cache helpers", () => {
	it("snapshots and restores every matching query", async () => {
		const qc = new QueryClient();
		qc.setQueryData(["items", 1], [1, 2]);
		qc.setQueryData(["items", 2], [3]);

		const snap = await snapshotQueries(qc, [{ queryKey: ["items"] }]);
		expect(snap.entries).toHaveLength(2);

		updateQueryData<number[]>(qc, { queryKey: ["items"] }, (prev) => [
			...prev,
			99,
		]);
		expect(qc.getQueryData(["items", 1])).toEqual([1, 2, 99]);

		restoreQueries(qc, snap);
		expect(qc.getQueryData(["items", 1])).toEqual([1, 2]);
		expect(qc.getQueryData(["items", 2])).toEqual([3]);
	});

	it("invalidates queries and passes the filter through", async () => {
		const qc = new QueryClient();
		const invalidate = vi.spyOn(qc, "invalidateQueries");
		await invalidateQueries(qc, [{ queryKey: ["items"] }]);
		expect(invalidate).toHaveBeenCalledWith({ queryKey: ["items"] });
	});
});
