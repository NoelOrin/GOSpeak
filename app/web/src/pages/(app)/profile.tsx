import { createFileRoute } from "@tanstack/react-router";
import ProfileForm from "@/components/profile/profileForm";
import UserGroupPanel from "@/components/profile/UserGroupPanel";

export const Route = createFileRoute("/(app)/profile")({
	component: ProfilePage,
	staticData: {
		title: "个人资料",
	},
});

function ProfilePage() {
	return (
		<div className="flex h-full w-full items-start justify-center overflow-auto p-4">
			<div className="w-full max-w-5xl">
				<div className="grid gap-4 lg:grid-cols-[minmax(0,1.2fr)_minmax(0,0.8fr)]">
					<section className="border-border bg-card rounded-lg border p-5 shadow-sm">
						<div>
							<h2 className="text-xl font-semibold">个人资料</h2>
							<p className="text-muted-foreground mt-1 text-sm">
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
