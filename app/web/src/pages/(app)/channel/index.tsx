import { createFileRoute } from "@tanstack/solid-router";

export const Route = createFileRoute("/(app)/channel/")({
	component: RouteComponent,
	staticData: {
		title: "频道",
		icon: "icon-channel",
	},
});
// 组件保活在route.tsx
function RouteComponent() {
	return null;
}
