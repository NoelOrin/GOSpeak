import { toast } from "@gospeak/components";
import { localizedApiMessage } from "@/utils/errorMessages";

/**
 * 后台 / 被动加载失败的用户可见反馈。
 *
 * 这类加载会在挂载、切域、WS 重连时反复触发，逐个弹 toast 会刷屏；
 * 同一个 scope 在一次页面会话里只提示一次，成功后 clearBackgroundFailure
 * 允许将来再次失败时重新提示。
 *
 * apiClient 拦截器已经弹过的错误带 `__toastShown: true`，这里不重复弹。
 * 启动期探测（麦克风权限、设备变化、localStorage）不应调用本函数，
 * 免得刚进应用就因为浏览器能力限制被弹窗打扰。
 */
const seen = new Set<string>();

function alreadyToasted(error: unknown): boolean {
	return (error as { __toastShown?: boolean } | null)?.__toastShown === true;
}

export function reportBackgroundFailure(
	scope: string,
	error: unknown,
	fallback: string,
): string {
	const message = localizedApiMessage(error, fallback);
	if (!seen.has(scope)) {
		seen.add(scope);
		if (!alreadyToasted(error)) toast.error(message);
	}
	return message;
}

/** 加载成功后重置去重，保证之后的新失败仍能被看见。 */
export function clearBackgroundFailure(scope: string): void {
	seen.delete(scope);
}

/** 仅用于测试隔离。 */
export function resetBackgroundFailures(): void {
	seen.clear();
}
