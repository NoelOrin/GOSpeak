import domainStore from "@/stores/domainStore";
import { hasPermission } from "@/utils/permissions";

export function hasDomainPermission(domainUUID: string, code: string): boolean {
	if (hasPermission(code)) return true;
	const perms = domainStore.state.myRolePermissions[domainUUID];
	return !!perms?.includes(code);
}
