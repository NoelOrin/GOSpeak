export function userRoleLabel(role?: string): string {
	switch (role) {
		case "admin":
			return "管理员";
		case "user":
			return "普通用户";
		case "guest":
			return "访客";
		case "owner":
			return "拥有者";
		case "ban":
			return "封禁";
		default:
			return role || "普通用户";
	}
}
