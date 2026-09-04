import {
	createFileRoute,
	Outlet,
	redirect,
	useLocation,
} from "@tanstack/solid-router";
import { createEffect, createMemo } from "solid-js";
import { hasManageAccess } from "@/utils/permissions";

export const Route = createFileRoute("/(app)/manage")({
	beforeLoad: () => {
		if (!hasManageAccess()) {
			throw redirect({ to: "/" });
		}
	},
	component: ManageLayout,
	staticData: {
		title: "管理",
		icon: "icon-manage",
	},
});

function ManageLayout() {
	const location = useLocation();
	const contentKey = createMemo(() => location().pathname);
	let scrollRef: HTMLDivElement | undefined;

	// 子页切换时只滚内部容器，避免整页重挂 + window 滚顶叠加跳闪
	createEffect(() => {
		contentKey();
		if (scrollRef) {
			scrollRef.scrollTop = 0;
			scrollRef.scrollLeft = 0;
		}
	});

	return (
		<div class="manage-shell flex h-full w-full min-h-0 min-w-0 flex-col bg-base-200">
			<div
				ref={(el) => {
					scrollRef = el;
				}}
				class="manage-shell-scroll min-h-0 flex-1 overflow-auto"
			>
				<div
					class="manage-shell-content flex h-full min-h-full flex-col"
					data-path={contentKey()}
				>
					<Outlet />
				</div>
			</div>
		</div>
	);
}
