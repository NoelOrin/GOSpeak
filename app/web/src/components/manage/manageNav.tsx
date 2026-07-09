import { useLocation, useNavigate } from "@tanstack/solid-router";
import { createMemo, For } from "solid-js";
import ShieldCheck from "lucide-solid/icons/shield-check";
import ServerCog from "lucide-solid/icons/server-cog";
import Users from "lucide-solid/icons/users";
import Gavel from "lucide-solid/icons/gavel";
import Ban from "lucide-solid/icons/ban";
import HardDrive from "lucide-solid/icons/hard-drive";
import Mail from "lucide-solid/icons/mail";
import Activity from "lucide-solid/icons/activity";
import KeyRound from "lucide-solid/icons/key-round";

const MANAGE_TABS = [
	{ path: "permission", label: "权限", icon: ShieldCheck },
	{ path: "sfu", label: "SFU", icon: ServerCog },
	{ path: "users", label: "用户管理", icon: Users },
	{ path: "mute", label: "禁言", icon: Gavel },
	{ path: "ban", label: "封禁", icon: Ban },
	{ path: "storage", label: "存储", icon: HardDrive },
	{ path: "email", label: "邮箱", icon: Mail },
	{ path: "monitor", label: "服务监控", icon: Activity },
	{ path: "apikey", label: "Bot 密钥", icon: KeyRound },
] as const;

const ManageNav = () => {
	const navigate = useNavigate();
	const location = useLocation();

	const currentPath = createMemo(() => {
		const segments = location().pathname.split("/");
		return segments[2] || "users";
	});

	const isActive = (path: string) => {
		const current = currentPath();
		if (current === path) return true;
		// /manage 根路径默认激活 users
		if (!current && path === "users") return true;
		return false;
	};

	return (
		<div class="flex flex-col gap-1 p-2 select-none">
			<div class="px-2 py-2 font-bold text-base">管理</div>
			<For each={MANAGE_TABS}>
				{(tab) => {
					const Icon = tab.icon;
					return (
						<button
							type="button"
							class="btn btn-ghost justify-start gap-2"
							classList={{
								"btn-active": isActive(tab.path),
							}}
							onClick={() => navigate({ to: `/manage/${tab.path}` })}
						>
							<Icon size={16} />
							<span>{tab.label}</span>
						</button>
					);
				}}
			</For>
		</div>
	);
};

export default ManageNav;
export { MANAGE_TABS };
