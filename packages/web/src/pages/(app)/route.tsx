import { createFileRoute, Outlet, redirect } from "@tanstack/solid-router";
import Layout from "@/layouts/layout";
import userStore from "@/stores/userStore";

export const Route = createFileRoute("/(app)")({
	beforeLoad: () => {
		if (!userStore.isLoggedIn()) {
			throw redirect({ to: "/login" });
		}
	},
	component: RouteComponent,
});

function RouteComponent() {
	return (
		<Layout>
			<Outlet />
		</Layout>
	)
}
