import { Outlet, useLocation } from "@tanstack/react-router";
import {
	createContext,
	lazy,
	type ReactNode,
	Suspense,
	useContext,
	useState,
} from "react";
import { SkeletonPage } from "@/components/common/Skeletons";

/**
 * 路由级 keep-alive：切走再切回时页面不重建，表单、搜索词、展开态和滚动位置
 * 都留在原处。
 *
 * 为什么不直接缓存 <Outlet />：TanStack Router 的 Match 组件只接受 routeId，
 * 渲染时内部从「当前」match store 取数据；切走后旧 match 已不在链上，且
 * matchContext 没有对外导出。也就是说公开 API 无法同时渲染多个 location 的
 * 页面树，只能把页面从「路由挂载」改成「常驻挂载」，让路由退化为开关。
 *
 * 隐藏方式用 absolute + visibility 而不是 display:none（或 React 的 Activity，
 * 它同样是 display:none）：后者会让滚动容器高度归零，浏览器随即把 scrollTop
 * 清零，恰好毁掉最该保留的东西。绝对定位让每页独立占满并各自滚动，DOM 与
 * 滚动位置都不受影响；inert 负责挡住隐藏页的点击与焦点。
 *
 * 常驻挂载的代价是「隐藏页的 effect 仍在跑」（chat 保持信令连接、discover 的
 * query 仍会聚焦刷新）。页面通过 {@link useKeepAliveActive} 读到自己当前是否
 * 可见，据此暂停副作用；默认 true，所以在 keep-alive 之外渲染时行为不变。
 *
 * 约束（违反会在开发环境抛 invariant）：
 * - key 必须是精确 pathname，不支持动态段；
 * - 页面内不得使用 useParams / useSearch / useRouteContext，它们依赖
 *   matchContext，常驻渲染时没有。需要读地址就用 useLocation()。
 */

// 懒加载：页面常驻是「访问过之后」的事，没理由在进入 (app) 时就把三个页面的
// chunk 一起拉下来（单 chat 就有 128 kB）。Suspense 只包住各自页面，避免一页
// 加载失败或变慢时把已渲染的页也一起退回占位。
const DiscoverPage = lazy(() =>
	import("@/pages/(app)/discover").then((m) => ({ default: m.DiscoverPage })),
);
const ChatPage = lazy(() =>
	import("@/pages/(app)/chat").then((m) => ({ default: m.ChatPage })),
);
const ProfilePage = lazy(() =>
	import("@/pages/(app)/profile").then((m) => ({ default: m.ProfilePage })),
);

/** 需要保活的页面：key 是精确 pathname，value 是常驻的元素。 */
export const KEEP_ALIVE_PAGES: Record<string, ReactNode> = {
	"/discover": <DiscoverPage />,
	"/chat": <ChatPage />,
	"/profile": <ProfilePage />,
};

/**
 * 当前页面是否处于前台（可见、可交互）。隐藏页据此暂停轮询 / 订阅等副作用。
 *
 * 用法（页面侧尚未接入，见 KeepAliveOutlet 的注释）：
 *   const isActive = useKeepAliveActive();
 *   useEffect(() => { if (!isActive) return; ... }, [isActive]);
 *
 * 默认 true：页面在 keep-alive 之外渲染（测试、独立路由）时等价于一直活跃。
 */
export const KeepAliveActiveContext = createContext(true);

export function useKeepAliveActive(): boolean {
	return useContext(KeepAliveActiveContext);
}

export interface KeepAliveOutletProps {
	/** 可注入的页面表，默认取 KEEP_ALIVE_PAGES；测试与扩展用。 */
	pages?: Record<string, ReactNode>;
}

export function KeepAliveOutlet({
	pages = KEEP_ALIVE_PAGES,
}: KeepAliveOutletProps) {
	const { pathname } = useLocation();
	const activePath = pages[pathname] ? pathname : null;

	// 访问过才挂载，之后不再卸载。页面数量固定，不做 LRU 淘汰。
	const [mounted, setMounted] = useState<string[]>(() =>
		activePath ? [activePath] : [],
	);

	// 渲染期同步补挂：等 effect 再挂会先出现一帧空白。
	if (activePath && !mounted.includes(activePath)) {
		setMounted((prev) =>
			prev.includes(activePath) ? prev : [...prev, activePath],
		);
	}

	// 从没进过保活页时保持原样，不额外套一层容器。
	if (mounted.length === 0) {
		return <Outlet />;
	}

	return (
		<div className="relative h-full min-h-0 w-full">
			{mounted.map((path) => {
				const isActive = path === activePath;
				return (
					<div
						key={path}
						data-keep-alive={path}
						className="absolute inset-0 overflow-x-hidden overflow-y-auto"
						style={{ visibility: isActive ? "visible" : "hidden" }}
						aria-hidden={!isActive}
						inert={!isActive}
					>
						<KeepAliveActiveContext.Provider value={isActive}>
							<Suspense fallback={<SkeletonPage />}>{pages[path]}</Suspense>
						</KeepAliveActiveContext.Provider>
					</div>
				);
			})}
			{!activePath && <Outlet />}
		</div>
	);
}
