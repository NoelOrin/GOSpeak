import { createFileRoute, redirect } from "@tanstack/solid-router";

export const Route = createFileRoute("/(app)/index/")({
	beforeLoad: () => {
		throw redirect({ to: "/discover" });
	},
	staticData: {
		title: "发现域",
		icon: "compass",
	},
});
