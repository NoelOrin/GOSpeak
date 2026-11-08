import {
	Outlet,
	createFileRoute,
	redirect,
	useLocation,
} from "@tanstack/react-router";
import { useEffect } from "react";
import Layout from "@/layouts/layout";
import { ErrorBoundary } from "@/components/common/errorBoundary";
import { useUserStore } from "@/stores/userStore";

export const Route = createFileRoute("/(app)")({
	beforeLoad: async () => {
		// access 过期时先尝试 refresh_token 无感续期，再决定是否跳登录
		const ok = await useUserStore.getState().ensureSession();
		if (!ok) {
			if (typeof window !== "undefined") {
				sessionStorage.setItem(
					"gospeak_redirect",
					window.location.pathname + window.location.search,
				);
			}
			throw redirect({ to: "/login" });
		}
	},
	component: RouteComponent,
});

function RouteComponent() {
	const location = useLocation();
	useEffect(() => {
		useUserStore.getState().fetchProfile();
	}, []);

	// 页面级边界：整个应用外壳（布局 + 路由出口）崩溃时给整页兜底，
	// 切换路由自动恢复，避免用户被卡在错误页。
	return (
		<ErrorBoundary
			name="app-shell"
			level="page"
			resetKeys={[location.pathname]}
			title="界面加载失败"
			description="应用界面遇到异常，错误已记录。可以重试或返回首页。"
		>
			<Layout>
				<Outlet />
			</Layout>
		</ErrorBoundary>
	);
}
