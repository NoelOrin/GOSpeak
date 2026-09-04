import { createFileRoute } from "@tanstack/solid-router";
import ProfileForm from "@/components/profile/profileForm";
import UserGroupPanel from "@/components/profile/UserGroupPanel";

export const Route = createFileRoute("/(app)/profile")({
	component: ProfilePage,
	staticData: {
		title: "个人资料",
		icon: "profile",
	},
});

function ProfilePage() {
	return (
		<div class="flex h-full w-full items-start justify-center overflow-auto p-4">
			<div class="w-full max-w-5xl">
				<div class="grid gap-4 lg:grid-cols-[minmax(0,1.2fr)_minmax(0,0.8fr)]">
					<section class="rounded-lg border border-base-300 bg-base-100 p-5 shadow-sm">
						<div>
							<h2 class="text-xl font-semibold">个人资料</h2>
							<p class="mt-1 text-sm text-base-content/55">
								管理头像与显示昵称。用户名与角色由系统分配，不可自行修改。
							</p>
						</div>
						<ProfileForm variant="page" initialMode="view" />
					</section>
					<UserGroupPanel />
				</div>
			</div>
		</div>
	);
}
