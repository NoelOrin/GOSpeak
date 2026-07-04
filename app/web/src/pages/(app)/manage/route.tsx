import { createFileRoute, redirect, Outlet } from "@tanstack/solid-router";
import userStore from "@/stores/userStore";

export const Route = createFileRoute("/(app)/manage")({
	beforeLoad: () => {
		if (userStore.user()?.role !== "admin") {
			throw redirect({ to: "/" });
		}
	},
	component: () => <Outlet />,
	staticData: {
		title: "管理",
		icon: "icon-manage",
	},
});
