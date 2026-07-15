import { useNavigate } from "@tanstack/solid-router";
import ProfileForm from "@/components/profile/profileForm";
import userStore from "@/stores/userStore";
import { Page, Section } from "./shared";
import type { SettingTabConfig } from "./types";

const AccountForm = () => {
	const navigate = useNavigate();

	return (
		<Page
			title="账户"
			desc="修改头像、昵称等基础资料；用户名与角色不可在此更改。"
		>
			<Section title="资料">
				<div class="rounded-box border border-base-300 bg-base-200/20 p-4">
					<ProfileForm
						variant="compact"
						initialMode="view"
						showUsername
						showRole={false}
					/>
				</div>
			</Section>

			<Section title="会话">
				<div class="flex flex-wrap gap-2">
					<button
						type="button"
						class="btn btn-outline btn-sm"
						onClick={() => navigate({ to: "/profile" })}
					>
						打开完整资料页
					</button>
					<button
						type="button"
						class="btn btn-outline btn-error btn-sm"
						onClick={async () => {
							await userStore.logout();
							navigate({ to: "/login" });
						}}
					>
						退出登录
					</button>
				</div>
			</Section>
		</Page>
	);
};

const account: SettingTabConfig = {
	id: "account",
	label: "账户",
	icon: "user",
	component: AccountForm,
};

export default account;
