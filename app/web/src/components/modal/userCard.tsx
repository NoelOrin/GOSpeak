import { useNavigate } from "@tanstack/solid-router";
import Check from "lucide-solid/icons/check";
import LogOut from "lucide-solid/icons/log-out";
import User from "lucide-solid/icons/user";
import { createSignal, Show } from "solid-js";
import { showToast } from "solid-notifications";
import { updateProfile } from "@/api/user";
import Avatar from "@/components/common/avatar";
import userStore from "@/stores/userStore";

interface UserCardProps {
	onClose: () => void;
	onOpenSettings: () => void;
}

const UserCard = (props: UserCardProps) => {
	const navigate = useNavigate();
	const user = () => userStore.user();
	const displayName = () => user()?.display_name || user()?.name || "";

	// 行内编辑昵称
	const [editing, setEditing] = createSignal(false);
	const [editName, setEditName] = createSignal("");
	const [saving, setSaving] = createSignal(false);

	const startEdit = () => {
		setEditName(displayName());
		setEditing(true);
	};

	const saveEdit = async () => {
		const name = editName().trim();
		if (!name || name === displayName()) {
			setEditing(false);
			return;
		}
		setSaving(true);
		try {
			await updateProfile({ display_name: name, avatar: user()?.avatar || "" });
			await userStore.fetchProfile();
			showToast("昵称已更新", { type: "success" });
		} catch (err: any) {
		} finally {
			setSaving(false);
			setEditing(false);
		}
	};

	const handleLogout = async () => {
		props.onClose();
		await userStore.logout();
		navigate({ to: "/login" });
	};

	const menuItems = [
		{
			label: "个人资料",
			icon: User,
			action: () => {
				props.onClose();
				navigate({ to: "/profile" });
			},
		},
		{ label: "退出登录", icon: LogOut, action: handleLogout },
	];

	return (
		<div class="flex flex-col w-56 bg-base-100 rounded-xl shadow-xl border border-base-300 overflow-hidden">
			{/* 头部：头像 + 昵称 */}
			<div class="flex items-center gap-3 p-3">
				<Avatar
					src={user()?.avatar}
					name={displayName()}
					alt={user()?.name}
					class="size-10"
				/>

				<div class="flex-1 min-w-0">
					<Show
						when={!editing()}
						fallback={
							<div class="flex items-center gap-1">
								<input
									type="text"
									class="input input-sm input-bordered w-full min-w-0"
									value={editName()}
									onInput={(e) => setEditName(e.target.value)}
									onKeyDown={(e) => {
										if (e.key === "Enter") saveEdit();
										if (e.key === "Escape") setEditing(false);
									}}
									disabled={saving()}
									autofocus
								/>
								<button
									type="button"
									class="btn btn-xs btn-ghost"
									onClick={saveEdit}
									disabled={saving()}
								>
									<Check size={14} />
								</button>
							</div>
						}
					>
						<button
							type="button"
							class="font-bold text-[14px] truncate w-full text-left hover:underline cursor-text"
							onClick={startEdit}
							title="点击修改昵称"
						>
							{displayName()}
						</button>
					</Show>
					<div class="text-xs text-base-content/50">{user()?.role ?? ""}</div>
				</div>
			</div>

			{/* 分割线 */}
			<div class="divider m-0" />

			{/* 菜单项 */}
			<ul class="menu p-0 w-full">
				{menuItems.map((item) => (
					<li>
						<button
							type="button"
							class="flex items-center gap-2 rounded-none"
							onClick={item.action}
						>
							<item.icon size={16} class="shrink-0" />
							<span>{item.label}</span>
						</button>
					</li>
				))}
			</ul>
		</div>
	);
};

export default UserCard;
