import { createFileRoute } from "@tanstack/solid-router";
import ProfileForm from "@/components/profile/profileForm";

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
			<div class="card w-full bg-base-100">
				<div class="card-body">
					<div>
						<h2 class="card-title text-xl">个人资料</h2>
						<p class="mt-1 text-sm text-base-content/55">
							管理头像与显示昵称。用户名与角色由系统分配，不可自行修改。
						</p>
					</div>
					<ProfileForm variant="page" initialMode="view" />
				</div>
			</div>
		</div>
	);
}
