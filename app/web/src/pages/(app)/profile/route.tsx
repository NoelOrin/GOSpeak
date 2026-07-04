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
		<div class="flex justify-center items-start w-full h-full overflow-auto p-4">
			<div class="card bg-base-100 w-full ">
				<div class="card-body">
					<h2 class="card-title text-xl mb-2">编辑资料</h2>
					<ProfileForm />
				</div>
			</div>
		</div>
	);
}
