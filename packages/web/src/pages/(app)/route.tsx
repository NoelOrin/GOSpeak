import { createFileRoute, Outlet } from "@tanstack/solid-router";
import Layout from "@/layouts/layout";

export const Route = createFileRoute("/(app)")({
	component: RouteComponent,
});

function RouteComponent() {
	return (
		<Layout>
			<Outlet />
		</Layout>
	)
}
