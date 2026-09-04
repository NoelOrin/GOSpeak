import { useLocation, useRouter } from "@tanstack/solid-router";
import { createEffect } from "solid-js";

const APP_NAME = "GOSpeak";

/**
 * 根据当前路由 staticData.title 动态设置浏览器标签标题。
 * 在顶层布局中调用一次即可覆盖所有页面。
 */
export function buildDocumentTitle(routeTitle?: string) {
	const title = routeTitle?.trim();
	return title ? `${title} | ${APP_NAME}` : APP_NAME;
}

export function useTitle() {
	const router = useRouter();
	const location = useLocation();

	createEffect(() => {
		const currentPath = location().pathname;
		const matchedRoutes = router.matchRoutes(currentPath);
		const lastMatch = matchedRoutes.at(-1);
		const routeTitle = (lastMatch?.staticData as any)?.title;
		document.title = buildDocumentTitle(routeTitle);
	});
}
