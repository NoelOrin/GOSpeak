package signal

import "GOSpeak/internal/pkg"

// permissionGranted 统一权限判定：token 显式权限优先，否则回退 role -> checker。
// 与 middleware.PermissionGranted 保持同一口径，但 signal 层不依赖 HTTP middleware。
func permissionGranted(claims *pkg.Claims, role, permCode string, checker permChecker) bool {
	if claims != nil && len(claims.Permissions) > 0 {
		for _, p := range claims.Permissions {
			if p == permCode {
				return true
			}
		}
		return false
	}
	return checker != nil && checker.HasPermission(role, permCode)
}
