import { createFileRoute } from "@tanstack/solid-router";
import Dashboard from "@/components/dashboard";

export const Route = createFileRoute("/(app)/index/")({
	component: RouteComponent,
	staticData: {
		title: "首页",
		icon: "home",
	},
});

function RouteComponent() {
	return <Dashboard />;
}

// import { createFileRoute } from "@tanstack/solid-router";

// // 纯占位 避免路由跳到 /index
// export const Route = createFileRoute("/(app)/index/")();
