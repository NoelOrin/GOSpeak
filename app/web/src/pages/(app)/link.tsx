import { createFileRoute, redirect } from "@tanstack/react-router";

// 「分享链接」已整合进首页（发现域）页面，保留 /link 路由仅作旧链接/书签的兼容跳转。
export const Route = createFileRoute("/(app)/link")({
	beforeLoad: () => {
		throw redirect({ to: "/discover" });
	},
	staticData: { title: "邀请码" },
});
