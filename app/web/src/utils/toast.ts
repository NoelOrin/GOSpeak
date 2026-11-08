import { isAxiosError } from "axios";
import { toast } from "@gospeak/components";
import { apiErrorMessage } from "@/utils/apiError";

/**
 * 写操作失败提示。
 *
 * 接口错误已由 axios 拦截器统一 toast（网络错误 / 4xx / 5xx / 业务码 / 禁言），
 * 拦截器弹过的错误会被打上 `__toastShown` 标记，这里只补拦截器覆盖不到的情况
 * （本地校验抛错、socket/上传等非 axios 错误），避免同一个失败弹两条。
 */
type ToastMarked = { __toastShown?: boolean };

function alreadyToasted(error: unknown): boolean {
	const marked = (error as ToastMarked)?.__toastShown;
	// false 表示调用方用 skipErrorToast 让拦截器闭嘴，仍需这里的文案
	if (marked === false) return false;
	if (marked === true) return true;
	// 兜底：错误被重新包装、丢了标记时，axios 错误也视作已由拦截器提示
	return isAxiosError(error);
}

/** 操作失败：拦截器已提示时静默，否则按后端 msg 提示。返回最终文案供内联展示。 */
export function toastActionError(
	error: unknown,
	fallback = "操作失败",
): string {
	const message = apiErrorMessage(error, fallback);
	if (!alreadyToasted(error)) toast.error(message);
	return message;
}
