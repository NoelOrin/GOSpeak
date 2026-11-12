import { Link, useLocation } from "@tanstack/react-router";
import {
	Activity,
	Ban,
	Gavel,
	HardDrive,
	LogIn,
	Mail,
	Megaphone,
	ShieldCheck,
	ServerCog,
	Users,
} from "lucide-react";
import { hasPermission } from "@/utils/permissions";

type ManagePath =
	| "domains"
	| "sfu"
	| "users"
	| "roles"
	| "mute"
	| "ban"
	| "storage"
	| "email"
	| "monitor"
	| "oauth"
	| "announcements";

type ManageTab = {
	path: ManagePath;
	to: `/manage/${ManagePath}`;
	label: string;
	icon: typeof Users;
	/** 进入该页需要的权限码（任一） */
	permissions: string[];
};

export const MANAGE_TABS: ManageTab[] = [
	{
		path: "domains",
		to: "/manage/domains",
		label: "域",
		icon: ServerCog,
		permissions: ["domain:read"],
	},
	{
		path: "users",
		to: "/manage/users",
		label: "用户管理",
		icon: Users,
		permissions: ["user:read"],
	},
	{
		path: "roles",
		to: "/manage/roles",
		label: "角色权限",
		icon: ShieldCheck,
		permissions: ["role:read"],
	},
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
	{
		path: "oauth",
		to: "/manage/oauth",
		label: "OAuth",
		icon: LogIn,
		permissions: ["oauth:read", "oauth:manage"],
	},
	{
		path: "announcements",
		to: "/manage/announcements",
		label: "公告",
		icon: Megaphone,
		permissions: ["announcement:manage", "domain:manage"],
	},
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

export default function ManageNav() {
	const location = useLocation();

	const visibleTabs = MANAGE_TABS.filter((tab) =>
		tab.permissions.some((p) => hasPermission(p)),
	);

	const segments = location.pathname.split("/").filter(Boolean);
	const currentPath = segments[1] || "users";

	return (
		<div className="flex flex-col gap-1 p-2 select-none">
			<div className="hidden px-2 py-2 text-base font-bold md:block">管理</div>
			<div className="flex flex-col gap-1 overflow-x-auto pb-1 md:overflow-visible md:pb-0">
				{visibleTabs.map((tab) => {
					const Icon = tab.icon;
					const active = currentPath === tab.path;
					return (
						<Link
							key={tab.path}
							to={tab.to}
							preload="intent"
							className={`hover:bg-muted flex h-11 min-h-11 shrink-0 items-center justify-start gap-2 rounded-sm px-3 no-underline transition-colors ${active ? "bg-muted" : ""}`}
							aria-current={active ? "page" : undefined}
						>
							<Icon size={16} />
							<span className="text-sm whitespace-nowrap">{tab.label}</span>
						</Link>
					);
				})}
			</div>
		</div>
	);
}
