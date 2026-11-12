import { Outlet, createRootRouteWithContext } from "@tanstack/react-router";
import type { QueryClient } from "@tanstack/react-query";
import { RouteErrorComponent } from "@/components/common/errorBoundary";
import DesktopInviteBridge from "@/components/common/desktopInviteBridge";
import DesktopTitleBar from "@/components/titleBar/desktopTitleBar";

export interface RouterContext {
	queryClient: QueryClient;
}

function PlainRoot() {
	return (
		<>
			<DesktopInviteBridge />
			<Outlet />
		</>
	);
}

function DesktopRoot() {
	return (
		<div className="flex h-full min-h-0 flex-col">
			<DesktopInviteBridge />
			<DesktopTitleBar />
			<div className="min-h-0 flex-1">
				<Outlet />
			</div>
		</div>
	);
}

export const Route = createRootRouteWithContext<RouterContext>()({
	// 只有桌面构建加标题栏容器：Web 构建的 DOM 与改动前一致。
	component: __GOSPEAK_DESKTOP__ ? DesktopRoot : PlainRoot,
	// 根路由自身的错误兜底（子路由错误由 router 的 defaultErrorComponent 处理）
	errorComponent: RouteErrorComponent,
});
