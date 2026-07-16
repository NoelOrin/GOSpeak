import { createFileRoute, Outlet, redirect } from "@tanstack/solid-router";
import { onMount } from "solid-js";
import Layout from "@/layouts/layout";
import userStore from "@/stores/userStore";

export const Route = createFileRoute("/(app)")({
	beforeLoad: async () => {
		// access 过期时先尝试 refresh_token 无感续期，再决定是否跳登录
		const ok = await userStore.ensureSession();
		if (!ok) {
			throw redirect({ to: "/login" });
		}
	},
	component: RouteComponent,
});

function RouteComponent() {
	onMount(() => {
		userStore.fetchProfile();
	});

	return (
		<Layout>
			<Outlet />
		</Layout>
	);
}
