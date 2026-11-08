package signal

import "GOSpeak/internal/pkg"

// permissionGranted 统一权限判定：role -> checker。
// 与 middleware.PermissionGranted 保持同一口径，但 signal 层不依赖 HTTP middleware。
func permissionGranted(_ *pkg.Claims, role, permCode string, checker permChecker) bool {
	return checker != nil && checker.HasPermission(role, permCode)
}
