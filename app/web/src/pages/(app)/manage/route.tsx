import { Outlet, createFileRoute, useLocation } from "@tanstack/react-router";
import { useEffect, useRef } from "react";
import { denyAccess } from "@/utils/accessDenied";
import { hasManageAccess } from "@/utils/permissions";

export const Route = createFileRoute("/(app)/manage")({
	beforeLoad: () => {
		if (!hasManageAccess()) {
			denyAccess("你没有进入管理后台的权限");
		}
	},
	component: ManageLayout,
	staticData: {
		title: "管理",
	},
});

function ManageLayout() {
	const location = useLocation();
	const scrollRef = useRef<HTMLDivElement>(null);

	// 子页切换时只滚内部容器，避免整页重挂 + window 滚顶叠加跳闪
	useEffect(() => {
		if (scrollRef.current) {
			scrollRef.current.scrollTop = 0;
			scrollRef.current.scrollLeft = 0;
		}
	}, [location.pathname]);

	return (
		<div className="manage-shell bg-muted flex h-full w-full min-h-0 min-w-0 flex-col">
			{/* 管理导航由外层布局统一渲染（桌面 DynamicRender / 移动端 ManageNavMobile），
			    此处不要再放一份，否则左侧会出现两列重复菜单。 */}
			<div
				ref={scrollRef}
				className="min-h-0 w-full flex-1 overflow-y-auto overflow-x-hidden"
			>
				<Outlet />
			</div>
		</div>
	);
}
