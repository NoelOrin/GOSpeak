import { Link, useLocation } from "@tanstack/solid-router";
import Activity from "lucide-solid/icons/activity";
import Ban from "lucide-solid/icons/ban";
import Blocks from "lucide-solid/icons/blocks";
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

type ManagePath =
	| "permission"
	| "sfu"
	| "users"
	| "mute"
	| "ban"
	| "storage"
	| "email"
	| "monitor"
	| "apikey"
	| "oauth"
	| "bot-plugins";

type ManageTab = {
	path: ManagePath;
	to:
		| "/manage/permission"
		| "/manage/sfu"
		| "/manage/users"
		| "/manage/mute"
		| "/manage/ban"
		| "/manage/storage"
		| "/manage/email"
		| "/manage/monitor"
		| "/manage/apikey"
		| "/manage/oauth"
		| "/manage/bot-plugins";
	label: string;
	icon: typeof Users;
	/** 进入该页需要的权限码（任一） */
	permissions: string[];
};

const MANAGE_TABS: ManageTab[] = [
	// 用户与权限
	{
		path: "users",
		to: "/manage/users",
		label: "用户管理",
		icon: Users,
		permissions: ["user:read"],
	},
	{
		path: "permission",
		to: "/manage/permission",
		label: "权限",
		icon: ShieldCheck,
		permissions: ["role:manage", "role:read"],
	},
	// 风控
	{
		path: "mute",
		to: "/manage/mute",
		label: "禁言",
		icon: Gavel,
		permissions: ["mute:manage"],
	},
	{
		path: "ban",
		to: "/manage/ban",
		label: "封禁",
		icon: Ban,
		permissions: ["user:update"],
	},
	// 基础设施
	{
		path: "sfu",
		to: "/manage/sfu",
		label: "SFU",
		icon: ServerCog,
		permissions: ["sfu:manage"],
	},
	{
		path: "storage",
		to: "/manage/storage",
		label: "存储",
		icon: HardDrive,
		permissions: ["storage:read", "storage:manage"],
	},
	{
		path: "email",
		to: "/manage/email",
		label: "邮箱",
		icon: Mail,
		permissions: ["email_config:read", "email_config:manage"],
	},
	// 集成
	{
		path: "oauth",
		to: "/manage/oauth",
		label: "OAuth",
		icon: LogIn,
		permissions: ["oauth:read", "oauth:manage"],
	},
	{
		path: "apikey",
		to: "/manage/apikey",
		label: "BOT 密钥",
		icon: KeyRound,
		permissions: ["bot:manage"],
	},
	{
		path: "bot-plugins",
		to: "/manage/bot-plugins",
		label: "BOT 插件",
		icon: Blocks,
		permissions: ["plugin:read", "plugin:manage", "bot:manage"],
	},
	// 监控暂无独立权限码，沿用 role:manage 作为管理面入口
	{
		path: "monitor",
		to: "/manage/monitor",
		label: "服务监控",
		icon: Activity,
		permissions: ["role:manage"],
	},
];

export function firstAccessibleManagePath(): string | null {
	for (const tab of MANAGE_TABS) {
		if (tab.permissions.some((p) => hasPermission(p))) return tab.path;
	}
	return null;
}

const ManageNav = () => {
	const location = useLocation();

	const visibleTabs = createMemo(() =>
		MANAGE_TABS.filter((tab) => tab.permissions.some((p) => hasPermission(p))),
	);

	const currentPath = createMemo(() => {
		const segments = location().pathname.split("/").filter(Boolean);
		// /manage/users -> users
		return segments[1] || "users";
	});

	const isActive = (path: string) => currentPath() === path;

	return (
		<div class="flex flex-col gap-1 p-2 select-none">
			<div class="px-2 py-2 font-bold text-base">管理</div>
			<For each={visibleTabs()}>
				{(tab) => {
					const Icon = tab.icon;
					return (
						<Link
							to={tab.to}
							preload="intent"
							class="btn btn-ghost justify-start gap-2 no-underline"
							classList={{
								"btn-active": isActive(tab.path),
							}}
							aria-current={isActive(tab.path) ? "page" : undefined}
						>
							<Icon size={16} />
							<span>{tab.label}</span>
						</Link>
					);
				}}
			</For>
		</div>
	);
};

export default ManageNav;
export { MANAGE_TABS };
