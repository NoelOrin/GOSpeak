import domainStore from "@/stores/domainStore";
import { hasPermission } from "@/utils/permissions";

export function hasDomainPermission(domainUUID: string, code: string): boolean {
	const perms = domainStore.state.myRolePermissions[domainUUID];
	if (perms?.includes(code)) return true;
	return hasPermission(code);
}
