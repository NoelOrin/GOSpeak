import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createRoot } from "react-dom/client";
import { RouterProvider, createRouter } from "@tanstack/react-router";
import AppToaster from "@/components/common/appToaster";
import { RouteErrorComponent } from "@/components/common/errorBoundary";
import { requestAudioPermission } from "@/hooks/media";
import { initTheme } from "@/stores/themeStore";
import { installGlobalErrorHandlers } from "@/utils/errorReport";
import { loadPublicBaseUrl } from "@/utils/publicBaseUrl";
import { routeTree } from "./routeTree.gen";
import "./styles.css";

export const queryClient = new QueryClient({
	defaultOptions: {
		queries: {
			staleTime: 5 * 60 * 1000, // 5分钟内不重新请求
			retry: 1,
		},
	},
});

const router = createRouter({
	routeTree,
	context: { queryClient },
	defaultPreload: "intent",
	// 所有路由的默认错误 UI；loader 抛错时 invalidate 才会重新发起请求
	defaultErrorComponent: (props) => (
		<RouteErrorComponent {...props} onRetry={() => void router.invalidate()} />
	),
	defaultNotFoundComponent: NotFound,
});

function NotFound() {
	return (
		<div className="bg-background text-foreground flex min-h-[calc(100dvh-var(--mygo-titlebar-height,0px))] flex-col items-center justify-center gap-4">
			<span className="text-primary text-2xl font-semibold">GOSpeak</span>
			<p className="text-muted-foreground text-sm">
				页面不存在，或该页面尚未从 Solid 迁移到 React。
			</p>
			<a href="/" className="text-primary text-sm underline underline-offset-4">
				返回首页
			</a>
		</div>
	);
}

declare module "@tanstack/react-router" {
	interface Register {
		router: typeof router;
	}
}

// 主题必须在应用根部应用一次：登录/注册/访客页没有 Header，
// 只在 Header 里 initTheme 会让这些页面永远停在浅色。
initTheme();

// 启动即申请麦克风权限：授权后 enumerateDevices 才给真实 deviceId / label，
// 设备变化检测与热切换都依赖它。失败不阻塞应用（拒绝授权时降级为系统默认）。
void requestAudioPermission();

// 桌面端分享链接要用后端服务器地址而不是回环 origin，先预热缓存再渲染。
void loadPublicBaseUrl();

// 错误边界捕获不到事件回调 / Promise 里的错误，由全局监听补齐上报
const uninstallGlobalErrorHandlers = installGlobalErrorHandlers();
if (import.meta.hot) {
	import.meta.hot.dispose(uninstallGlobalErrorHandlers);
}

const appEl = document.getElementById("app");
if (appEl) {
	createRoot(appEl).render(
		<QueryClientProvider client={queryClient}>
			<RouterProvider router={router} />
			<AppToaster />
		</QueryClientProvider>,
	);
}
