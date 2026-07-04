import { createFileRoute, redirect } from "@tanstack/solid-router";
import userStore from "@/stores/userStore";

export const Route = createFileRoute("/(app)/manage/")({
	beforeLoad: () => {
		if (userStore.user()?.role !== "admin") {
			throw redirect({ to: "/" });
		}
		throw redirect({ to: "/manage/users" });
	},
	staticData: {
		title: "管理",
		icon: "icon-manage",
	},
});
