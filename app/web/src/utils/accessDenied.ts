import { toast } from "@gospeak/components";
import { redirect } from "@tanstack/react-router";

/**
 * 权限不足时的统一出口：**先给提示再跳转**。
 *
 * 之前所有 manage 子页都是 `throw redirect({ to: "/discover" })`，
 * 用户点了个链接被弹回首页，完全不知道为什么。toast 是全局的，
 * 导航不会把它清掉，所以能跨路由存活。
 */
export function denyAccess(message = "你没有访问该页面的权限"): never {
	toast.error(message);
	throw redirect({ to: "/discover" });
}
