import { Button, Input, toast } from "@gospeak/components";
import { useNavigate } from "@tanstack/react-router";
import { Check, LogOut, Settings, User } from "lucide-react";
import { useState } from "react";
import { updateProfile } from "@/api/user";
import Avatar from "@/components/common/avatar";
import { useUserStore } from "@/stores/userStore";
import { toastActionError } from "@/utils/toast";

interface UserCardProps {
	onClose: () => void;
	onOpenSettings: () => void;
}

export default function UserCard({ onClose, onOpenSettings }: UserCardProps) {
	const navigate = useNavigate();
	const user = useUserStore((s) => s.user);
	const displayName = user?.display_name || user?.name || "";

	// 行内编辑昵称
	const [editing, setEditing] = useState(false);
	const [editName, setEditName] = useState("");
	const [saving, setSaving] = useState(false);

	const startEdit = () => {
		setEditName(displayName);
		setEditing(true);
	};

	const saveEdit = async () => {
		const name = editName.trim();
		if (!name || name === displayName) {
			setEditing(false);
			return;
		}
		setSaving(true);
		try {
			await updateProfile({ display_name: name, avatar: user?.avatar || "" });
			await useUserStore.getState().fetchProfile();
			toast.success("昵称已更新");
		} catch (error) {
			toastActionError(error, "昵称更新失败");
		} finally {
			setSaving(false);
			setEditing(false);
		}
	};

	const handleLogout = async () => {
		onClose();
		await useUserStore.getState().logout();
		navigate({ to: "/login" });
	};

	const menuItems = [
		{
			label: "个人资料",
			icon: User,
			action: () => {
				onClose();
				navigate({ to: "/profile" });
			},
		},
		{
			label: "设置",
			icon: Settings,
			action: () => {
				onClose();
				onOpenSettings();
			},
		},
		{ label: "退出登录", icon: LogOut, action: handleLogout },
	];

	return (
		<div className="border-border bg-card flex w-56 flex-col overflow-hidden rounded-lg border shadow-xl">
			{/* 头部：头像 + 昵称 */}
			<div className="flex items-center gap-3 p-3">
				<Avatar
					src={user?.avatar}
					name={displayName}
					alt={user?.name}
					className="size-10"
				/>
				<div className="min-w-0 flex-1">
					{!editing ? (
						<button
							type="button"
							className="w-full truncate cursor-text text-left text-sm font-bold hover:underline"
							onClick={startEdit}
							title="点击修改昵称"
						>
							{displayName}
						</button>
					) : (
						<div className="flex items-center gap-1">
							<Input
								className="h-8 min-w-0 w-full"
								value={editName}
								autoFocus
								disabled={saving}
								onChange={(e) => setEditName(e.target.value)}
								onKeyDown={(e) => {
									if (e.key === "Enter") void saveEdit();
									if (e.key === "Escape") setEditing(false);
								}}
							/>
							<Button
								variant="ghost"
								className="size-8 shrink-0 p-0"
								onClick={saveEdit}
								disabled={saving}
							>
								<Check size={14} />
							</Button>
						</div>
					)}
					<div className="text-faint-foreground text-xs">
						{user?.role ?? ""}
					</div>
				</div>
			</div>

			{/* 分割线 */}
			<div className="bg-border h-px" />

			{/* 菜单项 */}
			<ul className="w-full p-1">
				{menuItems.map((item) => (
					<li key={item.label}>
						<button
							type="button"
							className="hover:bg-muted flex w-full items-center gap-2 rounded-sm px-3 py-2 text-left text-sm transition-colors"
							onClick={item.action}
						>
							<item.icon size={16} className="shrink-0" />
							<span>{item.label}</span>
						</button>
					</li>
				))}
			</ul>
		</div>
	);
}
