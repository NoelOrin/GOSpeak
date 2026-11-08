import { useCallback, useState } from "react";
import { normalizeError } from "@/utils/errorReport";

/**
 * React 错误边界只捕获渲染期抛出的错误，事件回调 / Promise / setTimeout 里的
 * 异步错误不会被捕获。这个 hook 把异步错误「重新抛」到渲染阶段，交给最近的边界。
 *
 * ```tsx
 * const throwError = useErrorHandler();
 * const load = async () => {
 *   try { await fetchData(); } catch (e) { throwError(e); }
 * };
 * ```
 */
export function useErrorHandler(): (error: unknown) => void {
	const [error, setError] = useState<Error | null>(null);
	const throwError = useCallback(
		(caught: unknown) => setError(normalizeError(caught)),
		[],
	);

	// 在渲染阶段抛出，位于 useState 之后，保证被上层 ErrorBoundary 捕获
	if (error) throw error;
	return throwError;
}
