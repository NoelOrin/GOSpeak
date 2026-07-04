import { createFileRoute } from "@tanstack/solid-router";
import Dashboard from "@/components/dashboard";

export const Route = createFileRoute("/(app)/")({
	component: RouteComponent,
	staticData: {
		title: "首页",
		icon: "home",
	},
});

function RouteComponent() {
	return <Dashboard />;
}
