import { createFileRoute, redirect } from "@tanstack/solid-router";
import { firstAccessibleManagePath } from "@/components/manage/manageNav";
import { hasManageAccess } from "@/utils/permissions";

const MANAGE_PATHS = [
	"permission",
	"sfu",
	"users",
	"mute",
	"ban",
	"storage",
	"email",
	"monitor",
	"apikey",
	"oauth",
] as const;

type ManagePath = (typeof MANAGE_PATHS)[number];

function resolveManagePath(path: string | null): ManagePath {
	if (path && (MANAGE_PATHS as readonly string[]).includes(path)) {
		return path as ManagePath;
	}
	return "users";
}

export const Route = createFileRoute("/(app)/manage/")({
	beforeLoad: () => {
		if (!hasManageAccess()) {
			throw redirect({ to: "/" });
		}
		const first = resolveManagePath(firstAccessibleManagePath());
		throw redirect({ to: `/manage/${first}` });
	},
	staticData: {
		title: "管理",
		icon: "icon-manage",
	},
});
