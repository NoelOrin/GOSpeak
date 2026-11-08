import { useLocation, useMatches } from "@tanstack/react-router";
import { resolveRouteTitle } from "./routeTitle";

/** 当前路由的窗口标题：静态路由标题优先，其次路径段兜底。 */
export function useRouteTitle(): string {
	const matches = useMatches();
	const location = useLocation();
	return resolveRouteTitle(matches, location.pathname);
}
