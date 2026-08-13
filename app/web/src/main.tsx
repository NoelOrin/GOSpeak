import {
	DEFAULT_SFU_PROVIDER,
	isSFUProviderEnabled,
	type SFUProvider,
} from "@gospeak/sfu-client";
import { QueryClient, QueryClientProvider } from "@tanstack/solid-query";
import {
	createRouter,
	ErrorComponent,
	RouterProvider,
} from "@tanstack/solid-router";
import { render } from "solid-js/web";
import { routeTree } from "./routeTree.gen";
import "./styles.css";
import "cui-solid/dist/styles/cui.css";
import {
	getRememberedSfuProvider,
	preloadSfuClient,
} from "@/components/room/services/loadSfuClient";

export const queryClient = new QueryClient({
	defaultOptions: {
		queries: {
			// 全局默认配置（可选）
			staleTime: 5 * 60 * 1000, // 5分钟内不重新请求
			retry: 1,
		},
	},
});

const router = createRouter({
	routeTree,
	defaultErrorComponent: ({ error }) => <ErrorComponent error={error} />,
	context: {
		queryClient: queryClient,
	},
	defaultPreload: "intent",
	scrollRestoration: true,
	// 管理子页共用滚动上下文，避免切换时浏览器/路由强制滚顶造成跳闪
	getScrollRestorationKey: (location) => {
		if (
			location.pathname === "/manage" ||
			location.pathname.startsWith("/manage/")
		) {
			return "/manage";
		}
		return location.state.key ?? location.href;
	},
	defaultPreloadStaleTime: 0,
});

declare module "@tanstack/solid-router" {
	interface Register {
		router: typeof router;
	}
}

const rootElement = document.getElementById("app");
const initialProvider =
	getRememberedSfuProvider() ||
	(import.meta.env.VITE_SFU_PROVIDER as SFUProvider | undefined) ||
	DEFAULT_SFU_PROVIDER;

if (isSFUProviderEnabled(initialProvider)) {
	void preloadSfuClient(initialProvider);
}

if (rootElement) {
	render(
		() => (
			<QueryClientProvider client={queryClient}>
				<RouterProvider router={router} />
			</QueryClientProvider>
		),
		rootElement,
	);
}
