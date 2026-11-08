import { Button } from "@gospeak/components";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import ConfirmModal from "@/components/common/ConfirmModal";
import ProfileForm from "@/components/profile/profileForm";
import { socketStore } from "@/stores/socketStore";
import { useUserStore } from "@/stores/userStore";
import { Page, Section } from "./shared";
import type { SettingTabConfig } from "./types";

function AccountForm() {
	const navigate = useNavigate();
	const [confirmLogout, setConfirmLogout] = useState(false);
	const [loggingOut, setLoggingOut] = useState(false);

	return (
		<Page
			title="账户"
			desc="修改头像、昵称等基础资料；用户名与角色不可在此更改。"
		>
			<Section title="资料">
				<div className="border-border bg-muted/20 rounded-lg border p-4">
					<ProfileForm
						variant="compact"
						initialMode="view"
						showUsername
						showRole={false}
					/>
				</div>
			</Section>

			<Section title="会话">
				<div className="flex flex-wrap gap-2">
					<Button
						type="button"
						variant="outline"
						onClick={() => navigate({ to: "/profile" })}
					>
						打开完整资料页
					</Button>
					<button
						type="button"
						className="border-destructive/50 text-destructive hover:bg-destructive/10 inline-flex h-9 cursor-pointer items-center rounded-sm border px-3 text-sm transition-colors"
						onClick={() => setConfirmLogout(true)}
					>
						退出登录
					</button>
				</div>
			</Section>

			<ConfirmModal
				open={confirmLogout}
				title="退出登录"
				message={
					<span>
						退出后需要重新登录才能使用语音与消息功能；如果你在语音房间里，会先断开连接。
					</span>
				}
				confirmText="退出登录"
				loading={loggingOut}
				onClose={() => setConfirmLogout(false)}
				onConfirm={async () => {
					setLoggingOut(true);
					try {
						socketStore.disconnect();
						await useUserStore.getState().logout();
						navigate({ to: "/login" });
					} finally {
						setLoggingOut(false);
					}
				}}
			/>
		</Page>
	);
}

const account: SettingTabConfig = {
	id: "account",
	label: "账户",
	icon: "user",
	component: AccountForm,
};

export default account;
