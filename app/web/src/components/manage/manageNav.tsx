import { useLocation, useNavigate } from "@tanstack/solid-router";
import Activity from "lucide-solid/icons/activity";
import Ban from "lucide-solid/icons/ban";
import Gavel from "lucide-solid/icons/gavel";
import HardDrive from "lucide-solid/icons/hard-drive";
import KeyRound from "lucide-solid/icons/key-round";
import LogIn from "lucide-solid/icons/log-in";
import Mail from "lucide-solid/icons/mail";
import ServerCog from "lucide-solid/icons/server-cog";
import ShieldCheck from "lucide-solid/icons/shield-check";
import Users from "lucide-solid/icons/users";
import { createMemo, For } from "solid-js";
import { hasPermission } from "@/utils/permissions";

type ManageTab = {
	path: string;
	label: string;
	icon: typeof Users;
	/** 进入该页需要的权限码（任一） */
	permissions: string[];
};

const MANAGE_TABS: ManageTab[] = [
	{
		path: "permission",
		label: "权限",
		icon: ShieldCheck,
		permissions: ["role:manage", "role:read"],
	},
	{ path: "sfu", label: "SFU", icon: ServerCog, permissions: ["sfu:manage"] },
	{ path: "users", label: "用户管理", icon: Users, permissions: ["user:read"] },
	{ path: "mute", label: "禁言", icon: Gavel, permissions: ["mute:manage"] },
	{ path: "ban", label: "封禁", icon: Ban, permissions: ["user:update"] },
	{
		path: "storage",
		label: "存储",
		icon: HardDrive,
		permissions: ["storage:read", "storage:manage"],
	},
	{
		path: "email",
		label: "邮箱",
		icon: Mail,
		permissions: ["email_config:read", "email_config:manage"],
	},
	// 监控暂无独立权限码，沿用 role:manage 作为管理面入口
	{
		path: "monitor",
		label: "服务监控",
		icon: Activity,
		permissions: ["role:manage"],
	},
	{
		path: "apikey",
		label: "BOT 密钥",
		icon: KeyRound,
		permissions: ["bot:manage"],
	},
	{
		path: "oauth",
		label: "OAuth",
		icon: LogIn,
		permissions: ["oauth:read", "oauth:manage"],
	},
];

export function firstAccessibleManagePath(): string | null {
	for (const tab of MANAGE_TABS) {
		if (tab.permissions.some((p) => hasPermission(p))) return tab.path;
	}
	return null;
}

const ManageNav = () => {
	const navigate = useNavigate();
	const location = useLocation();

	const visibleTabs = createMemo(() =>
		MANAGE_TABS.filter((tab) => tab.permissions.some((p) => hasPermission(p))),
	);

	const currentPath = createMemo(() => {
		const segments = location().pathname.split("/");
		return segments[2] || "users";
	});

	const isActive = (path: string) => {
		const current = currentPath();
		if (current === path) return true;
		if (!current && path === "users") return true;
		return false;
	};

	return (
		<div class="flex flex-col gap-1 p-2 select-none">
			<div class="px-2 py-2 font-bold text-base">管理</div>
			<For each={visibleTabs()}>
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
