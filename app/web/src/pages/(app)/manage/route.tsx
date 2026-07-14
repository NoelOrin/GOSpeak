import { createFileRoute, Outlet, redirect } from "@tanstack/solid-router";
import { hasManageAccess } from "@/utils/permissions";

export const Route = createFileRoute("/(app)/manage")({
	beforeLoad: () => {
		if (!hasManageAccess()) {
			throw redirect({ to: "/" });
		}
	},
	component: () => <Outlet />,
	staticData: {
		title: "管理",
		icon: "icon-manage",
	},
});
